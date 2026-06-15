-- beehive-im MySQL 初始化（与 conf/*.xml 中 testdb / root / 111111 一致）

USE testdb;

CREATE TABLE IF NOT EXISTS IM_SID_GEN_TAB(
    type tinyint NOT NULL DEFAULT 0 COMMENT '会话类型',
    sid bigint NOT NULL DEFAULT 1 COMMENT '会话ID',
    PRIMARY KEY(type)
);

CREATE TABLE IF NOT EXISTS IM_SEQ_GEN_TAB(
    id bigint NOT NULL DEFAULT 0 COMMENT '段编号',
    seq bigint NOT NULL DEFAULT 100 COMMENT '序列号最新值',
    PRIMARY KEY(id)
);

CREATE TABLE IF NOT EXISTS CHAT_ROOM_INFO_TAB(
    rid bigint NOT NULL AUTO_INCREMENT COMMENT '房间ID',
    name varchar(64) NOT NULL COMMENT '房间名称',
    type tinyint NOT NULL DEFAULT 0 COMMENT '房间类型',
    level tinyint NOT NULL DEFAULT 0 COMMENT '房间级别',
    owner bigint NOT NULL COMMENT '房主UID',
    status tinyint NOT NULL DEFAULT 0 COMMENT '房间状态',
    image varchar(1024) NOT NULL DEFAULT '' COMMENT '房间封面',
    description varchar(256) NOT NULL DEFAULT '' COMMENT '房间描述',
    create_time bigint NOT NULL DEFAULT 0 COMMENT '创建时间',
    update_time bigint NOT NULL DEFAULT 0 COMMENT '更新时间',
    PRIMARY KEY(rid),
    INDEX(owner)
);

CREATE TABLE IF NOT EXISTS IM_RID_GEN_TAB(
    id tinyint NOT NULL DEFAULT 0 COMMENT '编号',
    rid bigint NOT NULL DEFAULT 1 COMMENT '聊天室ID',
    PRIMARY KEY(id)
);

INSERT INTO IM_SID_GEN_TAB (type, sid) VALUES (0, 1000000), (1, 2000000)
    ON DUPLICATE KEY UPDATE sid=sid;
INSERT INTO IM_SEQ_GEN_TAB (id, seq) VALUES (0, 100)
    ON DUPLICATE KEY UPDATE seq=seq;
INSERT INTO IM_RID_GEN_TAB (id, rid) VALUES (0, 10001)
    ON DUPLICATE KEY UPDATE rid=rid;

INSERT INTO CHAT_ROOM_INFO_TAB
    (rid, name, type, level, owner, status, image, description, create_time, update_time)
VALUES
    (10001, 'demo-room', 0, 0, 100001, 1, '', 'demo chat room', UNIX_TIMESTAMP(), UNIX_TIMESTAMP())
ON DUPLICATE KEY UPDATE name=VALUES(name);
