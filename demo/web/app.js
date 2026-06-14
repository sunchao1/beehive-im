const CMD = {
  ONLINE: 0x0101,
  ONLINE_ACK: 0x0102,
  OFFLINE: 0x0103,
  ROOM_JOIN: 0x0405,
  ROOM_JOIN_ACK: 0x0406,
  ROOM_QUIT: 0x0407,
  ROOM_CHAT: 0x040b,
  ROOM_BC: 0x040d,
};

const HEAD_SIZE = 52;
// 通过 serve-demo.sh 代理时与页面同源；直接打开 index.html 时回退到 :8000
const USRSVR =
  window.location.protocol === "file:"
    ? "http://127.0.0.1:8000"
    : window.location.origin;

const state = {
  uid: 100001,
  rid: 10001,
  sid: 0,
  token: "",
  wsURL: "",
  gid: 0,
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
    len: view.getUint32(4, false),
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

async function register() {
  state.uid = Number($("uid").value);
  state.rid = Number($("rid").value);
  const url = `${USRSVR}/im/register?uid=${state.uid}&nation=1&city=1&town=1`;
  const res = await fetch(url);
  const data = await res.json();
  if (data.code !== 0 || !data.sid) throw new Error(data.errmsg || "register failed");
  state.sid = data.sid;
  state.seq = 1;
  log(`register ok uid=${state.uid} sid=${state.sid}`);
}

async function iplist() {
  const q = new URLSearchParams({
    type: "2",
    uid: String(state.uid),
    sid: String(state.sid),
    clientip: "127.0.0.1",
  });
  const res = await fetch(`${USRSVR}/im/iplist?${q}`);
  const data = await res.json();
  if (data.code !== 0 || !data.len) throw new Error(data.errmsg || "iplist empty");
  state.token = data.token;
  state.wsURL = `ws://${data.list[0]}/im`;
  log(`iplist ok ws=${state.wsURL}`);
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
    ws.onclose = () => setStatus("WS closed", false);
    ws.onmessage = (ev) => onMessage(ev.data);
  });
}

function send(cmd, bodyBytes) {
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
      log("ONLINE-ACK ok");
      send(CMD.ROOM_JOIN, PB.encodeRoomJoin({ uid: state.uid, rid: state.rid }));
      break;
    }
    case CMD.ROOM_JOIN_ACK: {
      const ack = PB.decodeRoomJoinAck(msg.body);
      if (ack.code !== 0) {
        log(`JOIN-ACK error: ${ack.errmsg}`, "err");
        return;
      }
      state.gid = ack.gid;
      log(`JOIN-ACK ok rid=${ack.rid} gid=${ack.gid}`, "sys");
      $("chatBox").disabled = false;
      $("sendBtn").disabled = false;
      break;
    }
    case CMD.ROOM_CHAT: {
      const chat = PB.decodeRoomChat(msg.body);
      log(`[${chat.uid}] ${chat.text}`);
      break;
    }
    case CMD.ROOM_BC: {
      // BC body wraps serialized room chat in field 6 (bytes)
      try {
        const f = decodeBc(msg.body);
        if (f.chat) {
          const chat = PB.decodeRoomChat(f.chat);
          log(`[${chat.uid}] ${chat.text}`);
        }
      } catch (e) {
        log("ROOM-BC (parse err)", "err");
      }
      break;
    }
    default:
      log(`recv cmd=0x${msg.cmd.toString(16)}`, "sys");
  }
}

function decodeBc(bytes) {
  let i = 0;
  const out = {};
  while (i < bytes.length) {
    let tag = 0;
    let shift = 0;
    while (true) {
      const b = bytes[i++];
      tag |= (b & 0x7f) << shift;
      if ((b & 0x80) === 0) break;
      shift += 7;
    }
    const field = tag >> 3;
    const wire = tag & 7;
    if (wire === 0) {
      while ((bytes[i++] & 0x80) !== 0) {}
    } else if (wire === 2) {
      let len = 0;
      shift = 0;
      while (true) {
        const b = bytes[i++];
        len |= (b & 0x7f) << shift;
        if ((b & 0x80) === 0) break;
        shift += 7;
      }
      const slice = bytes.slice(i, i + len);
      i += len;
      if (field === 6) out.chat = slice;
    }
  }
  return out;
}

async function goOnline() {
  $("chatBox").disabled = true;
  $("sendBtn").disabled = true;
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

function sendChat() {
  const text = $("chatBox").value.trim();
  if (!text || !state.ws) return;
  send(
    CMD.ROOM_CHAT,
    PB.encodeRoomChat({
      uid: state.uid,
      rid: state.rid,
      gid: state.gid,
      level: 0,
      time: Math.floor(Date.now() / 1000),
      text,
    })
  );
  $("chatBox").value = "";
}

$("goBtn").addEventListener("click", () => {
  goOnline().catch((e) => {
    setStatus(String(e.message || e), false);
    log(String(e.message || e), "err");
  });
});
$("sendBtn").addEventListener("click", sendChat);
$("chatBox").addEventListener("keydown", (e) => {
  if (e.key === "Enter") sendChat();
});

setStatus("idle", false);
