const CMD = {
  ONLINE: 0x0101,
  ONLINE_ACK: 0x0102,
  GROUP_CREAT: 0x0301,
  GROUP_CREAT_ACK: 0x0302,
  GROUP_DISMISS: 0x0303,
  GROUP_DISMISS_ACK: 0x0304,
  GROUP_JOIN: 0x0305,
  GROUP_JOIN_ACK: 0x0306,
  GROUP_QUIT: 0x0307,
  GROUP_QUIT_ACK: 0x0308,
  GROUP_INVITE: 0x0309,
  GROUP_INVITE_ACK: 0x030a,
  GROUP_CHAT: 0x030b,
  GROUP_CHAT_ACK: 0x030c,
  GROUP_JOIN_NTF: 0x0350,
  GROUP_QUIT_NTF: 0x0352,
};

const HEAD_SIZE = 52;

const state = {
  uid: 100001,
  sid: 0,
  token: "",
  wsURL: "",
  gid: 0,
  inGroup: false,
  seq: 1,
  ws: null,
};

const $ = (id) => document.getElementById(id);

function packMsg(cmd, sid, seq, body) {
  const buf = new ArrayBuffer(HEAD_SIZE + body.length);
  const view = new DataView(buf);
  view.setUint32(0, cmd, false);
  view.setUint32(4, body.length, false);
  setUint64(view, 8, sid);
  setUint64(view, 28, seq);
  new Uint8Array(buf, HEAD_SIZE).set(body);
  return buf;
}

function setUint64(view, offset, value) {
  const hi = Math.floor(value / 0x100000000);
  const lo = value >>> 0;
  view.setUint32(offset, hi, false);
  view.setUint32(offset + 4, lo, false);
}

function getUint64(view, offset) {
  const hi = view.getUint32(offset, false);
  const lo = view.getUint32(offset + 4, false);
  return hi * 0x100000000 + lo;
}

function parseMsg(buf) {
  const view = new DataView(buf);
  return {
    cmd: view.getUint32(0, false),
    sid: getUint64(view, 8),
    body: new Uint8Array(buf, HEAD_SIZE),
  };
}

function log(line, cls) {
  const el = document.createElement("div");
  el.className = "msg" + (cls ? " " + cls : "");
  el.textContent = `[${new Date().toLocaleTimeString()}] ${line}`;
  $("log").prepend(el);
}

function setStatus(text, ok) {
  $("status").textContent = text;
  $("status").className = ok ? "ok" : "err";
}

function setChatEnabled(on) {
  $("chatBox").disabled = !on;
  $("sendBtn").disabled = !on;
  state.inGroup = on;
}

function parseGidFromOk(errmsg) {
  if (errmsg.startsWith("Ok:")) {
    return Number(errmsg.slice(3));
  }
  return 0;
}

function updateGidField(gid) {
  state.gid = gid;
  $("gid").value = gid ? String(gid) : "";
}

async function register() {
  state.uid = Number($("uid").value);
  await DemoCommon.registerUser(state, log);
}

async function iplist() {
  await DemoCommon.fetchIplist(state, log);
}

function connectWs() {
  return new Promise((resolve, reject) => {
    if (state.ws) state.ws.close();
    const ws = new WebSocket(state.wsURL);
    ws.binaryType = "arraybuffer";
    state.ws = ws;
    ws.onopen = () => {
      setStatus("WS connected", true);
      log("websocket connected");
      resolve();
    };
    ws.onerror = () => reject(new Error("websocket error"));
    ws.onclose = () => {
      setStatus("WS closed", false);
      setChatEnabled(false);
    };
    ws.onmessage = (ev) => onMessage(ev.data);
  });
}

function send(cmd, bodyBytes) {
  if (!state.ws || state.ws.readyState !== WebSocket.OPEN) {
    throw new Error("websocket not connected");
  }
  const body = bodyBytes || new Uint8Array(0);
  state.ws.send(packMsg(cmd, state.sid, state.seq++, body));
}

