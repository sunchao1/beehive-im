#!/bin/bash
# RTMQ 单机压测：只依赖 frwder（28888/28889），不启 websocket/msgsvr。
#
# 用法:
#   ./scripts/rtmq-bench.sh                    # publish 模式，默认 10s
#   ./scripts/rtmq-bench.sh unicast 15s        # unicast 模式，15 秒
#   ./scripts/rtmq-bench.sh publish 10s 8 2    # 8 生产者协程，2 个 BACKEND 消费者
#
# 环境: 优先在 beehive-runner 容器内跑；若无容器则尝试本机（需 Linux frwder）。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MODE="${1:-publish}"
DUR="${2:-10s}"
PRODUCERS="${3:-4}"
CONSUMERS="${4:-1}"

run_bench() {
	local extra=()
	extra+=(-mode "${MODE}")
	extra+=(-duration "${DUR}")
	extra+=(-producers "${PRODUCERS}")
	extra+=(-consumers "${CONSUMERS}")
	cd "${ROOT}/src/golang"
	exec go run -mod=mod ../../tools/rtmq-bench/main.go "${extra[@]}"
}

start_frwder_in_runner() {
	docker exec beehive-runner bash -lc '
		set -e
		cd /workspace/bin
		if (echo >/dev/tcp/127.0.0.1/28889) 2>/dev/null; then
			echo "[rtmq-bench] frwder already listening on 28889"
			exit 0
		fi
		/workspace/docker/gen-conf.sh /workspace/.run-conf
		export LD_LIBRARY_PATH=/workspace/3rd/install/lib
		nohup ./frwder.v.1.1 -c /workspace/.run-conf/frwder.xml -d -l error \
			>/workspace/log/frwder-bench.log 2>&1 &
		for i in $(seq 1 30); do
			if (echo >/dev/tcp/127.0.0.1/28889) 2>/dev/null; then
				echo "[rtmq-bench] frwder started"
				exit 0
			fi
			sleep 1
		done
		echo "[rtmq-bench] frwder failed to start"
		exit 1
	'
}

if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx beehive-runner; then
	echo "[rtmq-bench] using beehive-runner container"
	start_frwder_in_runner
	docker exec -e BEEHIVE_RTMQ_FORWARD=127.0.0.1:28888 \
		-e BEEHIVE_RTMQ_BACKEND=127.0.0.1:28889 \
		-w /workspace/src/golang beehive-runner \
		go run -mod=mod ../../tools/rtmq-bench/main.go \
		-mode "${MODE}" -duration "${DUR}" \
		-producers "${PRODUCERS}" -consumers "${CONSUMERS}"
	exit 0
fi

echo "[rtmq-bench] no beehive-runner; trying local frwder + go"
if ! (echo >/dev/tcp/127.0.0.1/28889) 2>/dev/null; then
	RUN_CONF="${ROOT}/.run-conf"
	"${ROOT}/docker/gen-conf.sh" "${RUN_CONF}"
	export LD_LIBRARY_PATH="${ROOT}/3rd/install/lib:${LD_LIBRARY_PATH:-}"
	cd "${ROOT}/bin"
	if [ -x ./frwder.v.1.1 ]; then
		nohup ./frwder.v.1.1 -c "${RUN_CONF}/frwder.xml" -d -l error \
			>"${ROOT}/log/frwder-bench.log" 2>&1 &
		sleep 2
	fi
fi

run_bench
