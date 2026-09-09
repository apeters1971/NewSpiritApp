
const gate = document.getElementById("gate");
const dash = document.getElementById("dash");
const gateError = document.getElementById("gate-error");
const peopleError = document.getElementById("people-error");
const dateError = document.getElementById("date-error");

let catalog = { roles: [], categories: [] };
let state = { users: [], dates: [], online: 0, ranking: { entries: [], bySubrole: [] } };
let selectedUser = "";
let selectedDate = "";
let pollRows = [];
let pollFrozen = false;

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

function setBringForm(bring = {}) {
  document.getElementById("bring-mic").checked = !!bring.mic;
  document.getElementById("bring-cable").checked = !!bring.cable;
  document.getElementById("bring-stand").checked = !!bring.stand;
  const dress = bring.dress || "";
  document.querySelectorAll("input[name=dress]").forEach((el) => {
    el.checked = el.value === dress;
  });
}

function readBringForm() {
  return {
    mic: document.getElementById("bring-mic").checked,
    cable: document.getElementById("bring-cable").checked,
    stand: document.getElementById("bring-stand").checked,
    dress: document.querySelector("input[name=dress]:checked")?.value || "",
  };
}

function voteLabel(choice) {
  return I18N.vote(choice || "unknown");
}

function formatWhen(iso) {
  if (!iso) return "";
  return new Date(iso).toLocaleString(I18N.locale(), {
    weekday: "short", day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit",
  });
}

function formatRange(start, end) {
  let s = formatWhen(start);
  if (end) s += " – " + formatWhen(end);
  return s;
}

function renderPollRows() {
  const box = document.getElementById("poll-options");
  box.innerHTML = pollRows.map((row, i) => `
    <div class="poll-row">
      <label>${I18N.t("optionStart")}
        <input type="datetime-local" data-poll="${i}" data-field="start" value="${row.startsAt}" ${pollFrozen ? "disabled" : ""} />
      </label>
      <label>${I18N.t("optionEnd")}
        <input type="datetime-local" data-poll="${i}" data-field="end" value="${row.endsAt}" ${pollFrozen ? "disabled" : ""} />
      </label>
      <button type="button" class="btn ghost" data-remove-poll="${i}" ${pollFrozen ? "disabled" : ""}>${I18N.t("removePollOption")}</button>
    </div>`).join("");
  syncPollMode();
  document.getElementById("btn-add-poll").hidden = pollFrozen;
}

function syncPollMode() {
  const filled = [...document.querySelectorAll("#poll-options [data-field=start]")].filter((el) => el.value).length;
  const pollMode = filled >= 2 && !pollFrozen;
  document.getElementById("date-start-wrap").hidden = pollMode;
  document.getElementById("date-end-wrap").hidden = pollMode;
  document.getElementById("date-start").required = false;
}

function readPollRows() {
  return [...document.querySelectorAll(".poll-row")].map((row, i) => {
    const start = row.querySelector("[data-field=start]")?.value || "";
    const end = row.querySelector("[data-field=end]")?.value || "";
    return {
      id: pollRows[i]?.id || "",
      startsAt: start ? toISO(start) : "",
      endsAt: end ? toISO(end) : "",
    };
  }).filter((r) => r.startsAt);
}

function setPollRows(options = [], frozen = false) {
  pollFrozen = !!frozen;
  pollRows = (options || []).map((o) => ({
    id: o.id || "",
    startsAt: toLocalInput(o.startsAt),
    endsAt: toLocalInput(o.endsAt),
  }));
  renderPollRows();
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
  const roleVal = roleSel.value;
  const catVal = document.getElementById("date-category").value;
  const checkedRoles = [...document.querySelectorAll("#date-roles input:checked")].map((el) => el.value);
  roleSel.innerHTML = catalog.roles.map((r) => `<option value="${r.id}">${I18N.role(r.id)}</option>`).join("");
  if (roleVal) roleSel.value = roleVal;
  fillSubroles();
  document.getElementById("date-roles").innerHTML = catalog.roles.map((r) => `
    <label><input type="checkbox" name="role" value="${r.id}" ${checkedRoles.includes(r.id) ? "checked" : ""} /> ${I18N.role(r.id)}</label>
  `).join("");
  const catSel = document.getElementById("date-category");
  catSel.innerHTML = (catalog.categories || []).map((c) => `<option value="${c.id}">${I18N.category(c.id)}</option>`).join("");
  catSel.value = catVal || catSel.value;
  if (!catSel.value) catSel.value = "event";
}

