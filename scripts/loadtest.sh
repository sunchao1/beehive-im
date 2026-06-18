#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

GATE="${LOADTEST_SKIP_GATE:-0}"
if [ "${GATE}" != "1" ]; then
    ./scripts/loadtest-gate.sh
fi

cd "${ROOT}/src/golang"
mkdir -p "${ROOT}/reports/loadtest"

SCENARIO="${LOADTEST_SCENARIO:-custom}"
REPORT="${LOADTEST_REPORT:-${ROOT}/reports/loadtest/latest.json}"
CSV="${LOADTEST_CSV:-${ROOT}/reports/loadtest/latest.csv}"

exec go run -mod=mod ../../tools/loadtest/*.go \
    -scenario "${SCENARIO}" \
    -uid-base "${LOADTEST_UID_BASE:-200000}" \
    -conns "${LOADTEST_CONNS:-200}" \
    -rid "${LOADTEST_RID:-10001}" \
    -rate "${LOADTEST_RATE:-1}" \
    -duration "${LOADTEST_DURATION:-60s}" \
    -mode "${LOADTEST_MODE:-chat}" \
    -ramp "${LOADTEST_RAMP:-10s}" \
    -usrsvr "${BEEHIVE_USRSVR_URL:-http://127.0.0.1:8000}" \
    -ws-addr "${BEEHIVE_WS_ADDR:-}" \
    -report "${REPORT}" \
    -csv "${CSV}" \
    ${LOADTEST_EXTRA:-} \
    "$@"
