# ── Stage 1: Build frontend ─────────────────────────────────────────────────
# Base images pinned by digest (Scorecard Pinned-Dependencies); Dependabot's
# docker updates bump both the tag and the digest.
FROM node:26-alpine@sha256:9c0e1e52125d6b67d505cf75b4880fcf1290ccea5c480849910e1d57b2cf72b5 AS frontend-builder
WORKDIR /frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ── Stage 2: Build backend ───────────────────────────────────────────────────
FROM golang:1.26-alpine@sha256:f1ddd9fe14fffc091dd98cb4bfa999f32c5fc77d2f2305ea9f0e2595c5437c14 AS backend-builder
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /sempa ./cmd/server

# ── Stage 3: Final image ─────────────────────────────────────────────────────
# Pinned base (not :latest) so rebuilds are reproducible; Dependabot's docker
# updates bump it. wget (busybox, already in alpine) backs the health check.
FROM alpine:3.21@sha256:48b0309ca019d89d40f670aa1bc06e426dc0931948452e8491e3d65087abc07d
RUN apk --no-cache add ca-certificates tzdata \
    && addgroup -S sempa && adduser -S -G sempa -u 10001 sempa
WORKDIR /app
COPY --from=backend-builder /sempa ./sempa
COPY --from=frontend-builder /frontend/build ./frontend/build
# Own the app + data dir as the non-root user. Declaring VOLUME after this chown
# means a FRESH named volume is initialised with sempa's ownership, so new
# installs are writable out of the box. (Existing root-owned volumes need a
# one-time `chown -R 10001:10001` — see the upgrade note in SECURITY.md / README.)
RUN mkdir -p /data && chown -R sempa:sempa /app /data
USER sempa
EXPOSE 9001
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:9001/api/v1/health || exit 1
CMD ["./sempa"]
