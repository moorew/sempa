FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /aura-server ./cmd/server

# ---

FROM scratch

COPY --from=builder /aura-server /aura-server
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8080
VOLUME ["/data"]

CMD ["/aura-server"]
