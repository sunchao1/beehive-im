// Shared helpers for demo/web (register + iplist retry).
(function (global) {
  function usrsvrOrigin() {
    return global.location.protocol === "file:"
      ? "http://127.0.0.1:8000"
      : global.location.origin;
  }

  /**
   * Fetch iplist with retry. websocket/listend register via monitor into Redis;
   * if TTL expired or stack just started, the first calls may fail.
   */
  async function fetchIplist(state, log) {
    const q = new URLSearchParams({
      type: "2",
      uid: String(state.uid),
      sid: String(state.sid),
      clientip: "127.0.0.1",
    });
    const url = `${usrsvrOrigin()}/im/iplist?${q}`;
    const maxTry = 15;
    let lastErr = "iplist empty";

    for (let i = 0; i < maxTry; i++) {
      const res = await fetch(url);
      const data = await res.json();
      if (data.code === 0 && data.len) {
        state.token = data.token;
        state.wsURL = wsURLFromIplistEntry(data.list[0]);
        log(`iplist ok ws=${state.wsURL}`);
        return;
      }
      lastErr = data.errmsg || lastErr;
      if (i + 1 < maxTry) {
        log(`iplist 等待接入点注册 (${i + 1}/${maxTry})…`, "sys");
        await sleep(2000);
      }
    }

    throw new Error(
      `${lastErr}。接入点未注册或已过期，请执行：\n` +
        "  docker compose --profile run restart runner\n" +
        "等待约 20 秒后刷新页面重试。"
    );
  }

  async function registerUser(state, log) {
    const url =
      `${usrsvrOrigin()}/im/register?uid=${state.uid}&nation=1&city=1&town=1`;
    const res = await fetch(url);
    const data = await res.json();
    if (data.code !== 0 || !data.sid) {
      throw new Error(data.errmsg || "register failed");
    }
    state.sid = data.sid;
    state.seq = 1;
    log(`register ok uid=${state.uid} sid=${state.sid}`);
  }

  function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  /** host:port 或完整 ws(s):// URL（K8s Ingress 方案 A） */
  function wsURLFromIplistEntry(entry) {
    const s = String(entry || "").trim();
    if (/^wss?:\/\//i.test(s)) {
      return s;
    }
    return `ws://${s}/im`;
  }

  global.DemoCommon = {
    usrsvrOrigin,
    registerUser,
    fetchIplist,
    wsURLFromIplistEntry,
  };
})(window);
