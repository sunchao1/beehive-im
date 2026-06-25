#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

echo "[stop-linux] stopping runner and demo-web..."
docker compose --profile run --profile demo stop runner demo-web 2>/dev/null || true
docker rm -f beehive-runner beehive-demo-web 2>/dev/null || true

echo "[stop-linux] done (middleware still running; use 'docker compose down' to stop all)"
