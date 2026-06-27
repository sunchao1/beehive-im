#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""从 src/clang/lib/rtmq 与 incl/rtmq 生成 doc/rtmq 带注释阅读文档。"""

from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SRC_LIB = ROOT / "src/clang/lib/rtmq"
SRC_INC = ROOT / "src/clang/incl/rtmq"
OUT = ROOT / "doc/rtmq"

# 模块元数据：序号、标题、源文件、阅读要点
MODULES = [
    {
        "id": "01",
        "slug": "01-architecture",
        "title": "架构总览与线程模型",
        "files": [],
        "is_readme_part": True,
    },
    {
        "id": "02",
        "slug": "02-rtmq_mesg",
        "title": "协议层 rtmq_mesg",
        "files": [("header", SRC_INC / "rtmq_mesg.h")],
        "intro": "RTMQ 自有协议：系统命令 vs 业务扩展消息（flag 区分）。必嗨 IM 业务包走 RTMQ_EXP_MESG + type=CMD_*。",
    },
    {
        "id": "03",
        "slug": "03-rtmq_comm",
        "title": "公共类型 rtmq_comm",
        "files": [("header", SRC_INC / "rtmq_comm.h")],
        "intro": "错误码、收发快照 rtmq_snap_t、worker 结构、Register 回调类型 rtmq_reg_cb_t。",
    },
    {
        "id": "04",
        "slug": "04-rtmq_sub_and_recv_h",
        "title": "订阅模型 rtmq_sub 与 Server 上下文 rtmq_recv.h",
        "files": [
            ("header", SRC_INC / "rtmq_sub.h"),
            ("header", SRC_INC / "rtmq_recv.h"),
        ],
        "intro": "全局 rtmq_cntx_t：队列、线程池、订阅 hash、node→rsvr 映射。对外 API：init/register/publish/async_send。",
    },
    {
        "id": "05",
        "slug": "05-rtmq_proxy_h",
        "title": "Proxy 头文件 rtmq_proxy",
        "files": [
            ("header", SRC_INC / "rtmq_proxy_tsvr.h"),
            ("header", SRC_INC / "rtmq_proxy.h"),
        ],
        "intro": "客户端侧 rtmq_proxy_t：send/work 线程池、reg AVL、sendq/recvq。frwder/Go 进程连 Server 时用 Proxy。",
    },
    {
        "id": "06",
        "slug": "06-rtmq_recv_api",
        "title": "Server 初始化与对外 API",
        "files": [("server", SRC_LIB / "server/rtmq_recv.c")],
        "intro": "rtmq_init/launch/register/publish/async_send 及队列、线程池创建。Server 端大脑。",
        "sections": [
            ("rtmq_init", "rtmq_launch", "rtmq_register", "rtmq_async_send", "rtmq_publish"),
            ("rtmq_creat_connq", "rtmq_creat_recvq", "rtmq_creat_sendq", "rtmq_creat_distq"),
            ("rtmq_creat_recvs", "rtmq_creat_workers", "rtmq_auth_init", "rtmq_pub_group_trav_cb"),
        ],
    },
    {
        "id": "07",
        "slug": "07-rtmq_lsn",
        "title": "Server 监听与连接接入",
        "files": [("server", SRC_LIB / "server/rtmq_lsn.c")],
        "intro": "独立 listen 线程 accept → connq → pipe 唤醒 rsvr 线程 ADD_SCK。",
    },
    {
        "id": "08",
        "slug": "08-rtmq_dist",
        "title": "Server 分发线程 rtmq_dist",
        "files": [("server", SRC_LIB / "server/rtmq_dist.c")],
        "intro": "从 distq 取按 nid 路由的包，投递到对应 rsvr 的 send 路径。async_send 的下游。",
    },
    {
        "id": "09",
        "slug": "09-rtmq_comm_server",
        "title": "Server 订阅与 node 映射 rtmq_comm",
        "files": [("server", SRC_LIB / "server/rtmq_comm.c")],
        "intro": "publish 依赖的 sub hash；link 鉴权；nid→哪个 rsvr 线程。",
    },
    {
        "id": "10",
        "slug": "10-rtmq_rsvr_part1",
        "title": "Server 接收线程 rtmq_rsvr（上）",
        "files": [("server", SRC_LIB / "server/rtmq_rsvr.c")],
        "intro": "rsvr 主循环：select 收发包、系统/扩展消息分发、连接管理。",
        "line_ranges": [(1, 520)],
    },
    {
        "id": "11",
        "slug": "11-rtmq_rsvr_part2",
        "title": "Server 接收线程 rtmq_rsvr（下）",
        "files": [("server", SRC_LIB / "server/rtmq_rsvr.c")],
        "intro": "鉴权、订阅 SUB、keepalive、dist_data 下发。",
        "line_ranges": [(521, 9999)],
    },
    {
        "id": "12",
        "slug": "12-rtmq_worker",
        "title": "Server 工作线程 rtmq_worker",
        "files": [("server", SRC_LIB / "server/rtmq_worker.c")],
        "intro": "从 recvq 取完整包，调 reg 回调（业务 CMD 入口）；与 Go/C 业务进程对接。",
    },
    {
        "id": "13",
        "slug": "13-rtmq_proxy",
        "title": "Proxy 初始化 rtmq_proxy",
        "files": [("proxy", SRC_LIB / "proxy/rtmq_proxy.c")],
        "intro": "proxy_init/launch/reg_add/async_send：业务进程发消息到 Server。",
    },
    {
        "id": "14",
        "slug": "14-rtmq_proxy_tsvr_part1",
        "title": "Proxy 发送线程 rtmq_proxy_tsvr（上）",
        "files": [("proxy", SRC_LIB / "proxy/rtmq_proxy_tsvr.c")],
        "intro": "epoll/select 连 Server、鉴权、订阅、收包。",
        "line_ranges": [(1, 580)],
    },
    {
        "id": "15",
        "slug": "15-rtmq_proxy_tsvr_part2",
        "title": "Proxy 发送线程 rtmq_proxy_tsvr（下）",
        "files": [("proxy", SRC_LIB / "proxy/rtmq_proxy_tsvr.c")],
        "intro": "writev 发送、保活、重连、业务包交给 worker。",
        "line_ranges": [(581, 9999)],
    },
    {
        "id": "16",
        "slug": "16-rtmq_proxy_worker",
        "title": "Proxy 工作线程 rtmq_proxy_worker",
        "files": [("proxy", SRC_LIB / "proxy/rtmq_proxy_worker.c")],
        "intro": "收到 Server 下行包后调 reg 回调 → frwder 默认 handler。",
    },
    {
        "id": "17",
        "slug": "17-rtmq_proxy_go",
        "title": "Go 封装对照 rtmq_proxy.go",
        "files": [("go", ROOT / "src/golang/lib/rtmq/rtmq_proxy.go")],
        "intro": "Go 版 Proxy：与 C 同协议，websocket/usrsvr/msgsvr 实际用的这层。",
        "line_ranges": [(1, 400)],
    },
]

