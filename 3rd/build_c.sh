#!/bin/bash
###############################################################################
## 下载并编译 C 第三方库，安装到 3rd/install/
## 依赖: git, make, gcc/clang
## 可选: cmake (cJSON), autoconf/automake/libtool (protobuf-c)
###############################################################################
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="${SCRIPT_DIR}/build"
PREFIX="${SCRIPT_DIR}/install"
SRC_DIR="${BUILD_DIR}/src"

if command -v nproc >/dev/null 2>&1; then
    JOBS="$(nproc)"
elif command -v sysctl >/dev/null 2>&1; then
    JOBS="$(sysctl -n hw.ncpu)"
else
    JOBS=4
fi

export PKG_CONFIG_PATH="${PREFIX}/lib/pkgconfig:${PKG_CONFIG_PATH:-}"
export LDFLAGS="-L${PREFIX}/lib ${LDFLAGS:-}"
export CPPFLAGS="-I${PREFIX}/include ${CPPFLAGS:-}"
export PATH="${PREFIX}/bin:${PATH}"

log() { echo "[3rd] $*"; }
die() { echo "[3rd] ERROR: $*" >&2; exit 1; }

clone_repo() {
    local name="$1"
    local url="$2"
    local tag="${3:-}"
    local dir="${SRC_DIR}/${name}"

    mkdir -p "${SRC_DIR}"
    if [ -d "${dir}/.git" ]; then
        log "update ${name}"
        git -C "${dir}" fetch --depth 1 origin ${tag:+tag/${tag}}
        git -C "${dir}" checkout -f ${tag:-FETCH_HEAD}
    else
        if [ -n "${tag}" ]; then
            log "clone ${name} (${tag}) from ${url}"
            git clone --depth 1 --branch "${tag}" "${url}" "${dir}"
        else
            log "clone ${name} from ${url}"
            git clone --depth 1 "${url}" "${dir}"
        fi
    fi
}

build_zlib() {
    local dir="${SRC_DIR}/zlib"
    clone_repo zlib https://github.com/madler/zlib.git
    cd "${dir}"
    ./configure --prefix="${PREFIX}"
    make -j"${JOBS}"
    make install
    log "zlib installed"
}

build_openssl() {
    local ver="3.3.2"
    local dir="${BUILD_DIR}/openssl-${ver}"
    fetch_tarball "openssl-${ver}" \
        "https://github.com/openssl/openssl/releases/download/openssl-${ver}/openssl-${ver}.tar.gz"
    cd "${dir}"
    ./config --prefix="${PREFIX}" --openssldir="${PREFIX}/ssl" shared zlib
    make -j"${JOBS}"
    make install_sw
    log "openssl installed"
}

build_cjson() {
    local dir="${SRC_DIR}/cJSON"
    clone_repo cJSON https://github.com/DaveGamble/cJSON.git
    cd "${dir}"

    if command -v cmake >/dev/null 2>&1; then
        rm -rf build
        mkdir -p build
        cd build
        cmake .. \
            -DCMAKE_INSTALL_PREFIX="${PREFIX}" \
            -DENABLE_CJSON_UTILS=Off \
            -DENABLE_CJSON_TEST=Off \
            -DBUILD_SHARED_LIBS=On
        make -j"${JOBS}"
        make install
    else
        make -j"${JOBS}" CJSON_LIB=libcjson.a
        make PREFIX="${PREFIX}" install-cjson
    fi
    log "cJSON installed"
}

fetch_tarball() {
    local name="$1"
    local url="$2"
    local dir="${BUILD_DIR}/${name}"

    if [ -d "${dir}" ]; then
        log "use cached ${name} at ${dir}"
        return 0
    fi

    mkdir -p "${BUILD_DIR}"
    log "download ${name} from ${url}"
    curl -fsSL "${url}" | tar -xz -C "${BUILD_DIR}"
}

build_protobuf_c() {
    local ver="1.5.0"
    local dir="${BUILD_DIR}/protobuf-c-${ver}"
    fetch_tarball "protobuf-c-${ver}" \
        "https://github.com/protobuf-c/protobuf-c/releases/download/v${ver}/protobuf-c-${ver}.tar.gz"
    cd "${dir}"
    ./configure --prefix="${PREFIX}" --disable-protoc
    make -j"${JOBS}"
    make install
    log "protobuf-c installed"
}

build_curl() {
    local ver="8.12.1"
    local dir="${BUILD_DIR}/curl-${ver}"
    fetch_tarball "curl-${ver}" \
        "https://curl.se/download/curl-${ver}.tar.gz"
    cd "${dir}"
    ./configure \
        --prefix="${PREFIX}" \
        --with-openssl="${PREFIX}" \
        --with-zlib="${PREFIX}" \
        --disable-ldap \
        --disable-ldaps \
        --without-libpsl
    make -j"${JOBS}"
    make install
    log "libcurl installed"
}

verify_install() {
    check_lib() {
        local base="$1"
        [ -f "${PREFIX}/lib/lib${base}.a" ] \
            || [ -f "${PREFIX}/lib/lib${base}.dylib" ] \
            || [ -f "${PREFIX}/lib/lib${base}.so" ]
    }

    local libs="z ssl crypto cjson protobuf-c curl"
    local name
    for name in ${libs}; do
        check_lib "${name}" || die "缺少 lib${name}，请检查 ${PREFIX}/lib"
    done

    [ -f "${PREFIX}/include/cjson/cJSON.h" ] \
        || die "缺少 cJSON 头文件 ${PREFIX}/include/cjson/cJSON.h"
    [ -f "${PREFIX}/include/protobuf-c/protobuf-c.h" ] \
        || die "缺少 protobuf-c 头文件"
    [ -f "${PREFIX}/include/curl/curl.h" ] \
        || die "缺少 curl 头文件"

    log "install verify OK -> ${PREFIX}"
}

main() {
    command -v git >/dev/null 2>&1 || die "git 未安装"
    command -v make >/dev/null 2>&1 || die "make 未安装"
    command -v curl >/dev/null 2>&1 || die "curl 未安装"
    command -v gcc >/dev/null 2>&1 || command -v clang >/dev/null 2>&1 || die "C 编译器未安装"

    mkdir -p "${BUILD_DIR}" "${PREFIX}/lib" "${PREFIX}/include"

    log "prefix=${PREFIX}, jobs=${JOBS}"

    if [ ! -f "${PREFIX}/lib/libz.a" ] && [ ! -f "${PREFIX}/lib/libz.dylib" ]; then
        build_zlib
    else
        log "skip zlib (already installed)"
    fi

    if [ ! -f "${PREFIX}/lib/libssl.a" ] && [ ! -f "${PREFIX}/lib/libssl.dylib" ]; then
        build_openssl
    else
        log "skip openssl (already installed)"
    fi

    if [ ! -f "${PREFIX}/include/cjson/cJSON.h" ]; then
        build_cjson
    else
        log "skip cJSON (already installed)"
    fi

    if [ ! -f "${PREFIX}/include/protobuf-c/protobuf-c.h" ]; then
        build_protobuf_c
    else
        log "skip protobuf-c (already installed)"
    fi

    if [ ! -f "${PREFIX}/include/curl/curl.h" ]; then
        build_curl
    else
        log "skip curl (already installed)"
    fi

    verify_install
    log "全部 C 第三方库编译完成"
}

main "$@"
