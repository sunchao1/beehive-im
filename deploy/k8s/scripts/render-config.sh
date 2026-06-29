#!/bin/bash
# 渲染 K8s 用 XML 配置到 deploy/k8s/.rendered/
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
OUT="${ROOT}/deploy/k8s/.rendered"

export BEEHIVE_REDIS_HOST="${BEEHIVE_REDIS_HOST:-redis.beehive.svc.cluster.local}"
export BEEHIVE_MYSQL_HOST="${BEEHIVE_MYSQL_HOST:-mysql.beehive.svc.cluster.local}"
export BEEHIVE_MONGO_HOST="${BEEHIVE_MONGO_HOST:-mongo.beehive.svc.cluster.local}"
export BEEHIVE_FRWDER_HOST="${BEEHIVE_FRWDER_HOST:-frwder.beehive.svc.cluster.local}"
export BEEHIVE_SEQSVR_HOST="${BEEHIVE_SEQSVR_HOST:-seqsvr.beehive.svc.cluster.local}"
export BEEHIVE_ACCESS_IP="${BEEHIVE_ACCESS_IP:-127.0.0.1}"
export BEEHIVE_WS_IP="${BEEHIVE_WS_IP:-websocket.beehive.svc.cluster.local}"

"${ROOT}/docker/gen-conf.sh" "${OUT}"

echo "[k8s render] output: ${OUT}"
echo "[k8s render] frwder=${BEEHIVE_FRWDER_HOST} redis=${BEEHIVE_REDIS_HOST}"
