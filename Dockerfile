# ── Stage 1: Build frontend ─────────────────────────────────────────────────
# Base images pinned by digest (Scorecard Pinned-Dependencies); Dependabot's
# docker updates bump both the tag and the digest.
#
# --platform=$BUILDPLATFORM pins the builder to the runner's NATIVE arch so the
# frontend never builds under QEMU emulation. Its output (static JS/HTML) is
# arch-independent, so the single build is copied into every target image. This
# is the difference between a ~2-min and a >30-min (timed-out) multi-arch build.
FROM --platform=$BUILDPLATFORM node:26-alpine@sha256:9c0e1e52125d6b67d505cf75b4880fcf1290ccea5c480849910e1d57b2cf72b5 AS frontend-builder
WORKDIR /frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ── Stage 2: Build backend ───────────────────────────────────────────────────
# Also native (--platform=$BUILDPLATFORM): the Go toolchain runs un-emulated and
# CROSS-compiles to the requested arch via GOARCH=$TARGETARCH. CGO is off and the
# SQLite driver is pure Go (modernc.org/sqlite), so cross-compilation is clean.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS backend-builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /sempa ./cmd/server

# ── Stage 3: Final image ─────────────────────────────────────────────────────
# Pinned base (not :latest) so rebuilds are reproducible; Dependabot's docker
# updates bump it. wget (busybox, already in alpine) backs the health check.
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
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
