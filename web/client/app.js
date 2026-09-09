const ROLE_LABELS = { choir: "Choir", band: "Band", orchestra: "Orchestra", technician: "Technician" };
const CHOICES = [
  { id: "yes", label: "Yes" },
  { id: "maybe", label: "Maybe" },
  { id: "no", label: "No" },
  { id: "unknown", label: "Unknown" },
];

const loginView = document.getElementById("login-view");
const appView = document.getElementById("app-view");
const loginForm = document.getElementById("login-form");
const loginError = document.getElementById("login-error");
const datesEl = document.getElementById("dates");
const emptyEl = document.getElementById("empty");

let me = null;

async function api(path, opts = {}) {
  const res = await fetch(path, {
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", ...(opts.headers || {}) },
    ...opts,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}

function showError(el, msg) {
  el.hidden = !msg;
  el.textContent = msg || "";
}

function labelRole(role) {
  return ROLE_LABELS[role] || role;
}

function formatWhen(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  return d.toLocaleString(undefined, { weekday: "short", day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" });
}

function formatRange(date) {
  let s = formatWhen(date.startsAt);
  if (date.endsAt) s += " – " + formatWhen(date.endsAt);
  if (date.location) s += " · " + date.location;
  return s;
}

function voteLabel(choice) {
  return (CHOICES.find((c) => c.id === choice) || { label: "Unknown" }).label;
}

function renderRoster(date) {
  const rows = date.roster.map((entry) => {
    const changed = date.status === "accepted" && entry.initialChoice && entry.initialChoice !== entry.choice;
    const vote = changed
      ? `<span class="badge ${entry.choice}">${voteLabel(entry.choice)}</span> <span class="changed">was ${voteLabel(entry.initialChoice)}</span>`
      : `<span class="badge ${entry.choice}">${voteLabel(entry.choice)}</span>`;
    return `<tr>
      <td>${escapeHtml(entry.nickname)}</td>
      <td>${escapeHtml(labelRole(entry.role))}</td>
      <td>${escapeHtml(entry.subrole)}</td>
      <td>${vote}</td>
    </tr>`;
  }).join("");
  return `<table class="roster">
    <thead><tr><th>Name</th><th>Role</th><th>Subrole</th><th>Vote</th></tr></thead>
    <tbody>${rows || `<tr><td colspan="4" class="muted">No people in these roles yet.</td></tr>`}</tbody>
  </table>`;
}

function renderCounts(date) {
  if (!date.subroleCounts?.length) return "";
  return `<div class="counts">${date.subroleCounts.map((c) => `
    <div class="count">
      <strong>${escapeHtml(labelRole(c.role))} · ${escapeHtml(c.subrole)}</strong>
      <span>Yes ${c.yes} · Maybe ${c.maybe} · No ${c.no} · Unknown ${c.unknown}</span>
    </div>`).join("")}</div>`;
}

function renderDate(date) {
  const locked = date.status === "cancelled";
  const buttons = CHOICES.map((c) => `
    <button type="button" data-id="${date.id}" data-choice="${c.id}" class="${c.id}${date.myChoice === c.id ? " on" : ""}" ${locked ? "disabled" : ""}>${c.label}</button>
  `).join("");
  const mine = date.status === "accepted" && date.myInitial && date.myInitial !== date.myChoice
    ? `<p class="changed">Your initial vote: ${voteLabel(date.myInitial)}</p>`
    : "";
  const notes = date.notes ? `<p class="notes">${escapeHtml(date.notes)}</p>` : "";
  return `<article class="card">
    <div class="card-head">
      <div>
        <p class="brand">${date.roles.map(labelRole).join(" · ")}</p>
        <h2>${escapeHtml(date.title)}</h2>
        <p class="when">${escapeHtml(formatRange(date))}</p>
        ${notes}
      </div>
      <span class="badge ${date.status}">${date.status}</span>
    </div>
    <div class="vote-row">${buttons}</div>
    ${mine}
    ${renderCounts(date)}
    ${renderRoster(date)}
  </article>`;
}

function escapeHtml(s) {
  return String(s ?? "").replace(/[&<>"']/g, (ch) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[ch]));
}

function render(dates) {
  datesEl.innerHTML = dates.map(renderDate).join("");
  emptyEl.hidden = dates.length > 0;
}

async function loadDates() {
  const data = await api("/api/dates");
  render(data.dates || []);
}

async function boot() {
  try {
    const data = await api("/api/me");
    me = data.user;
    loginView.hidden = true;
    appView.hidden = false;
    document.getElementById("who-name").textContent = me.nickname;
    document.getElementById("who-meta").textContent = `${labelRole(me.role)} · ${me.subrole} · ${me.email}`;
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

datesEl.addEventListener("click", async (e) => {
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

function connectWS() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${proto}://${location.host}/ws/client`);
  ws.onmessage = () => { loadDates().catch(() => {}); };
  ws.onclose = () => setTimeout(connectWS, 2000);
}

boot();
