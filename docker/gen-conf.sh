#!/bin/bash
# 从 conf/templates/ 渲染运行时配置到 .run-conf/
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${1:-${ROOT}/.run-conf}"

REDIS_HOST="${BEEHIVE_REDIS_HOST:-redis}"
MYSQL_HOST="${BEEHIVE_MYSQL_HOST:-mysql}"
MONGO_HOST="${BEEHIVE_MONGO_HOST:-mongo}"
FRWDER_HOST="${BEEHIVE_FRWDER_HOST:-frwder}"
SEQSVR_HOST="${BEEHIVE_SEQSVR_HOST:-seqsvr}"
ACCESS_IP="${BEEHIVE_ACCESS_IP:-127.0.0.1}"
WS_IP="${BEEHIVE_WS_IP:-127.0.0.1}"

REDIS_ADDR="${REDIS_HOST}:6379"
MYSQL_ADDR="${MYSQL_HOST}:3306"
MONGO_ADDR="${MONGO_HOST}:27017"
FRWDER_FORWARD="${FRWDER_HOST}:28888"
FRWDER_BACKEND="${FRWDER_HOST}:28889"
SEQSVR_ADDR="${SEQSVR_HOST}:50000"

mkdir -p "${OUT}"

for f in "${ROOT}/conf/templates/"*.xml; do
    base="$(basename "${f}")"
    sed \
        -e "s|{{REDIS_ADDR}}|${REDIS_ADDR}|g" \
        -e "s|{{MYSQL_ADDR}}|${MYSQL_ADDR}|g" \
        -e "s|{{MONGO_ADDR}}|${MONGO_ADDR}|g" \
        -e "s|{{FRWDER_FORWARD}}|${FRWDER_FORWARD}|g" \
        -e "s|{{FRWDER_BACKEND}}|${FRWDER_BACKEND}|g" \
        -e "s|{{SEQSVR_ADDR}}|${SEQSVR_ADDR}|g" \
        -e "s|{{ACCESS_IP}}|${ACCESS_IP}|g" \
        -e "s|{{WS_IP}}|${WS_IP}|g" \
        "${f}" > "${OUT}/${base}"
done

echo "[gen-conf] rendered ${OUT} (redis=${REDIS_ADDR} mysql=${MYSQL_ADDR} mongo=${MONGO_ADDR} ws_ip=${WS_IP})"
