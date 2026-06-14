#!/bin/bash
# 静态托管 demo/web，并代理 /im/* 到 usrsvr（避免 8088→8000 CORS）
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export DEMO_WEB_PORT="${DEMO_WEB_PORT:-8088}"
export BEEHIVE_USRSVR_URL="${BEEHIVE_USRSVR_URL:-http://127.0.0.1:8000}"

if command -v lsof >/dev/null 2>&1; then
  old_pid="$(lsof -ti :"${DEMO_WEB_PORT}" 2>/dev/null || true)"
  if [ -n "${old_pid}" ]; then
    echo "[serve-demo] port ${DEMO_WEB_PORT} is used by pid(s): ${old_pid}"
    echo "[serve-demo] stopping old demo server..."
    kill ${old_pid} 2>/dev/null || true
    sleep 0.5
  fi
fi

exec python3 "${ROOT}/scripts/demo-proxy.py"