function fillSubroles() {
  const role = document.getElementById("user-role").value;
  const sel = document.getElementById("user-subrole");
  const cur = sel.value;
  sel.innerHTML = subrolesFor(role).map((s) => `<option value="${s}">${I18N.subrole(s)}</option>`).join("");
  if (cur) sel.value = cur;
}

document.getElementById("user-role").addEventListener("change", fillSubroles);

function renderPeople() {
  document.getElementById("stat-people").textContent = state.users.length;
  document.getElementById("people-body").innerHTML = state.users.map((u) => `
    <tr data-id="${u.id}" class="${u.id === selectedUser ? "active" : ""}">
      <td>${escapeHtml(u.nickname)}</td>
      <td>${escapeHtml(u.email)}</td>
      <td>${escapeHtml(I18N.role(u.role))}</td>
      <td>${escapeHtml(I18N.subrole(u.subrole))}</td>
    </tr>
  `).join("");
}

function renderDates() {
  document.getElementById("stat-dates").textContent = state.dates.length;
  document.getElementById("stat-online").textContent = state.online;
  document.getElementById("date-list").innerHTML = state.dates.map((d) => `
    <article class="date-item ${d.id === selectedDate ? "active" : ""}" data-id="${d.id}">
      <span class="badge ${d.status}">${I18N.status(d.status)}</span>
      ${d.pollOpen ? `<span class="badge voting">${I18N.t("pollOpen")}</span>` : d.frozenOptionId ? `<span class="badge accepted">${I18N.t("pollFrozen")}</span>` : ""}
      <h3>${escapeHtml(d.title)}</h3>
      <p>${escapeHtml(I18N.category(d.category))}${bringList(d.bring).length ? " · " + escapeHtml(bringList(d.bring).join(", ")) : ""}</p>
      <p>${escapeHtml(d.pollOpen ? I18N.t("severalTimes") : formatWhen(d.startsAt))}${d.location ? " · " + escapeHtml(d.location) : ""}</p>
      <p>${d.roles.map((r) => I18N.role(r)).join(" · ")}</p>
    </article>
  `).join("") || `<p class="lede" style="padding:1rem">${I18N.t("noDatesYet")}</p>`;
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
  document.getElementById("btn-accept").hidden = !!d.pollOpen;
  const counts = (d.subroleCounts || []).map((c) => `
    <div class="count">
      <strong>${escapeHtml(I18N.role(c.role))} · ${escapeHtml(I18N.subrole(c.subrole))}</strong>
      <span>${I18N.t("yes")} ${c.yes} · ${I18N.t("maybe")} ${c.maybe} · ${I18N.t("no")} ${c.no} · ${I18N.t("unknown")} ${c.unknown}</span>
    </div>`).join("");
  const rows = (d.roster || []).map((e) => {
    const changed = e.initialChoice && e.initialChoice !== e.choice;
    const vote = changed
      ? `<span class="badge ${e.choice}">${voteLabel(e.choice)}</span> <span class="changed">${I18N.t("firstVote")} ${voteLabel(e.initialChoice)}</span>`
      : `<span class="badge ${e.choice}">${voteLabel(e.choice)}</span>`;
    return `<tr><td>${escapeHtml(e.nickname)}</td><td>${escapeHtml(I18N.subrole(e.subrole))}</td><td>${vote}</td></tr>`;
  }).join("");
  const commentCount = (d.comments || []).length;
  const poll = (d.options || []).length >= 2 ? `
    <h3>${I18N.t("poll")}</h3>
    ${d.pollOpen ? `<p class="muted">${I18N.t("freezeHint")}</p>` : ""}
    ${(d.options || []).map((o) => `
      <article class="poll-result ${o.frozen ? "frozen" : ""}">
        <p><strong>${escapeHtml(formatRange(o.startsAt, o.endsAt))}</strong>
          ${o.frozen ? ` <span class="badge accepted">${I18N.t("chosenTime")}</span>` : ""}</p>
        <p class="muted">${I18N.t("yes")} ${o.yes} · ${I18N.t("maybe")} ${o.maybe} · ${I18N.t("no")} ${o.no} · ${I18N.t("unknown")} ${o.unknown}</p>
        ${d.pollOpen ? `<button type="button" class="btn" data-freeze="${o.id}">${I18N.t("freezePoll")}</button>` : ""}
      </article>`).join("")}
  ` : "";
  const voteBlock = d.pollOpen ? "" : `
    <h3>${I18N.t("votes")}</h3>
    <div class="counts">${counts}</div>
    <table>
      <thead><tr><th>${I18N.t("name")}</th><th>${I18N.t("subrole")}</th><th>${I18N.t("vote")}</th></tr></thead>
      <tbody>${rows || `<tr><td colspan="3" class="muted">${I18N.t("noPeopleRoles")}</td></tr>`}</tbody>
    </table>`;
  box.innerHTML = `
    ${poll}
    ${voteBlock}
    <div class="drawer-actions">
      <button type="button" class="btn ghost" id="btn-comments">${I18N.t("comments")}${commentCount ? ` (${commentCount})` : ""}</button>
    </div>`;
}

