1. 中间件 — 成功
三个容器均为 healthy：

服务	地址	凭证
Redis
127.0.0.1:6379
密码 111111
MySQL
127.0.0.1:3306
root/111111，库 testdb
Mongo
127.0.0.1:27017
beehive/111111，库 chat
种子数据已就绪（demo-room rid=10001）。

2. Linux 编译 — 成功
在 linux/amd64 容器内 make all 已通过，关键修复包括：

builder 强制 platform: linux/amd64（C 原子操作为 x86 专用）
容器内重建 Linux 版 3rd/install
OpenSSL / curl 链接、Go 模块路径等若干编译问题
当前 bin/ 下 11 个二进制均为 Linux x86-64，在 macOS 上不能直接运行。

3. 启动 9 进程 — 成功（需在 Linux 容器内）
./bin/start.sh 在 macOS 上不可用（依赖 epoll + Linux 二进制）。已用 scripts/start-linux.sh 在容器内启动，9 个进程均在运行：

frwder, seqsvr, msgsvr, tasker, usrsvr, monitor, chatroom, websocket, listend
当前有一个测试容器在跑：beehive-runner-test（Up）。

你本机怎么用
# 1. 中间件
./scripts/init-db.sh
# 2. 编译（首次较慢，约 15–40 分钟）
./scripts/build-linux.sh
# 3. 启动（Linux 容器内跑 9 进程）
./scripts/start-linux.sh
# 或后台：docker compose --profile run run -d --name beehive-runner runner
查看状态：

docker exec beehive-runner-test ps aux | grep '\.v\.1\.1'
tail -f log/monitor.log log/seqsvr.log
停止：

docker stop beehive-runner-test
docker compose down   # 如需停中间件
说明
启动初期 monitor/tasker 可能短暂报 38889 connection refused（等 frwder 就绪），之后多数服务已连上 RTMQ 并完成 Auth。
chatroom.log / usrsvr.log 仍为空，可能是 beego 日志路径或尚未有 HTTP 请求，不影响进程存活。
若需从 Mac 访问 WebSocket（8002），要在 docker-compose.yml 的 runner 上增加端口映射。
需要的话我可以继续帮你加端口映射，或写一个简单的客户端连 demo-room 做联调验证。