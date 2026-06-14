#!/bin/bash
# 在 compose 网络内启动 9 进程；DB 地址替换为 redis/mysql/mongo 服务名
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

REDIS_HOST="${BEEHIVE_REDIS_HOST:-redis}"
MYSQL_HOST="${BEEHIVE_MYSQL_HOST:-mysql}"
MONGO_HOST="${BEEHIVE_MONGO_HOST:-mongo}"

RUN_CONF="${ROOT}/.run-conf"
rm -rf "${RUN_CONF}"
mkdir -p "${RUN_CONF}"

for f in conf/*.xml; do
    base="$(basename "${f}")"
    sed -e "s|ADDR=\"127.0.0.1:6379\"|ADDR=\"${REDIS_HOST}:6379\"|g" \
        -e "s|ADDR=\"127.0.0.1:3306\"|ADDR=\"${MYSQL_HOST}:3306\"|g" \
        -e "s|ADDR=\"127.0.0.1:27017\"|ADDR=\"${MONGO_HOST}:27017\"|g" \
        -e "s|ADDR=\"127.0.0.1:7379\"|ADDR=\"${MYSQL_HOST}:3306\"|g" \
        -e "s|ADDR=\"127.0.0.1:8379\"|ADDR=\"${MONGO_HOST}:27017\"|g" \
        "${f}" > "${RUN_CONF}/${base}"
done

rm -fr "${ROOT}/log/"*
mkdir -p "${ROOT}/log"

cd "${ROOT}/bin"

echo "[run] starting frwder..."
./frwder.v.1.1 -c "${RUN_CONF}/frwder.xml" -d -l debug
sleep 4

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
sleep 1
start_bg msgsvr "${RUN_CONF}/msgsvr.xml"
start_bg tasker "${RUN_CONF}/tasker.xml"
start_bg usrsvr "${RUN_CONF}/usrsvr.xml"
start_bg monitor "${RUN_CONF}/monitor.xml"
start_bg chatroom "${RUN_CONF}/chatroom.xml"
start_bg websocket "${RUN_CONF}/websocket.xml"
sleep 2

echo "[run] starting listend (foreground, keep container alive)..."
exec ./listend.v.1.1 -c "${RUN_CONF}/listend.xml" -l debug
