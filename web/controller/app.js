
const gate = document.getElementById("gate");
const dash = document.getElementById("dash");
const gateError = document.getElementById("gate-error");
const peopleError = document.getElementById("people-error");
const dateError = document.getElementById("date-error");

const CHOIR_VOICES = ["Sopran", "Alt", "Tenor/Bass"];

let catalog = { roles: [], categories: [] };
let state = { users: [], dates: [], online: 0, ranking: { entries: [], bySubrole: [] }, archive: [], channels: [], proposals: [] };
let selectedUser = "";
let selectedDate = "";
let selectedArchive = "";
let dateTitleIDs = [];
let dateFormClean = "";
let pollRows = [];
let pollFrozen = false;
let pendingPhoto = null;
let pendingPhotoURL = "";
let pendingInfo = { address: "", phone: "", birthday: "", altEmail: "" };
let chatRoom = "";
let chatMessages = [];
let chatUnread = {};
let galleryDateId = "";
let galleryItems = [];

const CHAT_ROOMS = ["choir", "band", "orchestra", "live"];
const CHAT_API = "/api/controller/chats";
const VOICE_MAX_MS = 120000;
const CHAT_EMOJIS = ["👍", "❤️", "😂", "😮", "😢", "🎉"];
const COMPOSE_EMOJIS = [
  "😀", "😂", "😊", "😍", "🥰", "😘", "😎", "🤩", "🥳", "😇",
  "😉", "😜", "🤔", "🙄", "😴", "😭", "😤", "😮", "😱", "🥺",
  "👍", "👎", "👏", "🙌", "🙏", "💪", "✌️", "👋", "❤️", "🔥",
  "⭐", "✨", "🎉", "💯", "👀", "✅", "🎵", "🎶", "🎤", "🎸",
  "🎹", "🥁",
];
const MEMBER_COLORS = ["#3dd6c6", "#f0a35e", "#8cb4ff", "#e38cff", "#7fd99a", "#f07178", "#ffd166", "#9ad0c8"];

function currentPerson() {
  return state.users.find((u) => u.id === selectedUser) || {
    nickname: document.getElementById("user-nickname")?.value || "",
    ...pendingInfo,
  };
}

function hasInfo(user) {
  return !!(user?.address || user?.phone || user?.birthday || user?.altEmail);
}

function paintInfoButton(btn, user) {
  if (!btn) return;
  btn.textContent = I18N.t(user?.birthday ? "modifyInfo" : "addInfo");
  btn.classList.toggle("has-info", hasInfo(user));
}

function formatBirthday(iso) {
  if (!iso) return "";
  const parts = String(iso).split("-");
  if (parts.length !== 3) return iso;
  const d = new Date(Number(parts[0]), Number(parts[1]) - 1, Number(parts[2]));
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString(I18N.locale(), { day: "numeric", month: "short", year: "numeric" });
}

function paintPersonPhoto() {
  const person = currentPerson();
  Photo.paint(document.getElementById("user-photo"), person, pendingPhotoURL);
  paintInfoButton(document.getElementById("user-info"), person);
}

function fillInfoForm(user = {}) {
  document.getElementById("info-address").value = user.address || "";
  document.getElementById("info-phone").value = user.phone || "";
  document.getElementById("info-alt-email").value = user.altEmail || "";
  document.getElementById("info-birthday").value = user.birthday || "";
  showError(document.getElementById("info-error"), "");
}

function readInfoForm() {
  return {
    address: document.getElementById("info-address").value,
    phone: document.getElementById("info-phone").value,
    altEmail: document.getElementById("info-alt-email").value,
    birthday: document.getElementById("info-birthday").value,
  };
}

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

function dateFormSnapshot() {
  return JSON.stringify({
    title: document.getElementById("date-title").value,
    category: document.getElementById("date-category").value,
    start: document.getElementById("date-start").value,
    end: document.getElementById("date-end").value,
    location: document.getElementById("date-location").value,
    notes: document.getElementById("date-notes").value,
    schedule: document.getElementById("date-schedule").value,
    roles: [...document.querySelectorAll("#date-roles input:checked")].map((el) => el.value).sort(),
    bring: readBringForm(),
    options: pollRows.map((r) => ({ id: r.id || "", startsAt: r.startsAt || "", endsAt: r.endsAt || "" })),
    titleIds: dateTitleIDs.slice(),
  });
}

function paintDateSave() {
  document.getElementById("btn-date-save")?.classList.toggle("dirty", dateFormSnapshot() !== dateFormClean);
}

