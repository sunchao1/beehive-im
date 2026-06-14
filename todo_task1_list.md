# beehive-im 本地跑通任务清单

> **总目标**：优先打通完整 IM/弹幕链路（能跑 → 能玩），框架现代化（gin / mongo-driver / Go mod）放后面。  
> **原则**：先修编译与基础设施，再联调，最后补产品功能；`beego → gin`、`mgo → mongo-driver` **不在当前冲刺范围**。

---

## 0. 优先级与阶段总览

```
第一阶段（1～2 迭代）  能跑：编译 + 中间件 + 9 进程启动 + 种子数据
        ↓
第二阶段（1 迭代）      能玩：demo 客户端 + 进房发弹幕 + 基础 HTTP
        ↓
第三阶段（按需）        群聊/推送/风控/压测
        ↓
Task 2（后续）          Go mod 深化 + gin + mongo-driver（不阻塞前两阶段）
```

| 阶段 | 交付标准 | 预估 |
|------|----------|------|
| 第一阶段 | `make all` 成功；docker-compose 起中间件；`start.sh` 拉起 9 进程无 crash | 1～2 迭代 |
| 第二阶段 | 网页/CLI demo 完成「注册→上线→进房→发弹幕→他人收到」 | 1 迭代 |
| 第三阶段 | 按产品优先级增量交付 | 按需 |
| Task 2（延后） | gin + mongo-driver + vendor 清理 | 1～2 周 |

---

## 1. 第一阶段：打通「能跑」

> 本阶段 **保留 beego / mgo / GOPATH-vendor**，不替换框架；Go mod 可作为构建改进项并行推进，但不作为阻塞条件。

### 1.1 修编译链

#### C 第三方库（已完成 ✅）

- [x] `3rd/build_c.sh` 编译 zlib / openssl / cjson / protobuf-c / curl → `3rd/install/`
- [x] `make/options.mak` 与 clang Makefile 增加 `3rd/install` 路径

#### C 编译阻塞项

- [ ] **C1** 修复 `libutils.a` 缺失  
  - 现状：`listend`、`frwder` Makefile 链接 `libutils.a`，仓库中已无该库且无 `utils_` 符号引用  
  - 方案：从 `STATIC_LIB_LIST` 中移除 `libutils.a`（`src/clang/exec/listend/Makefile`、`frwder/Makefile`）

- [ ] **C2** 修复 `libchat` 对外部 `cctrl` 的依赖  
  - 现状：`src/clang/lib/chat/Makefile` 引用 `../cctrl/src/incl`、`../cctrl/lib`，仓库内无 cctrl  
  - 方案：`atomic.h` 已在 `src/clang/incl/`，改 INCLUDE 为 `-I$(PROJ)/src/clang/incl`，去掉 cctrl 路径

- [ ] **C3** macOS Makefile / 编译器兼容  
  - 现状：clang 对 `-fbounds-check`、`-rdynamic` 在 `-c` 阶段报 `-Werror`  
  - 方案：在 `make/build.mak` 中对 Darwin 去掉或条件编译这些 flag；或 Linux 容器内编译 C 部分

- [ ] **C4** 修复根 `Makefile` macOS shell 续行问题（`ARCHITECTURE.md` 已记录第 70 行注释风险）  
  - 检查 `@for ITEM in ${DIR}` 循环在 macOS `/bin/sh` 下是否正常

- [ ] **C5** 全量 C 编译验证  
  ```bash
  cd 3rd && ./build_c.sh          # 若未安装
  make all                        # 或分模块 make DIR=...
  ls bin/*.v.1.1 lib/*.so lib/*.a
  ```
  - 期望产物：`frwder`、`listend` + `libcore.so` 等

#### Go 编译与依赖

- [ ] **G1** 补齐 Go vendor（当前阶段仍用 vendor，不阻塞跑通）  
  ```bash
  export GOPATH=${PROJ}/gopath GO111MODULE=off
  make   # 会自动创建 gopath/src/beehive-im 软链
  # 若缺包：按 src/golang/vendor/vendor.txt 版本手动补 vendor 或 govendor fetch
  ```

- [ ] **G2** 7 个 Go 服务全部编译到 `bin/`  
  - `frwder` 之外：`seqsvr`、`msgsvr`、`tasker`、`usrsvr`、`monitor`、`chatroom`、`websocket`

