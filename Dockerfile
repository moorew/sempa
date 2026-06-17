# ── Stage 1: Build frontend ─────────────────────────────────────────────────
FROM node:26-alpine AS frontend-builder
WORKDIR /frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ── Stage 2: Build backend ───────────────────────────────────────────────────
FROM golang:1.26.4-alpine AS backend-builder
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /sempa ./cmd/server

# ── Stage 3: Final image ─────────────────────────────────────────────────────
# Pinned base (not :latest) so rebuilds are reproducible; Dependabot's docker
# updates bump it. wget (busybox, already in alpine) backs the health check.
FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=backend-builder /sempa ./sempa
COPY --from=frontend-builder /frontend/build ./frontend/build
RUN mkdir -p /data
EXPOSE 9001
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:9001/api/v1/health || exit 1
CMD ["./sempa"]

# NOTE: the container still runs as root. Switching to a non-root USER is
# deliberately deferred: the /data SQLite volume on existing self-hosted installs
# is root-owned, so changing the runtime user would break writes on upgrade
# without a coordinated one-time volume chown. Tracked in SECURITY.md.
