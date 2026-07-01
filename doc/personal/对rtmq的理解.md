我对必嗨的理解：
1、首先，和用户的会话接入只有websocket 组件，把消息发送给各个会话cid ，以及接受用户发送的消息都是由
websocket 来处理， websocket是和用户最近的一百米。
2、frwder 负责 收集用户的操作（所谓上行）请求。
    然后根据cmd 来决定 转发给对应的 handler 来处理，
而handler的cmd的绑定是在服务启动的时候注册的。例如，msg的handler 用户的handler 
而对应的业务handler 会根据请求进行各自的业务处理 。
msgsvr 的 handler 会根据 nid进行 下发消息给 对应的websocket接入节点 ，
websocket 接入节点 会根据参数，进行进程内的fanout ，直接发送给用户cid 对应的socket会话。