function onMessage(buf) {
  const msg = parseMsg(buf);
  switch (msg.cmd) {
    case CMD.ONLINE_ACK: {
      const ack = PB.decodeOnlineAck(msg.body);
      if (ack.code !== 0) {
        log(`ONLINE-ACK error: ${ack.errmsg}`, "err");
        return;
      }
      state.seq = Number(ack.seq) + 1;
      log("ONLINE-ACK ok，可进行建群/加群");
      $("creatBtn").disabled = false;
      $("joinBtn").disabled = false;
      $("inviteBtn").disabled = false;
      break;
    }
    case CMD.GROUP_CREAT_ACK: {
      const ack = PB.decodeSimpleAck(msg.body);
      if (ack.code !== 0) {
        log(`CREAT-ACK error: ${ack.errmsg}`, "err");
        return;
      }
      const gid = parseGidFromOk(ack.errmsg);
      updateGidField(gid);
      log(`建群成功 gid=${gid}`, "sys");
      setChatEnabled(true);
      break;
    }
    case CMD.GROUP_JOIN_ACK: {
      const ack = PB.decodeSimpleAck(msg.body);
      if (ack.code !== 0) {
        log(`JOIN-ACK error: ${ack.errmsg}`, "err");
        return;
      }
      log(`加群成功 gid=${state.gid}`, "sys");
      setChatEnabled(true);
      break;
    }
    case CMD.GROUP_INVITE_ACK: {
      const ack = PB.decodeSimpleAck(msg.body);
      if (ack.code !== 0) {
        log(`INVITE-ACK error: ${ack.errmsg}`, "err");
        return;
      }
      log(`邀请成功 uid=${$("inviteUid").value}`, "sys");
      break;
    }
    case CMD.GROUP_CHAT: {
      const chat = PB.decodeGroupChat(msg.body);
      log(`[uid ${chat.uid}] ${chat.text}`);
      break;
    }
    case CMD.GROUP_CHAT_ACK: {
      const ack = PB.decodeSimpleAck(msg.body);
      if (ack.code !== 0) {
        log(`CHAT-ACK error: ${ack.errmsg}`, "err");
      }
      break;
    }
    case CMD.GROUP_JOIN_NTF: {
      const ntf = PB.decodeGroupNtf(msg.body);
      log(`系统：uid ${ntf.uid} 加入了群 ${ntf.gid}`, "sys");
      break;
    }
    case CMD.GROUP_QUIT_NTF: {
      const ntf = PB.decodeGroupNtf(msg.body);
      log(`系统：uid ${ntf.uid} 退出了群 ${ntf.gid}`, "sys");
      break;
    }
    case CMD.GROUP_DISMISS_ACK: {
      const ack = PB.decodeSimpleAck(msg.body);
      if (ack.code !== 0) {
        log(`DISMISS-ACK error: ${ack.errmsg}`, "err");
        return;
      }
      log("群已解散", "sys");
      updateGidField(0);
      setChatEnabled(false);
      break;
    }
    case CMD.GROUP_QUIT_ACK: {
      const ack = PB.decodeSimpleAck(msg.body);
      if (ack.code !== 0) {
        log(`QUIT-ACK error: ${ack.errmsg}`, "err");
        return;
      }
      log("已退群", "sys");
      setChatEnabled(false);
      break;
    }
    default:
      log(`recv cmd=0x${msg.cmd.toString(16)}`, "sys");
  }
}

async function goOnline() {
  setChatEnabled(false);
  $("creatBtn").disabled = true;
  $("joinBtn").disabled = true;
  $("inviteBtn").disabled = true;
  await register();
  await iplist();
  await connectWs();
  send(
    CMD.ONLINE,
    PB.encodeOnline({
      uid: state.uid,
      sid: state.sid,
      token: state.token,
      app: "beehive-demo",
      version: "1.0",
      terminal: 1,
    })
  );
}

function createGroup() {
  const name = $("groupName").value.trim() || "demo-group";
  const desc = $("groupDesc").value.trim() || "beehive demo";
  send(
    CMD.GROUP_CREAT,
    PB.encodeGroupCreat({ uid: state.uid, name, desc })
  );
  log(`发送 GROUP-CREAT name=${name}`);
}

function joinGroup() {
  const gid = Number($("gid").value);
  if (!gid) {
    log("请先填写 GID（建群窗口会显示）", "err");
    return;
  }
  updateGidField(gid);
  send(CMD.GROUP_JOIN, PB.encodeGroupJoin({ uid: state.uid, gid }));
  log(`发送 GROUP-JOIN gid=${gid}`);
}

function inviteUser() {
  const gid = Number($("gid").value) || state.gid;
  const to = Number($("inviteUid").value);
  if (!gid || !to) {
    log("需要 gid 和 invite uid", "err");
    return;
  }
  updateGidField(gid);
  send(CMD.GROUP_INVITE, PB.encodeGroupInvite({ uid: state.uid, gid, to }));
  log(`发送 GROUP-INVITE gid=${gid} to=${to}`);
}

function quitGroup() {
  const gid = Number($("gid").value) || state.gid;
  if (!gid) return;
  send(CMD.GROUP_QUIT, PB.encodeGroupQuit({ uid: state.uid, gid }));
  log(`发送 GROUP-QUIT gid=${gid}`);
}

function dismissGroup() {
  const gid = Number($("gid").value) || state.gid;
  if (!gid) return;
  send(CMD.GROUP_DISMISS, PB.encodeGroupDismiss({ uid: state.uid, gid }));
  log(`发送 GROUP-DISMISS gid=${gid}`);
}

function sendChat() {
  const text = $("chatBox").value.trim();
  const gid = Number($("gid").value) || state.gid;
  if (!text || !state.ws || !gid) return;
  send(
    CMD.GROUP_CHAT,
    PB.encodeGroupChat({
      uid: state.uid,
      gid,
      level: 0,
      time: Math.floor(Date.now() / 1000),
      text,
    })
  );
  $("chatBox").value = "";
}

function bindClick(id, fn) {
  $(id).addEventListener("click", () => {
    try {
      fn();
    } catch (e) {
      setStatus(String(e.message || e), false);
      log(String(e.message || e), "err");
    }
  });
}

bindClick("goBtn", () => {
  goOnline().catch((e) => {
    setStatus(String(e.message || e), false);
    log(String(e.message || e), "err");
  });
});
bindClick("creatBtn", createGroup);
bindClick("joinBtn", joinGroup);
bindClick("inviteBtn", inviteUser);
bindClick("quitBtn", quitGroup);
bindClick("dismissBtn", dismissGroup);
bindClick("sendBtn", sendChat);
$("chatBox").addEventListener("keydown", (e) => {
  if (e.key === "Enter") sendChat();
});

setStatus("idle", false);
setChatEnabled(false);
