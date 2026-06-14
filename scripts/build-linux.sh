#!/bin/bash
# 在 Linux 容器内编译 C + Go（macOS 上请用此脚本，勿直接 make all）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

if ! command -v docker >/dev/null 2>&1; then
    echo "docker 未安装"
    exit 1
fi

TARGET="${1:-all}"
echo "[build-linux] target=${TARGET}"
docker compose --profile build run --rm builder "${TARGET}"
