#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

echo "[loadtest-gate] status check..."
if ! ./scripts/status.sh; then
    echo "[loadtest-gate] FAIL: demo stack not healthy. Run: ./scripts/up-demo.sh"
    exit 1
fi

echo "[loadtest-gate] smoke-test..."
if ! ./scripts/smoke-test.sh; then
    echo "[loadtest-gate] FAIL: smoke-test did not pass"
    exit 1
fi

echo "[loadtest-gate] OK"
