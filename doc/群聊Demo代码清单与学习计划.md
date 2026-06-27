# 群聊 Demo 代码清单与学习计划

> **直白结论**：群聊 Demo 真正会碰到的代码，**精读约 25 个文件、3500 行**（只读标注函数，不必整文件通读）；若把公共库和 ONLINE 全读，约 **40 个文件、6000 行**。  
> **配套详解**：[群聊Demo源码导读.md](群聊Demo源码导读.md)

---

## 1. 涉及哪些代码（总表）

按 **Demo 操作顺序** 排列。`必读行` = 建议只读这些函数/段落，别从头啃整文件。

### 1.1 前端 + 启动脚本（先看，约 650 行）

| # | 文件 | 行数 | Demo 干什么 | 必读 |
|---|------|------|-------------|------|
| 1 | `demo/web/group.html` | 52 | 双窗口 UI | 全文 |
| 2 | `demo/web/group-app.js` | 343 | 注册→上线→建群→加群→聊天 | 全文 |
| 3 | `demo/web/demo-common.js` | 69 | `/im/register`、`/im/iplist` | 全文 |
| 4 | `demo/web/pb.js` | 188 | 群相关 protobuf 编解码 | `encodeOnline`、`encodeGroupCreat/Join/Chat/Quit/Dismiss` |
| 5 | `demo/web/style.css` | — | 样式 | **可跳过** |
| 6 | `scripts/up-demo.sh` | 77 | 一键起栈 | 扫一眼 |
| 7 | `scripts/demo-proxy.py` | 83 | 8088 静态页 + 代理 usrsvr | 扫一眼 |
| 8 | `docker-compose.yml` | — | `demo-web`、`runner` 服务 | 搜 `demo-web`、`runner` 两段 |

### 1.2 协议与 Redis 键（对照用，约 330 行）

| # | 文件 | 行数 | 干什么 | 必读 |
|---|------|------|--------|------|
| 9 | `src/golang/lib/comm/mesg.go` | 252 | `CMD_*`、52 字节包头 | 群 cmd 段 + `MesgHeader` |
| 10 | `src/golang/lib/comm/key.go` | 76 | Redis 键名常量 | 全文（很短） |
| 11 | `doc/mesg/mesg.proto` | 1053 | 协议定义 | **只搜** `MesgGroup` / `MesgOnline`，别通读 |
| 12 | `src/clang/incl/cmd_list.h` | — | C 侧 cmd（与 Go 对齐） | 搜 `CMD_GROUP` |

### 1.3 HTTP：注册 + 接入调度（约 500 行）

| # | 文件 | 行数 | 干什么 | 必读 |
|---|------|------|--------|------|
| 13 | `src/golang/exec/usrsvr/routers/router.go` | 24 | `/im/register`、`/im/iplist` 路由 | 全文 |
| 14 | `src/golang/exec/usrsvr/controllers/register.go` | 159 | 分配 sid | `Register`、`register_handler` |
| 15 | `src/golang/exec/usrsvr/controllers/iplist.go` | 293 | 返回 WS 地址 + token | `Iplist`、`iplist_get` |
| 16 | `src/golang/exec/usrsvr/controllers/task.go` | — | 后台刷新 listend 字典 | 搜 `listend_dict`（iplist 数据来源） |

### 1.4 iplist 前置链：monitor + websocket 上报（约 700 行）

| # | 文件 | 行数 | 干什么 | 必读 |
|---|------|------|--------|------|
| 17 | `src/golang/exec/websocket/controllers/task.go` | 262 | 定时发 `CMD_LSND_INFO` | 搜 `LSND_INFO`、`gather` |
| 18 | `src/golang/exec/monitor/controllers/mesg.go` | 433 | 写 Redis `im:lsnd:*` | 搜 `lsnd_info` |

### 1.5 WebSocket 接入层（约 900 行精读区，文件总行数多）

| # | 文件 | 行数 | 干什么 | 必读 |
|---|------|------|--------|------|
| 19 | `src/golang/exec/websocket/controllers/listend.go` | 164 | 初始化、调 `MesgRegister` | 尾部 init 段 |
| 20 | `src/golang/exec/websocket/controllers/mesg.go` | 399 | **上行**：ONLINE、转发群 cmd | `MesgRegister` L20；`LsndMesgOnlineHandler` L130；`LsndMesgCommHandler` L77 |
| 21 | `src/golang/exec/websocket/controllers/upmesg.go` | 1069 | **下行**：群 ACK/CHAT/NTF fan-out | `UpMesgRegister` L23-70；`LsndUpMesgOnlineAckHandler`；`LsndUpMesgGroupMemberAckHandler` L952；`LsndUpMesgGroupChatHandler` L987；`LsndUpMesgGroupNtfHandler` L1008；`LsndRoomSendDataCb`（群/室共用下发） |