- [ ] **G3**（可选，不阻塞跑通）Go Modules 迁移 — 详见 **§5 Task 1**  
  - 与 vendor 二选一；跑通后可再做 mod 化

---

### 1.2 docker-compose 一键中间件

- [ ] **D1** 新增项目根 `docker-compose.yml`  
  - 服务：`redis`（6379）、`mysql`（3306）、`mongo`（27017）  
  - 挂载：`conf/redis.conf`（或简化版）、数据 volume

- [ ] **D2** 新增 `scripts/init-db.sh`（或 `docker/init/`）  
  - MySQL：执行 `doc/database/mysql.sh`（库名 `testdb`，密码与 conf 对齐）  
  - Mongo：执行 `doc/database/mongo.eval`（库 `chat`，建索引）  
  - 注意：统一 conf 里各服务的 DB 连接串与 compose 环境变量

- [ ] **D3** 核对各服务 XML 配置与 compose 一致  
  - `conf/usrsvr.xml`、`chatroom.xml`、`msgsvr.xml`、`seqsvr.xml` 等  
  - Redis：`conf/redis.conf` port 6379  
  - MySQL/Mongo 地址由 `127.0.0.1` 改为 compose 服务名（若服务跑在容器外则仍用 localhost）

- [ ] **D4** 文档：`doc/LOCAL_DEV.md`（或 README 一节）  
  ```bash
  docker compose up -d
  ./scripts/init-db.sh
  ```

---

### 1.3 修/补 start.sh（9 进程）

当前 `bin/start.sh` 问题：

- Redis 启动被注释
- 无工作目录/路径检查
- 无进程是否已存在、启动失败检测
- 启动顺序与 `doc/ARCHITECTURE.md` 一致但缺 health wait

- [ ] **S1** 重写 `bin/start.sh`  
  1. `cd` 到脚本所在目录（`bin/`）  
  2. 检查二进制 `*.v.1.1` 是否存在  
  3. （可选）检测 Redis/MySQL/Mongo 端口  
  4. 按顺序启动并记录 PID/log：

  | 顺序 | 进程 | 说明 |
  |:----:|------|------|
  | 1 | `frwder` | 必须先起，RTMQ 网关 |
  | 2 | `seqsvr` | ID 分配 |
  | 3 | `msgsvr` | 消息中心 |
  | 4 | `tasker` | 定时任务 |
  | 5 | `usrsvr` | 用户中心 :8000 |
  | 6 | `monitor` | 监控 |
  | 7 | `chatroom` | 聊天室 :8004 |
  | 8 | `websocket` | WS :8002 |
  | 9 | `listend` | TCP :9002 |

- [ ] **S2** 补 `bin/stop.sh`  
  - 按 PID 文件优雅停止，或完善 `pkill` 模式  
  - 补上 `websocket` 与版本化二进制名

- [ ] **S3** 补 `bin/status.sh`（可选）  
  - 检查 9 进程 + 3 中间件端口

- [ ] **S4** smoke test：启动后日志无 fatal，frwder/listend 监听 28888/9002

---

### 1.4 预置测试房间和用户

> 绕过未实现的 `ROOM-CREAT` / `ROOM-DISMISS`（0x0401/0x0403）

- [ ] **P1** MySQL 种子数据脚本 `scripts/seed-mysql.sql`  
  - 基于 `doc/database/mysql.sh` 中 `CHAT_ROOM_INFO_TAB`  
  - 预置房间：如 `rid=10001, name=demo-room, owner=100001, status=正常`  
  - 预置 `IM_SID_GEN_TAB` / `IM_SEQ_GEN_TAB` 初始行

- [ ] **P2** Mongo 种子（若业务启动时需要）  
  - 空集合 + 索引即可（`mongo.eval` 已含索引）  
  - 可选：插入 1～2 条历史弹幕样例

- [ ] **P3** Redis 种子（若 chatroom 依赖键结构）  
  - 对照 `doc/REDIS.md` 预置 room 拓扑 / 在线计数相关 key（如有）

- [ ] **P4** 测试用户约定（文档化）  
  - 如 `uid=100001`、`uid=100002`  
  - 通过 `GET /im/register?uid=...&nation=1&city=1&town=1` 获取 `sid`  
  - 记录 demo 用的 `rid / uid / sid` 表

---

### 1.5 第一阶段完成标准（DoD）

