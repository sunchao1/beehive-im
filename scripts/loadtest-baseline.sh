#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"
mkdir -p "${ROOT}/reports/loadtest"

run_scene() {
    local name="$1"
    shift
    echo ""
    echo "========== baseline: ${name} =========="
    LOADTEST_SCENARIO="${name}" LOADTEST_EXTRA="$*" ./scripts/loadtest.sh
}

# 场景 1：同房弹幕（默认略低于 spec 500，避免弱机 OOM；可 LOADTEST_BASELINE_HEAVY=1 拉满）
CONNS_S1="${LOADTEST_S1_CONNS:-200}"
RATE_S1="${LOADTEST_S1_RATE:-1}"
DUR_S1="${LOADTEST_S1_DURATION:-60s}"
if [ "${LOADTEST_BASELINE_HEAVY:-0}" = "1" ]; then
    CONNS_S1=500
fi
LOADTEST_CONNS="${CONNS_S1}" LOADTEST_RATE="${RATE_S1}" LOADTEST_DURATION="${DUR_S1}" \
    LOADTEST_MODE=chat LOADTEST_REPORT="${ROOT}/reports/loadtest/baseline-s1-chat.json" \
    LOADTEST_CSV="${ROOT}/reports/loadtest/baseline-s1-chat.csv" \
    run_scene "baseline-s1-room-chat"

# 场景 2：仅连接 + 心跳保活
CONNS_S2="${LOADTEST_S2_CONNS:-500}"
if [ "${LOADTEST_BASELINE_HEAVY:-0}" = "1" ]; then
    CONNS_S2=2000
fi
LOADTEST_CONNS="${CONNS_S2}" LOADTEST_RATE=0.2 LOADTEST_DURATION=120s \
    LOADTEST_MODE=keepalive LOADTEST_RAMP=30s \
    LOADTEST_REPORT="${ROOT}/reports/loadtest/baseline-s2-keepalive.json" \
    LOADTEST_CSV="${ROOT}/reports/loadtest/baseline-s2-keepalive.csv" \
    run_scene "baseline-s2-keepalive"

# 场景 3：少量连接高 QPS burst
LOADTEST_CONNS=100 LOADTEST_RATE=50 LOADTEST_DURATION=30s \
    LOADTEST_MODE=chat LOADTEST_RAMP=5s \
    LOADTEST_REPORT="${ROOT}/reports/loadtest/baseline-s3-burst.json" \
    LOADTEST_CSV="${ROOT}/reports/loadtest/baseline-s3-burst.csv" \
    run_scene "baseline-s3-burst" -burst

echo ""
echo "[loadtest-baseline] collecting redis stats..."
if docker exec beehive-redis redis-cli -a 111111 ZCARD im:sid:zset 2>/dev/null; then
    docker exec beehive-redis redis-cli -a 111111 ZCARD im:sid:zset 2>/dev/null | \
        awk '{print "  im:sid:zset count=" $0}'
fi

if command -v docker >/dev/null 2>&1 && docker ps --format '{{.Names}}' | grep -qx beehive-runner; then
    echo "[loadtest-baseline] docker stats snapshot (runner):"
    docker stats beehive-runner --no-stream --format '  CPU={{.CPUPerc}} MEM={{.MemUsage}}' 2>/dev/null || true
fi

echo ""
echo "[loadtest-baseline] done. Reports under reports/loadtest/"
echo "  Fill doc/LOADTEST_REPORT.md from JSON or copy latest numbers."
