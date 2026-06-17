#!/usr/bin/env bash
# Update Sempa to the latest code.
# Run from the project root: bash deploy/update.sh
set -euo pipefail

cd "$(dirname "$0")/.."

echo "→ Pulling latest code..."
git pull

echo "→ Rebuilding image..."
docker compose build

# The app runs as non-root (uid 10001). Make sure the /data volume is owned by it
# before restart — a no-op once migrated, and the one-time fix when upgrading from
# an older root-owned image. Best-effort; never blocks the update.
echo "→ Ensuring /data ownership (non-root app user)..."
docker compose run --rm --no-deps --user root --entrypoint sh sempa \
  -c 'chown -R 10001:10001 /data' >/dev/null 2>&1 || true

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