# 各篇在总地图中的位置 + 本篇流程图（mermaid）
MODULE_META: dict[str, dict] = {
    "02-rtmq_mesg": {
        "layer": "协议层",
        "position": "所有 TCP 包的「语法」：报头 + 系统 cmd + 业务 flag",
        "prev": "00-总地图",
        "next": "03-rtmq_comm",
        "flowchart": """flowchart LR
  TCP字节流 --> H["rtmq_header_t<br/>02 本篇"]
  H -->|flag=SYS| SYS["AUTH/SUB/DIST<br/>系统命令"]
  H -->|flag=EXP| EXP["type=CMD_*<br/>必嗨 IM 帧"]""",
    },
    "03-rtmq_comm": {
        "layer": "协议层",
        "position": "Server/Proxy 共用的类型：错误码、snap 缓冲、reg 回调签名",
        "prev": "02-rtmq_mesg",
        "next": "04-rtmq_sub_and_recv_h",
        "flowchart": """flowchart TB
  A["rtmq_reg_cb_t<br/>业务回调签名"] --> B["Server worker 12"]
  A --> C["Proxy worker 16"]
  D["rtmq_snap_t<br/>收发缓冲"] --> E["rsvr/tsvr 拼帧"]""",
    },
    "04-rtmq_sub_and_recv_h": {
        "layer": "Server 数据结构",
        "position": "Server 大脑 rtmq_cntx_t：队列、线程池、sub/reg/auth 表",
        "prev": "03-rtmq_comm",
        "next": "06-rtmq_recv_api",
        "flowchart": """flowchart TB
  CTX["rtmq_cntx_t 04 本篇"]
  CTX --> Q["conn/recv/send/dist 队列"]
  CTX --> T["listen/rsvr/worker/dist 线程"]
  CTX --> S["sub hash + reg AVL + auth"]""",
    },
    "05-rtmq_proxy_h": {
        "layer": "Proxy 数据结构",
        "position": "每个业务进程内的 rtmq_proxy_t：sendq/recvq/reg/sendtp/worktp",
        "prev": "04-rtmq_sub_and_recv_h",
        "next": "13-rtmq_proxy",
        "flowchart": """flowchart TB
  PXY["rtmq_proxy_t 05 本篇"]
  PXY --> SQ["sendq → tsvr 14-15"]
  PXY --> RQ["recvq → worker 16"]
  PXY --> REG["reg AVL<br/>Register type"]""",
    },
    "06-rtmq_recv_api": {
        "layer": "Server · API",
        "position": "Server 对外四接口：init/launch/register/publish/async_send",
        "prev": "04-rtmq_sub_and_recv_h",
        "next": "07-rtmq_lsn",
        "flowchart": """flowchart TB
  INIT["rtmq_init/launch<br/>06"] --> RUN[Server 运行]
  REG["rtmq_register<br/>reg AVL"] --> W12["12 worker"]
  PUB["rtmq_publish<br/>按 type"] --> S09["09 sub"]
  ASYNC["rtmq_async_send<br/>按 nid"] --> D08["08 dist"]""",
    },
    "07-rtmq_lsn": {
        "layer": "Server · 连接",
        "position": "唯一 accept 线程：新 TCP 进 connq，唤醒 rsvr",
        "prev": "06-rtmq_recv_api",
        "next": "10-rtmq_rsvr_part1",
        "flowchart": """flowchart LR
  Client["Proxy TCP"] --> L["07 listen<br/>accept"]
  L --> Q["connq"]
  Q -->|ADD_SCK| R["10 rsvr"]""",
    },
    "08-rtmq_dist": {
        "layer": "Server · 路由",
        "position": "async_send 专用：从 distq 取包，按 nid 投到某个 rsvr",
        "prev": "06-rtmq_recv_api",
        "next": "10-rtmq_rsvr_part1",
        "flowchart": """flowchart LR
  ASYNC["06 async_send<br/>入 distq"] --> D["08 dist"]
  D -->|nid→rsvr| R["10-11 rsvr<br/>send 队列"]""",
    },
    "09-rtmq_comm_server": {
        "layer": "Server · 路由表",
        "position": "SUB 登记 + publish 遍历 + nid 落在哪个 rsvr",
        "prev": "06-rtmq_recv_api",
        "next": "11-rtmq_rsvr_part2",
        "flowchart": """flowchart TB
  SUB["11 SUB 请求"] --> ADD["09 sub_add"]
  ADD --> HASH["sub hash<br/>type→nodes"]
  PUB["06 publish"] --> HASH
  HASH --> SEND["复制发到各连接"]
  NID["node→rsvr map"] --> D08["08 dist 选线程"]""",
    },
    "10-rtmq_rsvr_part1": {
        "layer": "Server · 收发",
        "position": "rsvr 主循环上半：ADD_SCK、select、收包拼帧、入 recvq",
        "prev": "07-rtmq_lsn",
        "next": "11-rtmq_rsvr_part2",
        "flowchart": """flowchart TB
  LOOP["10 rsvr 主循环"]
  LOOP --> A["ADD_SCK<br/>来自 07"]
  LOOP --> B["select 读"]
  B --> C["snap 拼帧"]
  C --> D{完整?}
  D -->|SYS| E["11 系统"]
  D -->|EXP| F["recvq→12"]""",
    },
    "11-rtmq_rsvr_part2": {
        "layer": "Server · 收发",
        "position": "rsvr 下半：AUTH、SUB、keepalive、writev 发出、dist 下行",
        "prev": "10-rtmq_rsvr_part1",
        "next": "12-rtmq_worker",
        "flowchart": """flowchart TB
  AUTH["AUTH_REQ"] --> OK["可 SUB"]
  SUB["SUB_REQ"] --> S09["09 登记"]
  KP["KPALIVE"] --> ALIVE["保活"]
  SEND["send 队列"] --> WIOV["writev→14-15"]""",
    },
    "12-rtmq_worker": {
        "layer": "Server · 业务入口",
        "position": "从 recvq 取完整 IM 帧，查 reg，调业务回调（Server 侧几乎不用，主要在 Proxy worker）",
        "prev": "10-rtmq_rsvr_part1",
        "next": "16-rtmq_proxy_worker",
        "flowchart": """flowchart LR
  Q["recvq"] -->|PROC_REQ| W["12 worker"]
  W --> REG["reg AVL"]
  REG --> CB["proc callback"]""",
    },
    "13-rtmq_proxy": {
        "layer": "Proxy · API",
        "position": "业务进程发消息的入口：reg_add + async_send 入 sendq",
        "prev": "05-rtmq_proxy_h",
        "next": "14-rtmq_proxy_tsvr_part1",
        "flowchart": """flowchart LR
  APP["业务 Register<br/>AsyncSend"] --> P["13 async_send"]
  P --> SQ["sendq[i]"]
  SQ -->|pipe| TS["14-15 tsvr\nTCP 发出"]""",
    },
    "14-rtmq_proxy_tsvr_part1": {
        "layer": "Proxy · 网络",
        "position": "tsvr 上半：连 Server、鉴权、SUB、epoll 收包",
        "prev": "13-rtmq_proxy",
        "next": "15-rtmq_proxy_tsvr_part2",
        "flowchart": """flowchart TB
  TS["14 tsvr 线程"]
  TS --> CONN["TCP connect Server"]
  CONN --> AUTH["link_auth 11 应答"]
  AUTH --> SUB["sub 所有 reg type"]
  SUB --> EP["epoll 收包"]""",
    },
    "15-rtmq_proxy_tsvr_part2": {
        "layer": "Proxy · 网络",
        "position": "tsvr 下半：writev 发送、保活、重连、下行帧交 worker",
        "prev": "14-rtmq_proxy_tsvr_part1",
        "next": "16-rtmq_proxy_worker",
        "flowchart": """flowchart TB
  SQ["sendq 有数据"] --> WIOV["writev 发出"]
  EP["收到完整帧"] --> RQ["recvq"]
  RQ -->|PROC_REQ| PW["16 proxy worker"]
  KP["超时"] --> RECON["reconn 重连"]""",
    },
    "16-rtmq_proxy_worker": {
        "layer": "Proxy · 业务入口",
        "position": "★ 必嗨主路径：Server 推来的 IM 帧 → reg → frwder/Go handler",
        "prev": "15-rtmq_proxy_tsvr_part2",
        "next": "17-rtmq_proxy_go",
        "flowchart": """flowchart LR
  RQ["recvq"] --> W["16 proxy worker"]
  W --> REG["查 reg(type)"]
  REG --> H["frwder/Go handler"]
  H --> CHAIN["publish/async_send<br/>链式转发"]""",
    },
    "17-rtmq_proxy_go": {
        "layer": "Go 对照",
        "position": "websocket/usrsvr/msgsvr 实际链接层，与 13-16 同角色",
        "prev": "16-rtmq_proxy_worker",
        "next": "00-总地图 回放",
        "flowchart": """flowchart LR
  GO["Go rtmq_proxy.go<br/>17"] -.->|同协议| C["C proxy 13-16"]
  GO --> REG["Register(cmd)"]
  GO --> SEND["AsyncSend→Server"]
  REG --> WH["websocket\nupmesg/mesg"]""",
    },
}

