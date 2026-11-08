// CALUSE 管理后台
window.AdminApp = (function () {
  const { t } = window.I18N;
  const API = window.API;

  const SECTIONS = [
    ["dash", "admin_dash"], ["review", "admin_review"], ["reports", "admin_reports"],
    ["users", "admin_users"], ["emails", "admin_emails"], ["trust", "admin_trust"],
    ["weekly", "admin_weekly"], ["logs", "admin_logs"], ["words", "admin_words"],
  ];

  function escHtml(s) { return String(s ?? "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;"); }
  function fmtTime(ts) { if (!ts) return "-"; const d = new Date(ts * 1000); const p = (n) => String(n).padStart(2, "0"); return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`; }
  function fmtBytes(b) { if (b >= 1 << 30) return (b / (1 << 30)).toFixed(2) + " GB"; if (b >= 1 << 20) return (b / (1 << 20)).toFixed(1) + " MB"; return b + " B"; }
  function toast(msg, type) {
    const wrap = document.getElementById("toastWrap");
    const node = document.createElement("div");
    node.className = "toast " + (type || "");
    node.textContent = msg;
    wrap.appendChild(node);
    setTimeout(() => node.remove(), 3000);
  }

  const app = document.getElementById("app");
  let section = "dash";

  async function route() {
    const m = location.hash.match(/^#\/admin\/(\w+)/);
    section = m ? m[1] : "dash";
    const u = API.getUser();
    if (!u || u.role !== "admin") { app.innerHTML = `<div class="container single"><div class="card empty">403</div></div>`; return; }
    app.innerHTML = `<div class="container admin-layout" style="grid-template-columns:210px 1fr">
      <div class="admin-side glass">
        ${SECTIONS.map(([id, key]) => `<a class="admin-link ${section === id ? "active" : ""}" href="#/admin/${id}">${t(key)}<span class="admin-badge hidden" id="badge-${id}">0</span></a>`).join("")}
        <div class="divider"></div>
        <a class="admin-link" href="#/">← ${t("home")}</a>
      </div>
      <div class="main-col" id="adminContent"><div class="card"><div class="empty">…</div></div></div>
    </div>`;
    const content = document.getElementById("adminContent");
    try {
      switch (section) {
        case "dash": await viewDash(content); break;
        case "review": await viewReview(content); break;
        case "reports": await viewReports(content); break;
        case "users": await viewUsers(content); break;
        case "emails": await viewEmails(content); break;
        case "trust": await viewTrust(content); break;
        case "weekly": await viewWeekly(content); break;
        case "logs": await viewLogs(content); break;
        case "words": await viewWords(content); break;
        default: content.innerHTML = `<div class="card empty">?</div>`;
      }
    } catch (e) {
      content.innerHTML = `<div class="card empty">${escHtml(e.message)}</div>`;
    }
  }

  // ---------- 仪表盘 ----------
  async function viewDash(content) {
    const d = await API.get("/api/admin/dashboard");
    const s = d.stats || {};
    content.innerHTML = `
      <div class="card flex spread">
        <div><h3>${t("admin_dash")}</h3><span class="muted" style="font-size:13px">${d.admin_name}</span></div>
        <button class="btn ${s.shielded ? "btn-danger" : "btn-primary"}" id="shieldBtn">${s.shielded ? t("shield_off") : t("shield_on")}</button>
      </div>
      <div class="stat-cards">
        ${statCard("rps_now", s.rps ?? 0, "accent")}
        ${statCard("rps_baseline", s.baseline_10min ?? 0)}
        ${statCard("visits_today", s.total_today ?? 0)}
        ${statCard("online", s.unique_ip_today ?? 0, "accent")}
        ${statCard("pending_count", d.pending_posts + d.pending_reports, "amber")}
        ${statCard("admin_users", d.total_users, "")}
        ${statCard("total_posts", d.total_posts, "")}
        ${statCard("blocked_ips", s.blocked_ips ?? 0, "danger")}
      </div>
      ${s.shielded ? `<div class="card"><span class="status-chip pending">${t("shield_active")} (${s.shield_by || "auto"})</span></div>` : ""}
      <div class="card"><div class="side-title">${t("week_visits")}</div><canvas id="visitsChart" style="width:100%;height:150px"></canvas></div>
      <div class="card"><div class="side-title">${t("posts")} / 7d</div><canvas id="postsChart" style="width:100%;height:120px"></canvas></div>
      ${s.top_ips && s.top_ips.length ? `<div class="card"><div class="side-title">Top IP</div><div class="table-wrap"><table class="tbl">
        <thead><tr><th>IP</th><th>${t("views")}/min</th></tr></thead><tbody>
        ${s.top_ips.map((x) => `<tr><td class="mono">${escHtml(x.ip)}</td><td>${x.count}</td></tr>`).join("")}
      </tbody></table></div></div>` : ""}`;
    document.getElementById("shieldBtn").addEventListener("click", async () => {
      if (!confirm(t("confirm") + "?")) return;
      try {
        const r = await API.post("/api/admin/shield", { enable: !s.shielded });
        toast(r.shielded ? t("shield_active") : "OK", "ok");
        route();
      } catch (e) { toast(e.message, "err"); }
    });
    if (d.visits_7d) Charts.line(document.getElementById("visitsChart"), d.visits_7d);
    if (d.posts_7d) Charts.bar(document.getElementById("postsChart"), d.posts_7d);
  }

  function statCard(label, value, cls) {
    return `<div class="card stat-card"><div class="sc-label">${t(label)}</div><div class="sc-value ${cls || ""}">${value}</div></div>`;
  }

  // ---------- 内容审核 ----------
  async function viewReview(content) {
    const d = await API.get("/api/admin/review/posts");
    content.innerHTML = `<div class="card"><div class="side-title">${t("admin_review")} (${d.items.length})</div><div id="reviewList"></div></div>`;
    const listEl = document.getElementById("reviewList");
    if (!d.items.length) { listEl.innerHTML = `<div class="empty">-</div>`; return; }
    listEl.innerHTML = d.items.map((p) => `
      <div style="border:1px solid var(--hairline);border-radius:12px;padding:14px;margin-bottom:12px">
        <div class="flex mb8">
          <span class="tag danger">S${p.flag_severity}</span>
          <strong>${escHtml(p.title)}</strong>
          <span class="muted" style="font-size:12.5px">@${escHtml(p.username)} · ${fmtTime(p.created_at)}</span>
        </div>
        <div class="muted mb8" style="font-size:12.5px">${t("detail")}: ${escHtml(p.flag_reason)}</div>
        <div class="post-content mb8" style="max-height:200px;overflow-y:auto;font-size:14px">${window.MDRenderer.render(p.content)}</div>
        <div class="flex">
          <button class="btn btn-primary btn-sm" data-act="approve" data-id="${p.id}">${t("approve")}</button>
          <button class="btn btn-danger btn-sm" data-act="reject" data-id="${p.id}">${t("reject")}</button>
          <input type="text" class="grow" placeholder="${t("review_note")}" data-note="${p.id}" style="max-width:280px">
        </div>
      </div>`).join("");
    listEl.querySelectorAll("[data-act]").forEach((b) => b.addEventListener("click", async () => {
      const id = b.dataset.id;
      const note = listEl.querySelector(`[data-note="${id}"]`).value.trim();
      const act = b.dataset.act;
      if (!confirm(act === "approve" ? t("review_approve_confirm") : t("review_reject_confirm"))) return;
      try {
        const r = await API.post(`/api/admin/review/posts/${id}`, { action: act, note });
        toast(r.ok, "ok"); route();
      } catch (e) { toast(e.message, "err"); }
    }));
  }

  // ---------- 举报处理 ----------
  async function viewReports(content) {
    const d = await API.get("/api/admin/reports");
    content.innerHTML = `<div class="card"><div class="flex spread"><div class="side-title">${t("admin_reports")}</div>
      <div class="flex"><a class="btn btn-sm ${!location.hash.includes("status") ? "btn-primary" : ""}" href="#/admin/reports">${t("all")}</a>
      <a class="btn btn-sm" href="#/admin/reports?status=pending">${t("pending_count")}</a></div></div>
      <div id="reportList"></div></div>`;
    const listEl = document.getElementById("reportList");
    if (!d.reports.length) { listEl.innerHTML = `<div class="empty">-</div>`; return; }
    listEl.innerHTML = `<div class="table-wrap"><table class="tbl"><thead><tr>
      <th>#</th><th>${t("detail")}</th><th>${t("report_reason")}</th><th>${t("status")}</th><th>${t("time")}</th><th>${t("actions")}</th>
    </tr></thead><tbody>` + d.reports.map((r) => {
      const target = r.post_id ? `#/post/${r.post_id}` : "#";
      return `<tr>
        <td>${r.id}</td>
        <td><a href="${target}">${escHtml(r.post_title || ("#" + (r.comment_id || "")))}</a><br><span class="muted" style="font-size:12px">${escHtml(r.reporter || "anonymous")}</span></td>
        <td>${escHtml(r.reason)}</td>
        <td>${r.status}</td>
        <td class="muted">${fmtTime(r.created_at)}</td>
        <td>${r.status === "pending" ? `<div class="flex">
          <button class="btn btn-primary btn-sm" data-appr="${r.id}">${t("approve")}</button>
          <button class="btn btn-danger btn-sm" data-conf="${r.id}">${t("reject")}</button>
        </div>` : escHtml(r.note || "-")}</td>
      </tr>`;
    }).join("") + "</tbody></table></div>";
    listEl.querySelectorAll("[data-appr]").forEach((b) => b.addEventListener("click", () => reviewReport(b.dataset.appr, "approve")));
    listEl.querySelectorAll("[data-conf]").forEach((b) => b.addEventListener("click", () => reviewReport(b.dataset.conf, "confirm")));
  }

  async function reviewReport(id, action) {
    if (!confirm(action === "approve" ? t("confirm_approve") : t("confirm_confirm"))) return;
    const sev = action === "confirm" ? prompt(t("severity") + " (1-5) " + t("severity_tip"), "3") : "0";
    try {
      const r = await API.post(`/api/admin/reports/${id}/review`, { action, severity: parseInt(sev || "1", 10), note: "" });
      toast(r.ok, "ok"); route();
    } catch (e) { toast(e.message, "err"); }
  }

  // ---------- 用户管理 ----------
  async function viewUsers(content) {
    const d = await API.get("/api/admin/users");
    content.innerHTML = `<div class="card"><div class="flex spread"><div class="side-title">${t("admin_users")}</div>
      <input type="text" id="userSearch" placeholder="${t("search")}…" style="max-width:240px"></div>
      <div id="userList"></div></div>`;
    const render = (users) => {
      const listEl = document.getElementById("userList");
      listEl.innerHTML = `<div class="table-wrap"><table class="tbl"><thead><tr>
        <th>ID</th><th>${t("username")}</th><th>${t("level")}</th><th>${t("quota_used")}</th><th>${t("status")}</th><th>${t("actions")}</th>
      </tr></thead><tbody>` + users.map((u) => {
        const banned = u.banned_until === -1 || u.banned_until * 1000 > Date.now();
        return `<tr>
          <td>${u.id}</td>
          <td>${escHtml(u.username)} ${u.role === "admin" ? '<span class="lv-badge admin">ADMIN</span>' : ""}<br><span class="muted" style="font-size:11.5px">${escHtml(u.email || "")}</span></td>
          <td>LV${u.level}</td>
          <td>${fmtBytes(u.upload_used_bytes)}${u.quota_bonus_bytes ? ` <span class="tag amber">+${fmtBytes(u.quota_bonus_bytes)}</span>` : ""}</td>
          <td>${banned ? `<span class="status-chip removed">${t("banned_tip")}</span>` : `<span class="status-chip published">OK</span>`}</td>
          <td><div class="flex">
            ${banned ? `<button class="btn btn-sm" data-unban="${u.id}">${t("unban")}</button>` : `<button class="btn btn-sm btn-danger" data-ban="${u.id}">${t("ban")}</button>`}
            <button class="btn btn-sm" data-level="${u.id}">${t("set_level")}</button>
            <button class="btn btn-sm" data-quota="${u.id}">${t("grant_quota")}</button>
          </div></td>
        </tr>`;
      }).join("") + "</tbody></table></div>";
      listEl.querySelectorAll("[data-unban]").forEach((b) => b.addEventListener("click", async () => {
        await API.post(`/api/admin/users/${b.dataset.unban}`, { action: "unban" }); toast("OK", "ok"); route();
      }));
      listEl.querySelectorAll("[data-ban]").forEach((b) => b.addEventListener("click", async () => {
        const sev = prompt(t("severity") + " (1-5) " + t("severity_tip"), "3");
        const reason = prompt(t("ban_reason_ph"), "");
        await API.post(`/api/admin/users/${b.dataset.ban}`, { action: "ban", severity: parseInt(sev || "1", 10), reason: reason || "-" });
        toast("OK", "ok"); route();
      }));
      listEl.querySelectorAll("[data-level]").forEach((b) => b.addEventListener("click", async () => {
        const lv = prompt("LV (1-10)", "1");
        await API.post(`/api/admin/users/${b.dataset.level}`, { action: "set_level", level: parseInt(lv || "1", 10) });
        toast("OK", "ok"); route();
      }));
      listEl.querySelectorAll("[data-quota]").forEach((b) => b.addEventListener("click", async () => {
        const mb = prompt("MB", "1024");
        await API.post(`/api/admin/users/${b.dataset.quota}`, { action: "grant_quota", quota_mb: parseInt(mb || "0", 10) });
        toast("OK", "ok"); route();
      }));
    };
    render(d.users);
    document.getElementById("userSearch").addEventListener("keydown", async (e) => {
      if (e.key === "Enter") {
        const q = e.target.value.trim();
        const r = q ? await API.get("/api/admin/users?q=" + encodeURIComponent(q)) : d;
        render(r.users);
      }
    });
  }

  // ---------- 告警邮箱 ----------
  async function viewEmails(content) {
    const d = await API.get("/api/admin/emails");
    content.innerHTML = `<div class="card"><div class="side-title">${t("admin_emails")}</div>
      <div id="emailList"></div>
      <div class="divider"></div>
      <div class="flex">
        <input type="email" id="newEmail" data-i18n-placeholder="email" placeholder="${t("email")}" class="grow">
        <input type="text" id="newEmailNote" data-i18n-placeholder="email_note" placeholder="${t("email_note")}" style="max-width:200px">
        <button class="btn btn-primary" id="addEmailBtn">${t("add_email")}</button>
      </div></div>`;
    const listEl = document.getElementById("emailList");
    listEl.innerHTML = d.emails.map((e) => `<div class="flex spread" style="padding:8px 0;border-bottom:1px dashed var(--hairline)">
      <span class="mono">${escHtml(e.email)} <span class="muted">${escHtml(e.note)}</span></span>
      <button class="btn btn-sm btn-danger" data-del="${e.id}">${t("delete")}</button></div>`).join("");
    listEl.querySelectorAll("[data-del]").forEach((b) => b.addEventListener("click", async () => {
      await API.del(`/api/admin/emails/${b.dataset.del}`); toast("OK", "ok"); route();
    }));
    document.getElementById("addEmailBtn").addEventListener("click", async () => {
      try {
        await API.post("/api/admin/emails", {
          email: document.getElementById("newEmail").value.trim(),
          note: document.getElementById("newEmailNote").value.trim(),
        });
        toast("OK", "ok"); route();
      } catch (e) { toast(e.message, "err"); }
    });
  }

  // ---------- 可信设备 ----------
  async function viewTrust(content) {
    const d = await API.get("/api/admin/trust");
    content.innerHTML = `<div class="card"><div class="side-title">${t("admin_trust")}</div><div id="trustList"></div></div>`;
    const listEl = document.getElementById("trustList");
    if (!d.devices.length) { listEl.innerHTML = `<div class="empty">-</div>`; return; }
    listEl.innerHTML = d.devices.map((x) => `
      <div style="border:1px solid var(--hairline);border-radius:12px;padding:14px;margin-bottom:10px">
        <div class="flex spread mb8"><strong>${escHtml(x.username)}</strong>
          <button class="btn btn-sm btn-danger" data-clear="${x.user_id}">${t("clear_trust")}</button></div>
        <div class="muted" style="font-size:12.5px">${t("trusted_ips")}: ${x.ips.map((i) => `<span class="mono">${escHtml(i)}</span>`).join(" ") || "-"}</div>
        <div class="muted" style="font-size:12.5px">${t("fingerprints")}: ${x.fingerprints.length} 条 · ${fmtTime(x.updated_at)}</div>
      </div>`).join("");
    listEl.querySelectorAll("[data-clear]").forEach((b) => b.addEventListener("click", async () => {
      if (!confirm(t("clear_trust_confirm"))) return;
      await API.post(`/api/admin/trust/${b.dataset.clear}/clear`, {});
      toast("OK", "ok"); route();
    }));
  }

  // ---------- 周报 ----------
  async function viewWeekly(content) {
    const d = await API.get("/api/admin/weekly");
    content.innerHTML = `<div class="card"><div class="flex spread"><div class="side-title">${t("weekly_list")}</div>
      <button class="btn btn-primary btn-sm" id="genBtn">${t("gen_weekly")}</button></div>
      <div id="weeklyList"></div></div>`;
    const listEl = document.getElementById("weeklyList");
    if (!d.reports.length) { listEl.innerHTML = `<div class="empty">${t("no_reports")}</div>`; }
    else {
      listEl.innerHTML = d.reports.map((r) => {
        const x = r.data || {};
        return `<details style="border:1px solid var(--hairline);border-radius:12px;padding:12px 14px;margin-bottom:10px">
          <summary style="cursor:pointer;font-weight:600">${r.week_start} ~ ${r.week_end}</summary>
          <div class="table-wrap mt8"><table class="tbl"><tbody>
            <tr><td>${t("visits_today")}</td><td>${x.visits ?? 0}</td><td>${t("online")}</td><td>${x.unique_ips ?? 0}</td></tr>
            <tr><td>${t("total_users")}</td><td>${x.new_users ?? 0}</td><td>${t("total_posts")}</td><td>${x.new_posts ?? 0}</td></tr>
            <tr><td>${t("total_comments")}</td><td>${x.new_comments ?? 0}</td><td>${t("uploads")}</td><td>${x.uploads ?? 0} (${fmtBytes(x.upload_bytes || 0)})</td></tr>
            <tr><td>${t("admin_reports")}</td><td>${x.reports ?? 0}/${x.reports_resolved ?? 0}</td><td>${t("ban")}</td><td>${x.bans ?? 0}</td></tr>
          </tbody></table></div>
        </details>`;
      }).join("");
    }
    document.getElementById("genBtn").addEventListener("click", async () => {
      try { const r = await API.post("/api/admin/weekly/generate", {}); toast("OK " + r.week, "ok"); route(); }
      catch (e) { toast(e.message, "err"); }
    });
  }

  // ---------- 日志 ----------
  async function viewLogs(content) {
    const d = await API.get("/api/admin/logs");
    content.innerHTML = `<div class="card"><div class="side-title">${t("admin_logs")}</div>
      <div class="table-wrap"><table class="tbl"><thead><tr><th>${t("time")}</th><th>${t("username")}</th><th>${t("actions")}</th><th>${t("detail")}</th></tr></thead><tbody>
      ${d.logs.map((l) => `<tr><td class="muted">${fmtTime(l.created_at)}</td><td>${escHtml(l.admin_name || "-")}</td>
        <td><span class="tag">${escHtml(l.action)}</span></td><td style="font-size:12.5px">${escHtml(l.detail)}</td></tr>`).join("")}
      </tbody></table></div></div>`;
  }

  // ---------- 违禁词库 ----------
  async function viewWords(content) {
    const d = await API.get("/api/admin/banned-words");
    content.innerHTML = `<div class="card"><div class="flex spread"><div class="side-title">${t("admin_words")} · ${t("total_words")}: ${d.total}</div>
      <button class="btn btn-sm" id="reloadBtn">${t("reload_words")}</button></div>
      <p class="muted" style="font-size:12.5px">${t("severity_tip")}</p>
      <div class="table-wrap mt8"><table class="tbl"><thead><tr><th>${t("detail")}</th><th>${t("severity")}</th><th>Lang</th><th>${t("review_note")}</th></tr></thead><tbody>
      ${d.words.map((w) => `<tr><td class="mono">${escHtml(w.word)}</td><td><span class="tag ${w.severity >= 4 ? "danger" : w.severity >= 3 ? "amber" : ""}">S${w.severity}</span></td>
        <td>${escHtml(w.lang)}</td><td class="muted" style="font-size:12.5px">${escHtml(w.note || "")}</td></tr>`).join("")}
      </tbody></table></div></div>`;
    document.getElementById("reloadBtn").addEventListener("click", async () => {
      try { await API.post("/api/admin/banned-words/reload", {}); toast("OK", "ok"); route(); }
      catch (e) { toast(e.message, "err"); }
    });
  }

  return { route };
})();
