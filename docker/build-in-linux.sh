#!/bin/bash
set -euo pipefail

PROJ="${PROJ:-/workspace}"
cd "${PROJ}"

echo "[builder] PROJ=${PROJ}"

need_3rd_rebuild=0
required_libs=(libz libssl libcrypto libcurl cjson protobuf-c)
for lib in "${required_libs[@]}"; do
    if [ ! -f "${PROJ}/3rd/install/lib/lib${lib}.a" ] && \
       [ ! -f "${PROJ}/3rd/install/lib/lib${lib}.so" ] && \
       [ ! -f "${PROJ}/3rd/install/lib/lib${lib}.dylib" ]; then
        need_3rd_rebuild=1
        break
    fi
done

if [ "${need_3rd_rebuild}" -eq 0 ] && [ -f "${PROJ}/3rd/install/lib/libz.a" ]; then
    tmpdir="$(mktemp -d)"
    if ! (cd "${tmpdir}" && ar x "${PROJ}/3rd/install/lib/libz.a" 2>/dev/null && obj="$(ls *.o 2>/dev/null | head -1)" && file "${obj}" | grep -q 'ELF'); then
        need_3rd_rebuild=1
    fi
    rm -rf "${tmpdir}"
fi

if [ "${need_3rd_rebuild}" -eq 1 ]; then
    echo "[builder] building C third-party libs for Linux..."
    rm -rf "${PROJ}/3rd/install" "${PROJ}/3rd/build"
    "${PROJ}/3rd/build_c.sh"
else
    echo "[builder] reuse Linux 3rd/install"
fi

TARGET="${1:-all}"
echo "[builder] make ${TARGET}"
make -j"$(nproc)" "${TARGET}"

echo "[builder] done. binaries in ${PROJ}/bin/"
