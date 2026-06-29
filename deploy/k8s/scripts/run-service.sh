#!/bin/bash
# 单进程启动器：deploy/k8s Pod 内使用
set -euo pipefail

SERVICE="${1:?service name required}"
CONF="${2:?config path required}"
BIN="/opt/beehive/bin/${SERVICE}.v.1.1"

if [ ! -x "${BIN}" ]; then
  echo "missing binary: ${BIN}" >&2
  exit 1
fi

mkdir -p /opt/beehive/log
cd /opt/beehive/bin

case "${SERVICE}" in
  frwder)
    exec "${BIN}" -c "${CONF}" -d -l debug
    ;;
  listend)
    exec "${BIN}" -c "${CONF}" -d -l debug
    ;;
  *)
    exec "${BIN}" -c "${CONF}"
    ;;
esac