function resetUserForm() {
  selectedUser = "";
  document.getElementById("people-form-title").textContent = I18N.t("addPerson");
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
  document.getElementById("people-form-title").textContent = I18N.t("editPerson");
  document.getElementById("user-id").value = u.id;
  document.getElementById("user-nickname").value = u.nickname;
  document.getElementById("user-email").value = u.email;
  document.getElementById("user-password").value = "";
  document.getElementById("user-password").required = false;
  document.getElementById("pw-hint").textContent = I18N.t("passwordKeep");
  document.getElementById("user-role").value = u.role;
  fillSubroles();
  document.getElementById("user-subrole").value = u.subrole;
  document.getElementById("btn-user-delete").disabled = false;
  renderPeople();
}

function resetDateForm() {
  selectedDate = "";
  document.getElementById("date-form-title").textContent = I18N.t("addDate");
  document.getElementById("date-id").value = "";
  document.getElementById("date-title").value = "";
  document.getElementById("date-category").value = "event";
  document.getElementById("date-start").value = "";
  document.getElementById("date-end").value = "";
  document.getElementById("date-location").value = "";
  document.getElementById("date-notes").value = "";
  document.querySelectorAll("#date-roles input").forEach((el) => { el.checked = false; });
  setBringForm();
  setPollRows([], false);
  document.getElementById("btn-date-delete").disabled = true;
  renderDates();
  showError(dateError, "");
}

function fillDateForm(d) {
  selectedDate = d.id;
  document.getElementById("date-form-title").textContent = I18N.t("editDate");
  document.getElementById("date-id").value = d.id;
  document.getElementById("date-title").value = d.title;
  document.getElementById("date-category").value = d.category || "event";
  document.getElementById("date-start").value = toLocalInput(d.startsAt);
  document.getElementById("date-end").value = toLocalInput(d.endsAt);
  document.getElementById("date-location").value = d.location || "";
  document.getElementById("date-notes").value = d.notes || "";
  document.querySelectorAll("#date-roles input").forEach((el) => {
    el.checked = (d.roles || []).includes(el.value);
  });
  setBringForm(d.bring);
  setPollRows(d.options, !d.pollOpen && (d.options || []).length >= 2);
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
  renderRanking();
}

