#!/bin/bash
# 在 compose 网络内启动 9 进程
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

RUN_CONF="${ROOT}/.run-conf"
"${ROOT}/docker/gen-conf.sh" "${RUN_CONF}"

rm -fr "${ROOT}/log/"*
mkdir -p "${ROOT}/log"

wait_tcp() {
    local host="$1"
    local port="$2"
    local label="$3"
    local max="${4:-60}"
    local i=0
    while [ "${i}" -lt "${max}" ]; do
        if (echo >/dev/tcp/"${host}"/"${port}") 2>/dev/null; then
            echo "[run] ${label} ready (${host}:${port})"
            return 0
        fi
        i=$((i + 1))
        sleep 1
    done
    echo "[run] ERROR: ${label} not ready on ${host}:${port} after ${max}s"
    return 1
}

echo "[run] waiting for middleware..."
wait_tcp "${BEEHIVE_REDIS_HOST:-redis}" 6379 "redis"
wait_tcp "${BEEHIVE_MYSQL_HOST:-mysql}" 3306 "mysql"
wait_tcp "${BEEHIVE_MONGO_HOST:-mongo}" 27017 "mongo"
sleep 3

cd "${ROOT}/bin"

echo "[run] starting frwder..."
./frwder.v.1.1 -c "${RUN_CONF}/frwder.xml" -d -l debug
wait_tcp "${BEEHIVE_FRWDER_HOST:-frwder}" 28889 "frwder-backend"

start_bg() {
    local name="$1"
    local conf="$2"
    echo "[run] starting ${name}..."
    if [ -n "${conf}" ]; then
        ./"${name}".v.1.1 -c "${conf}" &
    else
        ./"${name}".v.1.1 &
    fi
}

start_bg seqsvr "${RUN_CONF}/seqsvr.xml"
wait_tcp "${BEEHIVE_SEQSVR_HOST:-127.0.0.1}" 50000 "seqsvr"
sleep 2

start_bg msgsvr "${RUN_CONF}/msgsvr.xml"
sleep 3
start_bg tasker "${RUN_CONF}/tasker.xml"
sleep 3
start_bg usrsvr "${RUN_CONF}/usrsvr.xml"
sleep 5
start_bg monitor "${RUN_CONF}/monitor.xml"
sleep 2
start_bg chatroom "${RUN_CONF}/chatroom.xml"
sleep 3
start_bg websocket "${RUN_CONF}/websocket.xml"

wait_tcp "127.0.0.1" 8000 "usrsvr" 90
wait_tcp "127.0.0.1" 8002 "websocket" 30
wait_tcp "127.0.0.1" 8004 "chatroom" 30

echo "[run] starting listend (foreground, keep container alive)..."
exec ./listend.v.1.1 -c "${RUN_CONF}/listend.xml" -l debug