# 关键函数的中文阅读注释（插入在函数块前）
FUNC_NOTES: dict[str, str] = {
    "rtmq_init": "【Server 启动前】分配 rtmq_cntx_t，依次建 auth 表、node→rsvr 映射、订阅 hash、reg AVL、conn/recv/send/dist 队列与 pipe、recvtp/worktp、listen 端口。失败则 return NULL。",
    "rtmq_launch": "【真正跑起来】recvtp 跑 rtmq_rsvr_routine，worktp 跑 rtmq_worker_routine，另起 listen 线程 + 单个 dist 线程。",
    "rtmq_register": "【业务订阅 cmd】在 Server 侧 reg AVL 插入 (type, callback)。Go/C 业务进程连上并 SUB 后，publish(type) 会调到这些回调。",
    "rtmq_async_send": "【按 nid 单播】包头 nid=dest，入 distq，唤醒 dist 线程。frwder 下行 async_send(forward,nid) 走这条。",
    "rtmq_publish": "【按 type 广播】查 sub hash → 遍历各 gid 组 → 对每个订阅连接复制发送。frwder 上行 publish 走这条。",
    "rtmq_lsn_routine": "【accept 循环】select 监听端口，accept 后按 sid%recv_thd_num 投入 connq，pipe 通知对应 rsvr。",
    "rtmq_lsn_accept": "【新 TCP 连接】非阻塞 fd + connq push + RTMQ_CMD_ADD_SCK。",
    "rtmq_dsvr_routine": "【分发线程】从 distq 取包，按 nid 找目标 rsvr，放入其 send 队列。",
    "rtmq_sub_add": "【客户端 SUB】在全局 sub hash 和 socket 的 sub_list 双向登记 type。",
    "rtmq_rsvr_routine": "【接收线程主循环】处理 ADD_SCK、select 读写、收包拼帧、系统/扩展消息分支。",
    "rtmq_rsvr_exp_mesg_proc": "【业务包】完整 IM 帧交给 recvq，pipe 通知 worker 处理。",
    "rtmq_worker_routine": "【业务回调线程】从 recvq 取包，查 reg AVL 调 proc(type, orig, data, len, param)。",
    "rtmq_proxy_init": "【Proxy 侧】解析 Server IP 列表、建 reg/sendq/recvq、sendtp/worktp。",
    "rtmq_proxy_async_send": "【业务进程发送】组 RTMQ 头+体，ring_push sendq，pipe 唤醒 tsvr 线程 writev 到 Server。",
    "rtmq_proxy_tsvr_routine": "【Proxy 发送线程】连 Server、鉴权、SUB 所有 reg 的 type、epoll 收发包。",
    "rtmq_proxy_worker_routine": "【Proxy 收下行】Server 推来的包调 reg → 即业务/frwder 注册的 handler。",
    "rtmq_link_auth_req": "【建链】Proxy 发 AUTH，携带 gid/usr/passwd，Server rtmq_link_auth_req_hdl 校验。",
    "rtmq_rsvr_link_auth_req_hdl": "【AUTH 服务端】校验 usr/passwd/gid，成功则记录 sck->nid/gid，允许后续 SUB。",
    "rtmq_rsvr_sub_req_hdl": "【SUB 服务端】把 (type,sck) 登记到全局 sub hash + sck->sub_list。",
    "rtmq_rsvr_keepalive_req_hdl": "【心跳】30s 级保活，断线检测。",
    "rtmq_proxy_reg_add": "【Proxy 注册】与 Server rtmq_register 对称；本地 AVL，worker 收包时查。",
    "rtmq_proxy_tsvr_init": "【tsvr 初始化】创建 epoll、连接 Server 的 cmd_sck + data_sck。",
    "rtmq_node_to_svr_map_add": "【负载】记录 nid 落在哪个 rsvr 线程，async_send 路由用。",
    "rtmq_pub_group_trav_cb": "【publish 遍历】对每个订阅 gid 组内的连接复制一份 payload 发送。",
}