function pct(n) {
  return `${Math.round((n || 0) * 100)}%`;
}

function renderRanking() {
  const rank = state.ranking || { entries: [], bySubrole: [] };
  const leader = rank.leader;
  document.getElementById("ranking-stats").innerHTML = `
    <div class="rank-stat"><span>${rank.year || "—"}</span><label>${I18N.t("spiritOfTheYear")}</label>
      <p>${leader ? `${escapeHtml(leader.nickname)} · ${leader.score} ${I18N.t("spiritPoints")}` : I18N.t("spiritEmpty")}</p></div>
    <div class="rank-stat"><span>${rank.events || 0}</span><label data-i18n="choirEvents">${I18N.t("choirEvents")}</label></div>
    <div class="rank-stat"><span>${rank.members || 0}</span><label data-i18n="people">${I18N.t("people")}</label></div>
    <div class="rank-stat"><span>${(rank.avgScore || 0).toFixed(1)}</span><label data-i18n="avgScore">${I18N.t("avgScore")}</label></div>
    <div class="rank-stat"><span>${pct(rank.participation)}</span><label data-i18n="participation">${I18N.t("participation")}</label></div>
    <div class="rank-stat"><span>${rank.totalFlipped || 0}</span><label data-i18n="flipped">${I18N.t("flipped")}</label></div>
    ${(rank.bySubrole || []).map((s) => `
      <div class="rank-stat">
        <span>${escapeHtml(I18N.subrole(s.subrole))}</span>
        <label>${s.members} · ${(s.avgScore || 0).toFixed(1)} ${I18N.t("spiritPoints")}</label>
        <p>${I18N.t("yes")} ${s.yes} · ${I18N.t("maybe")} ${s.maybe} · ${I18N.t("no")} ${s.no}</p>
      </div>`).join("")}`;
  const body = document.getElementById("ranking-body");
  const rows = rank.entries || [];
  if (!rows.length) {
    body.innerHTML = `<tr><td colspan="9" class="muted">${I18N.t("noChoirMembers")}</td></tr>`;
    return;
  }
  let lastScore = null;
  let lastRank = 0;
  body.innerHTML = rows.map((e, i) => {
    if (e.score !== lastScore) {
      lastRank = i + 1;
      lastScore = e.score;
    }
    return `<tr class="${leader && e.userId === leader.userId ? "active" : ""}">
      <td>${lastRank}</td>
      <td>${escapeHtml(e.nickname)}</td>
      <td>${escapeHtml(I18N.subrole(e.subrole))}</td>
      <td><strong>${e.score}</strong></td>
      <td>${e.yes}</td>
      <td>${e.maybe}</td>
      <td>${e.no}</td>
      <td>${e.unknown}</td>
      <td>${e.flipped || 0}</td>
    </tr>`;
  }).join("");
}

