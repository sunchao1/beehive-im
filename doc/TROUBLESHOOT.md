# 故障排查手册

> Top 10 常见问题 + 日志地图。演示现场优先配合 [INTERVIEW_DEMO.md](INTERVIEW_DEMO.md) 三个错误场景。

---

## 1. Top 10 失败现象

| # | 现象 | 优先查 | 所属 |
|---|------|--------|------|
| 1 | **iplist 空** / `len=0` | websocket/listend 是否向 monitor 注册；`log/monitor.log` | 接入未注册 |
| 2 | **8000/8002 connection refused** | `./scripts/up-demo.sh`；`docker ps` runner | 栈未起 |
| 3 | **38889 / 28889 refused** | frwder 是否先于 Go 启动；`docker logs beehive-runner` | frwder 顺序 |
| 4 | **JOIN 失败** | MySQL 种子 rid=10001；`CHAT_ROOM_INFO_TAB.status=1` | 房间元数据 |
| 5 | **JOIN 成功但收不到弹幕** | chatroom/frwder/listend 进程；Redis `room:rid:10001:*` | fan-out 拓扑 |
| 6 | **Mongo 认证失败** | compose 用 Mongo **4.4** + `beehive/111111`；旧 volume 需删 | 中间件版本 |
| 7 | **demo/web Failed to fetch** | 必须用 `./scripts/serve-demo.sh`（8088 代理） | 跨域 |
| 8 | **smoke 超时 ROOM-CHAT** | `./scripts/status.sh` 9 进程；重建 Go 后需 `build-linux.sh` | 二进制过旧 |
| 9 | **群聊/推送无响应** | usrsvr/msgsvr 是否注册 frwder；`smoke-group.sh` | 业务进程 |
| 10 | **端口占用** | `lsof -i :8000` 等；`docker compose down` | 环境冲突 |

---

## 2. 快速诊断流程

```bash
./scripts/status.sh
curl -s 'http://127.0.0.1:8000/im/register?uid=100001&nation=1&city=1&town=1'
./scripts/smoke-test.sh
docker logs beehive-runner --tail 100
```

全失败 → 中间件 health；部分失败 → 对照下表查具体服务日志。

---

## 3. 日志地图

日志目录：**`log/`**（runner 内工作目录为 `/workspace`，挂载到仓库根）

| 文件 | 进程 | 关注点 |
|------|------|--------|
| `frwder.log` | frwder (C) | RTMQ 连接、鉴权、队列满 |
| `listend.log` | listend (C) | TCP 接入、转发 upstream |
| `websocket.log` | websocket (Go/C) | WS 连接、ONLINE |
| `usrsvr.log` | usrsvr | register/iplist/群 0x03xx/推送 |
| `chatroom.log` | chatroom | ROOM-JOIN/CHAT/CREAT/DISMISS |
| `msgsvr.log` | msgsvr | 私聊/群聊 fan-out |
| `monitor.log` | monitor | **LSND_INFO** 接入注册 |
| `seqsvr.log` | seqsvr | rid/gid 分配 |
| `tasker.log` | tasker | 定时任务 |
| `bin/listend.log` | 有时 listend 前台日志 | 容器启动早期 |

**容器内查看**：

```bash
docker exec -it beehive-runner sh -c 'ls -la /workspace/log/'
docker exec -it beehive-runner tail -50 /workspace/log/monitor.log
```

### 常用 grep

```bash
# 接入是否注册（iplist 依赖）
grep -i 'LSND\|listend\|websocket' log/monitor.log | tail -20

# 房间 join / chat
grep -i 'room-join\|room-chat\|Join' log/chatroom.log | tail -20

# 群聊
grep -i 'group' log/usrsvr.log log/msgsvr.log | tail -20

# frwder / RTMQ
grep -iE 'error|fail|refused' log/frwder.log | tail -20

# 上线失败
grep -i 'online' log/usrsvr.log log/websocket.log | tail -20
```

---

## 4. 典型故障详解

### iplist 为空

**原因链**：websocket 未启动 → 未发 `CMD_LSND_INFO` → monitor 未写 Redis `im:lsnd:*` → usrsvr iplist 查不到 WS 地址。

**检查**：

```bash
docker exec beehive-redis redis-cli -a 111111 ZRANGE im:lsnd:nid:zset 0 -1 WITHSCORES
curl -s 'http://127.0.0.1:8000/im/iplist?type=2&uid=100001&sid=<sid>&clientip=127.0.0.1'
```

**修复**：等 runner 启动 10～30s；确认 `BEEHIVE_WS_IP=127.0.0.1`；重启 runner。

### frwder 后端 28889 refused

Go 服务 `FRWDER` 配置指向 `127.0.0.1:28889`（容器内）。frwder 未监听则全部 RTMQ 失败。

**检查**：`./scripts/status.sh` 进程数；`log/frwder.log` 是否有 bind error。

### chatroom.xml / 配置错误

历史问题：错误根标签导致 chatroom 起不来。现用 **`conf/templates/` + `docker/gen-conf.sh`** 渲染到 `.run-conf/`。

**检查**：`.run-conf/chatroom.xml` 根节点为 `CHATROOM`（与 `ChatRoomConfXmlData` 一致）。

### MySQL 种子房间

```bash
docker exec beehive-mysql mysql -uroot -p111111 testdb \
  -e "SELECT rid,name,status FROM CHAT_ROOM_INFO_TAB WHERE rid=10001;"
```

期望 `status=1`。若为 0：`JOIN` 可能报 Room closed。

### Mongo 连接

- 镜像：**mongo:4.4**（mgo.v2 兼容）
- 升级 6.x 后旧 data volume 可能导致 auth 失败：`docker volume rm beehive-im_mongo_data`（会清空数据）

---

## 5. 演示专用：三个错误场景

见 [INTERVIEW_DEMO.md §3](INTERVIEW_DEMO.md#3-错误场景演示体现真系统)：

| 场景 | 工具 |
|------|------|
| A frwder/栈不完整 | stop runner + status |
| B 未进房 chat | `smoke-fail.sh` / demo/web |
| C 非法 rid / 重复 join | `smoke-fail.sh` |

---

## 6. 仍无法解决时

1. 全量重启：`docker compose down && ./scripts/up-demo.sh`
2. 重新编译：`docker compose --profile build run --rm builder /workspace/scripts/build-linux.sh`
3. 对照 [DEMO.md](DEMO.md)、[doc/ARCHITECTURE.md](ARCHITECTURE.md)