function captureDateForm() {
  dateFormClean = dateFormSnapshot();
  paintDateSave();
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
      <td>${Photo.html(u, "sm")}</td>
      <td>${escapeHtml(u.nickname)}</td>
      <td>${escapeHtml(u.email)}</td>
      <td>${escapeHtml(I18N.role(u.role))}</td>
      <td>${escapeHtml(I18N.subrole(u.subrole))}</td>
      <td>${u.streamer ? escapeHtml(I18N.t("streamer")) : "—"}</td>
    </tr>
  `).join("");
}

function renderContacts() {
  const body = document.getElementById("contacts-body");
  if (!state.users.length) {
    body.innerHTML = `<tr><td colspan="7" class="muted">${I18N.t("noContacts")}</td></tr>`;
    return;
  }
  body.innerHTML = state.users.map((u) => `
    <tr data-id="${u.id}">
      <td>${Photo.html(u, "sm")}</td>
      <td>${escapeHtml(u.nickname)}</td>
      <td>${escapeHtml(u.email)}</td>
      <td>${escapeHtml(u.altEmail || "")}</td>
      <td>${escapeHtml(u.address || "")}</td>
      <td>${escapeHtml(u.phone || "")}</td>
      <td>${escapeHtml(formatBirthday(u.birthday))}</td>
    </tr>
  `).join("");
}

function showTab(name) {
  document.querySelectorAll(".tab").forEach((t) => t.classList.toggle("active", t.dataset.tab === name));
  document.getElementById("tab-people").hidden = name !== "people";
  document.getElementById("tab-contacts").hidden = name !== "contacts";
  document.getElementById("tab-dates").hidden = name !== "dates";
  document.getElementById("tab-archive").hidden = name !== "archive";
  document.getElementById("tab-channels").hidden = name !== "channels";
  document.getElementById("tab-proposals").hidden = name !== "proposals";
  document.getElementById("tab-ranking").hidden = name !== "ranking";
  document.getElementById("tab-settings").hidden = name !== "settings";
}

function choirVoiceYes(date) {
  const counts = Object.fromEntries(CHOIR_VOICES.map((v) => [v, 0]));
  if (date.pollOpen && (date.options || []).length) {
    for (const voice of CHOIR_VOICES) {
      counts[voice] = Math.max(0, ...(date.options || []).map((o) =>
        (o.roster || []).filter((e) => e.role === "choir" && e.subrole === voice && e.choice === "yes" && !e.attendance).length));
    }
    return counts;
  }
  for (const e of date.roster || []) {
    if (e.role === "choir" && e.choice === "yes" && !e.attendance && counts[e.subrole] !== undefined) {
      counts[e.subrole]++;
    }
  }
  return counts;
}

function participationMood(date) {
  if (!(date.roles || []).includes("choir")) return null;
  const counts = choirVoiceYes(date);
  const min = Math.min(...CHOIR_VOICES.map((v) => counts[v]));
  if (min < 2) return { emoji: "😰", key: "moodLow" };
  if (min === 2) return { emoji: "😟", key: "moodWorry" };
  if (min === 3) return { emoji: "🙂", key: "moodOk" };
  return { emoji: "😄", key: "moodGreat" };
}

function moodHTML(date) {
  const mood = participationMood(date);
  if (!mood) return "";
  const counts = choirVoiceYes(date);
  const detail = CHOIR_VOICES.map((v) => `${I18N.subrole(v)} ${counts[v]}`).join(" · ");
  return `<span class="mood" title="${escapeHtml(detail)}" aria-label="${escapeHtml(I18N.t(mood.key))}">${mood.emoji}</span>`;
}

function renderDates() {
  document.getElementById("stat-dates").textContent = state.dates.length;
  document.getElementById("stat-online").textContent = state.online;
  document.getElementById("date-list").innerHTML = state.dates.map((d) => `
    <article class="date-item ${d.id === selectedDate ? "active" : ""}" data-id="${d.id}">
      ${moodHTML(d)}
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
    const mark = e.attendance === "absent"
      ? ` <span class="badge no">${I18N.t("absent")}</span>`
      : e.attendance === "excused"
        ? ` <span class="badge maybe">${I18N.t("excused")}</span>`
        : "";
    const actions = e.choice === "yes"
      ? `<div class="attendance-actions">
          <button type="button" class="btn ghost${e.attendance === "absent" ? " on" : ""}" data-attendance="absent" data-user="${e.userId}" aria-pressed="${e.attendance === "absent"}">${I18N.t("absent")}</button>
          <button type="button" class="btn ghost${e.attendance === "excused" ? " on" : ""}" data-attendance="excused" data-user="${e.userId}" aria-pressed="${e.attendance === "excused"}">${I18N.t("excused")}</button>
        </div>`
      : "";
    return `<tr><td>${escapeHtml(e.nickname)}</td><td>${escapeHtml(I18N.subrole(e.subrole))}</td><td>${vote}${mark}</td><td>${actions}</td></tr>`;
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
      <thead><tr><th>${I18N.t("name")}</th><th>${I18N.t("subrole")}</th><th>${I18N.t("vote")}</th><th>${I18N.t("attendance")}</th></tr></thead>
      <tbody>${rows || `<tr><td colspan="4" class="muted">${I18N.t("noPeopleRoles")}</td></tr>`}</tbody>
    </table>`;
  box.innerHTML = `
    ${poll}
    ${voteBlock}
    <div class="drawer-actions">
      <button type="button" class="btn ghost" id="btn-comments">${I18N.t("comments")}${commentCount ? ` (${commentCount})` : ""}</button>
      <button type="button" class="btn ghost" id="btn-titles">${I18N.t("titles")}${(d.titles || []).length ? ` (${d.titles.length})` : ""}</button>
      ${d.chatOpen ? `<button type="button" class="btn ghost" id="btn-event-chat">${I18N.t("eventChat")}</button>` : ""}
      <button type="button" class="btn ghost" id="btn-gallery">${I18N.t("gallery")}${d.galleryCount ? ` (${d.galleryCount})` : ""}</button>
      <button type="button" class="btn ghost" id="btn-promo">${I18N.t("promo")}${d.promoCount ? ` (${d.promoCount})` : ""}</button>
    </div>`;
}

function clearPendingPhoto() {
  if (pendingPhotoURL) URL.revokeObjectURL(pendingPhotoURL);
  pendingPhoto = null;
  pendingPhotoURL = "";
}

function resetUserForm() {
  selectedUser = "";
  pendingInfo = { address: "", phone: "", birthday: "", altEmail: "" };
  clearPendingPhoto();
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
  document.getElementById("user-streamer").checked = false;
  renderPeople();
  paintPersonPhoto();
  paintUserChannels();
  showError(peopleError, "");
}

function fillUserForm(u) {
  selectedUser = u.id;
  pendingInfo = { address: u.address || "", phone: u.phone || "", birthday: u.birthday || "", altEmail: u.altEmail || "" };
  clearPendingPhoto();
  document.getElementById("people-form-title").textContent = I18N.t("editPerson");
  document.getElementById("user-id").value = u.id;
  document.getElementById("user-nickname").value = u.nickname;
  document.getElementById("user-email").value = u.email;
  document.getElementById("user-password").value = "";
  document.getElementById("user-password").required = false;
  document.getElementById("pw-hint").textContent = u.mustChangePassword ? I18N.t("passwordPending") : I18N.t("passwordKeep");
  document.getElementById("user-role").value = u.role;
  fillSubroles();
  document.getElementById("user-subrole").value = u.subrole;
  document.getElementById("user-streamer").checked = !!u.streamer;
  document.getElementById("btn-user-delete").disabled = false;
  renderPeople();
  paintPersonPhoto();
  paintUserChannels();
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
  document.getElementById("date-schedule").value = "";
  document.querySelectorAll("#date-roles input").forEach((el) => { el.checked = false; });
  setBringForm();
  setPollRows([], false);
  dateTitleIDs = [];
  const pickSearch = document.getElementById("archive-pick-search");
  if (pickSearch) pickSearch.value = "";
  clearTitleNewForm();
  document.getElementById("btn-date-delete").disabled = true;
  renderDates();
  renderDateTitles();
  showError(dateError, "");
  captureDateForm();
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
  document.getElementById("date-schedule").value = d.schedule || "";
  document.querySelectorAll("#date-roles input").forEach((el) => {
    el.checked = (d.roles || []).includes(el.value);
  });
  setBringForm(d.bring);
  setPollRows(d.options, !d.pollOpen && (d.options || []).length >= 2);
  dateTitleIDs = (d.titles || []).map((t) => t.id);
  const pickSearch = document.getElementById("archive-pick-search");
  if (pickSearch) pickSearch.value = "";
  clearTitleNewForm();
  document.getElementById("btn-date-delete").disabled = false;
  renderDates();
  renderDateTitles();
  captureDateForm();
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
  renderContacts();
  renderRanking();
  renderArchive();
  renderChannels();
  renderProposals();
  fillSettingsForm();
  paintPersonPhoto();
  paintUserChannels();
  if (document.getElementById("gallery-dialog")?.open && galleryDateId) {
    api(`/api/controller/dates/${encodeURIComponent(galleryDateId)}/gallery`).then((data) => {
      galleryItems = data.gallery || [];
      renderGallery();
    }).catch(() => {});
  }
}

function fillSettingsForm() {
  const input = document.getElementById("admin-alias");
  const ticker = document.getElementById("news-ticker");
  if (!input || !ticker) return;
  input.value = state.adminAlias || "Admin";
  ticker.value = state.newsTicker || "";
  showError(document.getElementById("settings-error"), "");
}

function channelPeople() {
  return (state.users || []).filter((u) => ["choir", "chorleiter", "band", "orchestra", "ehemalige"].includes(u.role));
}

function paintUserChannels() {
  const el = document.getElementById("user-channels");
  if (!el) return;
  const user = state.users.find((u) => u.id === selectedUser);
  const channels = user?.channels || [];
  el.hidden = channels.length === 0;
  el.textContent = channels.map((ch) => {
    const note = ch.comment ? ` · ${ch.comment}` : "";
    const v48 = ch.v48 ? ` · ${I18N.t("channel48v")}` : "";
    return `${I18N.t("channel")} ${ch.number}${note}${v48}`;
  }).join(" · ");
}

function renderChannels() {
  const body = document.getElementById("channels-body");
  if (!body) return;
  const q = (document.getElementById("channel-search")?.value || "").trim().toLowerCase();
  const people = channelPeople();
  const channels = state.channels || [];
  body.innerHTML = channels.filter((ch) => {
    if (!q) return true;
    const hay = `${ch.number} ${ch.nickname || ""} ${ch.comment || ""} ${I18N.role(ch.role || "")}${ch.v48 ? " 48v" : ""}`.toLowerCase();
    return hay.includes(q);
  }).map((ch) => {
    const opts = [`<option value="">${I18N.t("channelNone")}</option>`]
      .concat(people.map((u) => `<option value="${u.id}" ${u.id === ch.userId ? "selected" : ""}>${escapeHtml(u.nickname)} · ${escapeHtml(I18N.role(u.role))}</option>`))
      .join("");
    return `<tr>
      <td>${ch.number}</td>
      <td><select data-channel-user="${ch.number}">${opts}</select></td>
      <td><input data-channel-comment="${ch.number}" value="${escapeHtml(ch.comment || "")}" maxlength="200" /></td>
      <td><button type="button" class="btn ghost v48-btn${ch.v48 ? " on" : ""}" data-channel-v48="${ch.number}" aria-pressed="${ch.v48 ? "true" : "false"}">${I18N.t("channel48v")}</button></td>
    </tr>`;
  }).join("");
}

async function saveChannel(number, v48) {
  const userEl = document.querySelector(`[data-channel-user="${number}"]`);
  const commentEl = document.querySelector(`[data-channel-comment="${number}"]`);
  const v48El = document.querySelector(`[data-channel-v48="${number}"]`);
  if (!userEl || !commentEl) return;
  const phantom = typeof v48 === "boolean" ? v48 : v48El?.getAttribute("aria-pressed") === "true";
  showError(document.getElementById("channels-error"), "");
  try {
    const data = await api(`/api/controller/channels/${number}`, {
      method: "PATCH",
      body: JSON.stringify({ userId: userEl.value, comment: commentEl.value, v48: !!phantom }),
    });
    const idx = (state.channels || []).findIndex((c) => c.number === number);
    if (idx >= 0) state.channels[idx] = data.channel;
    (state.users || []).forEach((u) => {
      u.channels = (u.channels || []).filter((c) => c.number !== number);
    });
    if (data.channel?.userId) {
      const u = state.users.find((x) => x.id === data.channel.userId);
      if (u) {
        u.channels = [...(u.channels || []), data.channel].sort((a, b) => a.number - b.number);
      }
    }
    if (v48El) {
      v48El.classList.toggle("on", !!data.channel.v48);
      v48El.setAttribute("aria-pressed", data.channel.v48 ? "true" : "false");
    }
    paintUserChannels();
  } catch (err) {
    showError(document.getElementById("channels-error"), err.message);
  }
}

function proposalBadge(status) {
  if (status === "accepted") return "accepted";
  if (status === "declined") return "cancelled";
  return "voting";
}

function proposalLink(url) {
  if (!url) return "—";
  return `<a href="${escapeHtml(url)}" target="_blank" rel="noopener noreferrer">${escapeHtml(url)}</a>`;
}

function renderProposals() {
  const body = document.getElementById("proposals-body");
  if (!body) return;
  const q = (document.getElementById("proposal-search")?.value || "").trim().toLowerCase();
  const items = (state.proposals || []).filter((p) => {
    if (!q) return true;
    const hay = `${p.title} ${p.url || ""} ${p.nickname} ${p.status} ${I18N.status(p.status)} ${p.comment || ""}`.toLowerCase();
    return hay.includes(q);
  });
  if (!items.length) {
    body.innerHTML = `<tr><td colspan="6" class="muted">${I18N.t("noProposals")}</td></tr>`;
    return;
  }
  body.innerHTML = items.map((p) => `
    <tr data-proposal="${p.id}">
      <td>
        <strong>${escapeHtml(p.title)}</strong>
        <p class="proposal-by">${escapeHtml(I18N.t("proposedBy"))} <b>${escapeHtml(p.nickname || "—")}</b></p>
      </td>
      <td>${escapeHtml(p.nickname || "—")}</td>
      <td class="proposal-url">${proposalLink(p.url)}</td>
      <td><span class="badge ${proposalBadge(p.status)}">${escapeHtml(I18N.status(p.status))}</span></td>
      <td><input data-proposal-comment="${p.id}" value="${escapeHtml(p.comment || "")}" maxlength="2000" /></td>
      <td class="proposal-actions">
        ${p.status !== "accepted" ? `<button type="button" class="btn ghost" data-proposal-status="accepted">${I18N.t("acceptProposal")}</button>` : ""}
        ${p.status !== "declined" ? `<button type="button" class="btn ghost" data-proposal-status="declined">${I18N.t("declineProposal")}</button>` : ""}
        <button type="button" class="btn ghost danger" data-proposal-delete>${I18N.t("delete")}</button>
      </td>
    </tr>
  `).join("");
}

async function patchProposal(id, body) {
  showError(document.getElementById("proposals-error"), "");
  try {
    await api(`/api/controller/proposals/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    });
    await loadState();
  } catch (err) {
    showError(document.getElementById("proposals-error"), err.message);
  }
}

function archiveItems() {
  return state.archive || [];
}

function archiveByID(id) {
  return archiveItems().find((x) => x.id === id);
}

function archiveFileURL(id, file) {
  if (!file?.id) return "#";
  const q = new URLSearchParams();
  if (file.updatedAt) q.set("v", file.updatedAt);
  const qs = q.toString();
  return `/api/controller/archive/${encodeURIComponent(id)}/files/${encodeURIComponent(file.id)}${qs ? `?${qs}` : ""}`;
}

function archiveRoleLabel(role) {
  const id = String(role || "").trim();
  if (!id) return "";
  if (["choir", "chorleiter", "band", "orchestra", "technician", "ehemalige"].includes(id)) return I18N.role(id);
  return id;
}

function archiveKindAccept(kind) {
  if (kind === "audio" || kind === "tracks") return "audio/*";
  if (kind === "lyrics") return "application/pdf,text/plain,image/*";
  if (kind === "midi") return ".mid,.midi,.kar,.xml,.musicxml,.mxl,audio/midi,application/xml,application/vnd.recordare.musicxml+xml,application/vnd.recordare.musicxml";
  return "application/pdf,image/*";
}

