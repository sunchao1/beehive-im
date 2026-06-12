#!/bin/sh
###############################################################################
## 第三方依赖入口脚本
##   1. Go 依赖 (govendor / GOPATH)
##   2. C 第三方库 (3rd/build_c.sh)
###############################################################################
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJ="$(cd "${SCRIPT_DIR}/.." && pwd)"

# ---------- Go 依赖 ----------
if [ -z "${GOPATH:-}" ]; then
    export GOPATH="${PROJ}/gopath"
fi
export GO111MODULE=off
root="${GOPATH}/src"

LIBS="
github.com/gorilla/websocket
github.com/astaxie/beego/logs
github.com/golang/protobuf/proto
github.com/garyburd/redigo/redis
labix.org/v2/mgo
"

echo "[3rd] Go dependencies:"
for item in ${LIBS}; do
    if [ ! -e "${root}/${item}" ]; then
        echo "go get ${item}"
        GO111MODULE=off go get "${item}" || echo "WARN: go get ${item} failed"
    else
        echo "${root}/${item} exists"
    fi
done

# ---------- C 第三方库 ----------
echo "[3rd] C libraries:"
exec "${SCRIPT_DIR}/build_c.sh"
