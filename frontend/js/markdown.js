// CALUSE 安全 Markdown 渲染器：先转义所有 HTML，再按语法生成安全标签（防 XSS）
window.MDRenderer = (function () {
  function esc(s) {
    return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
            .replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }

  function escapeUrl(url) {
    url = url.trim();
    if (/^(https?:\/\/|mailto:|#|\/)/i.test(url)) return url;
    return "#";
  }

  function inline(text) {
    let out = esc(text);
    // 行内代码（优先，防止内部其他语法）
    out = out.replace(/`([^`]+)`/g, (m, c) => `<code>${c}</code>`);
    // 链接 [text](url)
    out = out.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (m, txt, url) =>
      `<a href="${escapeUrl(url)}" target="_blank" rel="noopener noreferrer nofollow">${txt}</a>`);
    // 粗体 / 斜体 / 删除线
    out = out.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
    out = out.replace(/__([^_]+)__/g, "<strong>$1</strong>");
    out = out.replace(/(^|[^*])\*([^*\n]+)\*/g, "$1<em>$2</em>");
    out = out.replace(/(^|[^_])_([^_\n]+)_/g, "$1<em>$2</em>");
    out = out.replace(/~~([^~]+)~~/g, "<del>$1</del>");
    return out;
  }

  function render(src) {
    if (!src) return "";
    const lines = src.split("\n");
    let html = "";
    let inCode = false, codeBuf = [], inList = false, listType = "", inQuote = false, quoteBuf = [];

    const flushList = () => { if (inList) { html += `</${listType}>`; inList = false; } };
    const flushQuote = () => { if (inQuote) { html += "</blockquote>"; inQuote = false; } };

    for (const raw of lines) {
      const line = raw.replace(/\r$/, "");
      // 代码块
      if (/^```/.test(line)) {
        if (inCode) { html += `<pre><code>${esc(codeBuf.join("\n"))}</code></pre>`; codeBuf = []; inCode = false; }
        else { flushList(); flushQuote(); inCode = true; }
        continue;
      }
      if (inCode) { codeBuf.push(line); continue; }

      // 引用
      if (/^>\s?/.test(line)) {
        flushList();
        if (!inQuote) { inQuote = true; quoteBuf = []; }
        quoteBuf.push(line.replace(/^>\s?/, ""));
        continue;
      } else { flushQuote(); }

      // 列表
      const ulMatch = line.match(/^[-*]\s+(.*)$/);
      const olMatch = line.match(/^\d+[.)]\s+(.*)$/);
      if (ulMatch || olMatch) {
        const want = ulMatch ? "ul" : "ol";
        const content = (ulMatch || olMatch)[1];
        if (!inList || listType !== want) { flushList(); html += `<${want}>`; inList = true; listType = want; }
        html += `<li>${inline(content)}</li>`;
        continue;
      } else { flushList(); }

      if (line.trim() === "") { html += ""; continue; }

      // 标题
      const h = line.match(/^(#{1,4})\s+(.*)$/);
      if (h) { html += `<h${h[1].length}>${inline(h[2])}</h${h[1].length}>`; continue; }
      // 分隔线
      if (/^([-*_]\s?){3,}$/.test(line.trim())) { html += "<hr>"; continue; }
      html += `<p>${inline(line)}</p>`;
    }
    if (inCode) html += `<pre><code>${esc(codeBuf.join("\n"))}</code></pre>`;
    flushList(); flushQuote();
    return html;
  }

  // 生成纯文本摘要
  function excerpt(src, max) {
    const text = (src || "").replace(/```[\s\S]*?```/g, " ").replace(/[#>*`_~\[\]()!-]/g, " ").replace(/\s+/g, " ").trim();
    return text.length > max ? text.slice(0, max) + "…" : text;
  }

  return { render, excerpt };
})();