- [ ] `3rd/build_c.sh` + `make all` 产出全部 C/Go 二进制
- [ ] `docker compose up -d` + init 脚本后，MySQL/Mongo/Redis 可用
- [ ] `bin/start.sh` 9 进程稳定运行 ≥5 分钟
- [ ] 种子房间 `rid=10001` 在 MySQL 中可查
- [ ] `usrsvr` `/im/register` 返回有效 `sid`

---

## 2. 第二阶段：打通「能玩」

### 2.1 最小 Demo 客户端

二选一或都做：

- [ ] **M1** Web 版（推荐）  
  - 路径建议：`demo/web/` 或 `tools/danmaku-demo/`  
  - 流程：注册 → 拉 iplist → WebSocket 连 `:8002` → ONLINE → ROOM-JOIN → ROOM-CHAT  
  - 协议：48 字节头 + Protobuf（参考 `doc/PROTOCOL.md`、`doc/mesg/mesg.proto`）  
  - UI：连接状态、房间号输入、消息输入框、弹幕列表

- [ ] **M2** CLI 版（可选）  
  - 基于现有 `src/clang/exec/client` 或 Go 小工具  
  - 支持两个终端各连一个 uid，互发 ROOM-CHAT

- [ ] **M3** Demo 使用说明 `demo/README.md`  
  - 启动中间件 → start.sh → 打开两个浏览器窗口 → 步骤截图/命令

---

### 2.2 端到端验证清单

- [ ] **E1** 用户 A、B 分别 `/im/register` 拿到不同 `sid`
- [ ] **E2** A、B 均 WS 上线（`ONLINE` / `ONLINE-ACK`）
- [ ] **E3** A、B 加入同一 `rid`（`ROOM-JOIN` / `ROOM-JOIN-ACK`）
- [ ] **E4** A 发 `ROOM-CHAT`，B 收到弹幕（C 路径：`listend`；Go 路径：`websocket` 各测一次更佳）
- [ ] **E5** 退出房间 / 下线无 panic（`ROOM-QUIT`、`OFFLINE`）

---

### 2.3 补关键缺口（跑通 demo 必需）

- [ ] **H1** 错误应答  
  - 非法参数、未进房发消息、重复进房等场景返回 `ERROR` 或对应 ACK 错误码（对照 `doc/ERRNO.md`）  
  - 优先：`chatroom` `ROOM-JOIN` / `ROOM-CHAT` 失败路径

- [ ] **H2** 基础 HTTP 查询（若 demo 需要）  
  - 在线人数：对照 `ROOM-USR-NUM`（0x0410）或 `GET /room/query`（`doc/HTTPSVR.md`）  
  - 房间列表：MySQL `CHAT_ROOM_INFO_TAB` 或新增简单 `GET /room/list`（若现有 query 不够用则最小实现）

- [ ] **H3** 日志与排障  
  - 统一 log 目录 `log/`  
  - demo 文档中列出常见失败：frwder 未起、Redis 键缺失、sid 过期等

---

### 2.4 第二阶段完成标准（DoD）

- [ ] 两个客户端同房互发弹幕，延迟可接受（本地 <100ms）
- [ ] Demo 文档 15 分钟内可复现全流程
- [ ] 至少 3 种错误场景有明确 ACK/HTTP 错误码

---

## 3. 第三阶段：按产品需求补功能（按需）

> 不影响前两阶段交付，按优先级排期。

### 3.1 协议与业务

- [ ] **F1** 群聊全套 `0x03xx`（创建/加入/群消息/踢人/禁言…）
- [ ] **F2** 推送 `0x05xx`（BC / P2P）
- [ ] **F3** 聊天室 `ROOM-CREAT` / `ROOM-DISMISS`（替代种子数据方案）
- [ ] **F4** HTTP 接口补全（`doc/HTTPSVR.md` 中标记未完成的 query/config）

### 3.2 弹幕生产级能力

- [ ] **F5** 敏感词过滤（`chatroom/mesg.go` 已有 TODO）
- [ ] **F6** 限流与背压（用户/房间 QPS）
- [ ] **F7** 异步写 Mongo（先广播后落库，热路径不与 DB 同步阻塞）
- [ ] **F8** 超大规模房间：消息采样 / 客户端合并策略

### 3.3 工程与容量

