const ROLE_LABELS = { choir: "Choir", band: "Band", orchestra: "Orchestra", technician: "Technician" };

const gate = document.getElementById("gate");
const dash = document.getElementById("dash");
const gateError = document.getElementById("gate-error");
const peopleError = document.getElementById("people-error");
const dateError = document.getElementById("date-error");

let catalog = { roles: [] };
let state = { users: [], dates: [], online: 0 };
let selectedUser = "";
let selectedDate = "";

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

function voteLabel(choice) {
  return ({ yes: "Yes", maybe: "Maybe", no: "No", unknown: "Unknown" }[choice] || "Unknown");
}

function formatWhen(iso) {
  if (!iso) return "";
  return new Date(iso).toLocaleString(undefined, {
    weekday: "short", day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit",
  });
}

function toLocalInput(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  const pad = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function toISO(local) {
  if (!local) return "";
  return new Date(local).toISOString();
}

function subrolesFor(role) {
  return (catalog.roles.find((r) => r.id === role) || { subroles: [] }).subroles;
}

function fillRoleSelects() {
  const roleSel = document.getElementById("user-role");
  roleSel.innerHTML = catalog.roles.map((r) => `<option value="${r.id}">${r.label}</option>`).join("");
  fillSubroles();
  document.getElementById("date-roles").innerHTML = catalog.roles.map((r) => `
    <label><input type="checkbox" name="role" value="${r.id}" /> ${r.label}</label>
  `).join("");
}

function fillSubroles() {
  const role = document.getElementById("user-role").value;
  const sel = document.getElementById("user-subrole");
  sel.innerHTML = subrolesFor(role).map((s) => `<option value="${s}">${s}</option>`).join("");
}

document.getElementById("user-role").addEventListener("change", fillSubroles);

function renderPeople() {
  document.getElementById("stat-people").textContent = state.users.length;
  document.getElementById("people-body").innerHTML = state.users.map((u) => `
    <tr data-id="${u.id}" class="${u.id === selectedUser ? "active" : ""}">
      <td>${escapeHtml(u.nickname)}</td>
      <td>${escapeHtml(u.email)}</td>
      <td>${escapeHtml(labelRole(u.role))}</td>
      <td>${escapeHtml(u.subrole)}</td>
    </tr>
  `).join("");
}

function renderDates() {
  document.getElementById("stat-dates").textContent = state.dates.length;
  document.getElementById("stat-online").textContent = state.online;
  document.getElementById("date-list").innerHTML = state.dates.map((d) => `
    <article class="date-item ${d.id === selectedDate ? "active" : ""}" data-id="${d.id}">
      <span class="badge ${d.status}">${d.status}</span>
      <h3>${escapeHtml(d.title)}</h3>
      <p>${escapeHtml(formatWhen(d.startsAt))}${d.location ? " · " + escapeHtml(d.location) : ""}</p>
      <p>${d.roles.map(labelRole).join(" · ")}</p>
    </article>
  `).join("") || `<p class="lede" style="padding:1rem">No dates yet.</p>`;
  renderDateDetail();
}

function renderDateDetail() {
  const d = state.dates.find((x) => x.id === selectedDate);
  const box = document.getElementById("date-detail");
  const fin = document.getElementById("date-finalize");
  if (!d) {
    box.innerHTML = "";
    fin.hidden = true;
    return;
  }
  fin.hidden = d.status !== "voting";
  const counts = (d.subroleCounts || []).map((c) => `
    <div class="count">
      <strong>${escapeHtml(labelRole(c.role))} · ${escapeHtml(c.subrole)}</strong>
      <span>Yes ${c.yes} · Maybe ${c.maybe} · No ${c.no} · Unknown ${c.unknown}</span>
    </div>`).join("");
  const rows = (d.roster || []).map((e) => {
    const changed = d.status === "accepted" && e.initialChoice && e.initialChoice !== e.choice;
    const vote = changed
      ? `<span class="badge ${e.choice}">${voteLabel(e.choice)}</span> <span class="changed">was ${voteLabel(e.initialChoice)}</span>`
      : `<span class="badge ${e.choice}">${voteLabel(e.choice)}</span>`;
    return `<tr><td>${escapeHtml(e.nickname)}</td><td>${escapeHtml(e.subrole)}</td><td>${vote}</td></tr>`;
  }).join("");
  box.innerHTML = `
    <h3>Votes</h3>
    <div class="counts">${counts}</div>
    <table>
      <thead><tr><th>Name</th><th>Subrole</th><th>Vote</th></tr></thead>
      <tbody>${rows || `<tr><td colspan="3" class="muted">No people in these roles yet.</td></tr>`}</tbody>
    </table>`;
}

function resetUserForm() {
  selectedUser = "";
  document.getElementById("people-form-title").textContent = "Add person";
  document.getElementById("user-id").value = "";
  document.getElementById("user-nickname").value = "";
  document.getElementById("user-email").value = "";
  document.getElementById("user-password").value = "";
  document.getElementById("user-password").required = true;
  document.getElementById("pw-hint").textContent = "";
  document.getElementById("btn-user-delete").disabled = true;
  if (catalog.roles[0]) document.getElementById("user-role").value = catalog.roles[0].id;
  fillSubroles();
  renderPeople();
  showError(peopleError, "");
}

function fillUserForm(u) {
  selectedUser = u.id;
  document.getElementById("people-form-title").textContent = "Edit person";
  document.getElementById("user-id").value = u.id;
  document.getElementById("user-nickname").value = u.nickname;
  document.getElementById("user-email").value = u.email;
  document.getElementById("user-password").value = "";
  document.getElementById("user-password").required = false;
  document.getElementById("pw-hint").textContent = "(leave blank to keep)";
  document.getElementById("user-role").value = u.role;
  fillSubroles();
  document.getElementById("user-subrole").value = u.subrole;
  document.getElementById("btn-user-delete").disabled = false;
  renderPeople();
}

function resetDateForm() {
  selectedDate = "";
  document.getElementById("date-form-title").textContent = "Add date";
  document.getElementById("date-id").value = "";
  document.getElementById("date-title").value = "";
  document.getElementById("date-start").value = "";
  document.getElementById("date-end").value = "";
  document.getElementById("date-location").value = "";
  document.getElementById("date-notes").value = "";
  document.querySelectorAll("#date-roles input").forEach((el) => { el.checked = false; });
  document.getElementById("btn-date-delete").disabled = true;
  renderDates();
  showError(dateError, "");
}

function fillDateForm(d) {
  selectedDate = d.id;
  document.getElementById("date-form-title").textContent = "Edit date";
  document.getElementById("date-id").value = d.id;
  document.getElementById("date-title").value = d.title;
  document.getElementById("date-start").value = toLocalInput(d.startsAt);
  document.getElementById("date-end").value = toLocalInput(d.endsAt);
  document.getElementById("date-location").value = d.location || "";
  document.getElementById("date-notes").value = d.notes || "";
  document.querySelectorAll("#date-roles input").forEach((el) => {
    el.checked = (d.roles || []).includes(el.value);
  });
  document.getElementById("btn-date-delete").disabled = false;
  renderDates();
}

function escapeHtml(s) {
  return String(s ?? "").replace(/[&<>"']/g, (ch) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[ch]));
}

async function loadState() {
  state = await api("/api/controller/state");
  if (selectedUser && !state.users.some((u) => u.id === selectedUser)) resetUserForm();
  if (selectedDate && !state.dates.some((d) => d.id === selectedDate)) resetDateForm();
  else {
    renderPeople();
    renderDates();
  }
}

async function boot() {
  try {
    catalog = await api("/api/catalog");
    fillRoleSelects();
    await loadState();
    gate.hidden = true;
    dash.hidden = false;
    connectWS();
  } catch {
    gate.hidden = false;
    dash.hidden = true;
  }
}

document.getElementById("gate-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  showError(gateError, "");
  try {
    await api("/api/controller/login", {
      method: "POST",
      body: JSON.stringify({ secret: document.getElementById("secret").value }),
    });
    await boot();
  } catch (err) {
    showError(gateError, err.message);
  }
});

document.getElementById("btn-logout").addEventListener("click", async () => {
  await api("/api/controller/logout", { method: "POST" });
  await boot();
});

document.querySelectorAll(".tab").forEach((tab) => {
  tab.addEventListener("click", () => {
    document.querySelectorAll(".tab").forEach((t) => t.classList.toggle("active", t === tab));
    document.getElementById("tab-people").hidden = tab.dataset.tab !== "people";
    document.getElementById("tab-dates").hidden = tab.dataset.tab !== "dates";
  });
});

document.getElementById("people-body").addEventListener("click", (e) => {
  const row = e.target.closest("tr[data-id]");
  if (!row) return;
  const user = state.users.find((u) => u.id === row.dataset.id);
  if (user) fillUserForm(user);
});

document.getElementById("btn-user-new").addEventListener("click", resetUserForm);

document.getElementById("people-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  showError(peopleError, "");
  const id = document.getElementById("user-id").value;
  const body = {
    nickname: document.getElementById("user-nickname").value,
    email: document.getElementById("user-email").value,
    password: document.getElementById("user-password").value,
    role: document.getElementById("user-role").value,
    subrole: document.getElementById("user-subrole").value,
  };
  try {
    const data = id
      ? await api(`/api/controller/users/${id}`, { method: "PATCH", body: JSON.stringify(body) })
      : await api("/api/controller/users", { method: "POST", body: JSON.stringify(body) });
    await loadState();
    fillUserForm(data.user);
  } catch (err) {
    showError(peopleError, err.message);
  }
});

document.getElementById("btn-user-delete").addEventListener("click", async () => {
  const id = document.getElementById("user-id").value;
  if (!id || !confirm("Delete this person?")) return;
  try {
    await api(`/api/controller/users/${id}`, { method: "DELETE" });
    resetUserForm();
    await loadState();
  } catch (err) {
    showError(peopleError, err.message);
  }
});

document.getElementById("date-list").addEventListener("click", (e) => {
  const item = e.target.closest("[data-id]");
  if (!item) return;
  const d = state.dates.find((x) => x.id === item.dataset.id);
  if (d) fillDateForm(d);
});

document.getElementById("btn-date-new").addEventListener("click", resetDateForm);

document.getElementById("date-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  showError(dateError, "");
  const id = document.getElementById("date-id").value;
  const body = {
    title: document.getElementById("date-title").value,
    startsAt: toISO(document.getElementById("date-start").value),
    endsAt: toISO(document.getElementById("date-end").value),
    location: document.getElementById("date-location").value,
    notes: document.getElementById("date-notes").value,
    roles: [...document.querySelectorAll("#date-roles input:checked")].map((el) => el.value),
  };
  try {
    const data = id
      ? await api(`/api/controller/dates/${id}`, { method: "PATCH", body: JSON.stringify(body) })
      : await api("/api/controller/dates", { method: "POST", body: JSON.stringify(body) });
    await loadState();
    fillDateForm(data.date);
  } catch (err) {
    showError(dateError, err.message);
  }
});

document.getElementById("btn-date-delete").addEventListener("click", async () => {
  const id = document.getElementById("date-id").value;
  if (!id || !confirm("Delete this date?")) return;
  try {
    await api(`/api/controller/dates/${id}`, { method: "DELETE" });
    resetDateForm();
    await loadState();
  } catch (err) {
    showError(dateError, err.message);
  }
});

async function setStatus(status) {
  const id = document.getElementById("date-id").value;
  if (!id) return;
  try {
    const data = await api(`/api/controller/dates/${id}/status`, {
      method: "POST",
      body: JSON.stringify({ status }),
    });
    await loadState();
    fillDateForm(data.date);
  } catch (err) {
    showError(dateError, err.message);
  }
}

document.getElementById("btn-accept").addEventListener("click", () => setStatus("accepted"));
document.getElementById("btn-cancel").addEventListener("click", () => setStatus("cancelled"));

function connectWS() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${proto}://${location.host}/ws/controller`);
  ws.onmessage = () => { loadState().catch(() => {}); };
  ws.onclose = () => { if (!dash.hidden) setTimeout(connectWS, 2000); };
}

boot();