### 1.6 C 转发层（约 110 行，必看）

| # | 文件 | 行数 | 干什么 | 必读 |
|---|------|------|--------|------|
| 22 | `src/clang/exec/frwder/frwd_mesg.c` | 109 | RTMQ 上下行路由 | **全文**（就两个 handler） |
| 23 | `conf/templates/frwder.xml` | — | 28888 / 28889 端口 | 扫一眼 |

> Demo **不走** TCP listend，但架构对称：`src/clang/exec/listend/lsnd_mesg.c` 可面试前扫 5 分钟。

### 1.7 上线（usrsvr，约 200 行精读）

| # | 文件 | 行数 | 干什么 | 必读 |
|---|------|------|--------|------|
| 24 | `src/golang/exec/usrsvr/controllers/mesg.go` | 904 | ONLINE 写 Redis 会话 | `UsrSvrOnlineHandler` L383；`online_handler` L304 附近 |

### 1.8 群生命周期（usrsvr，约 700 行）

| # | 文件 | 行数 | 干什么 | 必读 |
|---|------|------|--------|------|
| 25 | `src/golang/exec/usrsvr/controllers/gmesg.go` | 512 | 建群/加群/退群/解散… | Demo 用到的：`GroupCreat` L82、`Join` L152、`Quit` L187、`Dismiss` L117、`Invite` L218；`allocGid` L72 |
| 26 | `src/golang/exec/usrsvr/controllers/gmesg_send.go` | 78 | 按 gid→nid 广播通知 | `groupBroadcast` 全文 |
| 27 | `src/golang/lib/chat/group_ops.go` | 384 | Redis 群成员/在线路由 | `GroupRegister`、`GroupJoinOnline`、`GroupJoin`、`GroupQuit`、`GroupGetGidToNidSet` |
| 28 | `src/golang/lib/chat/group.go` | 103 | 群结构体/常量 | 扫一眼 |

### 1.9 群消息 + 第一段 fan-out（msgsvr，约 500 行）

| # | 文件 | 行数 | 干什么 | 必读 |
|---|------|------|--------|------|
| 29 | `src/golang/exec/msgsvr/controllers/gmesg.go` | 408 | `GROUP-CHAT` 校验、fan-out、落库 | `MsgSvrGroupChatHandler` L197；`group_chat_handler` 内 fan-out 循环 |
| 30 | `src/golang/exec/msgsvr/controllers/comm.go` | 44 | 按 nid 发 frwder | `send_data` |

### 1.10 第二段 fan-out：本节点群会话表（约 150 行精读）

| # | 文件 | 行数 | 干什么 | 必读 |
|---|------|------|--------|------|
| 31 | `src/golang/lib/chat_tab/chat.go` | 689 | `ImGroupJoin/Quit/TravImGroupSession` | L603-620 附近 |
| 32 | `src/golang/lib/chat_tab/comm.go` | 615 | 内存索引实现 | `session_join_im_group` L96；`session_quit_im_group` L126 |

### 1.11 RTMQ 怎么连 frwder（卡住再看，约 150 行）

| # | 文件 | 行数 | 干什么 | 必读 |
|---|------|------|--------|------|
| 33 | `src/golang/lib/rtmq/rtmq_proxy.go` | 954 | `Register`、`AsyncSend` | L321 `Register`、L365 `AsyncSend` 两段 |

### 1.12 终端验收（对照前端，约 380 行）

| # | 文件 | 行数 | 干什么 | 必读 |
|---|------|------|--------|------|
| 34 | `tools/smoke-group/main.go` | 383 | 无 UI 跑通建群/加群/互发 | 全文（和 `group-app.js` 一一对应） |
| 35 | `scripts/smoke-group.sh` | 8 | `go run` 包装 | 扫一眼 |

### 1.13 Demo 不跑但进程在栈里（**周末可跳过**）

| 文件 | 说明 |
|------|------|
| `src/golang/exec/chatroom/*` | 聊天室，和群 fan-out **模式相同** |
| `src/golang/exec/seqsvr/*` | register 调 Thrift 分配 sid |
| `src/clang/exec/listend/*` | TCP 接入，Demo 用 Go websocket |
| `src/golang/lib/mesg/mesg.pb.go` | 生成代码，查字段用 proto 即可 |

---

