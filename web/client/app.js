const CHOICES = ["yes", "maybe", "no", "unknown"];

const loginView = document.getElementById("login-view");
const appView = document.getElementById("app-view");
const loginForm = document.getElementById("login-form");
const loginError = document.getElementById("login-error");
const datesEl = document.getElementById("dates");
const emptyEl = document.getElementById("empty");

let me = null;
let dates = [];
let commentDateId = "";

async function api(path, opts = {}) {
  const res = await fetch(path, {
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", ...(opts.headers || {}) },
    ...opts,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(I18N.error(data.error || res.statusText));
  return data;
}

function showError(el, msg) {
  el.hidden = !msg;
  el.textContent = msg || "";
}

function bringList(bring) {
  const items = [];
  if (bring?.mic) items.push(I18N.gear("mic"));
  if (bring?.cable) items.push(I18N.gear("cable"));
  if (bring?.stand) items.push(I18N.gear("stand"));
  if (bring?.dress) items.push(I18N.dress(bring.dress));
  return items;
}

function renderBring(bring) {
  const items = bringList(bring);
  if (!items.length) return "";
  return `<p class="bring">${escapeHtml(I18N.t("bringPrefix"))}: ${items.map((item) => `<span class="chip">${escapeHtml(item)}</span>`).join("")}</p>`;
}

function formatWhen(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  return d.toLocaleString(I18N.locale(), { weekday: "short", day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" });
}

function formatRange(date) {
  let s = formatWhen(date.startsAt);
  if (date.endsAt) s += " – " + formatWhen(date.endsAt);
  if (date.location) s += " · " + date.location;
  return s;
}

function voteLabel(choice) {
  return I18N.vote(choice || "unknown");
}

function firstVoteChanged(entry) {
  return entry.initialChoice && entry.initialChoice !== entry.choice;
}

function renderRoster(date) {
  const rows = date.roster.map((entry) => {
    const vote = firstVoteChanged(entry)
      ? `<span class="badge ${entry.choice}">${voteLabel(entry.choice)}</span> <span class="changed">${I18N.t("firstVote")} ${voteLabel(entry.initialChoice)}</span>`
      : `<span class="badge ${entry.choice}">${voteLabel(entry.choice)}</span>`;
    return `<tr>
      <td>${escapeHtml(entry.nickname)}</td>
      <td>${escapeHtml(I18N.role(entry.role))}</td>
      <td>${escapeHtml(I18N.subrole(entry.subrole))}</td>
      <td>${vote}</td>
    </tr>`;
  }).join("");
  return `<table class="roster">
    <thead><tr><th>${I18N.t("name")}</th><th>${I18N.t("role")}</th><th>${I18N.t("subrole")}</th><th>${I18N.t("vote")}</th></tr></thead>
    <tbody>${rows || `<tr><td colspan="4" class="muted">${I18N.t("noPeopleRoles")}</td></tr>`}</tbody>
  </table>`;
}

function renderCounts(date) {
  if (!date.subroleCounts?.length) return "";
  return `<div class="counts">${date.subroleCounts.map((c) => `
    <div class="count">
      <strong>${escapeHtml(I18N.role(c.role))} · ${escapeHtml(I18N.subrole(c.subrole))}</strong>
      <span>${I18N.t("yes")} ${c.yes} · ${I18N.t("maybe")} ${c.maybe} · ${I18N.t("no")} ${c.no} · ${I18N.t("unknown")} ${c.unknown}</span>
    </div>`).join("")}</div>`;
}

function renderDate(date) {
  const locked = date.status === "cancelled";
  const buttons = CHOICES.map((c) => `
    <button type="button" data-id="${date.id}" data-choice="${c}" class="${c}${date.myChoice === c ? " on" : ""}" ${locked ? "disabled" : ""}>${voteLabel(c)}</button>
  `).join("");
  const mine = date.myInitial && date.myInitial !== date.myChoice
    ? `<p class="changed">${I18N.t("yourFirstVote")}: ${voteLabel(date.myInitial)}</p>`
    : "";
  const notes = date.notes ? `<p class="notes">${escapeHtml(date.notes)}</p>` : "";
  const commentCount = (date.comments || []).length;
  return `<article class="card">
    <div class="card-head">
      <div>
        <p class="brand">${escapeHtml(I18N.category(date.category))} · ${date.roles.map((r) => I18N.role(r)).join(" · ")}</p>
        <h2 id="date-${date.id}">${escapeHtml(date.title)}</h2>
        <p class="when">${escapeHtml(formatRange(date))}</p>
        ${renderBring(date.bring)}
        ${notes}
      </div>
      <span class="badge ${date.status}">${I18N.status(date.status)}</span>
    </div>
    <div class="vote-row">${buttons}</div>
    ${mine}
    <div class="card-actions">
      <button type="button" class="btn ghost" data-comments="${date.id}">${I18N.t("comments")}${commentCount ? ` (${commentCount})` : ""}</button>
    </div>
    ${renderCounts(date)}
    ${renderRoster(date)}
  </article>`;
}

function escapeHtml(s) {
  return String(s ?? "").replace(/[&<>"']/g, (ch) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[ch]));
}

function upcomingDates() {
  const start = new Date();
  start.setHours(0, 0, 0, 0);
  const end = new Date(start);
  end.setMonth(end.getMonth() + 3);
  return dates.filter((d) => {
    const t = new Date(d.startsAt);
    return t >= start && t < end;
  });
}

function renderOverview() {
  const overview = document.getElementById("overview");
  const body = document.getElementById("overview-body");
  const empty = document.getElementById("overview-empty");
  overview.hidden = false;
  const rows = upcomingDates();
  empty.hidden = rows.length > 0;
  body.innerHTML = rows.map((d) => `
    <tr data-jump="${d.id}">
      <td>${escapeHtml(formatWhen(d.startsAt))}</td>
      <td>${escapeHtml(I18N.category(d.category))}</td>
      <td>${escapeHtml(d.title)}</td>
      <td>${escapeHtml(bringList(d.bring).join(", ") || "—")}</td>
      <td><span class="badge ${d.status}">${I18N.status(d.status)}</span></td>
      <td><span class="badge ${d.myChoice}">${voteLabel(d.myChoice)}</span></td>
    </tr>`).join("");
}

function render() {
  renderOverview();
  datesEl.innerHTML = dates.map(renderDate).join("");
  emptyEl.hidden = dates.length > 0;
  if (commentDateId && document.getElementById("comment-dialog").open) {
    fillCommentDialog(dates.find((d) => d.id === commentDateId));
  }
}

function fillCommentDialog(date) {
  const list = document.getElementById("comment-list");
  if (!date) {
    list.innerHTML = `<p class="muted">${I18N.t("dateNotFound")}</p>`;
    return;
  }
  document.getElementById("comment-title").textContent = date.title;
  const items = date.comments || [];
  list.innerHTML = items.length
    ? items.map((c) => `
      <article class="comment-card">
        <p class="meta">${escapeHtml(c.nickname)} · ${escapeHtml(formatWhen(c.createdAt))}</p>
        <p>${escapeHtml(c.text)}</p>
      </article>`).join("")
    : `<p class="muted">${I18N.t("noComments")}</p>`;
}

function openComments(id) {
  const date = dates.find((d) => d.id === id);
  if (!date) return;
  commentDateId = id;
  document.getElementById("comment-error").hidden = true;
  document.getElementById("comment-text").value = "";
  fillCommentDialog(date);
  document.getElementById("comment-dialog").showModal();
}

async function loadDates() {
  const data = await api("/api/dates");
  dates = data.dates || [];
  render();
}

async function boot() {
  try {
    const data = await api("/api/me");
    me = data.user;
    loginView.hidden = true;
    appView.hidden = false;
    document.getElementById("who-name").textContent = me.nickname;
    document.getElementById("who-meta").textContent = `${I18N.role(me.role)} · ${I18N.subrole(me.subrole)} · ${me.email}`;
    await loadDates();
    connectWS();
  } catch {
    loginView.hidden = false;
    appView.hidden = true;
  }
}

loginForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  showError(loginError, "");
  try {
    await api("/api/login", {
      method: "POST",
      body: JSON.stringify({
        email: document.getElementById("email").value,
        password: document.getElementById("password").value,
      }),
    });
    await boot();
  } catch (err) {
    showError(loginError, err.message);
  }
});

document.getElementById("btn-logout").addEventListener("click", async () => {
  await api("/api/logout", { method: "POST" });
  me = null;
  await boot();
});

document.getElementById("overview-body").addEventListener("click", (e) => {
  const row = e.target.closest("tr[data-jump]");
  if (!row) return;
  document.getElementById(`date-${row.dataset.jump}`)?.scrollIntoView({ behavior: "smooth", block: "start" });
});

datesEl.addEventListener("click", async (e) => {
  const commentsBtn = e.target.closest("button[data-comments]");
  if (commentsBtn) {
    openComments(commentsBtn.dataset.comments);
    return;
  }
  const btn = e.target.closest("button[data-choice]");
  if (!btn || btn.disabled) return;
  try {
    await api(`/api/dates/${btn.dataset.id}/vote`, {
      method: "POST",
      body: JSON.stringify({ choice: btn.dataset.choice }),
    });
    await loadDates();
  } catch (err) {
    alert(err.message);
  }
});

document.getElementById("comment-close").addEventListener("click", () => {
  document.getElementById("comment-dialog").close();
  commentDateId = "";
});

document.getElementById("comment-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const errEl = document.getElementById("comment-error");
  showError(errEl, "");
  try {
    await api(`/api/dates/${commentDateId}/comments`, {
      method: "POST",
      body: JSON.stringify({ text: document.getElementById("comment-text").value }),
    });
    document.getElementById("comment-text").value = "";
    await loadDates();
  } catch (err) {
    showError(errEl, err.message);
  }
});

function connectWS() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${proto}://${location.host}/ws/client`);
  ws.onmessage = () => { loadDates().catch(() => {}); };
  ws.onclose = () => setTimeout(connectWS, 2000);
}

I18N.onChange(() => {
  I18N.apply();
  if (me) {
    document.getElementById("who-name").textContent = me.nickname;
    document.getElementById("who-meta").textContent = `${I18N.role(me.role)} · ${I18N.subrole(me.subrole)} · ${me.email}`;
    render();
  }
});

boot();
