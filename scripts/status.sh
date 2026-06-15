#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

ok=0
fail=0

check() {
    local label="$1"
    shift
    if "$@"; then
        echo "  OK   ${label}"
        ok=$((ok + 1))
    else
        echo "  FAIL ${label}"
        fail=$((fail + 1))
    fi
}

port_open() {
    local port="$1"
    (echo >/dev/tcp/127.0.0.1/"${port}") 2>/dev/null
}

container_healthy() {
    local name="$1"
    local status
    status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${name}" 2>/dev/null || echo missing)"
    [ "${status}" = "healthy" ] || [ "${status}" = "running" ]
}

middleware_healthy() {
    local name="$1"
    local status
    status="$(docker inspect -f '{{.State.Health.Status}}' "${name}" 2>/dev/null || echo missing)"
    [ "${status}" = "healthy" ]
}

echo "=== middleware ==="
check "redis" middleware_healthy beehive-redis
check "mysql" middleware_healthy beehive-mysql
check "mongo" middleware_healthy beehive-mongo

echo "=== app ports (host) ==="
check "usrsvr :8000" port_open 8000
check "websocket :8002" port_open 8002
check "websocket-2 :8003" port_open 8003
check "chatroom :8004" port_open 8004
check "listend :9002" port_open 9002
check "listend-2 :9003" port_open 9003

echo "=== runner container ==="
if docker ps --format '{{.Names}}' | grep -qx beehive-runner; then
    echo "  OK   runner running"
    ok=$((ok + 1))
    procs="$(docker exec beehive-runner sh -c 'ps aux 2>/dev/null | grep -E "\.(v\.1\.1)" | grep -v grep | wc -l' 2>/dev/null || echo 0)"
    if [ "${procs}" -ge 9 ] 2>/dev/null; then
        echo "  OK   9 processes (${procs} matched)"
        ok=$((ok + 1))
    else
        echo "  FAIL 9 processes (found ${procs})"
        fail=$((fail + 1))
    fi
else
    echo "  FAIL runner not running"
    fail=$((fail + 1))
fi

echo "=== summary ==="
echo "pass=${ok} fail=${fail}"
[ "${fail}" -eq 0 ]