function archiveFilesOf(item, kind) {
  return (item.files || []).filter((f) => f.kind === kind);
}

function archiveQuery() {
  return (document.getElementById("archive-search")?.value || "").trim().toLowerCase();
}

function renderArchive() {
  const q = archiveQuery();
  const rows = archiveItems().filter((item) => {
    if (!q) return true;
    return `${item.title} ${item.composer || ""}`.toLowerCase().includes(q);
  });
  const body = document.getElementById("archive-body");
  const empty = document.getElementById("archive-empty");
  empty.hidden = rows.length > 0;
  body.innerHTML = rows.map((item) => {
    const sheetRoles = archiveFilesOf(item, "sheet").map((f) => archiveRoleLabel(f.role) || f.name).filter(Boolean).join(", ") || "—";
    const count = (kind) => {
      const n = archiveFilesOf(item, kind).length;
      return n ? String(n) : "—";
    };
    return `<tr data-id="${item.id}" class="${item.id === selectedArchive ? "active" : ""}">
      <td>${escapeHtml(item.title)}</td>
      <td>${escapeHtml(item.composer || "")}</td>
      <td>${count("audio")}</td>
      <td>${count("tracks")}</td>
      <td>${count("lyrics")}</td>
      <td>${escapeHtml(sheetRoles)}</td>
      <td>${count("midi")}</td>
      <td>${count("link")}</td>
    </tr>`;
  }).join("");
  renderArchiveFiles();
  renderDateTitleSelects();
}

function resetArchiveForm() {
  selectedArchive = "";
  document.getElementById("archive-form-title").textContent = I18N.t("newItem");
  document.getElementById("archive-id").value = "";
  document.getElementById("archive-title").value = "";
  document.getElementById("archive-composer").value = "";
  document.getElementById("btn-archive-delete").disabled = true;
  showError(document.getElementById("archive-error"), "");
  renderArchive();
}

function fillArchiveForm(item) {
  selectedArchive = item.id;
  document.getElementById("archive-form-title").textContent = item.title;
  document.getElementById("archive-id").value = item.id;
  document.getElementById("archive-title").value = item.title;
  document.getElementById("archive-composer").value = item.composer || "";
  document.getElementById("btn-archive-delete").disabled = false;
  showError(document.getElementById("archive-error"), "");
  renderArchive();
}

function renderArchiveFiles() {
  const box = document.getElementById("archive-files");
  const hint = document.getElementById("archive-files-hint");
  const item = archiveByID(selectedArchive);
  if (!item) {
    box.hidden = true;
    hint.hidden = false;
    box.innerHTML = "";
    return;
  }
  hint.hidden = true;
  box.hidden = false;
  const kinds = [
    { kind: "audio", label: I18N.t("archiveAudio") },
    { kind: "tracks", label: I18N.t("archiveTracks") },
    { kind: "lyrics", label: I18N.t("archiveLyrics") },
    { kind: "sheet", label: I18N.t("archiveSheet") },
    { kind: "midi", label: I18N.t("archiveMIDI") },
  ];
  const fileSections = kinds.map((slot) => {
    const files = archiveFilesOf(item, slot.kind);
    const rows = files.length
      ? files.map((file) => `
        <div class="archive-file" data-file="${file.id}">
          <div class="archive-file-fields">
            <label>
              <span>${I18N.t("archiveFileName")}</span>
              <input data-file-name value="${escapeHtml(file.name || "")}" maxlength="120" />
            </label>
            <label>
              <span>${I18N.t("archiveFileRole")}</span>
              <input data-file-role value="${escapeHtml(file.role || "")}" maxlength="40" placeholder="${escapeHtml(I18N.t("archiveRoleHint"))}" />
            </label>
          </div>
          <div class="archive-file-actions">
            <a class="btn ghost" href="${archiveFileURL(item.id, file)}" target="_blank" rel="noopener">${I18N.t("fileOpen")}</a>
            <button type="button" class="btn ghost danger" data-clear-file="${file.id}">${I18N.t("delete")}</button>
          </div>
        </div>`).join("")
      : `<p class="muted">${I18N.t("archiveNoFile")}</p>`;
    return `<section class="archive-kind">
      <p>${escapeHtml(slot.label)}</p>
      ${rows}
      <button type="button" class="btn ghost" data-upload="${slot.kind}" data-accept="${archiveKindAccept(slot.kind)}">${I18N.t("archiveAddFile")}</button>
    </section>`;
  }).join("");
  const links = archiveFilesOf(item, "link");
  const linkRows = links.length
    ? links.map((file) => `
      <div class="archive-file" data-file="${file.id}" data-link="1">
        <div class="archive-file-fields">
          <label>
            <span>${I18N.t("archiveURLName")}</span>
            <input data-file-name value="${escapeHtml(file.name || "")}" maxlength="120" />
          </label>
          <label>
            <span>${I18N.t("archiveURL")}</span>
            <input data-file-url value="${escapeHtml(file.url || "")}" maxlength="2000" />
          </label>
        </div>
        <div class="archive-file-actions">
          <a class="btn ghost" href="${escapeHtml(file.url || "#")}" target="_blank" rel="noopener">${I18N.t("fileOpen")}</a>
          <button type="button" class="btn ghost danger" data-clear-file="${file.id}">${I18N.t("delete")}</button>
        </div>
      </div>`).join("")
    : `<p class="muted">${I18N.t("archiveNoLink")}</p>`;
  box.innerHTML = `${fileSections}
    <section class="archive-kind">
      <p>${escapeHtml(I18N.t("archiveShareURL"))}</p>
      ${linkRows}
      <div class="archive-link-add">
        <input data-new-link-name maxlength="120" placeholder="${escapeHtml(I18N.t("archiveURLName"))}" />
        <input data-new-link-url maxlength="2000" placeholder="https://" />
        <button type="button" class="btn ghost" data-add-link>${I18N.t("archiveAddURL")}</button>
      </div>
    </section>`;
}

function archivePickQuery() {
  return (document.getElementById("archive-pick-search")?.value || "").trim().toLowerCase();
}

function renderDateTitleSelects() {
  const inherit = document.getElementById("inherit-titles");
  if (inherit) {
    inherit.innerHTML = `<option value="">${I18N.t("inheritTitlesPick")}</option>` +
      state.dates
        .filter((d) => d.id !== selectedDate && (d.titles || []).length)
        .map((d) => `<option value="${d.id}">${escapeHtml(d.title)}</option>`)
        .join("");
  }
  renderArchivePick();
}

function renderArchivePick() {
  const list = document.getElementById("archive-pick-list");
  const empty = document.getElementById("archive-pick-empty");
  const search = document.getElementById("archive-pick-search");
  if (!list || !empty) return;
  if (search) search.placeholder = I18N.t("archivePickSearch");
  const q = archivePickQuery();
  if (!q) {
    list.innerHTML = "";
    empty.hidden = false;
    empty.textContent = I18N.t("archivePickHint");
    return;
  }
  const rows = archiveItems()
    .filter((item) => !dateTitleIDs.includes(item.id))
    .filter((item) => `${item.title} ${item.composer || ""}`.toLowerCase().includes(q));
  if (!rows.length) {
    list.innerHTML = "";
    empty.hidden = false;
    empty.textContent = I18N.t("archivePickEmpty");
    return;
  }
  empty.hidden = true;
  list.innerHTML = rows.map((item) => `
    <button type="button" class="archive-pick-item" data-add-title="${item.id}">
      <strong>${escapeHtml(item.title)}</strong>
      ${item.composer ? `<span>${escapeHtml(item.composer)}</span>` : ""}
    </button>`).join("");
}

function addTitleToDate(id) {
  if (!id || dateTitleIDs.includes(id)) return;
  dateTitleIDs.push(id);
  renderDateTitles();
  paintDateSave();
}

function clearTitleNewForm() {
  const name = document.getElementById("title-new-name");
  const composer = document.getElementById("title-new-composer");
  if (name) name.value = "";
  if (composer) composer.value = "";
  showError(document.getElementById("title-new-error"), "");
}

async function createTitleForDate() {
  const errEl = document.getElementById("title-new-error");
  showError(errEl, "");
  const title = document.getElementById("title-new-name")?.value || "";
  const composer = document.getElementById("title-new-composer")?.value || "";
  try {
    const data = await api("/api/controller/archive", {
      method: "POST",
      body: JSON.stringify({ title, composer }),
    });
    const item = data.item;
    if (item) {
      state.archive = [...archiveItems().filter((x) => x.id !== item.id), item];
      addTitleToDate(item.id);
    }
    clearTitleNewForm();
    renderArchive();
  } catch (err) {
    showError(errEl, err.message);
  }
}

function renderDateTitles() {
  const box = document.getElementById("date-titles");
  if (!box) return;
  const copyBtn = document.getElementById("date-titles-copy");
  if (copyBtn) copyBtn.disabled = dateTitleIDs.length === 0;
  box.innerHTML = dateTitleIDs.map((id, i) => {
    const item = archiveByID(id);
    const title = item?.title || id;
    const composer = item?.composer || "";
    return `<div class="title-pick" data-title="${id}">
      <div>
        <strong>${escapeHtml(title)}</strong>
        ${composer ? `<span>${escapeHtml(composer)}</span>` : ""}
      </div>
      <div class="title-pick-actions">
        <button type="button" class="btn ghost" data-move="-1" ${i === 0 ? "disabled" : ""}>↑</button>
        <button type="button" class="btn ghost" data-move="1" ${i === dateTitleIDs.length - 1 ? "disabled" : ""}>↓</button>
        <button type="button" class="btn ghost danger" data-remove-title>${I18N.t("removeTitle")}</button>
      </div>
    </div>`;
  }).join("");
  renderDateTitleSelects();
}