- [ ] **F9** 压测脚本（多 WS 连接、同房弹幕 fan-out）
- [ ] **F10** 基础指标（QPS、延迟、在线数、房间人数）— 为后续 K8s 做准备
- [ ] **F11** 多 frwder / 多 listend 实例验证（一致性哈希、NID 路由）

---

## 4. Task 2（延后）：框架现代化

> **明确后置**：不影响「能跑 / 能玩」。前两阶段完成后或并行低优先级进行。

### 4.1 Go Modules（详见原 Task 1 细节）

- [ ] 在 `src/golang/` 建立 `go.mod`（`module beehive-im`）
- [ ] 批量改 import：`beehive-im/src/golang/` → `beehive-im/`
- [ ] 处理 Thrift：`git.apache.org/thrift.git` → `github.com/apache/thrift`
- [ ] 改根 `Makefile` Go 编译段，去掉 `GOPATH` / `GO111MODULE=off`
- [ ] 删除 `vendor/`，保留 `go.sum`
- [ ] 7 服务编译 + 联调回归

### 4.2 beego → gin

- [ ] 迁移 `usrsvr`、`chatroom` 路由与 Controller
- [ ] `lib/log` 脱离 `beego/logs`（zap / slog）
- [ ] 其余服务仅换日志依赖

### 4.3 mgo → mongo-driver

- [ ] 重写 `lib/mongo/mongo.go`
- [ ] 改 `blacklist.go`、`gmesg.go`、`chatroom/mesg.go` 调用方
- [ ] `go.mod` 移除 `gopkg.in/mgo.v2`

建议顺序：

```
Go Modules → 日志库替换 → gin → mongo-driver
```

---

## 5. Task 1 附录：Go Modules 详细步骤

> 与 **§1.1 G3** 相同内容，展开供实施时查阅。

### 5.1 准备

- [ ] Go ≥ 1.21
- [ ] 分支 `feat/go-mod`
- [ ] 记录迁移前 7 服务编译 baseline

### 5.2 初始化

- [ ] `cd src/golang && go mod init beehive-im`
- [ ] 批量替换 internal import（约 70 文件）
- [ ] Thrift import 迁移或 `replace`
- [ ] `go mod tidy` + pin 关键版本（beego v1.12.3、mgo v2、redigo、protobuf…）

### 5.3 Makefile

- [ ] 根 `Makefile` golang 段改为从 `src/golang` 执行 `go build -o ../../bin/... ./exec/...`
- [ ] 移除 `func_gopath_setup`（Go 部分）

### 5.4 验证与清理

- [ ] 7 服务 build + smoke test
- [ ] 删 vendor、gopath 软链
- [ ] 更新 `doc/ARCHITECTURE.md` 构建说明

---

## 6. 已知阻塞项速查

| 项 | 状态 | 处理阶段 |
|----|------|----------|
| C 第三方库 `3rd/install` | ✅ 已解决 | — |
| `libutils.a` 缺失 | ❌ | 第一阶段 C1 |
| `libchat` → cctrl 依赖 | ❌ | 第一阶段 C2 |
| macOS clang `-Werror` flags | ❌ | 第一阶段 C3 |
| Go vendor 不完整 | ❌ | 第一阶段 G1 |
| 无 docker-compose | ❌ | 第一阶段 D1 |
| `ROOM-CREAT` 未实现 | ⚠️ 绕过 | 第一阶段 P1 种子数据 |
| beego/mgo 老旧 | ⚠️ 可跑 | Task 2 延后 |
| gin / mongo-driver | — | Task 2 延后 |

---

## 7. 建议执行顺序（给协作/AI 用）

1. **C1 → C2 → C3 → C5**（C 能编过）
2. **G1 → G2**（Go 能编过）
3. **D1 → D2 → D3**（中间件）
4. **P1 → P4**（种子数据）
5. **S1 → S4**（启动脚本 + smoke）
6. **M1 → E1～E5**（demo + 联调）
7. **H1 → H2**（缺口补齐）
8. 第三阶段 / Task 2 按产品排期

---

## 8. 工作量粗估

| 块 | 人日 |
|----|------|
| 第一阶段（编译+compose+启动+种子） | 3～5 |
| 第二阶段（demo+联调+HTTP/错误） | 2～4 |
| 第三阶段（按功能项） | 每项 1～5 |
| Task 2（mod+gin+mongo） | 5～10（整体后置） |
