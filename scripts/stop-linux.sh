#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

echo "[stop-linux] stopping runner..."
docker compose --profile run stop runner 2>/dev/null || true
docker rm -f beehive-runner 2>/dev/null || true

echo "[stop-linux] done (middleware still running; use 'docker compose down' to stop all)"
