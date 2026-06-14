# Demo Web 客户端

## 三步启动

1. 启动全栈：`./scripts/up-demo.sh`
2. 打开页面：`./scripts/serve-demo.sh`，浏览器访问 http://127.0.0.1:8088/
3. 开 **两个** 浏览器窗口：
   - 窗口 A：uid=`100001`，rid=`10001`，点「注册并连接」
   - 窗口 B：uid=`100002`，rid=`10001`，点「注册并连接」
   - 互发弹幕

## 截图位

<!-- TODO: 双窗口同房弹幕截图 -->

## 协议

- HTTP：`/im/register`、`/im/iplist?type=2`
- WebSocket：`ws://127.0.0.1:8002/im`
- 帧格式：52 字节大端头 + protobuf 体（见 `doc/PROTOCOL.md`）

## 依赖

无 CDN；`pb.js` 为内联最小 protobuf 编解码。
