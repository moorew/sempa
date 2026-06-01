#!/bin/bash
# Run after pulling new code to rebuild and restart.
set -e
cd /home/clevercode/aura

echo "→ Rebuilding backend..."
cd backend && go build -o bin/aura ./cmd/server/ && cd ..

echo "→ Rebuilding frontend..."
cd frontend && npm run build && cd ..

echo "→ Restarting service..."
sudo systemctl restart aura

echo "✓ Done"
systemctl status aura --no-pager