async function uploadArchiveFile(file, kind, role) {
  const fd = new FormData();
  fd.append("file", file);
  fd.append("kind", kind);
  if (role) fd.append("role", role);
  const res = await fetch(`/api/controller/archive/${encodeURIComponent(selectedArchive)}/files`, {
    method: "POST",
    credentials: "same-origin",
    body: fd,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(I18N.error(data.error || res.statusText));
  return data;
}

async function saveArchiveFileMeta(fileId, name, role, url) {
  const body = { name, role };
  if (url !== undefined) body.url = url;
  return api(`/api/controller/archive/${encodeURIComponent(selectedArchive)}/files/${encodeURIComponent(fileId)}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
}

async function addArchiveLink(url, name) {
  return api(`/api/controller/archive/${encodeURIComponent(selectedArchive)}/links`, {
    method: "POST",
    body: JSON.stringify({ url, name }),
  });
}

function pct(n) {
  return `${Math.round((n || 0) * 100)}%`;
}

function rankingLeaders(rank) {
  if (rank?.leaders?.length) return rank.leaders;
  return rank?.leader ? [rank.leader] : [];
}

function renderRanking() {
  const rank = state.ranking || { entries: [], bySubrole: [] };
  const leaders = rankingLeaders(rank);
  const leaderIds = new Set(leaders.map((l) => l.userId));
  const leaderLine = leaders.length
    ? `${leaders.map((l) => escapeHtml(l.nickname)).join(" · ")} · ${leaders[0].score} ${I18N.t("spiritPoints")}`
    : I18N.t("spiritEmpty");
  document.getElementById("ranking-stats").innerHTML = `
    <div class="rank-stat"><span>${rank.year || "—"}</span><label>${I18N.t("spiritOfTheYear")}</label>
      <p>${leaderLine}</p></div>
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
    return `<tr class="${leaderIds.has(e.userId) ? "active" : ""}">
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
    if (!selectedDate) captureDateForm();
    gate.hidden = true;
    dash.hidden = false;
    renderChatTabs();
    paintChatSize();
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
  closeChat();
  await api("/api/controller/logout", { method: "POST" });
  await boot();
});

document.querySelectorAll(".tab").forEach((tab) => {
  tab.addEventListener("click", () => showTab(tab.dataset.tab));
});

document.getElementById("people-body").addEventListener("click", (e) => {
  const row = e.target.closest("tr[data-id]");
  if (!row) return;
  const user = state.users.find((u) => u.id === row.dataset.id);
  if (user) fillUserForm(user);
});

document.getElementById("contacts-body").addEventListener("click", (e) => {
  const row = e.target.closest("tr[data-id]");
  if (!row) return;
  const user = state.users.find((u) => u.id === row.dataset.id);
  if (!user) return;
  fillUserForm(user);
  showTab("people");
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
    streamer: document.getElementById("user-streamer").checked,
  };
  try {
    const data = id
      ? await api(`/api/controller/users/${id}`, { method: "PATCH", body: JSON.stringify(body) })
      : await api("/api/controller/users", { method: "POST", body: JSON.stringify(body) });
    let user = data.user;
    if (!id && user?.id && hasInfo(pendingInfo)) {
      const info = await api(`/api/controller/users/${user.id}/info`, {
        method: "PATCH",
        body: JSON.stringify(pendingInfo),
      });
      user = info.user;
    }
    if (pendingPhoto && user?.id) {
      const photo = await Photo.upload(`/api/controller/users/${user.id}/photo`, pendingPhoto);
      user = photo.user;
    }
    await loadState();
    fillUserForm(user);
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

document.getElementById("date-form").addEventListener("input", paintDateSave);
document.getElementById("date-form").addEventListener("change", paintDateSave);

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
    schedule: document.getElementById("date-schedule").value,
    roles: [...document.querySelectorAll("#date-roles input:checked")].map((el) => el.value),
    bring: readBringForm(),
    options: readPollRows(),
    titleIds: dateTitleIDs,
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
  paintDateSave();
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
  paintDateSave();
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
  const attendBtn = e.target.closest("[data-attendance]");
  if (attendBtn && selectedDate) {
    const next = attendBtn.getAttribute("aria-pressed") === "true" ? "" : attendBtn.dataset.attendance;
    showError(dateError, "");
    try {
      const data = await api(`/api/controller/dates/${selectedDate}/attendance`, {
        method: "POST",
        body: JSON.stringify({ userId: attendBtn.dataset.user, attendance: next }),
      });
      await loadState();
      fillDateForm(data.date);
    } catch (err) {
      showError(dateError, err.message);
    }
    return;
  }
  if (e.target.closest("#btn-titles")) {
    document.getElementById("date-titles")?.scrollIntoView({ behavior: "smooth", block: "center" });
    return;
  }
  if (e.target.closest("#btn-gallery") && selectedDate) {
    openGallery(selectedDate).catch((err) => alert(err.message));
    return;
  }
  if (e.target.closest("#btn-event-chat") && selectedDate) {
    const event = state.dates.find((x) => x.id === selectedDate);
    if (!event) return;
    try {
      await openChat(`event:${event.id}`, event.title);
    } catch (err) {
      alert(err.message);
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

function galleryFileURL(dateId, item) {
  return `/api/controller/dates/${encodeURIComponent(dateId)}/gallery/${encodeURIComponent(item.id)}`;
}

function renderGallery() {
  const list = document.getElementById("gallery-list");
  const date = state.dates.find((d) => d.id === galleryDateId);
  document.getElementById("gallery-heading").textContent = date?.title || I18N.t("event");
  if (!galleryItems.length) {
    list.innerHTML = `<p class="muted">${I18N.t("galleryEmpty")}</p>`;
    return;
  }
  list.innerHTML = galleryItems.map((item) => {
    const url = galleryFileURL(galleryDateId, item);
    const media = item.kind === "video"
      ? `<video class="gallery-media" controls preload="metadata" src="${url}"></video>`
      : `<img class="gallery-media" src="${url}" alt="" />`;
    return `<article class="gallery-card">
      ${media}
      <p class="meta">${escapeHtml(item.nickname || "")}${item.name ? ` · ${escapeHtml(item.name)}` : ""}</p>
      <button type="button" class="btn ghost danger" data-gallery-del="${item.id}">${I18N.t("delete")}</button>
    </article>`;
  }).join("");
}

async function openGallery(id) {
  galleryDateId = id;
  showError(document.getElementById("gallery-error"), "");
  const data = await api(`/api/controller/dates/${encodeURIComponent(id)}/gallery`);
  galleryItems = data.gallery || [];
  renderGallery();
  const dialog = document.getElementById("gallery-dialog");
  if (!dialog.open) dialog.showModal();
  paintGallerySize();
}

function paintGallerySize() {
  const dialog = document.getElementById("gallery-dialog");
  const shrink = document.getElementById("gallery-shrink");
  const expand = document.getElementById("gallery-expand");
  if (!dialog || !shrink || !expand) return;
  const full = dialog.classList.contains("full");
  shrink.disabled = !full;
  expand.disabled = full;
  shrink.setAttribute("aria-pressed", full ? "false" : "true");
  expand.setAttribute("aria-pressed", full ? "true" : "false");
}

function setGalleryFull(full) {
  const dialog = document.getElementById("gallery-dialog");
  dialog.classList.toggle("full", full);
  paintGallerySize();
}

document.getElementById("gallery-close").addEventListener("click", () => {
  document.getElementById("gallery-dialog").close();
});

document.getElementById("gallery-shrink").addEventListener("click", () => setGalleryFull(false));
document.getElementById("gallery-expand").addEventListener("click", () => setGalleryFull(true));

document.getElementById("gallery-dialog").addEventListener("close", () => {
  galleryDateId = "";
  galleryItems = [];
  document.getElementById("gallery-dialog").classList.remove("full");
  document.getElementById("gallery-list").innerHTML = "";
  setGalleryProgress(false);
});

document.getElementById("gallery-add").addEventListener("click", () => {
  const input = document.getElementById("gallery-file");
  input.value = "";
  input.click();
});

const GALLERY_MAX_PHOTO = 8 * 1024 * 1024;
const GALLERY_MAX_VIDEO = 1024 * 1024 * 1024;
const GALLERY_PROGRESS_MIN = 1024 * 1024;

function isGalleryVideo(file) {
  return String(file.type || "").startsWith("video/") || /\.(mp4|m4v|webm|mov)$/i.test(file.name || "");
}

function galleryMaxFor(file) {
  return isGalleryVideo(file) ? GALLERY_MAX_VIDEO : GALLERY_MAX_PHOTO;
}

function setGalleryProgress(on, name, loaded, total) {
  const box = document.getElementById("gallery-progress");
  const nameEl = document.getElementById("gallery-progress-name");
  const pctEl = document.getElementById("gallery-progress-pct");
  const bar = document.getElementById("gallery-progress-bar");
  if (!box) return;
  box.hidden = !on;
  if (!on) {
    if (bar) bar.value = 0;
    if (nameEl) nameEl.textContent = "";
    if (pctEl) pctEl.textContent = "";
    return;
  }
  const pct = total > 0 ? Math.min(100, Math.round((loaded / total) * 100)) : 0;
  if (nameEl) nameEl.textContent = name || I18N.t("galleryUploading");
  if (pctEl) pctEl.textContent = `${pct}%`;
  if (bar) bar.value = pct;
}

function uploadGalleryFile(url, file, onProgress) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", url);
    xhr.withCredentials = true;
    xhr.upload.addEventListener("progress", (e) => {
      if (e.lengthComputable && onProgress) onProgress(e.loaded, e.total);
    });
    xhr.onload = () => {
      let data = {};
      try { data = JSON.parse(xhr.responseText || "{}"); } catch {}
      if (xhr.status >= 200 && xhr.status < 300) resolve(data);
      else reject(new Error(I18N.error(data.error || xhr.statusText || String(xhr.status))));
    };
    xhr.onerror = () => reject(new Error(I18N.t("errFile")));
    xhr.onabort = () => reject(new Error(I18N.t("errFile")));
    const fd = new FormData();
    fd.append("file", file);
    xhr.send(fd);
  });
}

async function uploadGalleryFiles(url, files, onProgress) {
  const errors = [];
  const showBar = files.some((f) => f.size >= GALLERY_PROGRESS_MIN || isGalleryVideo(f));
  let doneBytes = 0;
  const totalBytes = files.reduce((n, f) => n + (f.size || 0), 0);
  for (const file of files) {
    if (file.size > galleryMaxFor(file)) {
      errors.push(`${file.name}: ${I18N.t("errFileLarge")}`);
      doneBytes += file.size || 0;
      continue;
    }
    try {
      await uploadGalleryFile(url, file, (loaded) => {
        if (showBar && onProgress) onProgress(file.name, doneBytes + loaded, totalBytes);
      });
    } catch (err) {
      errors.push(`${file.name}: ${err.message}`);
    }
    doneBytes += file.size || 0;
    if (showBar && onProgress) onProgress(file.name, doneBytes, totalBytes);
  }
  return errors;
}

document.getElementById("gallery-file").addEventListener("change", async (e) => {
  const files = [...(e.target.files || [])];
  const errEl = document.getElementById("gallery-error");
  const addBtn = document.getElementById("gallery-add");
  showError(errEl, "");
  if (!files.length || !galleryDateId) return;
  addBtn.disabled = true;
  try {
    const errors = await uploadGalleryFiles(
      `/api/controller/dates/${encodeURIComponent(galleryDateId)}/gallery`,
      files,
      (name, loaded, total) => setGalleryProgress(true, name, loaded, total),
    );
    await loadState();
    await openGallery(galleryDateId);
    if (errors.length) showError(errEl, errors.join("\n"));
  } catch (err) {
    showError(errEl, err.message);
  } finally {
    setGalleryProgress(false);
    addBtn.disabled = false;
    e.target.value = "";
  }
});

document.getElementById("gallery-list").addEventListener("click", async (e) => {
  const del = e.target.closest("[data-gallery-del]");
  if (!del || !galleryDateId) return;
  if (!confirm(I18N.t("confirmDeleteGallery"))) return;
  showError(document.getElementById("gallery-error"), "");
  try {
    await api(`/api/controller/dates/${encodeURIComponent(galleryDateId)}/gallery/${encodeURIComponent(del.dataset.galleryDel)}`, { method: "DELETE" });
    await loadState();
    await openGallery(galleryDateId);
  } catch (err) {
    showError(document.getElementById("gallery-error"), err.message);
  }
});

function memberColor(id) {
  let n = 0;
  for (const ch of String(id || "")) n = (n + ch.charCodeAt(0)) % MEMBER_COLORS.length;
  return MEMBER_COLORS[n];
}

function chatDayKey(iso) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
}

function formatChatDay(iso) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const day = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  const diff = Math.round((today - day) / 86400000);
  if (diff === 0) return I18N.t("chatToday");
  if (diff === 1) return I18N.t("chatYesterday");
  return d.toLocaleDateString(I18N.locale(), {
    weekday: "long",
    day: "numeric",
    month: "long",
    year: now.getFullYear() === d.getFullYear() ? undefined : "numeric",
  });
}

function formatChatWhen(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleTimeString(I18N.locale(), { hour: "2-digit", minute: "2-digit" });
}

function chatAuthorKey(m) {
  return m.isAdmin ? "admin" : m.userId;
}

function applyUnread(map) {
  chatUnread = { ...(map || {}) };
}

function unreadCount(room) {
  return Number(chatUnread[room] || 0);
}

function chatBadge(n) {
  if (!n) return "";
  return `<span class="chat-unread">${n > 99 ? "99+" : n}</span>`;
}

function noteChatMessage(m) {
  if (!m?.room || !CHAT_ROOMS.includes(m.room)) return;
  if (chatRoom === m.room) {
    chatUnread[m.room] = 0;
    api(`/api/controller/chats/${encodeURIComponent(m.room)}/read`, { method: "POST" }).catch(() => {});
    renderChatTabs();
    return;
  }
  if (m.isAdmin) return;
  chatUnread[m.room] = unreadCount(m.room) + 1;
  renderChatTabs();
}

function renderChatTabs() {
  document.querySelectorAll("#chat-tabs [data-chat]").forEach((btn) => {
    const n = unreadCount(btn.dataset.chat);
    const label = btn.dataset.chat === "live" ? I18N.t("chat.live") : I18N.role(btn.dataset.chat);
    btn.innerHTML = `${escapeHtml(label)}${chatBadge(n)}`;
    btn.setAttribute("aria-label", n ? `${label}, ${n}` : label);
    btn.classList.toggle("on", chatRoom === btn.dataset.chat);
  });
}

function chatReactionsHTML(m) {
  const chips = (m.reactions || []).map((r) => `
    <button type="button" class="chat-react ${r.mine ? "on" : ""}" data-id="${m.id}" data-react="${r.emoji}">
      ${r.emoji}<span>${r.count}</span>
    </button>`).join("");
  const picks = CHAT_EMOJIS.map((emoji) => `
    <button type="button" data-id="${m.id}" data-react="${emoji}">${emoji}</button>`).join("");
  return `<div class="chat-reacts">
    ${chips}
    <button type="button" class="chat-react-add" data-pick="${m.id}" aria-label="${escapeHtml(I18N.t("chatReact"))}">+</button>
    <div class="chat-picker" hidden data-picker="${m.id}">${picks}</div>
  </div>`;
}

function formatVoiceDur(ms) {
  const sec = Math.max(0, Math.round((Number(ms) || 0) / 1000));
  return `${Math.floor(sec / 60)}:${String(sec % 60).padStart(2, "0")}`;
}

function chatVoiceURL(room, id) {
  return `${CHAT_API}/${encodeURIComponent(room)}/messages/${encodeURIComponent(id)}/voice`;
}

function chatMediaURL(room, id) {
  return `${CHAT_API}/${encodeURIComponent(room)}/messages/${encodeURIComponent(id)}/media`;
}

function chatBodyHTML(m) {
  if (m.kind === "voice") {
    const src = chatVoiceURL(m.room || chatRoom, m.id);
    const text = m.text ? `<p class="chat-voice-text">${escapeHtml(m.text)}</p>` : "";
    return `<div class="chat-voice">
      <button type="button" class="chat-voice-play" data-voice="${m.id}" aria-label="${escapeHtml(I18N.t("chatVoicePlay"))}"></button>
      <span class="chat-voice-track" aria-hidden="true"><i data-voice-bar="${m.id}"></i></span>
      <span class="chat-voice-dur" data-voice-dur="${m.id}">${formatVoiceDur(m.durationMs)}</span>
      <button type="button" class="chat-voice-del" data-voice-delete="${m.id}" aria-label="${escapeHtml(I18N.t("chatVoiceDelete"))}"></button>
      <audio preload="none" src="${escapeHtml(src)}" data-voice-audio="${m.id}"></audio>
    </div>${text}`;
  }
  if (m.kind === "image" || m.kind === "video") {
    const src = chatMediaURL(m.room || chatRoom, m.id);
    const media = m.kind === "video"
      ? `<video controls playsinline preload="metadata" src="${escapeHtml(src)}"></video>`
      : `<a href="${escapeHtml(src)}" target="_blank" rel="noopener"><img src="${escapeHtml(src)}" alt="" /></a>`;
    return `<div class="chat-media">${media}<button type="button" class="chat-voice-del" data-voice-delete="${m.id}" aria-label="${escapeHtml(I18N.t("chatMediaDelete"))}"></button></div>`;
  }
  if (editingChatId === m.id) {
    return `<form class="chat-edit-form" data-chat-save="${m.id}">
      <textarea maxlength="2000" rows="2" required>${escapeHtml(m.text || "")}</textarea>
      <div class="chat-edit-btns">
        <button type="submit">${escapeHtml(I18N.t("save"))}</button>
        <button type="button" class="btn ghost" data-chat-edit-cancel>${escapeHtml(I18N.t("cancel"))}</button>
      </div>
    </form>`;
  }
  return `<p class="chat-text">${escapeHtml(m.text)}</p>`;
}

function isChatText(m) {
  return !m?.kind || m.kind === "text";
}

function canEditChat(m) {
  return !!(m && isChatText(m));
}

function chatActionsHTML(m) {
  if (editingChatId === m.id) return "";
  const edit = canEditChat(m)
    ? `<button type="button" class="chat-edit-btn" data-chat-edit="${m.id}" aria-label="${escapeHtml(I18N.t("chatEdit"))}"></button>`
    : "";
  const del = isChatText(m)
    ? `<button type="button" class="chat-voice-del" data-voice-delete="${m.id}" aria-label="${escapeHtml(I18N.t("chatDelete"))}"></button>`
    : "";
  if (!edit && !del) return "";
  return `<div class="chat-tools">${edit}${del}</div>`;
}

function chatMessageHTML(m, stacked) {
  const mine = !!m.isAdmin;
  const color = m.isAdmin ? "#f0a35e" : memberColor(m.userId);
  const face = mine ? "" : Photo.html({
    id: m.userId,
    nickname: m.nickname,
    hasPhoto: m.hasPhoto,
    photoUpdatedAt: m.photoUpdatedAt,
  }, "sm");
  return `<article class="chat-row ${mine ? "mine" : "theirs"}${stacked ? " stack" : ""}" data-msg="${m.id}">
    ${face}
    <div class="chat-col">
      <div class="chat-bubble" data-msg="${m.id}">
        <p class="chat-name" style="color:${color}">${escapeHtml(m.nickname)}</p>
        ${chatBodyHTML(m)}
        <span class="chat-time">${escapeHtml(formatChatWhen(m.createdAt))}</span>
      </div>
      ${chatActionsHTML(m)}
      ${chatReactionsHTML(m)}
    </div>
  </article>`;
}

function chatTimeMs(iso) {
  const t = Date.parse(iso);
  return Number.isNaN(t) ? 0 : t;
}

function scrollChatToLastSeen(lastSeen) {
  const list = document.getElementById("chat-list");
  if (!list) return;
  const seenMs = chatTimeMs(lastSeen);
  if (seenMs) {
    let lastRead = null;
    for (const m of chatMessages) {
      if (chatTimeMs(m.createdAt) <= seenMs) lastRead = m;
    }
    const el = lastRead && list.querySelector(`[data-msg="${lastRead.id}"]`);
    if (el) {
      const top = el.getBoundingClientRect().top - list.getBoundingClientRect().top + list.scrollTop;
      list.scrollTop = Math.max(0, top - Math.max(0, (list.clientHeight - el.offsetHeight) / 2));
      return;
    }
  }
  list.scrollTop = list.scrollHeight;
}

function renderChat(keepTop, lastSeen) {
  const list = document.getElementById("chat-list");
  const atBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 48;
  list.innerHTML = chatMessages.length
    ? chatMessages.map((m, i) => {
      const prev = chatMessages[i - 1];
      const newDay = !prev || chatDayKey(prev.createdAt) !== chatDayKey(m.createdAt);
      const stacked = !newDay && !!(prev && chatAuthorKey(prev) === chatAuthorKey(m));
      const heading = newDay && formatChatDay(m.createdAt)
        ? `<p class="chat-day">${escapeHtml(formatChatDay(m.createdAt))}</p>`
        : "";
      return heading + chatMessageHTML(m, stacked);
    }).join("")
    : `<p class="muted">${I18N.t("noMessages")}</p>`;
  list.querySelectorAll("audio[data-voice-audio]").forEach(bindVoiceAudio);
  if (keepTop != null && !atBottom) {
    list.scrollTop = keepTop;
    return;
  }
  if (lastSeen) {
    scrollChatToLastSeen(lastSeen);
    return;
  }
  list.scrollTop = list.scrollHeight;
}

function applyReactions(messageId, reactions) {
  const m = chatMessages.find((x) => x.id === messageId);
  if (!m) return;
  const list = document.getElementById("chat-list");
  m.reactions = reactions || [];
  renderChat(list.scrollTop);
}

function appendChat(msg) {
  if (!msg?.id || chatMessages.some((m) => m.id === msg.id)) return;
  chatMessages.push(msg);
  renderChat();
}

function upsertChat(msg) {
  if (!msg?.id) return;
  const i = chatMessages.findIndex((m) => m.id === msg.id);
  if (i < 0) {
    appendChat(msg);
    return;
  }
  const list = document.getElementById("chat-list");
  chatMessages[i] = { ...chatMessages[i], ...msg };
  renderChat(list?.scrollTop);
}

let editingChatId = "";

async function saveChatEdit(id, text) {
  if (!chatRoom || !id) return;
  try {
    const data = await api(`/api/controller/chats/${encodeURIComponent(chatRoom)}/messages/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify({ text }),
    });
    editingChatId = "";
    upsertChat(data.message);
  } catch (err) {
    showError(document.getElementById("chat-error"), err.message);
  }
}

