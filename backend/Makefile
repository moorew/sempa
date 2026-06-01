.PHONY: build run dev test generate clean deps

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o ./bin/aura-server ./cmd/server

run: build
	./bin/aura-server

dev:
	go run ./cmd/server

test:
	go test ./...

# Requires: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
generate:
	sqlc generate

clean:
	rm -rf bin/

deps:
	go mod tidy
