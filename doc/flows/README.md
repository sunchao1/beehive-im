# 流程图子目录索引

主文档（**推荐从这里读**）：

→ **[../FLOWS_AND_GLOSSARY.md](../FLOWS_AND_GLOSSARY.md)** — 项目全景、端到端流程、各子系统流程图、术语表

---

## 按主题跳转

| 主题 | 位置 |
|------|------|
| 整体拓扑 | [FLOWS §1](../FLOWS_AND_GLOSSARY.md#1-项目全景) |
| 注册/上线 | [FLOWS §2.1](../FLOWS_AND_GLOSSARY.md#21-连接与会话所有能力的前置) |
| 聊天室弹幕 | [FLOWS §2.2](../FLOWS_AND_GLOSSARY.md#22-聊天室进房--弹幕room-chat) |
| 系统公告 push → 面板 | [PUSH_TO_PANEL_FLOW §3](../assets/PUSH_TO_PANEL_FLOW.md#3-路径-b运营-http-push--在线用户面板) |
| 弹幕 → 面板（详细） | [PUSH_TO_PANEL_FLOW §2](../assets/PUSH_TO_PANEL_FLOW.md#2-路径-a用户发弹幕--别人面板显示) |
| **按接口读代码（含 ↑↓ 方向 + 服务）** | **[弹幕核心接口.md](../弹幕核心接口.md)** · [§0 图例](../弹幕核心接口.md#0-图例方向--服务全文统一) |
| **NID/RID/SID/CID/GID 名词** | **[弹幕系统的名词解释.md](../弹幕系统的名词解释.md)** |
| **分层 Fan-out 寻址（核心设计）** | [名词解释 §5](../弹幕系统的名词解释.md#5-分层-fan-out-寻址从-rid-到-cid) · [ARCHITECTURE §3.3](../ARCHITECTURE.md#33-分层-fan-out-寻址聊天室--弹幕核心设计) · [FLOWS §1.2.1](../FLOWS_AND_GLOSSARY.md#121-分层-fan-out-寻址速览) |
| 私聊 / 群聊 | [FLOWS §2.4–2.5](../FLOWS_AND_GLOSSARY.md#24-私聊p2p-chat) |
| websocket 子系统 | [FLOWS §3.1](../FLOWS_AND_GLOSSARY.md#31-websocketwebsocket-接入层) |
| frwder / RTMQ | [FLOWS §3.3](../FLOWS_AND_GLOSSARY.md#33-frwder--rtmq消息网关) |
| chatroom 子系统 | [FLOWS §3.5](../FLOWS_AND_GLOSSARY.md#35-chatroom聊天室) |
| 术语表 | [FLOWS §4](../FLOWS_AND_GLOSSARY.md#4-术语表) |
| ROOM-CHAT 逐步时序（面试深读） | [assets/ROOM_CHAT_SEQUENCE.md](../assets/ROOM_CHAT_SEQUENCE.md) |
| 一页架构图 | [assets/ARCHITECTURE_DIAGRAM.md](../assets/ARCHITECTURE_DIAGRAM.md) |

---

## Mermaid 图预览说明

文档内流程图使用 **Mermaid**。在 GitHub、VS Code（装 Mermaid 插件）、Cursor、Typora 中可渲染；若纯文本阅读，对照各节 ASCII 说明即可。