async function boot() {
  try {
    catalog = await api("/api/catalog");
    fillRoleSelects();
    await loadState();
    document.getElementById("people-form-title").textContent = selectedUser ? I18N.t("editPerson") : I18N.t("addPerson");
    document.getElementById("date-form-title").textContent = selectedDate ? I18N.t("editDate") : I18N.t("addDate");
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
    document.getElementById("tab-ranking").hidden = tab.dataset.tab !== "ranking";
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
  if (!id || !confirm(I18N.t("confirmDeletePerson"))) return;
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
  syncPollMode();
  const id = document.getElementById("date-id").value;
  const body = {
    title: document.getElementById("date-title").value,
    category: document.getElementById("date-category").value,
    startsAt: toISO(document.getElementById("date-start").value),
    endsAt: toISO(document.getElementById("date-end").value),
    location: document.getElementById("date-location").value,
    notes: document.getElementById("date-notes").value,
    roles: [...document.querySelectorAll("#date-roles input:checked")].map((el) => el.value),
    bring: readBringForm(),
    options: readPollRows(),
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
  if (!id || !confirm(I18N.t("confirmDeleteDate"))) return;
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

document.getElementById("btn-add-poll").addEventListener("click", () => {
  if (pollFrozen) return;
  pollRows.push({ id: "", startsAt: "", endsAt: "" });
  renderPollRows();
});

document.getElementById("poll-options").addEventListener("input", (e) => {
  const input = e.target.closest("input[data-poll]");
  if (!input) return;
  const row = pollRows[Number(input.dataset.poll)];
  if (!row) return;
  if (input.dataset.field === "start") row.startsAt = input.value;
  if (input.dataset.field === "end") row.endsAt = input.value;
  syncPollMode();
});
document.getElementById("poll-options").addEventListener("change", () => syncPollMode());

document.getElementById("poll-options").addEventListener("click", (e) => {
  const btn = e.target.closest("[data-remove-poll]");
  if (!btn || pollFrozen) return;
  pollRows.splice(Number(btn.dataset.removePoll), 1);
  renderPollRows();
});

document.getElementById("date-detail").addEventListener("click", async (e) => {
  const freezeBtn = e.target.closest("[data-freeze]");
  if (freezeBtn && selectedDate) {
    showError(dateError, "");
    try {
      const data = await api(`/api/controller/dates/${selectedDate}/freeze`, {
        method: "POST",
        body: JSON.stringify({ optionId: freezeBtn.dataset.freeze }),
      });
      await loadState();
      fillDateForm(data.date);
    } catch (err) {
      showError(dateError, err.message);
    }
    return;
  }
  if (!e.target.closest("#btn-comments") || !selectedDate) return;
  const d = state.dates.find((x) => x.id === selectedDate);
  if (!d) return;
  document.getElementById("comment-title").textContent = d.title;
  const items = d.comments || [];
  document.getElementById("comment-list").innerHTML = items.length
    ? items.map((c) => `
      <article class="comment-card">
        <p class="meta">${escapeHtml(c.nickname)} · ${escapeHtml(formatWhen(c.createdAt))}</p>
        <p>${escapeHtml(c.text)}</p>
      </article>`).join("")
    : `<p class="muted">${I18N.t("noComments")}</p>`;
  document.getElementById("comment-dialog").showModal();
});

document.getElementById("comment-close").addEventListener("click", () => {
  document.getElementById("comment-dialog").close();
});

function connectWS() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${proto}://${location.host}/ws/controller`);
  ws.onmessage = () => { loadState().catch(() => {}); };
  ws.onclose = () => { if (!dash.hidden) setTimeout(connectWS, 2000); };
}

I18N.onChange(() => {
  I18N.apply();
  if (catalog.roles.length) fillRoleSelects();
  const peopleTitle = document.getElementById("people-form-title");
  const dateTitle = document.getElementById("date-form-title");
  peopleTitle.textContent = selectedUser ? I18N.t("editPerson") : I18N.t("addPerson");
  dateTitle.textContent = selectedDate ? I18N.t("editDate") : I18N.t("addDate");
  const pwHint = document.getElementById("pw-hint");
  if (selectedUser) pwHint.textContent = I18N.t("passwordKeep");
  renderPollRows();
  if (!dash.hidden) {
    renderPeople();
    renderDates();
    renderRanking();
    const dialog = document.getElementById("comment-dialog");
    if (dialog.open && selectedDate) {
      const d = state.dates.find((x) => x.id === selectedDate);
      if (d) {
        document.getElementById("comment-title").textContent = d.title;
        const items = d.comments || [];
        document.getElementById("comment-list").innerHTML = items.length
          ? items.map((c) => `
            <article class="comment-card">
              <p class="meta">${escapeHtml(c.nickname)} · ${escapeHtml(formatWhen(c.createdAt))}</p>
              <p>${escapeHtml(c.text)}</p>
            </article>`).join("")
          : `<p class="muted">${I18N.t("noComments")}</p>`;
      }
    }
  }
});

boot();
