#!/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}/src/golang"
exec go run -mod=mod ../../tools/smoke-multinode/main.go "$@"
