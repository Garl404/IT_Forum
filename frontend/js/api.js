// CALUSE API 封装
window.API = (function () {
  let token = localStorage.getItem("caluse_token") || "";
  let user = null;
  try { user = JSON.parse(localStorage.getItem("caluse_user") || "null"); } catch (e) { user = null; }

  function saveSession(t, u) {
    token = t; user = u;
    if (t) localStorage.setItem("caluse_token", t); else localStorage.removeItem("caluse_token");
    if (u) localStorage.setItem("caluse_user", JSON.stringify(u)); else localStorage.removeItem("caluse_user");
  }

  async function request(method, path, body, isForm) {
    const headers = {};
    if (token) headers["Authorization"] = "Bearer " + token;
    let payload;
    if (isForm) { payload = body; }
    else if (body !== undefined) { headers["Content-Type"] = "application/json"; payload = JSON.stringify(body); }
    const resp = await fetch(path, { method, headers, body: payload });
    let data = null;
    const ct = resp.headers.get("Content-Type") || "";
    if (ct.includes("application/json")) { try { data = await resp.json(); } catch (e) { data = null; } }
    if (!resp.ok) {
      const err = new Error((data && (data.message || data.error)) || ("HTTP " + resp.status));
      err.status = resp.status;
      err.code = data && data.error;
      err.data = data;
      throw err;
    }
    return data;
  }

  const api = {
    get: (p) => request("GET", p),
    post: (p, b) => request("POST", p, b),
    put: (p, b) => request("PUT", p, b),
    del: (p) => request("DELETE", p),
    upload: (p, formData) => request("POST", p, formData, true),
    getToken: () => token,
    getUser: () => user,
    setSession: saveSession,
    clearSession: () => saveSession("", null),
  };

  // 浏览器指纹（用于管理员设备可信校验）
  async function fingerprint() {
    try {
      const parts = [
        navigator.userAgent, navigator.language, screen.width + "x" + screen.height,
        screen.colorDepth, new Date().getTimezoneOffset(), !!navigator.hardwareConcurrency ? navigator.hardwareConcurrency : "",
        navigator.platform || "",
      ];
      let canvasFp = "";
      try {
        const c = document.createElement("canvas");
        c.width = 220; c.height = 60;
        const x = c.getContext("2d");
        x.fillStyle = "#f60"; x.fillRect(0, 0, 220, 60);
        x.fillStyle = "#069"; x.font = "16px sans-serif"; x.fillText("CALUSE fp 2026", 8, 34);
        canvasFp = c.toDataURL().slice(-64);
      } catch (e) { canvasFp = "n/a"; }
      parts.push(canvasFp);
      const buf = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(parts.join("||")));
      return Array.from(new Uint8Array(buf)).map((b) => b.toString(16).padStart(2, "0")).join("").slice(0, 48);
    } catch (e) { return "fp-unavailable"; }
  }

  return { ...api, fingerprint };
})();