## 2. 按 Demo 步骤：读哪个文件

| 你在页面点的按钮 | 前端 | 后端顺序 |
|------------------|------|----------|
| 上线 | `group-app.js` → `demo-common.js` | `register.go` → `iplist.go` → `group-app.js` WS → `mesg.go` ONLINE → `frwd_mesg.c` → `usrsvr/mesg.go` → `upmesg.go` ONLINE-ACK |
| 建群 | `pb.js` CREAT | `mesg.go` 转发 → `gmesg.go` Creat → `group_ops.go` → `upmesg.go` CREAT-ACK → `chat_tab` ImGroupJoin |
| 加群 | JOIN | 同上 Join + `GROUP-JOIN-NTF` → `gmesg_send.go` |
| 发消息 | CHAT | `msgsvr/gmesg.go` fan-out nid → `upmesg.go` GroupChatHandler → `TravImGroupSession` |
| 退群/解散 | QUIT/DISMISS | `gmesg.go` + `group_ops.go` + websocket ImGroupQuit |

---

## 3. 周末两天学习计划（可执行）

假设每天 **4～5 小时**，按「读代码 → 立刻验证」交替。

### 周六：环境 + 前半链路

| 时段 | 读什么 | 验证什么 |
|------|--------|----------|
| 0.5h | 本文 §1.1、`up-demo.sh` | 起栈：`docker compose --profile build run --rm --build builder`（改过 Go/C 才需要）<br>`docker compose --profile run --profile demo up -d` |
| 1h | `group.html` + `group-app.js` + `demo-common.js` | 打开 http://127.0.0.1:8088/group.html ，F12 看 Network：`/im/register`、`/im/iplist` |
| 0.5h | `comm/mesg.go` 包头 + `pb.js` | 对照 WS 二进制帧 |
| 1h | `register.go`、`iplist.go`、`task.go`(monitor/websocket) | 若卡在 iplist：`docker compose --profile run restart runner`，等 20s |
| 1h | `websocket/mesg.go` ONLINE + `frwd_mesg.c` + `usrsvr/mesg.go` ONLINE | 两窗口都点「上线」，日志区出现 ONLINE-ACK |
| 0.5h | `upmesg.go` ONLINE-ACK | 确认按钮「建群/加群」可点 |

**周六收工标准**：双窗口都能上线，知道 register / iplist / ONLINE 各在哪几个文件。

### 周日：群聊 + fan-out + 验收

| 时段 | 读什么 | 验证什么 |
|------|--------|----------|
| 1h | `gmesg.go`(Creat/Join) + `group_ops.go` + `gmesg_send.go` | A 建群，B 加群；日志有 gid |
| 1h | `msgsvr/gmesg.go` + `upmesg.go` GroupChat | A 发消息，**B 能看见** |
| 0.5h | `chat_tab` ImGroup 三段函数 | 理解：为何 msgsvr 推下来还要 websocket 再 fan-out |
| 0.5h | `gmesg.go` Quit/Dismiss | 退群、解散各点一次 |
| 1h | `tools/smoke-group/main.go` | `./scripts/smoke-group.sh` 终端跑通 |
| 0.5h | 口述一遍 fan-out（见下） | 不看文档能讲 3 分钟 |

**周日收工标准**：浏览器双窗口互发 OK + smoke 脚本 OK + 能讲 gid→nid→cid。

---

## 4. 验证清单（打勾即用）

### 4.1 环境

```bash
docker compose --profile run --profile demo up -d
docker compose ps    # runner、demo-web 应为 Up
curl -s "http://127.0.0.1:8000/im/register?uid=90001&nation=1&city=1&town=1"
```

### 4.2 浏览器（主验证）

- [ ] http://127.0.0.1:8088/group.html 打开两窗口
- [ ] 窗口 A uid=90001，B uid=90002，都「上线」成功
- [ ] A「建群」，日志出现 `Ok:{gid}`
- [ ] B 输入 gid「加群」
- [ ] A 发「hello」，B 日志收到 `[90001] hello`
- [ ] B 回复，A 也能收到
- [ ] （可选）退群、解散

### 4.3 终端 smoke

```bash
./scripts/smoke-group.sh
# 或指定 uid：./scripts/smoke-group.sh --uid-a 90001 --uid-b 90002
```

- [ ] 输出含 creat/join/chat 成功，无 panic

### 4.4 看日志（加深理解）

```bash
docker compose exec runner tail -f /app/log/msgsvr.log
docker compose exec runner tail -f /app/log/usrsvr.log
```

发一条群消息时，msgsvr 应有 fan-out 到多个 nid 的 trace。

