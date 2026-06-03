#!/usr/bin/env bash
# Update Sempa to the latest code.
# Run from the project root: bash deploy/update.sh
set -euo pipefail

cd "$(dirname "$0")/.."

echo "→ Pulling latest code..."
git pull

echo "→ Rebuilding image..."
docker compose build

echo "→ Restarting container..."
docker compose up -d

echo "→ Waiting for healthy..."
for _ in $(seq 1 20); do
  if curl -sf "http://localhost:${HOST_PORT:-9001}/api/v1/health" &>/dev/null; then
    echo "✓ Sempa updated and running."
    exit 0
  fi
  sleep 1
done

echo "⚠ Container started but health check timed out. Check: docker compose logs"
