# beehive-im 本地 Docker 演示

## 快速开始

```bash
# 1. Linux 编译（macOS 需在容器内编译 C 服务）
docker compose --profile build run --rm builder /workspace/scripts/build-linux.sh

# 2. 一键起全栈（中间件 + 9 进程）
./scripts/up-demo.sh

# 3. 自动化验收
./scripts/smoke-test.sh

# 4. 浏览器双用户弹幕
./scripts/serve-demo.sh
# 打开 http://127.0.0.1:8088/ ，见 demo/web/README.md
```

## 端口

| 端口 | 服务 |
|------|------|
| 6379 | Redis |
| 3306 | MySQL |
| 27017 | Mongo（**4.4**，兼容 mgo.v2；升级后若连接失败请 `docker volume rm beehive-im_mongo_data`） |
| 8000 | usrsvr HTTP（register / iplist） |
| 8002 | websocket WS `/im` |
| 8004 | chatroom HTTP |
| 9002 | listend TCP（可选 CLI） |

## 测试数据

| 项 | 值 |
|----|-----|
| 用户 | uid=`100001`、`100002` |
| 房间 | rid=`10001`（MySQL 种子房间 demo-room） |
| 密码 | Redis/MySQL/Mongo 均为 `111111` |

## 架构说明（单容器 runner）

`runner` 在一个 Linux 容器内启动 9 进程（frwder/listend + 7 个 Go 服务）。  
容器内 frwder/seqsvr 使用 `127.0.0.1`；中间件使用 compose 服务名 `redis`/`mysql`/`mongo`。  
`BEEHIVE_WS_IP` / `BEEHIVE_ACCESS_IP` 默认 `127.0.0.1`，供宿主机浏览器与 smoke 通过端口映射连接；websocket 进程绑定 `0.0.0.0:8002`。

配置渲染：`docker/gen-conf.sh` → `.run-conf/`（模板在 `conf/templates/`）。

## 验收命令

```bash
./scripts/up-demo.sh      # 起全栈
./scripts/status.sh       # 应 pass=9 fail=0
./scripts/smoke-test.sh   # 应输出 smoke-test OK
```

## 常见失败

| 现象 | 排查 |
|------|------|
| iplist 空 | `log/monitor.log` 是否有 websocket 注册；等 5–10s 后重试 |
| WS 连不上 | `./scripts/status.sh`；确认 8002 映射与 `BEEHIVE_WS_IP=127.0.0.1` |
| join 失败 / 弹幕收不到 | MySQL 种子 rid=10001；join 后 chatroom 需写入 `room:rid:zset`（已修）；`docker logs beehive-runner` |
| frwder 拒绝 | 容器内先起 frwder 再起 Go（见 `scripts/run-in-linux.sh`） |

## 运维脚本

```bash
./scripts/status.sh      # 中间件 + 端口 + 进程
./scripts/stop-linux.sh  # 停 runner（中间件仍运行）
docker compose down      # 停全部
```
