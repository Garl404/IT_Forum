// CALUSE 轻量 Canvas 图表
window.Charts = (function () {
  function lineChart(canvas, data, opts) {
    opts = opts || {};
    const dpr = window.devicePixelRatio || 1;
    const w = canvas.clientWidth || 300, h = canvas.clientHeight || 110;
    canvas.width = w * dpr; canvas.height = h * dpr;
    const ctx = canvas.getContext("2d");
    ctx.scale(dpr, dpr);
    ctx.clearRect(0, 0, w, h);
    const max = Math.max(1, ...data.map((d) => d.count));
    const pad = { l: 34, r: 8, t: 8, b: 18 };
    const iw = w - pad.l - pad.r, ih = h - pad.t - pad.b;
    const css = getComputedStyle(document.documentElement);
    const grid = css.getPropertyValue("--hairline").trim() || "rgba(0,0,0,.1)";
    const accent = "#14b8cf";
    const label = css.getPropertyValue("--text-faint").trim() || "#888";
    const areaTop = "#14b8cf33", areaBottom = "#14b8cf00";

    ctx.strokeStyle = grid; ctx.lineWidth = 1; ctx.font = "10px sans-serif"; ctx.fillStyle = label;
    for (let i = 0; i <= 3; i++) {
      const y = pad.t + (ih / 3) * i;
      ctx.beginPath(); ctx.moveTo(pad.l, y); ctx.lineTo(w - pad.r, y); ctx.stroke();
      ctx.fillText(String(Math.round(max - (max / 3) * i)), 2, y + 3);
    }
    const n = data.length;
    const pts = data.map((d, i) => ({
      x: pad.l + (n === 1 ? iw / 2 : (iw * i) / (n - 1)),
      y: pad.t + ih - (d.count / max) * ih,
    }));
    // 面积
    const grad = ctx.createLinearGradient(0, pad.t, 0, pad.t + ih);
    grad.addColorStop(0, areaTop); grad.addColorStop(1, areaBottom);
    ctx.beginPath(); ctx.moveTo(pts[0].x, pad.t + ih);
    pts.forEach((p) => ctx.lineTo(p.x, p.y));
    ctx.lineTo(pts[pts.length - 1].x, pad.t + ih); ctx.closePath();
    ctx.fillStyle = grad; ctx.fill();
    // 折线
    ctx.beginPath(); ctx.strokeStyle = accent; ctx.lineWidth = 2; ctx.lineJoin = "round";
    pts.forEach((p, i) => (i === 0 ? ctx.moveTo(p.x, p.y) : ctx.lineTo(p.x, p.y)));
    ctx.stroke();
    // 点
    pts.forEach((p) => { ctx.beginPath(); ctx.arc(p.x, p.y, 2.5, 0, Math.PI * 2); ctx.fillStyle = accent; ctx.fill(); });
    // x 轴标签
    ctx.fillStyle = label;
    data.forEach((d, i) => {
      if (n > 12 && i % Math.ceil(n / 8) !== 0 && i !== n - 1) return;
      ctx.fillText(d.date, pts[i].x - 13, h - 4);
    });
  }

  function barChart(canvas, data, opts) {
    opts = opts || {};
    const dpr = window.devicePixelRatio || 1;
    const w = canvas.clientWidth || 300, h = canvas.clientHeight || 110;
    canvas.width = w * dpr; canvas.height = h * dpr;
    const ctx = canvas.getContext("2d");
    ctx.scale(dpr, dpr);
    ctx.clearRect(0, 0, w, h);
    const max = Math.max(1, ...data.map((d) => d.count));
    const pad = { l: 34, r: 8, t: 8, b: 18 };
    const iw = w - pad.l - pad.r, ih = h - pad.t - pad.b;
    const css = getComputedStyle(document.documentElement);
    const grid = css.getPropertyValue("--hairline").trim() || "rgba(0,0,0,.1)";
    const accent = "#f0a94e";
    const label = css.getPropertyValue("--text-faint").trim() || "#888";
    ctx.strokeStyle = grid; ctx.lineWidth = 1; ctx.font = "10px sans-serif"; ctx.fillStyle = label;
    for (let i = 0; i <= 3; i++) {
      const y = pad.t + (ih / 3) * i;
      ctx.beginPath(); ctx.moveTo(pad.l, y); ctx.lineTo(w - pad.r, y); ctx.stroke();
      ctx.fillText(String(Math.round(max - (max / 3) * i)), 2, y + 3);
    }
    const n = data.length;
    const bw = Math.min(36, (iw / n) * 0.62);
    data.forEach((d, i) => {
      const x = pad.l + (iw * i) / n + ((iw / n) - bw) / 2;
      const bh = (d.count / max) * ih;
      const y = pad.t + ih - bh;
      const grad = ctx.createLinearGradient(0, y, 0, y + bh);
      grad.addColorStop(0, "#f0b869"); grad.addColorStop(1, "#e8982f");
      ctx.fillStyle = grad;
      ctx.beginPath();
      if (ctx.roundRect) ctx.roundRect(x, y, bw, Math.max(1.5, bh), 4); else ctx.rect(x, y, bw, Math.max(1.5, bh));
      ctx.fill();
      ctx.fillStyle = label;
      ctx.fillText(d.date, x - 4, h - 4);
    });
  }

  return { line: lineChart, bar: barChart };
})();
