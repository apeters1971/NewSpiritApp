(() => {
  function pad(n) {
    return String(n).padStart(2, "0");
  }

  function escape(s) {
    return String(s ?? "").replace(/[&<>"']/g, (ch) => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[ch]));
  }

  function splitWhen(local) {
    const s = String(local || "");
    const i = s.indexOf("T");
    if (i < 0) return { date: s.length >= 10 ? s.slice(0, 10) : "", time: "" };
    return { date: s.slice(0, i), time: s.slice(i + 1, i + 6) };
  }

  function joinWhen(date, time) {
    const d = String(date || "").trim();
    const t = String(time || "").trim();
    if (!d || !t) return "";
    return `${d}T${t}`;
  }

  function addHours(local, hours) {
    const s = String(local || "");
    if (!s.includes("T")) return "";
    const d = new Date(s);
    if (Number.isNaN(d.getTime())) return "";
    d.setHours(d.getHours() + hours);
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }

  function timeOptions(selected) {
    const times = [];
    for (let m = 0; m < 24 * 60; m += 15) {
      times.push(`${pad(Math.floor(m / 60))}:${pad(m % 60)}`);
    }
    if (selected && !times.includes(selected)) times.push(selected);
    times.sort();
    return `<option value=""></option>${times.map((t) =>
      `<option value="${escape(t)}"${t === selected ? " selected" : ""}>${escape(t)}</option>`).join("")}`;
  }

  function fillTimeSelect(sel, selected) {
    if (!sel) return;
    sel.innerHTML = timeOptions(selected);
    sel.value = selected || "";
  }

  function readPair(dateEl, timeEl) {
    return joinWhen(dateEl?.value, timeEl?.value);
  }

  function setPair(dateEl, timeEl, local) {
    const w = splitWhen(local);
    if (dateEl) dateEl.value = w.date;
    fillTimeSelect(timeEl, w.time);
  }

  function rowHTML(local, dateAttrs, timeAttrs) {
    const w = splitWhen(local);
    return `<div class="when-row">
      <input type="date" value="${escape(w.date)}" ${dateAttrs || ""} />
      <select ${timeAttrs || ""}>${timeOptions(w.time)}</select>
    </div>`;
  }

  window.When = { splitWhen, joinWhen, addHours, timeOptions, fillTimeSelect, readPair, setPair, rowHTML };
})();