READER_HEADER = """\
> **源文件**：`{relpath}`  
> **模块**：{title}  
> **说明**：{intro}  
> **阅读建议**：先看文首「函数索引」，再按函数块阅读；`/* ===== 阅读注释 ===== */` 为导读补充。

"""


def relpath(p: Path) -> str:
    try:
        return p.relative_to(ROOT).as_posix()
    except ValueError:
        return p.as_posix()


def extract_functions(content: str) -> list[tuple[str, int, int]]:
    """粗略按函数名索引（用于目录）。"""
    funcs = []
    for m in re.finditer(
        r"^(?:static\s+)?(?:void\s*\*|void|int|bool|rtmq_\w+\s*\*?)\s*(\w+)\s*\(",
        content,
        re.MULTILINE,
    ):
        name = m.group(1)
        if name in ("if", "while", "for", "switch", "return"):
            continue
        funcs.append((name, m.start(), content.find("\n", m.start())))
    return funcs


def slice_lines(content: str, ranges: list[tuple[int, int]] | None) -> str:
    if not ranges:
        return content
    lines = content.splitlines(keepends=True)
    out = []
    for start, end in ranges:
        out.extend(lines[start - 1 : end])
    return "".join(out)


def annotate_header_mesg(content: str) -> str:
    """为 rtmq_mesg.h 关键行追加行尾阅读注释。"""
    hints = {
        "RTMQ_CMD_AUTH_REQ": "// 【系统】Proxy 连 Server 后第一条：鉴权",
        "RTMQ_CMD_SUB_REQ": "// 【系统】向 Server 声明要收哪些 type（业务 cmd）",
        "RTMQ_CMD_DIST_REQ": "// 【系统】dist 线程：distq 有新包",
        "RTMQ_CMD_PROC_REQ": "// 【系统】通知 worker 从 recvq 取包处理",
        "RTMQ_CMD_SEND": "// 【系统】通知 rsvr 发送 send 队列",
        "rtmq_header_t": "// 【报头】每条 RTMQ 消息前有此头；后跟 length 字节 payload",
        "RTMQ_EXP_MESG": "// 【1】业务自定义 type，必嗨 IM 的 CMD_* 走这个",
        "RTMQ_SYS_MESG": "// 【0】type 为 rtmq_mesg_e 系统命令",
        "head->nid": "// 上行=源 NID；下行=目的 NID（async_send 路由键）",
    }
    out = []
    for line in content.splitlines(keepends=True):
        stripped = line.rstrip("\n")
        added = False
        for key, hint in hints.items():
            if key in stripped and "// 【" not in stripped:
                stripped = stripped + "  " + hint
                added = True
                break
        out.append(stripped + ("\n" if line.endswith("\n") else ""))
    return "".join(out)