async function deleteChatMessage(id) {
  if (!chatRoom || !id) return;
  if (!confirm(I18N.t("confirmDeleteMessage"))) return;
  try {
    await api(`/api/controller/chats/${encodeURIComponent(chatRoom)}/messages/${encodeURIComponent(id)}`, { method: "DELETE" });
    removeChat(id);
  } catch (err) {
    showError(document.getElementById("chat-error"), err.message);
  }
}

function removeChat(id) {
  const next = chatMessages.filter((m) => m.id !== id);
  if (next.length === chatMessages.length) return;
  const list = document.getElementById("chat-list");
  chatMessages = next;
  renderChat(list.scrollTop);
}

function closeChat() {
  const dialog = document.getElementById("chat-dialog");
  editingChatId = "";
  if (dialog.open) dialog.close();
  chatRoom = "";
  chatMessages = [];
  renderChatTabs();
}

function chatTitle(room, title) {
  if (title) return title;
  if (CHAT_ROOMS.includes(room)) return I18N.t(`chat.${room}`);
  return I18N.t("eventChat");
}

let voiceRec = null;

function voiceMime() {
  const types = ["audio/webm;codecs=opus", "audio/webm", "audio/mp4", "audio/ogg;codecs=opus"];
  if (!window.MediaRecorder) return "";
  for (const t of types) {
    if (MediaRecorder.isTypeSupported(t)) return t;
  }
  return "";
}

