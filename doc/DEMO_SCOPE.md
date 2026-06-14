# beehive-im Demo 范围（feature_spec_task_02）

## 实现边界

| 模块 | 协议/能力 | 状态 | 负责服务 |
|------|-----------|------|----------|
| 聊天室弹幕 | 0x04xx ROOM-* | ✅ | chatroom |
| 群聊 | 0x03xx GROUP-* | ✅ | usrsvr + msgsvr |
| 推送 BC/P2P | 0x05xx | ✅ | msgsvr + usrsvr HTTP `/im/push` |
| 聊天室建房/解散 | ROOM-CREAT/DISMISS | ✅ | chatroom |
| 多 listend | 双 WS/TCP 接入 | ✅ | `listend-2` / `websocket-2` |
| **敏感词** | 文本过滤 | ❌ **明确不做** | — |

## 推送职责边界

- **ROOM-BC（0x040D）**：聊天室内运营弹幕，走 chatroom HTTP `/room/push` 或协议，带 rid/msgid。
- **BC/P2P（0x05xx）**：全站或按 uid/sid 的透传通知，走 usrsvr HTTP `/im/push?dim=uid|sid|broadcast&kind=bc|p2p`。

## 群聊架构

- **不新增 groupsvr**：**usrsvr** 群生命周期，**msgsvr** `GROUP-CHAT` fan-out。
- 验收：`./scripts/smoke-group.sh`

## 聊天室 task_02 验收脚本

| 脚本 | 覆盖 |
|------|------|
| `./scripts/smoke-test.sh` | 种子房双用户弹幕 + `./scripts/smoke-fail.sh` |
| `./scripts/smoke-fail.sh` | 错误 token / 非法 join / 未进房 chat |
| `./scripts/smoke-room.sh` | CREAT → JOIN → CHAT → DISMISS |
| `./scripts/smoke-push.sh` | HTTP P2P/BC → WS 收包 |
| `./scripts/smoke-multinode.sh` | ws:8002 + ws:8003 同房互通 |
| `./scripts/smoke-group.sh` | 群聊 0x03xx |

## 敏感词（T02-46 / T02-47）

百万在线前必做项；当前 sprint **刻意不实现**。chatroom `roomChatHandler` 中 TODO 保留，不新增 filter 模块。