def annotate_content(content: str, relpath_str: str) -> str:
    if relpath_str.endswith("rtmq_mesg.h"):
        content = annotate_header_mesg(content)

    # 匹配函数定义行（含返回类型前缀）
    func_pat = re.compile(
        r"(^|\n)((?:static\s+)?(?:[\w\s\*]+?\s+)(rtmq_[a-z0-9_]+|rtmq_proxy_[a-z0-9_]+)\s*\()",
        re.MULTILINE,
    )
    out: list[str] = []
    last = 0
    for m in func_pat.finditer(content):
        name = m.group(3)
        insert_at = m.start(2)
        out.append(content[last:insert_at])
        note = FUNC_NOTES.get(name)
        if note:
            out.append(f"/* ===== 阅读注释：{name} =====\n * {note}\n */\n")
        last = insert_at
    out.append(content[last:])
    return "".join(out)


def render_you_are_here(mod: dict) -> str:
    slug = mod.get("slug", "")
    meta = MODULE_META.get(slug)
    if not meta:
        return ""
    parts = [
        f"\n## 你在这里（总地图定位）\n\n",
        f"> **层级**：{meta['layer']}  \n",
        f"> **在 RTMQ 中的位置**：{meta['position']}  \n",
        f"> **总地图**：[00-总地图.md](00-总地图.md) · 上一篇 `{meta.get('prev','')}` · 下一篇 `{meta.get('next','')}`\n\n",
        "```mermaid\n",
        meta.get("flowchart", ""),
        "\n```\n\n",
    ]
    return "".join(parts)