### 4.5 常见问题

| 现象 | 处理 |
|------|------|
| iplist 一直等 | `docker compose --profile run restart runner`，等 20s |
| 互看不见消息 | 确认改过 `upmesg.go` 群 CHAT handler；重启 runner |
| 改 Go/C 后无效 | 重新 `builder` 编译 + restart runner |

---

## 5. 你要几天能撸完？（评估）

按 **「看懂主链路 + 能跑验证 + 面试能讲 fan-out」** 来算：

| 目标 | 时间 | 说明 |
|------|------|------|
| **最低可用** | **1 天（6～8h）** | 只读 §1.1 + §1.8～1.10 标注函数，跑通 Demo + smoke；能说出 usrsvr/msgsvr 分工 |
| **周末档（推荐）** | **2 天（各 4～5h）** | 按 §3 计划；覆盖 HTTP、ONLINE、frwder、群生命周期、双 fan-out |
| **稳了能面试细讲** | **3～4 天（各 3h）** | 加读 `rtmq_proxy.go`、`online_handler` 全段、`group_ops.go` 全文、smoke 对照改参数实验 |
| **吃透群模块** | **5～7 天** | 再读 `gmesg.go` 里踢人/禁言/管理员；对比 `chatroom` 的 Room fan-out |

**个人基础差异**：

- 熟悉 Go + 长连接：**2 天够**。
- 第一次接触 RTMQ / 多进程 IM：**3 天更稳**。
- 还要 C 层 listend 一起讲：**+1 天**。

**不必妄想一次读完的文件**：`upmesg.go`(1069 行) 只读群相关 ~120 行；`mesg.proto` 只查群消息结构；`vendor/` **零阅读**。

---

## 6. 面试 3 分钟口述模板（验证自己是否真懂）

1. 浏览器 HTTP 注册拿 sid，iplist 拿 token 和 WS 地址（monitor 写 Redis，websocket 报 LSND-INFO）。
2. WS 发 ONLINE，usrsvr 写 `im:sid:*`，ACK 后接入层才转发群 cmd。
3. 建群/加群在 **usrsvr**，写 Redis `chat:gid:{gid}:to:nid:zset`（谁在哪个接入节点在线）。
4. 发 GROUP-CHAT 进 **msgsvr**，按 gid 查所有 nid，**第一段 fan-out** 推到各 websocket 节点。
5. 各节点 **TravImGroupSession**，**第二段 fan-out** 推到本机所有群成员连接。
6. 中间 **C frwder** 只认 cmd + nid，不懂群语义。

---

## 7. 打印用勾选表（35 项）

```
前端/脚本
[ ] demo/web/group.html
[ ] demo/web/group-app.js
[ ] demo/web/demo-common.js
[ ] demo/web/pb.js
[ ] scripts/up-demo.sh
[ ] scripts/demo-proxy.py

协议/键
[ ] lib/comm/mesg.go
[ ] lib/comm/key.go
[ ] doc/mesg/mesg.proto (仅 MesgGroup*)
[ ] clang/incl/cmd_list.h (CMD_GROUP*)

HTTP + iplist
[ ] usrsvr/routers/router.go
[ ] usrsvr/controllers/register.go
[ ] usrsvr/controllers/iplist.go
[ ] usrsvr/controllers/task.go (listend_dict)
[ ] websocket/controllers/task.go (LSND-INFO)
[ ] monitor/controllers/mesg.go

接入 + 转发
[ ] websocket/controllers/listend.go
[ ] websocket/controllers/mesg.go
[ ] websocket/controllers/upmesg.go (群段落)
[ ] clang/exec/frwder/frwd_mesg.c

上线
[ ] usrsvr/controllers/mesg.go (ONLINE)

群业务
[ ] usrsvr/controllers/gmesg.go (Demo 用到的 handler)
[ ] usrsvr/controllers/gmesg_send.go
[ ] lib/chat/group_ops.go
[ ] lib/chat/group.go
[ ] msgsvr/controllers/gmesg.go
[ ] msgsvr/controllers/comm.go
[ ] lib/chat_tab/chat.go (ImGroup*)
[ ] lib/chat_tab/comm.go (session_*_im_group)

可选/验收
[ ] lib/rtmq/rtmq_proxy.go (Register/AsyncSend)
[ ] tools/smoke-group/main.go
[ ] scripts/smoke-group.sh
```

---

*维护：与 `group.html` Demo 路径一致；更细的逐步说明见 [群聊Demo源码导读.md](群聊Demo源码导读.md)。*
