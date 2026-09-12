(() => {
  const VOICES = ["Sopran", "Alt", "Tenor/Bass"];
  const COLORS = {
    Sopran: "#e28b98",
    Alt: "#d4b07a",
    "Tenor/Bass": "#7fd99a",
  };

  function escape(s) {
    return String(s ?? "").replace(/[&<>"']/g, (ch) => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[ch]));
  }

  function polar(cx, cy, r, deg) {
    const rad = ((deg - 90) * Math.PI) / 180;
    return [cx + r * Math.cos(rad), cy + r * Math.sin(rad)];
  }

  function slicePath(cx, cy, r, start, end) {
    const sweep = end - start;
    if (sweep >= 359.999) {
      return `M ${cx} ${cy - r} A ${r} ${r} 0 1 1 ${cx} ${cy + r} A ${r} ${r} 0 1 1 ${cx} ${cy - r} Z`;
    }
    const [x1, y1] = polar(cx, cy, r, start);
    const [x2, y2] = polar(cx, cy, r, end);
    const large = sweep > 180 ? 1 : 0;
    return `M ${cx} ${cy} L ${x1} ${y1} A ${r} ${r} 0 ${large} 1 ${x2} ${y2} Z`;
  }

  function html(counts, labels) {
    const parts = VOICES.map((voice) => ({
      voice,
      n: Math.max(0, Number(counts?.[voice]) || 0),
      color: COLORS[voice],
      label: labels?.[voice] || voice,
    }));
    const total = parts.reduce((sum, part) => sum + part.n, 0);
    const detail = parts.map((part) => `${part.label} ${part.n}`).join(" · ");
    const cx = 18;
    const cy = 18;
    const r = 16;
    let slices;
    if (!total) {
      slices = `<circle class="voice-pie-empty" cx="${cx}" cy="${cy}" r="${r - 1}"></circle>`;
    } else {
      let angle = 0;
      slices = parts.filter((part) => part.n > 0).map((part) => {
        const sweep = (part.n / total) * 360;
        const start = angle;
        const end = angle + sweep;
        angle = end;
        return `<path d="${escape(slicePath(cx, cy, r, start, end))}" fill="${part.color}"></path>`;
      }).join("");
    }
    const nums = parts.map((part) => (
      `<span style="color:${part.color}">${part.n}</span>`
    )).join("<span class=\"voice-pie-dot\">·</span>");
    return `<div class="voice-pie" title="${escape(detail)}" role="img" aria-label="${escape(detail)}">
      <svg viewBox="0 0 36 36" width="52" height="52" aria-hidden="true">${slices}</svg>
      <p class="voice-pie-nums">${nums}</p>
    </div>`;
  }

  window.VoicePie = { html, VOICES, COLORS };
})();