def render_module_flowchart_footer(mod: dict) -> str:
    slug = mod.get("slug", "")
    meta = MODULE_META.get(slug)
    if not meta or not meta.get("flowchart"):
        return ""
    return (
        f"\n---\n\n## 本篇流程图\n\n"
        f"（与文首「你在这里」相同，便于从目录跳转）\n\n"
        f"```mermaid\n{meta['flowchart']}\n```\n\n"
        f"**回到总地图**：[00-总地图.md](00-总地图.md#2-十七篇文档在代码里的位置总组件图)\n"
    )


def render_module(mod: dict) -> str:
    if mod.get("is_readme_part"):
        return ""

    parts = [
        f"# {mod['id']} · {mod['title']}\n\n",
        READER_HEADER.format(
            relpath=", ".join(relpath(f[1]) for f in mod["files"]),
            title=mod["title"],
            intro=mod.get("intro", ""),
        ),
        render_you_are_here(mod),
    ]

    for kind, path in mod["files"]:
        if not path.exists():
            parts.append(f"\n> 源文件不存在：{path}\n")
            continue
        raw = path.read_text(encoding="utf-8", errors="replace")
        body = slice_lines(raw, mod.get("line_ranges"))
        rel = relpath(path)
        funcs = extract_functions(body)
        if funcs:
            parts.append("\n## 函数索引\n\n| 函数 | 约略位置 |\n|------|----------|\n")
            line_no = 1
            for name, _, _ in funcs[:40]:
                parts.append(f"| `{name}` | 见下方代码块 |\n")
            if len(funcs) > 40:
                parts.append(f"| … | 共 {len(funcs)} 个 |\n")

        parts.append(f"\n## 源码（带阅读注释）\n\n```c\n")
        parts.append(f"// 文件: {rel}\n")
        if mod.get("line_ranges"):
            parts.append(f"// 片段: 行 {mod['line_ranges']}\n")
        parts.append(annotate_content(body, rel))
        parts.append("\n```\n")

        if mod.get("sections"):
            parts.append("\n## 本节重点函数\n\n")
            for sec in mod["sections"]:
                if isinstance(sec, tuple):
                    parts.append("- " + " → ".join(f"`{x}`" for x in sec) + "\n")

    parts.append(render_module_flowchart_footer(mod))
    return "".join(parts)


