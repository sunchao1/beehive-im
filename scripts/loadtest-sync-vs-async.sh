#!/bin/bash
# 同步 ACK vs 异步 ACK 对比压测（改造前后 JSON）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"
mkdir -p "${ROOT}/reports/loadtest"

export LOADTEST_SKIP_GATE=1

CONNS="${LOADTEST_CMP_CONNS:-501}"
RATE="${LOADTEST_CMP_RATE:-5}"
DUR="${LOADTEST_CMP_DURATION:-60s}"

wait_iplist() {
    local tries="${1:-30}"
    local i=0
    while [ "${i}" -lt "${tries}" ]; do
        local reg sid
        reg="$(curl -sf 'http://127.0.0.1:8000/im/register?uid=299999&nation=1&city=1&town=1' 2>/dev/null || true)"
        sid="$(printf '%s' "${reg}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('sid',0))" 2>/dev/null || echo 0)"
        if [ "${sid}" != "0" ] && curl -sf "http://127.0.0.1:8000/im/iplist?type=2&uid=299999&sid=${sid}&clientip=127.0.0.1" | grep -q '"code":0'; then
            echo "[compare] iplist OK"
            return 0
        fi
        i=$((i + 1))
        sleep 2
    done
    echo "[compare] ERROR: iplist 不可用（frwder/listend 可能挂了，请 ./scripts/up-demo.sh 重启 runner）"
    return 1
}

restart_chatroom() {
    local async_env="$1"
    if ! docker ps --format '{{.Names}}' 2>/dev/null | grep -qx beehive-runner; then
        export BEEHIVE_CHATROOM_ASYNC_BROADCAST="${async_env}"
        echo "[warn] runner 未检测到，请手动重启 chatroom 并设置 BEEHIVE_CHATROOM_ASYNC_BROADCAST=${async_env}"
        return 0
    fi
    docker exec beehive-runner pkill -x chatroom.v.1.1 2>/dev/null || true
    sleep 2
    docker exec -d -e BEEHIVE_CHATROOM_ASYNC_BROADCAST="${async_env}" beehive-runner \
        bash -c 'cd /workspace/bin && exec ./chatroom.v.1.1 -c /workspace/.run-conf/chatroom.xml'
    sleep 5
    wait_iplist
}

run_one() {
    local tag="$1"
    local async_env="$2"
    echo ""
    echo "========== ${tag} (BEEHIVE_CHATROOM_ASYNC_BROADCAST=${async_env}) =========="
    restart_chatroom "${async_env}"
    LOADTEST_CONNS="${CONNS}" LOADTEST_RATE="${RATE}" LOADTEST_DURATION="${DUR}" \
        LOADTEST_RAMP=30s LOADTEST_MODE=chat \
        LOADTEST_SCENARIO="compare-${tag}" \
        LOADTEST_REPORT="${ROOT}/reports/loadtest/compare-${tag}.json" \
        LOADTEST_EXTRA="-senders 1 -rooms 1" \
        ./scripts/loadtest.sh
}

run_one "sync-ack" "0"
run_one "async-ack" "1"

echo ""
echo "[compare] reports:"
ls -la "${ROOT}/reports/loadtest/compare-"*.json 2>/dev/null || true
echo "对比 chat_qps / latency p99 / est_downstream_qps"
