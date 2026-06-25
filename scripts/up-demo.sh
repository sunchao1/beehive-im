#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

if ! command -v docker >/dev/null 2>&1; then
    echo "docker 未安装"
    exit 1
fi

wait_healthy() {
    local name="$1"
    local max="${2:-120}"
    local i=0
    while [ "${i}" -lt "${max}" ]; do
        status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${name}" 2>/dev/null || echo missing)"
        if [ "${status}" = "healthy" ] || [ "${status}" = "running" ]; then
            if [ "${name}" != "beehive-redis" ] && [ "${name}" != "beehive-mysql" ] && [ "${name}" != "beehive-mongo" ]; then
                return 0
            fi
            if [ "${status}" = "healthy" ]; then
                return 0
            fi
        fi
        i=$((i + 1))
        sleep 2
    done
    echo "[up-demo] timeout waiting for ${name} (last status: ${status})"
    return 1
}

wait_port() {
    local port="$1"
    local label="$2"
    local max="${3:-120}"
    local i=0
    while [ "${i}" -lt "${max}" ]; do
        if (echo >/dev/tcp/127.0.0.1/"${port}") 2>/dev/null; then
            echo "[up-demo] ${label} listening on :${port}"
            return 0
        fi
        i=$((i + 1))
        sleep 2
    done
    echo "[up-demo] timeout waiting for ${label} on :${port}"
    return 1
}

echo "[up-demo] starting middleware..."
docker compose up -d redis mysql mongo
wait_healthy beehive-redis 120
wait_healthy beehive-mysql 120
wait_healthy beehive-mongo 120

echo "[up-demo] starting app (runner + demo-web)..."
docker compose --profile run --profile demo up -d runner demo-web

wait_port 8000 "usrsvr"
wait_port 8002 "websocket"
wait_port 8088 "demo-web"

cat <<EOF

beehive-im demo stack is up.

  Register:  curl 'http://127.0.0.1:8000/im/register?uid=100001&nation=1&city=1&town=1'
  Iplist:    curl 'http://127.0.0.1:8000/im/iplist?type=2&uid=100001&sid=<sid>&clientip=127.0.0.1'
  WebSocket: ws://127.0.0.1:8002/im
  Demo web:  http://127.0.0.1:8088/group.html
  Smoke:     ./scripts/smoke-test.sh
  Status:    ./scripts/status.sh
  Stop:      docker compose --profile run --profile demo down

Test users: uid=100001 / 100002   room rid=10001

EOF
