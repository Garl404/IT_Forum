// CALUSE 主应用
(function () {
  const { t, setLang, applyDOM, fillLangSelect, current } = window.I18N;
  const { render, excerpt } = window.MDRenderer;
  const API = window.API;

  const app = document.getElementById("app");
  const authArea = document.getElementById("authArea");

  let categories = [];
  let pendingOTP = null;

  // ---------- 工具 ----------
  function el(html) {
    const div = document.createElement("div");
    div.innerHTML = html.trim();
    return div.firstElementChild;
  }
  function toast(msg, type) {
    const wrap = document.getElementById("toastWrap");
    const node = el(`<div class="toast ${type || ""}"></div>`);
    node.textContent = msg;
    wrap.appendChild(node);
    setTimeout(() => { node.style.opacity = "0"; node.style.transition = "opacity .3s"; }, 2600);
    setTimeout(() => node.remove(), 3000);
  }
  function fmtTime(ts) {
    if (!ts) return "";
    const d = new Date(ts * 1000);
    const now = Date.now();
    const diff = now - d.getTime();
    if (diff < 60000) return t("time") + ": " + Math.floor(diff / 1000) + "s";
    const p = (n) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
  }
  function fmtBytes(b) {
    if (b >= 1 << 30) return (b / (1 << 30)).toFixed(2) + " GB";
    if (b >= 1 << 20) return (b / (1 << 20)).toFixed(1) + " MB";
    if (b >= 1 << 10) return (b / (1 << 10)).toFixed(0) + " KB";
    return b + " B";
  }
  function lvBadge(level, role) {
    if (role === "admin") return `<span class="lv-badge admin">ADMIN</span>`;
    return `<span class="lv-badge ${level >= 10 ? "lv10" : ""}">LV${level}</span>`;
  }
  function statusChip(status) {
    const map = { published: ["published", t("published")], pending_review: ["pending", t("pending_review")], removed: ["removed", t("removed")] };
    const [cls, label] = map[status] || ["pending", status];
    return `<span class="status-chip ${cls}">${label}</span>`;
  }
  function requireLogin() {
    if (!API.getUser()) { toast(t("msg_login_first"), "err"); location.hash = "#/login"; return false; }
    return true;
  }
  function catName(slug) {
    const c = categories.find((x) => x.slug === slug);
    return c ? c.name_key : slug;
  }
  function openModal(html) {
    const mask = document.getElementById("modalMask");
    const box = document.getElementById("modalBox");
    box.innerHTML = html;
    mask.hidden = false;
    I18NApply(box);
    return box;
  }
  function closeModal() {
    document.getElementById("modalMask").hidden = true;
    document.getElementById("modalBox").innerHTML = "";
  }
  function I18NApply(root) {
    (root || document).querySelectorAll("[data-i18n]").forEach((x) => { x.textContent = t(x.dataset.i18n); });
    (root || document).querySelectorAll("[data-i18n-placeholder]").forEach((x) => { x.placeholder = t(x.dataset.i18nPlaceholder); });
  }

  // ---------- 顶栏 ----------
  function renderTopbar() {
    const u = API.getUser();
    if (u) {
      authArea.innerHTML = `
        <a class="btn" href="#/profile" style="padding:0 14px">${escHtml(u.username)} ${lvBadge(u.level, u.role)}</a>
        ${u.role === "admin" ? `<a class="btn btn-primary" href="#/admin" style="padding:0 14px">${t("admin")}</a>` : ""}
        <button class="btn btn-ghost" id="logoutBtn" style="padding:0 14px">${t("logout")}</button>`;
      document.getElementById("logoutBtn").addEventListener("click", () => { API.clearSession(); location.hash = "#/"; location.reload(); });
    } else {
      authArea.innerHTML = `
        <a class="btn" href="#/login" style="padding:0 16px">${t("login")}</a>
        <a class="btn btn-primary" href="#/register" style="padding:0 16px">${t("register")}</a>`;
    }
    // 分类导航
    const catNav = document.getElementById("catNav");
    const hash = location.hash;
    const isHome = hash === "" || hash === "#/" || hash.startsWith("#/sort/");
    const active = (hash.match(/^#\/category\/([\w-]+)/) || [])[1] || "";
    catNav.innerHTML = `<a class="cat-pill ${isHome ? "active" : ""}" href="#/">${t("home")}</a>` +
      categories.map((c) => `<a class="cat-pill ${active === c.slug ? "active" : ""}" href="#/category/${c.slug}">${c.name_key}</a>`).join("");
  }

  function escHtml(s) {
    return String(s ?? "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
  }

  function logoSVG(size) {
    return `<svg class="brand-mark" viewBox="0 0 64 64" aria-hidden="true" style="width:${size}px;height:${size}px">
      <defs><linearGradient id="authGrad" x1="0" y1="0" x2="1" y2="1">
        <stop offset="0" stop-color="#22d3ee"/><stop offset="1" stop-color="#0b7d99"/>
      </linearGradient></defs>
      <rect x="2" y="2" width="60" height="60" rx="17" fill="url(#authGrad)"/>
      <path d="M42 17 A15 15 0 1 0 42 47" stroke="#fff" stroke-width="7" stroke-linecap="round" fill="none"/>
      <rect x="46" y="35" width="9" height="12" rx="2.5" fill="#fff"/>
    </svg>`;
  }

  // ---------- 页面：首页 ----------
  async function viewHome() {
    app.innerHTML = `<div class="main-col"><div class="flex spread">
        <div class="flex"><a class="btn ${location.hash.includes("sort=hot") ? "" : "btn-primary"} btn-sm" href="#/sort/new">${t("sort_new")}</a>
        <a class="btn ${location.hash.includes("sort=hot") ? "btn-primary" : ""} btn-sm" href="#/sort/hot">${t("sort_hot")}</a></div>
        <a class="btn btn-primary" href="#/new">${t("new_post")}</a></div>
        <div id="postList"></div><div class="pager" id="pager"></div></div>
      <div class="side-col" id="sideCol"></div>`;
    renderSidebar();
    await loadPosts(1, "");
  }

  async function renderSidebar() {
    const side = document.getElementById("sideCol");
    side.innerHTML = `<div class="card"><div class="side-title">${t("site_stats")}</div><div id="statsBox"><div class="empty">…</div></div>
      <canvas class="mini-chart" id="statsChart"></canvas></div>`;
    try {
      const s = await API.get("/api/stats");
      document.getElementById("statsBox").innerHTML = `
        <div class="stat-row"><span class="k">${t("visits_today")}</span><span class="v">${s.today_visits}</span></div>
        <div class="stat-row"><span class="k">${t("online")}</span><span class="v">${s.online_now}</span></div>
        <div class="stat-row"><span class="k">${t("total_posts")}</span><span class="v">${s.total_posts}</span></div>
        <div class="stat-row"><span class="k">${t("total_users")}</span><span class="v">${s.total_users}</span></div>
        <div class="stat-row"><span class="k">${t("total_comments")}</span><span class="v">${s.total_comments}</span></div>`;
      if (s.visits_7d) Charts.line(document.getElementById("statsChart"), s.visits_7d);
    } catch (e) { document.getElementById("statsBox").innerHTML = `<div class="empty">-</div>`; }
  }

  async function loadPosts(page, extra) {
    const listEl = document.getElementById("postList") || app.querySelector("#postList");
    if (!listEl) return;
    listEl.innerHTML = `<div class="empty">…</div>`;
    try {
      const qs = new URLSearchParams({ page: String(page) });
      const m = location.hash.match(/category\/([\w-]+)/);
      if (m) qs.set("category", m[1]);
      if (location.hash.includes("sort=hot")) qs.set("sort", "hot");
      if (extra) qs.set("q", extra);
      const data = await API.get("/api/posts?" + qs.toString());
      if (!data.posts.length) { listEl.innerHTML = `<div class="empty">${t("no_posts")}</div>`; return; }
      listEl.innerHTML = data.posts.map(postCard).join("");
      listEl.querySelectorAll(".post-card").forEach((card) => {
        card.addEventListener("click", (e) => {
          if (e.target.closest("a")) return;
          location.hash = "#/post/" + card.dataset.id;
        });
      });
      const pager = document.getElementById("pager");
      if (pager && data.pages > 1) {
        pager.innerHTML = "";
        for (let i = 1; i <= Math.min(data.pages, 8); i++) {
          const a = el(`<a class="btn btn-sm ${i === page ? "btn-primary" : ""}" href="#">${i}</a>`);
          a.addEventListener("click", () => loadPosts(i, extra));
          pager.appendChild(a);
        }
      }
    } catch (e) {
      listEl.innerHTML = `<div class="empty">${e.code === "site_shielded" ? e.message : escHtml(e.message)}</div>`;
    }
  }

  function postCard(p) {
    return `<article class="card post-card" data-id="${p.id}">
      <div class="pc-top"><span class="tag">${escHtml(catName(p.category_slug))}</span>${statusChip(p.status)}</div>
      <h3 class="pc-title"><a href="#/post/${p.id}">${escHtml(p.title)}</a></h3>
      <div class="pc-excerpt">${escHtml(excerpt(p.content, 140))}</div>
      <div class="pc-meta">
        <span class="author">${escHtml(p.username)} ${lvBadge(p.user_level, "")}</span>
        <span class="sep">·</span><span>${fmtTime(p.created_at)}</span>
        <span class="sep">·</span><span>${p.views} ${t("views")}</span>
        <span class="sep">·</span><span>${p.comment_count} ${t("comments")}</span>
      </div></article>`;
  }

  // ---------- 页面：帖子详情 ----------
  async function viewPost(id) {
    app.innerHTML = `<div class="container single" style="grid-template-columns:1fr"><div class="main-col" id="postBox"><div class="empty">…</div></div></div>`;
    const box = document.getElementById("postBox");
    try {
      const { post } = await API.get(`/api/posts/${id}`);
      box.innerHTML = `
        <div class="card post-full">
          <div class="pc-top"><span class="tag">${escHtml(catName(post.category_slug))}</span>${statusChip(post.status)}</div>
          <h1 class="pf-title">${escHtml(post.title)}</h1>
          <div class="pc-meta mb8">
            <a href="#/user/${post.user_id}" class="author">${escHtml(post.username)} ${lvBadge(post.user_level, "")}</a>
            <span class="sep">·</span><span>${fmtTime(post.created_at)}</span>
            <span class="sep">·</span><span>${post.views} ${t("views")}</span>
          </div>
          <div class="post-content">${render(post.content)}</div>
          <div class="flex mt16">
            <button class="btn btn-sm" id="reportBtn">${t("report")}</button>
            ${API.getUser() && API.getUser().id === post.user_id ? `<button class="btn btn-sm" id="editBtn">✎</button><button class="btn btn-sm btn-danger" id="delBtn">${t("delete")}</button>` : ""}
          </div>
        </div>
        <div class="card"><div class="side-title">${t("comments")}</div><div id="commentList"></div>
          <div class="divider"></div>
          <textarea id="commentInput" data-i18n-placeholder="write_comment" placeholder="${t("write_comment")}" rows="3"></textarea>
          <div class="flex mt8"><button class="btn btn-primary" id="commentBtn">${t("submit_comment")}</button></div>
        </div>`;
      document.getElementById("reportBtn").addEventListener("click", () => reportModal("post", post.id));
      const delBtn = document.getElementById("delBtn");
      if (delBtn) delBtn.addEventListener("click", async () => {
        if (!confirm(t("confirm") + "?")) return;
        try { await API.del(`/api/posts/${post.id}`); toast("OK", "ok"); location.hash = "#/"; }
        catch (e) { toast(e.message, "err"); }
      });
      const editBtn = document.getElementById("editBtn");
      if (editBtn) editBtn.addEventListener("click", () => editPostModal(post));
      document.getElementById("commentBtn").addEventListener("click", async () => {
        if (!requireLogin()) return;
        const content = document.getElementById("commentInput").value.trim();
        if (!content) return;
        try {
          const r = await API.post(`/api/posts/${post.id}/comments`, { content });
          toast(r.status === "published" ? "OK" : r.message, r.status === "published" ? "ok" : "");
          loadComments(post.id);
          document.getElementById("commentInput").value = "";
        } catch (e) { toast(e.message, "err"); }
      });
      await loadComments(post.id);
    } catch (e) {
      box.innerHTML = `<div class="card empty">${e.status === 404 ? t("no_posts") : escHtml(e.message)}</div>`;
    }
  }

  async function loadComments(postId) {
    const listEl = document.getElementById("commentList");
    if (!listEl) return;
    try {
      const { comments } = await API.get(`/api/posts/${postId}/comments`);
      if (!comments.length) { listEl.innerHTML = `<div class="empty muted">-</div>`; return; }
      listEl.innerHTML = comments.map((c) => `
        <div class="comment-item">
          <div class="ci-head">
            <a href="#/user/${c.user_id}">${escHtml(c.username)}</a> ${lvBadge(c.user_level, "")}
            <span class="muted">${fmtTime(c.created_at)}</span>
            <button class="btn btn-sm btn-ghost" data-cid="${c.id}" style="margin-left:auto">${t("report")}</button>
          </div>
          <div class="ci-content">${render(c.content)}</div>
        </div>`).join("");
      listEl.querySelectorAll("[data-cid]").forEach((b) => b.addEventListener("click", () => reportModal("comment", b.dataset.cid)));
    } catch (e) { listEl.innerHTML = ""; }
  }

  function reportModal(type, targetId) {
    if (!requireLogin()) return;
    const box = openModal(`
      <h3>${t("report")}</h3>
      <label>${t("report_reason")}</label>
      <textarea id="reportReason" data-i18n-placeholder="report_reason_ph" placeholder="${t("report_reason_ph")}" rows="3"></textarea>
      <div class="flex mt8"><label style="margin:0;display:flex;align-items:center;gap:6px"><input type="checkbox" id="reportAnon" style="width:auto"> ${t("report_anonymous")}</label></div>
      <div class="flex mt16">
        <button class="btn btn-primary" id="reportSubmit">${t("submit_report")}</button>
        <button class="btn" id="reportCancel">${t("cancel")}</button>
      </div>`);
    box.querySelector("#reportCancel").addEventListener("click", closeModal);
    box.querySelector("#reportSubmit").addEventListener("click", async () => {
      const reason = box.querySelector("#reportReason").value.trim();
      const anonymous = box.querySelector("#reportAnon").checked;
      if (!reason) { toast(t("report_reason_ph"), "err"); return; }
      try {
        const path = type === "post" ? `/api/posts/${targetId}/report` : `/api/comments/${targetId}/report`;
        const r = await API.post(path, { reason, anonymous });
        toast(r.ok, "ok"); closeModal();
      } catch (e) { toast(e.message, "err"); }
    });
  }

  // ---------- 页面：发帖 ----------
  function viewNewPost() {
    if (!requireLogin()) return;
    app.innerHTML = `<div class="container single"><div class="card">
      <h3 style="margin-bottom:14px">${t("new_post")}</h3>
      <label>${t("post_category")}</label>
      <select id="postCat">${categories.map((c) => `<option value="${c.id}">${c.name_key}</option>`).join("")}</select>
      <label>${t("post_title")}</label>
      <input type="text" id="postTitle" maxlength="200" data-i18n-placeholder="title_ph" placeholder="${t("title_ph")}">
      <label>${t("post_content")}</label>
      <textarea id="postContent" rows="14" data-i18n-placeholder="content_ph" placeholder="${t("content_ph")}"></textarea>
      <div class="notice-box mt16">
        <h4>⚠ ${t("notice_title")}</h4>
        <ul>
          <li>${t("notice1")}</li>
          <li>${t("notice2")}</li>
          <li>${t("notice3")}</li>
          <li>${t("notice4")}</li>
          <li>${t("notice5")}</li>
        </ul>
      </div>
      <label class="agree-row" id="agreeRow">
        <input type="checkbox" id="agreeCheck">
        <span>${t("notice_agree")}</span>
      </label>
      <div class="flex mt16">
        <button class="btn btn-primary" id="publishBtn">${t("publish")}</button>
        <a class="btn" href="#/">${t("cancel")}</a>
      </div>
      <div class="form-msg" id="publishMsg"></div>
    </div></div>`;
    document.getElementById("publishBtn").addEventListener("click", async () => {
      if (!document.getElementById("agreeCheck").checked) {
        msg("publishMsg", t("notice_required"), "err");
        document.getElementById("agreeRow").classList.add("err");
        return;
      }
      document.getElementById("agreeRow").classList.remove("err");
      const body = {
        category_id: parseInt(document.getElementById("postCat").value, 10),
        title: document.getElementById("postTitle").value.trim(),
        content: document.getElementById("postContent").value.trim(),
      };
      if (!body.title) return msg("publishMsg", t("title_ph"), "err");
      if (!body.content) return msg("publishMsg", t("content_ph"), "err");
      if ([...body.content].length > 65536) return msg("publishMsg", "65536", "err");
      try {
        const r = await API.post("/api/posts", body);
        if (r.status === "published") { toast("OK", "ok"); location.hash = "#/post/" + r.id; }
        else { toast(r.message, ""); location.hash = "#/my"; }
      } catch (e) { msg("publishMsg", e.message, "err"); }
    });
  }

  function editPostModal(post) {
    const box = openModal(`
      <h3>✎ ${t("new_post")}</h3>
      <label>${t("post_title")}</label><input type="text" id="editTitle" maxlength="200" value="${escHtml(post.title)}">
      <label>${t("post_content")}</label><textarea id="editContent" rows="12">${escHtml(post.content)}</textarea>
      <div class="flex mt16">
        <button class="btn btn-primary" id="editSave">${t("confirm")}</button>
        <button class="btn" id="editCancel">${t("cancel")}</button>
      </div>`);
    box.querySelector("#editCancel").addEventListener("click", closeModal);
    box.querySelector("#editSave").addEventListener("click", async () => {
      try {
        const r = await API.put(`/api/posts/${post.id}`, {
          category_id: post.category_id,
          title: box.querySelector("#editTitle").value.trim(),
          content: box.querySelector("#editContent").value.trim(),
        });
        closeModal();
        if (r.status === "published") toast("OK", "ok"); else toast(r.status === "pending_review" ? t("msg_pending_review") : "OK", "");
        location.reload();
      } catch (e) { toast(e.message, "err"); }
    });
  }

  function msg(id, text, cls) {
    const m = document.getElementById(id);
    if (m) { m.textContent = text; m.className = "form-msg " + (cls || ""); }
  }

  // ---------- 页面：登录 / 注册 ----------
  function viewLogin() {
    app.innerHTML = `<div class="auth-wrap"><div class="card auth-card">
      <div class="auth-logo">${logoSVG(46)}</div>
      <h2 class="auth-title">CALUSE</h2><p class="auth-sub">${t("login_btn")}</p>
      <label>${t("username")}</label><input type="text" id="loginUser" autocomplete="username">
      <label>${t("password")}</label><input type="password" id="loginPwd" autocomplete="current-password">
      <div class="mt16"><button class="btn btn-primary" style="width:100%" id="loginBtn">${t("login_btn")}</button></div>
      <div class="form-msg" id="loginMsg"></div>
      <p class="muted mt16" style="text-align:center;font-size:13px"><a href="#/register">${t("go_register")}</a></p>
    </div></div>`;
    document.getElementById("loginBtn").addEventListener("click", doLogin);
    document.getElementById("loginPwd").addEventListener("keydown", (e) => e.key === "Enter" && doLogin());
  }

  async function doLogin() {
    const username = document.getElementById("loginUser").value.trim();
    const password = document.getElementById("loginPwd").value;
    if (!username || !password) return msg("loginMsg", "?", "err");
    try {
      const fp = await API.fingerprint();
      const r = await API.post("/api/auth/login", { username, password, fingerprint: fp });
      if (r.need_otp) {
        pendingOTP = { pending_id: r.pending_id, email: r.email };
        viewOTP();
        return;
      }
      API.setSession(r.token, r.user);
      toast("OK " + r.user.username, "ok");
      location.hash = r.user.role === "admin" ? "#/admin" : "#/";
    } catch (e) { msg("loginMsg", e.message, "err"); }
  }

  function viewOTP() {
    app.innerHTML = `<div class="auth-wrap"><div class="card auth-card">
      <div class="auth-logo">${logoSVG(46)}</div>
      <h2 class="auth-title">${t("otp_title")}</h2>
      <p class="auth-sub">${t("otp_desc")} ${pendingOTP ? escHtml(pendingOTP.email) : ""}</p>
      <label>${t("otp_ph")}</label><input type="text" id="otpCode" inputmode="numeric" maxlength="6" data-i18n-placeholder="otp_ph" placeholder="${t("otp_ph")}">
      <div class="mt16"><button class="btn btn-primary" style="width:100%" id="otpBtn">${t("otp_verify")}</button></div>
      <div class="form-msg" id="otpMsg"></div>
      <p class="muted mt16" style="text-align:center;font-size:13px"><a href="#/login">${t("back")}</a></p>
    </div></div>`;
    document.getElementById("otpBtn").addEventListener("click", async () => {
      const code = document.getElementById("otpCode").value.trim();
      if (!code) return;
      try {
        const fp = await API.fingerprint();
        const r = await API.post("/api/auth/verify-otp", { pending_id: pendingOTP.pending_id, code, fingerprint: fp });
        API.setSession(r.token, r.user);
        toast("OK", "ok"); location.hash = "#/admin";
      } catch (e) { msg("otpMsg", e.message, "err"); }
    });
  }

  function viewRegister() {
    app.innerHTML = `<div class="auth-wrap"><div class="card auth-card">
      <div class="auth-logo">${logoSVG(46)}</div>
      <h2 class="auth-title">CALUSE</h2><p class="auth-sub">${t("register_btn")}</p>
      <label>${t("username")}</label><input type="text" id="regUser" maxlength="24">
      <label>${t("email")}</label><input type="email" id="regEmail" data-i18n-placeholder="email_ph" placeholder="${t("email_ph")}">
      <label>${t("password")}</label><input type="password" id="regPwd" data-i18n-placeholder="password_ph" placeholder="${t("password_ph")}">
      <div class="mt16"><button class="btn btn-primary" style="width:100%" id="regBtn">${t("register_btn")}</button></div>
      <div class="form-msg" id="regMsg"></div>
      <p class="muted mt16" style="text-align:center;font-size:13px"><a href="#/login">${t("go_login")}</a></p>
    </div></div>`;
    document.getElementById("regBtn").addEventListener("click", async () => {
      const body = {
        username: document.getElementById("regUser").value.trim(),
        email: document.getElementById("regEmail").value.trim(),
        password: document.getElementById("regPwd").value,
      };
      if (!body.username || !body.email || !body.password) return msg("regMsg", "?", "err");
      try {
        const r = await API.post("/api/auth/register", body);
        API.setSession(r.token, r.user);
        toast("OK", "ok"); location.hash = "#/";
      } catch (e) { msg("regMsg", e.message, "err"); }
    });
  }

  // ---------- 页面：个人中心 ----------
  async function viewProfile() {
    if (!requireLogin()) return;
    app.innerHTML = `<div class="container single"><div class="card"><div class="empty">…</div></div></div>`;
    try {
      const { user } = await API.get("/api/auth/me");
      API.setSession(API.getToken(), user);
      const quota = user.upload_quota || 0;
      const pct = quota > 0 ? Math.min(100, (user.upload_used_bytes / quota) * 100) : 0;
      app.innerHTML = `<div class="container single">
        <div class="card">
          <div class="profile-head">
            <div class="avatar">${escHtml(user.username[0].toUpperCase())}</div>
            <div>
              <div class="flex"><h2>${escHtml(user.username)}</h2> ${lvBadge(user.level, user.role)}</div>
              <div class="muted" style="font-size:13px">${t("since")} ${fmtTime(user.created_at)}</div>
            </div>
            <div class="grow"></div>
            <button class="btn" id="pwdBtn">${t("change_password")}</button>
          </div>
          <div class="divider"></div>
          <div class="flex spread mb8"><span class="muted" style="font-size:13px">${t("level")} LV${user.level}</span><span class="muted" style="font-size:13px">${t("score")}: ${user.score}</span></div>
          <div class="progress"><div style="width:${Math.min(100, (user.score / 37000) * 100)}%"></div></div>
          <div class="flex spread mt16 mb8">
            <span class="muted" style="font-size:13px">${t("quota")}: ${fmtBytes(quota)}</span>
            <span class="muted" style="font-size:13px">${t("quota_used")}: ${fmtBytes(user.upload_used_bytes)}${user.quota_bonus_bytes ? " (+" + fmtBytes(user.quota_bonus_bytes) + " " + t("compensation_tip").slice(0, 6) + ")" : ""}</span>
          </div>
          <div class="progress amber"><div style="width:${pct}%"></div></div>
          <div class="flex mt16">
            <a class="btn" href="#/uploads">${t("uploads")}</a>
            <a class="btn" href="#/my">${t("my_posts")}</a>
          </div>
        </div>
        <div class="card"><div class="side-title">${t("upload_quota_info")}</div>
          <div class="stat-row"><span class="k">${t("posts_count")}</span><span class="v">${user.post_count}</span></div>
          <div class="stat-row"><span class="k">${t("comments_count")}</span><span class="v">${user.comment_count}</span></div>
        </div></div>`;
      document.getElementById("pwdBtn").addEventListener("click", () => {
        const box = openModal(`<h3>${t("change_pwd")}</h3>
          <label>${t("pwd_old")}</label><input type="password" id="pwdOld">
          <label>${t("pwd_new")}</label><input type="password" id="pwdNew">
          <div class="flex mt16"><button class="btn btn-primary" id="pwdSave">${t("confirm")}</button>
          <button class="btn" id="pwdCancel">${t("cancel")}</button></div>
          <div class="form-msg" id="pwdMsg"></div>`);
        box.querySelector("#pwdCancel").addEventListener("click", closeModal);
        box.querySelector("#pwdSave").addEventListener("click", async () => {
          try {
            await API.post("/api/auth/change-password", {
              old_password: box.querySelector("#pwdOld").value,
              new_password: box.querySelector("#pwdNew").value,
            });
            closeModal(); toast("OK", "ok");
          } catch (e) { msg("pwdMsg", e.message, "err"); }
        });
      });
    } catch (e) { app.innerHTML = `<div class="container single"><div class="card empty">${escHtml(e.message)}</div></div>`; }
  }

  // ---------- 页面：我的帖子 ----------
  async function viewMyPosts() {
    if (!requireLogin()) return;
    app.innerHTML = `<div class="container single"><div class="card"><div class="side-title">${t("my_posts")}</div><div id="myList"><div class="empty">…</div></div></div></div>`;
    try {
      const { posts } = await API.get("/api/my/posts");
      const listEl = document.getElementById("myList");
      if (!posts.length) { listEl.innerHTML = `<div class="empty">${t("no_posts")}</div>`; return; }
      listEl.innerHTML = posts.map((p) => `
        <div class="pc-meta mb8" style="border-bottom:1px dashed var(--hairline);padding:10px 0">
          ${statusChip(p.status)}
          <a href="#/post/${p.id}" style="font-weight:600;margin:0 8px">${escHtml(p.title)}</a>
          <span class="muted">${fmtTime(p.created_at)} · ${p.views} ${t("views")}</span>
        </div>`).join("");
    } catch (e) { document.getElementById("myList").innerHTML = `<div class="empty">${escHtml(e.message)}</div>`; }
  }

  // ---------- 页面：文件管理 ----------
  async function viewUploads() {
    if (!requireLogin()) return;
    app.innerHTML = `<div class="container single">
      <div class="card">
        <div class="side-title">${t("upload")}</div>
        <div class="upload-drop" id="dropZone">${t("upload_desc")}</div>
        <input type="file" id="fileInput" hidden>
        <div class="mt8 flex"><input type="number" id="attachPost" placeholder="${t("attach_to_post")}" min="1" style="max-width:220px"></div>
        <div class="form-msg" id="upMsg"></div>
        <p class="muted mt8" style="font-size:12.5px">${t("upload_quota_info")}</p>
      </div>
      <div class="card"><div class="side-title">${t("uploads")}</div><div id="fileList"><div class="empty">…</div></div></div>
    </div>`;
    const drop = document.getElementById("dropZone");
    const fileInput = document.getElementById("fileInput");
    drop.addEventListener("click", () => fileInput.click());
    drop.addEventListener("dragover", (e) => { e.preventDefault(); drop.classList.add("drag"); });
    drop.addEventListener("dragleave", () => drop.classList.remove("drag"));
    drop.addEventListener("drop", (e) => { e.preventDefault(); drop.classList.remove("drag"); if (e.dataTransfer.files[0]) uploadFile(e.dataTransfer.files[0]); });
    fileInput.addEventListener("change", () => { if (fileInput.files[0]) uploadFile(fileInput.files[0]); });

    async function uploadFile(file) {
      const fd = new FormData();
      fd.append("file", file);
      const pid = document.getElementById("attachPost").value;
      if (pid) fd.append("post_id", pid);
      msg("upMsg", "… 0%", "");
      try {
        const resp = await fetch("/api/uploads", { method: "POST", headers: { Authorization: "Bearer " + API.getToken() }, body: fd });
        const data = await resp.json().catch(() => null);
        if (!resp.ok) throw new Error((data && data.message) || resp.status);
        toast("OK", "ok"); msg("upMsg", "", ""); loadFiles();
      } catch (e) { msg("upMsg", e.message, "err"); }
    }
    async function loadFiles() {
      try {
        const { uploads } = await API.get("/api/uploads");
        const listEl = document.getElementById("fileList");
        if (!uploads.length) { listEl.innerHTML = `<div class="empty">${t("no_files")}</div>`; return; }
        listEl.innerHTML = `<div class="table-wrap"><table class="tbl"><thead><tr>
          <th>${t("file")}</th><th>${t("size")}</th><th>${t("uploaded_at")}</th><th>${t("actions")}</th></tr></thead><tbody>` +
          uploads.map((f) => `<tr>
            <td class="mono">${escHtml(f.filename)}${f.post_id ? ` <span class="tag">#${f.post_id}</span>` : ""}</td>
            <td>${fmtBytes(f.size)}</td><td class="muted">${fmtTime(f.created_at)}</td>
            <td><div class="flex">
              <a class="btn btn-sm" href="/api/uploads/${f.id}/download" download>${t("download")}</a>
              <button class="btn btn-sm btn-danger" data-del="${f.id}">${t("delete")}</button></div></td>
          </tr>`).join("") + "</tbody></table></div>";
        listEl.querySelectorAll("[data-del]").forEach((b) => b.addEventListener("click", async () => {
          if (!confirm(t("confirm") + "?")) return;
          try { await API.del(`/api/uploads/${b.dataset.del}`); loadFiles(); } catch (e) { toast(e.message, "err"); }
        }));
      } catch (e) { document.getElementById("fileList").innerHTML = `<div class="empty">${escHtml(e.message)}</div>`; }
    }
    loadFiles();
  }

  // ---------- 页面：静态内容页 ----------
  function viewPage(slug) {
    const P = window.PAGES || {};
    const lang = window.I18N.current();
    const p = (P[slug] && (P[slug][lang] || P[slug]["en-US"])) || null;
    if (!p) { location.hash = "#/"; return; }
    const blocks = (p.blocks || []).map((b) => {
      if (b.links) {
        return `<div class="flex mt8">` + b.links.map((l) =>
          `<a class="btn" href="${escHtml(l.href)}" target="_blank" rel="noopener noreferrer">${escHtml(l.text)} ↗</a>`).join("") + `</div>`;
      }
      if (b.h) return `<h3>${escHtml(b.h)}</h3>${b.p ? `<p>${escHtml(b.p)}</p>` : ""}`;
      if (b.p) return `<p>${escHtml(b.p)}</p>`;
      return "";
    }).join("");
    app.innerHTML = `<div class="container single"><div class="card page-card">
      <h2 class="page-title">${escHtml(p.title)}</h2>
      <div class="divider"></div>
      <div class="page-body">${blocks}</div>
      <div class="mt16"><a class="btn" href="#/">← ${t("home")}</a></div>
    </div></div>`;
  }

  // ---------- 页面：用户主页 ----------
  async function viewUser(id) {
    app.innerHTML = `<div class="container single"><div class="card"><div class="empty">…</div></div></div>`;
    try {
      const { user } = await API.get(`/api/user/${id}`);
      app.innerHTML = `<div class="container single"><div class="card">
        <div class="profile-head">
          <div class="avatar">${escHtml(user.username[0].toUpperCase())}</div>
          <div class="flex"><h2>${escHtml(user.username)}</h2> ${lvBadge(user.level, user.role)}</div>
        </div>
        <div class="divider"></div>
        <div class="stat-row"><span class="k">${t("since")}</span><span class="v">${fmtTime(user.created_at)}</span></div>
        <div class="stat-row"><span class="k">${t("posts_count")}</span><span class="v">${user.post_count}</span></div>
      </div></div>`;
    } catch (e) { app.innerHTML = `<div class="container single"><div class="card empty">${escHtml(e.message)}</div></div>`; }
  }

  // ---------- 路由 ----------
  async function route() {
    const hash = location.hash || "#/";
    const u = API.getUser();
    closeModal();
    renderTopbar();
    if (u && (u.banned_until === -1 || u.banned_until * 1000 > Date.now()) && hash !== "#/login") {
      app.innerHTML = `<div class="container single"><div class="card empty">${t("banned_tip")}${u.ban_reason ? "：" + escHtml(u.ban_reason) : ""}</div></div>`;
      return;
    }
    let m;
    if ((m = hash.match(/^#\/page\/([\w-]+)/))) return viewPage(m[1]);
    if ((m = hash.match(/^#\/post\/(\d+)/))) return viewPost(m[1]);
    if ((m = hash.match(/^#\/category\/([\w-]+)/))) return viewHome();
    if ((m = hash.match(/^#\/user\/(\d+)/))) return viewUser(m[1]);
    switch (hash) {
      case "#/": case "#/sort/new": case "#/sort/hot": return viewHome();
      case "#/login": return viewLogin();
      case "#/register": return viewRegister();
      case "#/new": return viewNewPost();
      case "#/profile": return viewProfile();
      case "#/my": return viewMyPosts();
      case "#/uploads": return viewUploads();
      case "#/admin": return window.AdminApp ? window.AdminApp.route() : (app.innerHTML = `<div class="container single"><div class="card empty">${t("admin")}</div></div>`);
      default: return viewHome();
    }
  }

  // ---------- 主题 ----------
  function initTheme() {
    const saved = localStorage.getItem("caluse_theme") || "auto";
    document.documentElement.dataset.theme = saved;
    const btn = document.getElementById("themeToggle");
    function updateIcon() {
      const dark = window.matchMedia("(prefers-color-scheme: dark)").matches;
      const mode = document.documentElement.dataset.theme;
      const isDark = mode === "dark" || (mode === "auto" && dark);
      btn.querySelector(".icon-moon").style.display = isDark ? "none" : "";
      btn.querySelector(".icon-sun").style.display = isDark ? "" : "none";
    }
    btn.addEventListener("click", () => {
      const order = ["auto", "light", "dark"];
      const next = order[(order.indexOf(document.documentElement.dataset.theme) + 1) % 3];
      document.documentElement.dataset.theme = next;
      localStorage.setItem("caluse_theme", next);
      updateIcon();
      toast("Theme: " + next, "ok");
    });
    window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", updateIcon);
    updateIcon();
  }

  // ---------- 启动 ----------
  async function boot() {
    fillLangSelect(document.getElementById("langSelect"));
    document.getElementById("langSelect").addEventListener("change", (e) => {
      setLang(e.target.value);
      route();
    });
    window.I18N.applyDOM(); // 初始化静态元素（页脚等）的文案
    initTheme();
    // 搜索
    const searchToggle = document.getElementById("searchToggle");
    const searchPop = document.getElementById("searchPop");
    searchToggle.addEventListener("click", () => { searchPop.hidden = !searchPop.hidden; if (!searchPop.hidden) document.getElementById("searchInput").focus(); });
    document.getElementById("searchGo").addEventListener("click", () => {
      const q = document.getElementById("searchInput").value.trim();
      searchPop.hidden = true;
      if (q) { location.hash = "#/"; loadPosts(1, q); }
    });
    document.getElementById("searchInput").addEventListener("keydown", (e) => {
      if (e.key === "Enter") document.getElementById("searchGo").click();
    });
    document.getElementById("modalMask").addEventListener("click", (e) => { if (e.target === e.currentTarget) closeModal(); });
    try {
      const data = await API.get("/api/categories");
      categories = data.categories || [];
    } catch (e) { categories = []; }
    window.addEventListener("hashchange", route);
    route();
  }

  document.addEventListener("DOMContentLoaded", boot);
  window.App = { route, renderTopbar, categories: () => categories };
})();
