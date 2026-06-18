#!/bin/bash
# 面试容量标定：L1 连接 / L2 fan-out / L3 多房（64G K8s 友好）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"
mkdir -p "${ROOT}/reports/loadtest"

GATE="${LOADTEST_SKIP_GATE:-1}"
export LOADTEST_SKIP_GATE="${GATE}"

run() {
    local name="$1"
    shift
    echo ""
    echo "========== ${name} =========="
    LOADTEST_SCENARIO="${name}" LOADTEST_EXTRA="$*" ./scripts/loadtest.sh
}

# L1：连接容量（keepalive），填外推表 C_pod
# 阶梯跑：改 CONNS 为 5000 / 10000 / 15000，取稳定最大 online_ok
CONNS="${LOADTEST_L1_CONNS:-5000}"
DUR="${LOADTEST_L1_DURATION:-120s}"
RAMP="${LOADTEST_L1_RAMP:-60s}"
C_POD="${LOADTEST_CONN_PER_POD:-5000}"
TARGET="${LOADTEST_TARGET_ONLINE:-1000000}"

LOADTEST_CONNS="${CONNS}" LOADTEST_RATE=0.2 LOADTEST_DURATION="${DUR}" \
    LOADTEST_RAMP="${RAMP}" LOADTEST_MODE=keepalive \
    LOADTEST_REPORT="${ROOT}/reports/loadtest/capacity-L1-keepalive.json" \
    run "capacity-L1-keepalive" \
    -target-online "${TARGET}" -conn-per-pod "${C_POD}"

# L2：1 发 N 看 fan-out（单房）
CONNS_L2="${LOADTEST_L2_CONNS:-501}"
RATE_L2="${LOADTEST_L2_RATE:-5}"
LOADTEST_CONNS="${CONNS_L2}" LOADTEST_RATE="${RATE_L2}" LOADTEST_DURATION=60s \
    LOADTEST_RAMP=30s LOADTEST_MODE=chat \
    LOADTEST_REPORT="${ROOT}/reports/loadtest/capacity-L2-fanout.json" \
    run "capacity-L2-fanout" \
    -senders 1 -rooms 1 \
    -target-online "${TARGET}" -conn-per-pod "${C_POD}"

# L3：多房 keepalive（rid 10001..10100）
CONNS_L3="${LOADTEST_L3_CONNS:-5000}"
ROOMS_L3="${LOADTEST_L3_ROOMS:-100}"
LOADTEST_CONNS="${CONNS_L3}" LOADTEST_RATE=0.2 LOADTEST_DURATION=90s \
    LOADTEST_RAMP=90s LOADTEST_MODE=keepalive \
    LOADTEST_REPORT="${ROOT}/reports/loadtest/capacity-L3-multiroom.json" \
    run "capacity-L3-multiroom" \
    -rooms "${ROOMS_L3}" \
    -target-online "${TARGET}" -conn-per-pod "${C_POD}"

echo ""
echo "[loadtest-capacity] done. Fill doc/assets/CAPACITY_EXTRAPOLATION.md from JSON:"
echo "  L1 online_ok -> C_pod; extrapolation.pods_for_target_online -> 百万 Pod 数"
echo "  L2 est_downstream_qps -> fan-out 能力"
ls -la "${ROOT}/reports/loadtest/capacity-"*.json 2>/dev/null || true