def write_architecture_readme():
    readme = OUT / "README.md"
    arch = OUT / "01-architecture.md"
    arch.write_text(
        """# 01 · RTMQ 架构与数据流

> **完整流程图与十七篇定位** → 请先读 [00-总地图.md](00-总地图.md)

## 1. 在必嗨 IM 中的位置

```text
[Go/C 业务进程]                    [RTMQ Server 中心]
  rtmq_proxy (客户端库)  ←TCP→   rtmq_recv / rsvr / worker
       ↑                                ↑
   frwder / usrsvr / msgsvr / websocket  单进程或多线程
```

- **28889 BACKEND**：业务进程 Proxy 连 Server，`publish` 收上行。
- **28888 FORWARD**：接入进程 Proxy 连 Server，`async_send(nid)` 收下行。

frwder 本身 **不实现 RTMQ**，只是 **Proxy 里 reg 的回调** 做 `publish` / `async_send`。

---

## 2. 一包数据的两种走法

### 上行（publish）

```text
接入 Proxy.async_send → Server → 业务 Proxy.recv → worker → reg(type)
```

### 下行（async_send + nid）

```text
msgsvr Proxy.async_send(type, nid, frame) → dist → 目标 rsvr → 接入 Proxy → websocket
```

---

## 3. Server / Proxy 线程对照

| Server | 文件 | Proxy | 文件 |
|--------|------|-------|------|
| listen | rtmq_lsn.c | tsvr | rtmq_proxy_tsvr.c |
| rsvr | rtmq_rsvr.c | worker | rtmq_proxy_worker.c |
| worker | rtmq_worker.c | — | — |
| dist | rtmq_dist.c | — | — |

下一篇：[00-总地图.md](00-总地图.md) → [02-rtmq_mesg.md](02-rtmq_mesg.md)
""",
        encoding="utf-8",
    )

    readme.write_text(
        """# RTMQ 源码阅读指南

> **先读总地图** → [00-总地图.md](00-总地图.md)（十七篇在整体中的位置 + 两条业务主线）  
> **源码位置**：`src/clang/lib/rtmq/`（约 5824 行 .c）+ `src/clang/incl/rtmq/`（约 673 行 .h）  
> **Go 对照**：`src/golang/lib/rtmq/rtmq_proxy.go`  
> **生成方式**：`python3 scripts/gen-rtmq-docs.py`（改源码后可重新生成）

---

## RTMQ 是什么

**RTMQ（Real-Time Message Queue）** 是必嗨自研的 **进程间 TCP 消息总线**：

- **Server**（`rtmq_recv` 等）：中心节点，维护连接、订阅表、按 **cmd/type** 和 **nid** 路由。
- **Proxy**（`rtmq_proxy` 等）：各业务/接入进程内的客户端库，连 Server 发收消息。
- **与 Kafka 区别**：低延迟、内存队列、**不持久化**；适合 IM 进程间转发，不是日志型 MQ。

必嗨 IM 里：**frwder / usrsvr / msgsvr / websocket** 各连一个 Proxy，Server 通常与 **frwder 同机** 或由独立 rtmq 进程承载（见 `conf/templates/frwder.xml` 28888/28889）。

---

## 两条核心 API（面试必背）

| API | 方向 | 语义 | frwder 对应 |
|-----|------|------|-------------|
| **publish(type, data)** | 广播 | 所有 SUB 了该 type 的连接都收到 | 上行：接入→业务 |
| **async_send(type, nid, data)** | 单播 | 只发给订阅了 type 且 nid 匹配的连接 | 下行：业务→指定接入 NID |

业务 IM 帧在 RTMQ 里包一层头：`flag=RTMQ_EXP_MESG`，`type=CMD_*`（与 `comm/mesg.go` 一致）。

---

## 线程模型（Server 端）

```text
[listen 线程]  accept → connq[idx]
       ↓ pipe RTMQ_CMD_ADD_SCK
[rsvr 线程 × N]  select 收发包 / 鉴权 / SUB / 拼帧
       ↓ 完整业务包 → recvq
[worker 线程 × M]  reg 回调 → 业务进程逻辑
       ↑
[dist 线程 × 1]  distq → 按 nid 投递到某个 rsvr 的发送队列
```

**Proxy 端**：`tsvr 线程`（连 Server、writev 发送）+ `worker 线程`（收下行、调 reg）。

---

## 阅读顺序（推荐）

1. **[00 总地图](00-总地图.md)** ← 建立脑海地图  
2. 按顺序 02 → 04 → 06 → 07 → 12 → 13 → 16 → 17  
3. 每读完一篇，回总地图 **§2 点亮对应编号**

| 顺序 | 文档 | 内容 |
|------|------|------|
| 0 | [00-总地图](00-总地图.md) | 整体流程图 + 两篇专属序列图 |
| 1 | [02-rtmq_mesg.md](02-rtmq_mesg.md) | 报头、系统 cmd、SUB/AUTH |
| 3 | [06-rtmq_recv_api.md](06-rtmq_recv_api.md) | init/publish/async_send |
| 4 | [07-rtmq_lsn.md](07-rtmq_lsn.md) | accept 流程 |
| 5 | [12-rtmq_worker.md](12-rtmq_worker.md) | 业务回调 |
| 6 | [13-rtmq_proxy.md](13-rtmq_proxy.md) | async_send 发出 |
| 7 | [16-rtmq_proxy_worker.md](16-rtmq_proxy_worker.md) | 下行进 frwder |
| 8 | [10/11 rsvr](10-rtmq_rsvr_part1.md) | 收包细节（可选深读） |
| 9 | [17-rtmq_proxy_go.md](17-rtmq_proxy_go.md) | 与 C 对照 |

---

## 文档目录

- **[00 总地图（必读）](00-总地图.md)**

- [01 架构与数据流](01-architecture.md)

""",
        encoding="utf-8",
    )

    lines = [readme.read_text(encoding="utf-8")]
    for mod in MODULES:
        if mod.get("is_readme_part"):
            continue
        fname = f"{mod['slug']}.md"
        path = OUT / fname
        body = render_module(mod)
        path.write_text(body, encoding="utf-8")
        lines.append(f"- [{mod['id']} {mod['title']}]({fname})\n")

    lines.append(
        """
---

## 与 core 库关系

RTMQ 依赖 `libcore`：`thread_pool`、`mref`、`queue`、`ring`、`avl_tree`、`pipe`、`wiov` 等。见 [两周投岗阅读清单](../两周投岗阅读清单.md) 附录 A。

---

## 重新生成

```bash
python3 scripts/gen-rtmq-docs.py
```
"""
    )
    readme.write_text("".join(lines), encoding="utf-8")


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    write_architecture_readme()
    print(f"Generated docs under {OUT}")


if __name__ == "__main__":
    main()