function setVoiceRecording(on) {
  const form = document.getElementById("chat-form");
  const rec = document.getElementById("chat-rec");
  if (form) form.classList.toggle("recording", on);
  if (rec) rec.hidden = !on;
}

function stopVoiceTracks() {
  voiceRec?.stream?.getTracks().forEach((t) => t.stop());
}

function discardVoiceRecord() {
  if (voiceRec?.timer) clearInterval(voiceRec.timer);
  if (voiceRec?.recorder && voiceRec.recorder.state !== "inactive") {
    voiceRec.discard = true;
    try { voiceRec.recorder.stop(); } catch {}
  }
  stopVoiceTracks();
  voiceRec = null;
  setVoiceRecording(false);
  const time = document.getElementById("chat-rec-time");
  if (time) time.textContent = "0:00";
}

async function startVoiceRecord() {
  const errEl = document.getElementById("chat-error");
  showError(errEl, "");
  if (!navigator.mediaDevices?.getUserMedia || !window.MediaRecorder) {
    showError(errEl, I18N.t("errVoiceUnsupported"));
    return;
  }
  setChatEmojiOpen(false);
  let stream;
  try {
    stream = await navigator.mediaDevices.getUserMedia({ audio: true });
  } catch {
    showError(errEl, I18N.t("errVoiceDenied"));
    return;
  }
  const mime = voiceMime();
  const recorder = mime ? new MediaRecorder(stream, { mimeType: mime }) : new MediaRecorder(stream);
  const chunks = [];
  const started = Date.now();
  recorder.ondataavailable = (e) => { if (e.data?.size) chunks.push(e.data); };
  recorder.onstop = () => {
    stopVoiceTracks();
    const discard = !voiceRec || voiceRec.discard;
    const type = recorder.mimeType || mime || "audio/webm";
    const ms = Date.now() - started;
    voiceRec = null;
    setVoiceRecording(false);
    if (discard) return;
    sendVoice(new Blob(chunks, { type }), ms);
  };
  voiceRec = {
    recorder,
    stream,
    started,
    discard: false,
    timer: setInterval(() => {
      const ms = Date.now() - started;
      const time = document.getElementById("chat-rec-time");
      if (time) time.textContent = formatVoiceDur(ms);
      if (ms >= VOICE_MAX_MS) finishVoiceRecord(false);
    }, 200),
  };
  setVoiceRecording(true);
  recorder.start(250);
}

function finishVoiceRecord(discard) {
  if (!voiceRec) return;
  if (voiceRec.timer) clearInterval(voiceRec.timer);
  voiceRec.discard = discard;
  if (voiceRec.recorder && voiceRec.recorder.state !== "inactive") {
    voiceRec.recorder.stop();
    return;
  }
  stopVoiceTracks();
  voiceRec = null;
  setVoiceRecording(false);
}

async function sendVoice(blob, ms) {
  const errEl = document.getElementById("chat-error");
  if (!chatRoom || !blob || blob.size < 64 || ms < 400) {
    showError(errEl, I18N.t("errVoiceEmpty"));
    return;
  }
  const ext = blob.type.includes("mp4") ? "m4a" : blob.type.includes("ogg") ? "ogg" : blob.type.includes("wav") ? "wav" : "webm";
  const form = new FormData();
  form.append("file", blob, `voice.${ext}`);
  form.append("durationMs", String(Math.min(VOICE_MAX_MS, Math.round(ms))));
  try {
    const res = await fetch(`${CHAT_API}/${encodeURIComponent(chatRoom)}/voice`, {
      method: "POST",
      credentials: "same-origin",
      body: form,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(I18N.error(data.error || res.statusText));
    appendChat(data.message);
  } catch (err) {
    showError(errEl, err.message);
  }
}

function bindVoiceAudio(audio) {
  const id = audio.dataset.voiceAudio;
  audio.addEventListener("timeupdate", () => {
    const bar = document.querySelector(`[data-voice-bar="${id}"]`);
    const durEl = document.querySelector(`[data-voice-dur="${id}"]`);
    if (bar && audio.duration) bar.style.width = `${(audio.currentTime / audio.duration) * 100}%`;
    if (durEl) durEl.textContent = formatVoiceDur((audio.duration ? audio.duration - audio.currentTime : audio.currentTime) * 1000);
  });
  audio.addEventListener("ended", () => {
    const btn = document.querySelector(`[data-voice="${id}"]`);
    const bar = document.querySelector(`[data-voice-bar="${id}"]`);
    const durEl = document.querySelector(`[data-voice-dur="${id}"]`);
    const msg = chatMessages.find((m) => m.id === id);
    if (btn) {
      btn.classList.remove("on");
      btn.setAttribute("aria-label", I18N.t("chatVoicePlay"));
    }
    if (bar) bar.style.width = "0%";
    if (durEl) durEl.textContent = formatVoiceDur(msg?.durationMs || (audio.duration || 0) * 1000);
  });
}

function toggleChatVoice(id) {
  const audio = document.querySelector(`audio[data-voice-audio="${id}"]`);
  const btn = document.querySelector(`[data-voice="${id}"]`);
  if (!audio || !btn) return;
  document.querySelectorAll("audio[data-voice-audio]").forEach((el) => {
    if (el === audio) return;
    el.pause();
    const other = document.querySelector(`[data-voice="${el.dataset.voiceAudio}"]`);
    if (other) {
      other.classList.remove("on");
      other.setAttribute("aria-label", I18N.t("chatVoicePlay"));
    }
  });
  if (!audio.paused) {
    audio.pause();
    btn.classList.remove("on");
    btn.setAttribute("aria-label", I18N.t("chatVoicePlay"));
    return;
  }
  audio.play().catch(() => {});
  btn.classList.add("on");
  btn.setAttribute("aria-label", I18N.t("chatVoicePause"));
}

async function openChat(room, title) {
  const event = room.startsWith("event:");
  if (!event && !CHAT_ROOMS.includes(room)) return;
  chatRoom = room;
  renderChatTabs();
  document.getElementById("chat-title").textContent = chatTitle(room, title);
  discardVoiceRecord();
  document.getElementById("chat-text").value = "";
  showError(document.getElementById("chat-error"), "");
  const data = await api(`/api/controller/chats/${encodeURIComponent(room)}`);
  chatMessages = data.messages || [];
  const input = document.getElementById("chat-text");
  input.placeholder = I18N.t("chatWrite");
  document.getElementById("chat-dialog").showModal();
  setChatFull(true);
  paintChatSize();
  renderChat(undefined, data.lastSeen || "");
  input.focus();
}

async function sendChat() {
  if (!chatRoom) return;
  const input = document.getElementById("chat-text");
  const errEl = document.getElementById("chat-error");
  showError(errEl, "");
  const text = input.value.trim();
  if (!text) return;
  try {
    const data = await api(`/api/controller/chats/${encodeURIComponent(chatRoom)}`, {
      method: "POST",
      body: JSON.stringify({ text }),
    });
    input.value = "";
    input.style.height = "";
    setChatEmojiOpen(false);
    appendChat(data.message);
  } catch (err) {
    showError(errEl, err.message);
  }
}

function connectWS() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${proto}://${location.host}/ws/controller`);
  ws.onmessage = (ev) => {
    let msg = {};
    try { msg = JSON.parse(ev.data); } catch { loadState().catch(() => {}); return; }
    if (msg.type === "chat") {
      noteChatMessage(msg.data);
      if (chatRoom && msg.data?.room === chatRoom) appendChat(msg.data);
      return;
    }
    if (msg.type === "chatUpdate") {
      if (chatRoom && msg.data?.room === chatRoom) upsertChat(msg.data);
      return;
    }
    if (msg.type === "react") {
      if (chatRoom && msg.data?.room === chatRoom) applyReactions(msg.data.messageId, msg.data.reactions);
      return;
    }
    if (msg.type === "chatDelete") {
      if (chatRoom && msg.data?.room === chatRoom) removeChat(msg.data.messageId);
      return;
    }
    loadState().catch(() => {});
  };
  ws.onclose = () => { if (!dash.hidden) setTimeout(connectWS, 2000); };
}

document.getElementById("chat-tabs").addEventListener("click", async (e) => {
  const btn = e.target.closest("[data-chat]");
  if (!btn) return;
  try {
    await openChat(btn.dataset.chat);
  } catch (err) {
    alert(err.message);
  }
});

function paintChatSize() {
  const dialog = document.getElementById("chat-dialog");
  const shrink = document.getElementById("chat-shrink");
  const expand = document.getElementById("chat-expand");
  if (!dialog || !shrink || !expand) return;
  const full = dialog.classList.contains("full");
  shrink.disabled = !full;
  expand.disabled = full;
  shrink.setAttribute("aria-pressed", full ? "false" : "true");
  expand.setAttribute("aria-pressed", full ? "true" : "false");
}

function setChatFull(full) {
  const dialog = document.getElementById("chat-dialog");
  dialog.classList.toggle("full", full);
  paintChatSize();
  const list = document.getElementById("chat-list");
  if (list) list.scrollTop = list.scrollHeight;
}

document.getElementById("chat-close").addEventListener("click", () => {
  discardVoiceRecord();
  document.getElementById("chat-dialog").close();
});
document.getElementById("chat-close-bottom").addEventListener("click", () => {
  document.getElementById("chat-dialog").close();
});

document.getElementById("chat-dialog").addEventListener("close", () => {
  discardVoiceRecord();
  editingChatId = "";
  chatRoom = "";
  chatMessages = [];
  renderChatTabs();
});

document.getElementById("chat-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  await sendChat();
});

