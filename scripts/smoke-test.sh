#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}/src/golang"

go run -mod=mod ../../tools/smoke/main.go "$@"
echo ""
echo "[smoke] running failure-path cases..."
cd "${ROOT}"
exec ./scripts/smoke-fail.sh
