#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

if ! command -v docker >/dev/null 2>&1; then
    echo "docker 未安装"
    exit 1
fi

echo "[start-linux] ensure middleware is up..."
docker compose up -d redis mysql mongo

echo "[start-linux] starting services in Linux container (Ctrl+C to stop)..."
docker compose --profile run run --rm runner