document.getElementById("chat-text").addEventListener("keydown", (e) => {
  if (e.key !== "Enter" || e.shiftKey) return;
  e.preventDefault();
  sendChat();
});

document.getElementById("chat-voice").addEventListener("click", () => startVoiceRecord());
document.getElementById("chat-media").addEventListener("click", () => {
  document.getElementById("chat-media-file").click();
});
document.getElementById("chat-media-file").addEventListener("change", async (e) => {
  const file = e.target.files?.[0];
  e.target.value = "";
  if (!file) return;
  await sendChatMedia(file);
});

async function sendChatMedia(file) {
  const errEl = document.getElementById("chat-error");
  if (!chatRoom || !file) return;
  const form = new FormData();
  form.append("file", file, file.name || "media");
  try {
    showError(errEl, "");
    const res = await fetch(`${CHAT_API}/${encodeURIComponent(chatRoom)}/media`, {
      method: "POST",
      credentials: "same-origin",
      body: form,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(I18N.error(data.error || res.statusText));
    appendChat(data.message);
  } catch (err) {
    showError(errEl, err.message);
  }
}
document.getElementById("chat-rec-cancel").addEventListener("click", () => finishVoiceRecord(true));
document.getElementById("chat-rec-send").addEventListener("click", () => finishVoiceRecord(false));

function setChatEmojiOpen(open) {
  const toggle = document.getElementById("chat-emoji-toggle");
  const panel = document.getElementById("chat-emoji-panel");
  if (!toggle || !panel) return;
  panel.hidden = !open;
  toggle.setAttribute("aria-expanded", open ? "true" : "false");
}

function insertChatEmoji(emoji) {
  const input = document.getElementById("chat-text");
  const start = input.selectionStart ?? input.value.length;
  const end = input.selectionEnd ?? input.value.length;
  const next = input.value.slice(0, start) + emoji + input.value.slice(end);
  if (input.maxLength > 0 && next.length > input.maxLength) return;
  input.value = next;
  const pos = start + [...emoji].length;
  input.focus();
  input.setSelectionRange(pos, pos);
}

(function setupChatEmojiPicker() {
  const toggle = document.getElementById("chat-emoji-toggle");
  const panel = document.getElementById("chat-emoji-panel");
  if (!toggle || !panel) return;
  panel.innerHTML = COMPOSE_EMOJIS.map((emoji) => `<button type="button" data-emoji="${emoji}">${emoji}</button>`).join("");
  toggle.addEventListener("click", (e) => {
    e.preventDefault();
    setChatEmojiOpen(panel.hidden);
  });
  panel.addEventListener("click", (e) => {
    const btn = e.target.closest("[data-emoji]");
    if (!btn) return;
    insertChatEmoji(btn.dataset.emoji);
  });
  document.addEventListener("pointerdown", (e) => {
    if (panel.hidden) return;
    if (e.target.closest(".chat-emoji")) return;
    setChatEmojiOpen(false);
  });
  document.getElementById("chat-dialog").addEventListener("close", () => setChatEmojiOpen(false));
})();

document.getElementById("chat-list").addEventListener("submit", async (e) => {
  const form = e.target.closest("[data-chat-save]");
  if (!form) return;
  e.preventDefault();
  const text = form.querySelector("textarea")?.value || "";
  await saveChatEdit(form.dataset.chatSave, text);
});

document.getElementById("chat-list").addEventListener("click", async (e) => {
  const edit = e.target.closest("[data-chat-edit]");
  if (edit) {
    editingChatId = edit.dataset.chatEdit;
    renderChat(document.getElementById("chat-list").scrollTop);
    document.querySelector(`[data-chat-save="${editingChatId}"] textarea`)?.focus();
    return;
  }
  if (e.target.closest("[data-chat-edit-cancel]")) {
    editingChatId = "";
    renderChat(document.getElementById("chat-list").scrollTop);
    return;
  }
  const trash = e.target.closest("[data-voice-delete]");
  if (trash) {
    await deleteChatMessage(trash.dataset.voiceDelete);
    return;
  }
  const voice = e.target.closest("[data-voice]");
  if (voice) {
    toggleChatVoice(voice.dataset.voice);
    return;
  }
  const pick = e.target.closest("[data-pick]");
  if (pick) {
    const box = document.querySelector(`[data-picker="${pick.dataset.pick}"]`);
    if (box) box.hidden = !box.hidden;
    return;
  }
  const btn = e.target.closest("[data-react]");
  if (!btn || !chatRoom) return;
  try {
    const data = await api(`/api/controller/chats/${encodeURIComponent(chatRoom)}/messages/${encodeURIComponent(btn.dataset.id)}/react`, {
      method: "POST",
      body: JSON.stringify({ emoji: btn.dataset.react }),
    });
    applyReactions(data.message.id, data.message.reactions);
  } catch (err) {
    showError(document.getElementById("chat-error"), err.message);
  }
});

document.getElementById("user-nickname").addEventListener("input", () => {
  if (!selectedUser && !pendingPhotoURL) paintPersonPhoto();
});

document.getElementById("user-info").addEventListener("click", () => {
  fillInfoForm(selectedUser ? currentPerson() : pendingInfo);
  document.getElementById("info-dialog").showModal();
});

document.getElementById("info-close").addEventListener("click", () => {
  document.getElementById("info-dialog").close();
});

document.getElementById("info-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const errEl = document.getElementById("info-error");
  showError(errEl, "");
  const info = readInfoForm();
  const id = document.getElementById("user-id").value;
  try {
    if (id) {
      const data = await api(`/api/controller/users/${id}/info`, {
        method: "PATCH",
        body: JSON.stringify(info),
      });
      await loadState();
      fillUserForm(data.user);
    } else {
      pendingInfo = info;
      paintPersonPhoto();
    }
    document.getElementById("info-dialog").close();
  } catch (err) {
    showError(errEl, err.message);
  }
});

Photo.bind({
  canRemove: () => !!(pendingPhoto || currentPerson().hasPhoto),
  onFile: async (blob) => {
    const id = document.getElementById("user-id").value;
    if (id) {
      const data = await Photo.upload(`/api/controller/users/${id}/photo`, blob);
      await loadState();
      fillUserForm(data.user);
      return;
    }
    clearPendingPhoto();
    pendingPhoto = blob;
    pendingPhotoURL = URL.createObjectURL(blob);
    paintPersonPhoto();
  },
  onRemove: async () => {
    const id = document.getElementById("user-id").value;
    if (id) {
      const data = await Photo.remove(`/api/controller/users/${id}/photo`);
      await loadState();
      fillUserForm(data.user);
      return;
    }
    clearPendingPhoto();
    paintPersonPhoto();
  },
});

document.getElementById("settings-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const errEl = document.getElementById("settings-error");
  showError(errEl, "");
  try {
    const data = await api("/api/controller/settings", {
      method: "PATCH",
      body: JSON.stringify({
        adminAlias: document.getElementById("admin-alias").value,
        newsTicker: document.getElementById("news-ticker").value,
      }),
    });
    state.adminAlias = data.adminAlias || "Admin";
    state.newsTicker = data.newsTicker || "";
    fillSettingsForm();
    chatMessages.forEach((m) => {
      if (m.isAdmin) m.nickname = state.adminAlias;
    });
    if (document.getElementById("chat-dialog").open) {
      renderChat(document.getElementById("chat-list").scrollTop);
    }
  } catch (err) {
    showError(errEl, err.message);
  }
});

document.getElementById("channel-search").addEventListener("input", () => renderChannels());
document.getElementById("proposal-search").addEventListener("input", () => renderProposals());

document.getElementById("proposals-body").addEventListener("change", (e) => {
  const input = e.target.closest("[data-proposal-comment]");
  if (!input) return;
  patchProposal(input.dataset.proposalComment, { comment: input.value });
});

document.getElementById("proposals-body").addEventListener("click", async (e) => {
  const row = e.target.closest("tr[data-proposal]");
  if (!row) return;
  const id = row.dataset.proposal;
  const statusBtn = e.target.closest("[data-proposal-status]");
  if (statusBtn) {
    const comment = row.querySelector("[data-proposal-comment]")?.value ?? "";
    await patchProposal(id, { status: statusBtn.dataset.proposalStatus, comment });
    return;
  }
  if (e.target.closest("[data-proposal-delete]")) {
    if (!confirm(I18N.t("confirmDeleteProposal"))) return;
    showError(document.getElementById("proposals-error"), "");
    try {
      await api(`/api/controller/proposals/${encodeURIComponent(id)}`, { method: "DELETE" });
      await loadState();
    } catch (err) {
      showError(document.getElementById("proposals-error"), err.message);
    }
  }
});

