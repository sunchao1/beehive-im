#!/bin/bash
# 静态托管 demo/web（避免 file:// CORS）
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PORT="${DEMO_WEB_PORT:-8088}"
echo "Serving demo/web at http://127.0.0.1:${PORT}/"
cd "${ROOT}/demo/web"
if command -v python3 >/dev/null 2>&1; then
  exec python3 -m http.server "${PORT}"
fi
exec python -m SimpleHTTPServer "${PORT}"