document.getElementById("channels-body").addEventListener("change", (e) => {
  const userEl = e.target.closest("[data-channel-user]");
  if (userEl) {
    saveChannel(Number(userEl.dataset.channelUser));
    return;
  }
  const commentEl = e.target.closest("[data-channel-comment]");
  if (commentEl) saveChannel(Number(commentEl.dataset.channelComment));
});

document.getElementById("channels-body").addEventListener("click", (e) => {
  const v48El = e.target.closest("[data-channel-v48]");
  if (!v48El) return;
  saveChannel(Number(v48El.dataset.channelV48), v48El.getAttribute("aria-pressed") !== "true");
});

document.getElementById("archive-search").addEventListener("input", () => renderArchive());

document.getElementById("archive-body").addEventListener("click", (e) => {
  const row = e.target.closest("[data-id]");
  const item = row && archiveByID(row.dataset.id);
  if (item) fillArchiveForm(item);
});

document.getElementById("btn-archive-new").addEventListener("click", resetArchiveForm);

document.getElementById("archive-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const errEl = document.getElementById("archive-error");
  showError(errEl, "");
  const id = document.getElementById("archive-id").value;
  const body = {
    title: document.getElementById("archive-title").value,
    composer: document.getElementById("archive-composer").value,
  };
  try {
    const data = id
      ? await api(`/api/controller/archive/${id}`, { method: "PATCH", body: JSON.stringify(body) })
      : await api("/api/controller/archive", { method: "POST", body: JSON.stringify(body) });
    await loadState();
    fillArchiveForm(data.item);
  } catch (err) {
    showError(errEl, err.message);
  }
});

document.getElementById("btn-archive-delete").addEventListener("click", async () => {
  const id = document.getElementById("archive-id").value;
  if (!id || !confirm(I18N.t("confirmDeleteArchive"))) return;
  try {
    await api(`/api/controller/archive/${id}`, { method: "DELETE" });
    dateTitleIDs = dateTitleIDs.filter((x) => x !== id);
    resetArchiveForm();
    await loadState();
    renderDateTitles();
  } catch (err) {
    showError(document.getElementById("archive-error"), err.message);
  }
});

document.getElementById("archive-files").addEventListener("click", (e) => {
  const addLink = e.target.closest("[data-add-link]");
  if (addLink && selectedArchive) {
    const box = addLink.closest(".archive-kind");
    const url = box?.querySelector("[data-new-link-url]")?.value || "";
    const name = box?.querySelector("[data-new-link-name]")?.value || "";
    showError(document.getElementById("archive-error"), "");
    addArchiveLink(url, name)
      .then(async (data) => {
        await loadState();
        fillArchiveForm(data.item);
      })
      .catch((err) => showError(document.getElementById("archive-error"), err.message));
    return;
  }
  const upload = e.target.closest("[data-upload]");
  if (upload && selectedArchive) {
    const input = document.getElementById("archive-file");
    input.dataset.kind = upload.dataset.upload;
    input.dataset.role = upload.dataset.role || "";
    input.accept = upload.dataset.accept || "";
    input.value = "";
    input.click();
    return;
  }
  const clear = e.target.closest("[data-clear-file]");
  if (!clear || !selectedArchive) return;
  if (!confirm(I18N.t("confirmDeleteFile"))) return;
  api(`/api/controller/archive/${encodeURIComponent(selectedArchive)}/files/${encodeURIComponent(clear.dataset.clearFile)}`, { method: "DELETE" })
    .then(async (data) => {
      await loadState();
      fillArchiveForm(data.item);
    })
    .catch((err) => showError(document.getElementById("archive-error"), err.message));
});

document.getElementById("archive-files").addEventListener("keydown", (e) => {
  if (e.key !== "Enter" || !e.target.closest("[data-file-name], [data-file-role], [data-file-url]")) return;
  e.preventDefault();
  e.target.blur();
});

document.getElementById("archive-files").addEventListener("change", async (e) => {
  const row = e.target.closest("[data-file]");
  if (!row || !selectedArchive || !e.target.closest("[data-file-name], [data-file-role], [data-file-url]")) return;
  showError(document.getElementById("archive-error"), "");
  try {
    const data = await saveArchiveFileMeta(
      row.dataset.file,
      row.querySelector("[data-file-name]")?.value || "",
      row.querySelector("[data-file-role]")?.value || "",
      row.dataset.link ? (row.querySelector("[data-file-url]")?.value || "") : undefined,
    );
    await loadState();
    fillArchiveForm(data.item);
    const next = document.querySelector(`[data-file="${row.dataset.file}"] ${e.target.matches("[data-file-role]") ? "[data-file-role]" : "[data-file-name]"}`);
    next?.focus();
  } catch (err) {
    showError(document.getElementById("archive-error"), err.message);
  }
});

document.getElementById("archive-file").addEventListener("change", async (e) => {
  const file = e.target.files?.[0];
  const kind = e.target.dataset.kind;
  if (!file || !kind || !selectedArchive) return;
  showError(document.getElementById("archive-error"), "");
  try {
    const data = await uploadArchiveFile(file, kind, e.target.dataset.role || "");
    await loadState();
    fillArchiveForm(data.item);
  } catch (err) {
    showError(document.getElementById("archive-error"), err.message);
  }
  e.target.value = "";
});

document.getElementById("inherit-titles").addEventListener("change", (e) => {
  const d = state.dates.find((x) => x.id === e.target.value);
  e.target.value = "";
  if (!d) return;
  dateTitleIDs = (d.titles || []).map((t) => t.id);
  renderDateTitles();
  paintDateSave();
});

document.getElementById("archive-pick-search").addEventListener("input", () => {
  renderArchivePick();
});

document.getElementById("archive-pick-list").addEventListener("click", (e) => {
  const btn = e.target.closest("[data-add-title]");
  if (!btn) return;
  addTitleToDate(btn.dataset.addTitle);
  document.getElementById("archive-pick-search").value = "";
  renderArchivePick();
});

document.getElementById("btn-title-new").addEventListener("click", () => {
  createTitleForDate().catch((err) => showError(document.getElementById("title-new-error"), err.message));
});

["title-new-name", "title-new-composer"].forEach((id) => {
  document.getElementById(id).addEventListener("keydown", (e) => {
    if (e.key !== "Enter") return;
    e.preventDefault();
    createTitleForDate().catch((err) => showError(document.getElementById("title-new-error"), err.message));
  });
});

function numberedTitleList(titles) {
  return titles.map((title, i) => `${i + 1}. ${title}`).join("\n");
}

function titleListHeader(date) {
  return [
    date?.title,
    date?.location,
    formatWhen(date?.startsAt),
  ].map((s) => String(s || "").trim()).filter(Boolean).join(" · ");
}

function titlesClipboardText(date, titles) {
  const list = numberedTitleList(titles);
  const head = titleListHeader(date);
  return head ? `${head}\n\n${list}` : list;
}

async function copyText(text) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const ta = document.createElement("textarea");
  ta.value = text;
  ta.setAttribute("readonly", "");
  ta.style.position = "fixed";
  ta.style.left = "-9999px";
  document.body.appendChild(ta);
  ta.select();
  document.execCommand("copy");
  ta.remove();
}

function flashCopyBtn(btn) {
  if (!btn) return;
  btn.classList.add("copied");
  btn.setAttribute("aria-label", I18N.t("titlesCopied"));
  clearTimeout(btn._copyFlash);
  btn._copyFlash = setTimeout(() => {
    btn.classList.remove("copied");
    btn.setAttribute("aria-label", I18N.t("copyTitles"));
  }, 1400);
}

document.getElementById("date-titles-copy").addEventListener("click", async () => {
  const titles = dateTitleIDs.map((id) => archiveByID(id)?.title || "").filter((t) => t);
  if (!titles.length) return;
  const date = {
    title: document.getElementById("date-title").value,
    location: document.getElementById("date-location").value,
    startsAt: toISO(document.getElementById("date-start").value),
  };
  try {
    await copyText(titlesClipboardText(date, titles));
    flashCopyBtn(document.getElementById("date-titles-copy"));
  } catch (err) {
    alert(err.message);
  }
});

document.getElementById("date-titles").addEventListener("click", (e) => {
  const row = e.target.closest("[data-title]");
  if (!row) return;
  const id = row.dataset.title;
  const idx = dateTitleIDs.indexOf(id);
  if (idx < 0) return;
  if (e.target.closest("[data-remove-title]")) {
    dateTitleIDs.splice(idx, 1);
    renderDateTitles();
    paintDateSave();
    return;
  }
  const move = e.target.closest("[data-move]");
  if (!move) return;
  const next = idx + Number(move.dataset.move);
  if (next < 0 || next >= dateTitleIDs.length) return;
  const swap = dateTitleIDs[next];
  dateTitleIDs[next] = dateTitleIDs[idx];
  dateTitleIDs[idx] = swap;
  renderDateTitles();
  paintDateSave();
});

I18N.onChange(() => {
  I18N.apply();
  if (catalog.roles.length) fillRoleSelects();
  const peopleTitle = document.getElementById("people-form-title");
  const dateTitle = document.getElementById("date-form-title");
  peopleTitle.textContent = selectedUser ? I18N.t("editPerson") : I18N.t("addPerson");
  dateTitle.textContent = selectedDate ? I18N.t("editDate") : I18N.t("addDate");
  const pwHint = document.getElementById("pw-hint");
  if (selectedUser) {
    const u = state.users.find((x) => x.id === selectedUser);
    pwHint.textContent = u?.mustChangePassword ? I18N.t("passwordPending") : I18N.t("passwordKeep");
  }
  renderPollRows();
  renderChatTabs();
  paintChatSize();
  if (document.getElementById("chat-dialog").open && chatRoom) {
    const date = chatRoom.startsWith("event:")
      ? state.dates.find((d) => d.id === chatRoom.slice("event:".length))
      : null;
    document.getElementById("chat-title").textContent = chatTitle(chatRoom, date?.title);
    document.getElementById("chat-text").placeholder = I18N.t("chatWrite");
    renderChat(document.getElementById("chat-list").scrollTop);
  }
  if (!dash.hidden) {
    renderPeople();
    renderContacts();
    renderDates();
    renderRanking();
    renderArchive();
    renderChannels();
    renderProposals();
    renderDateTitles();
    fillSettingsForm();
    paintUserChannels();
    paintPersonPhoto();
    if (document.getElementById("gallery-dialog")?.open) {
      paintGallerySize();
      renderGallery();
    }
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
