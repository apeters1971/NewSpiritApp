const CHOICES = ["yes", "maybe", "no", "unknown"];

const loginView = document.getElementById("login-view");
const pwView = document.getElementById("pw-view");
const appView = document.getElementById("app-view");
const loginForm = document.getElementById("login-form");
const loginError = document.getElementById("login-error");
const datesEl = document.getElementById("dates");
const emptyEl = document.getElementById("empty");

let me = null;
let dates = [];
let ranking = { year: 0, leaders: [] };
let proposals = [];
let commentDateId = "";
let titlesDateId = "";
let chatRoom = "";
let chatMessages = [];

const CHAT_ROOMS = ["choir", "band", "orchestra"];
const CHAT_EMOJIS = ["👍", "❤️", "😂", "😮", "😢", "🎉"];
const MEMBER_COLORS = ["#3dd6c6", "#f0a35e", "#8cb4ff", "#e38cff", "#7fd99a", "#f07178", "#ffd166", "#9ad0c8"];

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

function formatOptionRange(opt) {
  let s = formatWhen(opt.startsAt);
  if (opt.endsAt) s += " – " + formatWhen(opt.endsAt);
  return s;
}

function pollOpen(date) {
  return !!(date.pollOpen && date.options?.length >= 2);
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

function renderPoll(date) {
  const locked = date.status === "cancelled" || !date.pollOpen;
  const options = date.options || [];
  const blocks = options.map((o) => {
    const mine = o.myInitial && o.myInitial !== o.myChoice
      ? `<p class="changed">${I18N.t("yourFirstVote")}: ${voteLabel(o.myInitial)}</p>`
      : "";
    const buttons = CHOICES.map((c) => `
      <button type="button" data-id="${date.id}" data-option="${o.id}" data-choice="${c}" class="${c}${o.myChoice === c ? " on" : ""}" ${locked ? "disabled" : ""}>${voteLabel(c)}</button>
    `).join("");
    return `<div class="poll-option ${o.frozen ? "frozen" : ""}">
      <p class="when">${escapeHtml(formatOptionRange(o))}${o.frozen ? ` · ${I18N.t("chosenTime")}` : ""}</p>
      <div class="vote-row">${buttons}</div>
      ${mine}
      <p class="muted">${I18N.t("yes")} ${o.yes} · ${I18N.t("maybe")} ${o.maybe} · ${I18N.t("no")} ${o.no} · ${I18N.t("unknown")} ${o.unknown}</p>
    </div>`;
  }).join("");
  const people = date.roster || [];
  const head = `<tr><th>${I18N.t("name")}</th>${options.map((o) => `<th>${escapeHtml(formatOptionRange(o))}</th>`).join("")}</tr>`;
  const body = people.map((p) => {
    const cells = options.map((o) => {
      const entry = (o.roster || []).find((e) => e.userId === p.userId);
      const choice = entry?.choice || "unknown";
      const changed = entry?.initialChoice && entry.initialChoice !== choice;
      return `<td><span class="badge ${choice}">${voteLabel(choice)}</span>${changed ? ` <span class="changed">${I18N.t("firstVote")} ${voteLabel(entry.initialChoice)}</span>` : ""}</td>`;
    }).join("");
    return `<tr><td>${escapeHtml(p.nickname)}</td>${cells}</tr>`;
  }).join("");
  return `${blocks}
    <table class="roster poll-table">
      <thead>${head}</thead>
      <tbody>${body || `<tr><td colspan="${options.length + 1}" class="muted">${I18N.t("noPeopleRoles")}</td></tr>`}</tbody>
    </table>`;
}

function renderDate(date) {
  const locked = date.status === "cancelled";
  const isPoll = (date.options || []).length >= 2;
  const buttons = CHOICES.map((c) => `
    <button type="button" data-id="${date.id}" data-choice="${c}" class="${c}${date.myChoice === c ? " on" : ""}" ${locked ? "disabled" : ""}>${voteLabel(c)}</button>
  `).join("");
  const mine = date.myInitial && date.myInitial !== date.myChoice
    ? `<p class="changed">${I18N.t("yourFirstVote")}: ${voteLabel(date.myInitial)}</p>`
    : "";
  const notes = date.notes ? `<p class="notes">${escapeHtml(date.notes)}</p>` : "";
  const commentCount = (date.comments || []).length;
  const when = pollOpen(date)
    ? I18N.t("severalTimes") + (date.location ? " · " + date.location : "")
    : formatRange(date);
  const extraBadge = pollOpen(date)
    ? `<span class="badge voting">${I18N.t("poll")}</span>`
    : date.frozenOptionId ? `<span class="badge accepted">${I18N.t("chosenTime")}</span>` : "";
  return `<article class="card">
    <div class="card-head">
      <div>
        <p class="brand">${escapeHtml(I18N.category(date.category))} · ${date.roles.map((r) => I18N.role(r)).join(" · ")}</p>
        <h2 id="date-${date.id}">${escapeHtml(date.title)}</h2>
        <p class="when">${escapeHtml(when)}</p>
        ${renderBring(date.bring)}
        ${notes}
      </div>
      <div class="badges">
        ${extraBadge}
        <span class="badge ${date.status}">${I18N.status(date.status)}</span>
      </div>
    </div>
    ${isPoll ? renderPoll(date) : ""}
    ${pollOpen(date) ? "" : `<div class="vote-row">${buttons}</div>${mine}`}
    <div class="card-actions">
      <button type="button" class="btn ghost" data-comments="${date.id}">${I18N.t("comments")}${commentCount ? ` (${commentCount})` : ""}</button>
      ${date.schedule ? `<button type="button" class="btn ghost" data-schedule="${date.id}">${I18N.t("schedule")}</button>` : ""}
      <button type="button" class="btn ghost" data-titles="${date.id}">${I18N.t("titles")}${(date.titles || []).length ? ` (${date.titles.length})` : ""}</button>
      ${date.chatOpen ? `<button type="button" class="btn ghost" data-event-chat="${date.id}">${I18N.t("eventChat")}</button>` : ""}
    </div>
    ${pollOpen(date) ? "" : renderCounts(date)}
    ${pollOpen(date) ? "" : renderRoster(date)}
  </article>`;
}

function escapeHtml(s) {
  return String(s ?? "").replace(/[&<>"']/g, (ch) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[ch]));
}

function overviewPollVote(d) {
  const options = d.options || [];
  const voted = options.filter((o) => o.myChoice && o.myChoice !== "unknown").length;
  return `${voted}/${options.length}`;
}

function dateIsUpcoming(d) {
  if (!d || d.status === "cancelled") return false;
  const start = new Date();
  start.setHours(0, 0, 0, 0);
  return new Date(d.startsAt) >= start;
}

function userParticipates(d) {
  if (pollOpen(d)) {
    return (d.options || []).some((o) => o.myChoice === "yes" || o.myChoice === "maybe");
  }
  return d.myChoice === "yes" || d.myChoice === "maybe";
}

function upcomingDates() {
  return dates.filter(dateIsUpcoming).slice(0, 8);
}

function nextParticipatingDate() {
  return dates.find((d) => dateIsUpcoming(d) && d.status === "accepted" && userParticipates(d)) || null;
}

function mapsSearchURL(location) {
  return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(String(location || "").trim())}`;
}

function paintMyChannels() {
  const header = document.getElementById("who-channels");
  const box = document.getElementById("my-channels");
  const channels = me?.channels || [];
  const dirty = {};
  if (box) {
    box.querySelectorAll("form[data-channel] input[name=comment].dirty").forEach((input) => {
      const n = Number(input.closest("form").dataset.channel);
      if (input.value !== savedChannelComment(n)) dirty[String(n)] = input.value;
    });
  }
  if (header) {
    header.hidden = channels.length === 0;
    header.textContent = channels.map((ch) => {
      const note = ch.comment ? ` · ${ch.comment}` : "";
      const v48 = ch.v48 ? ` · ${I18N.t("channel48v")}` : "";
      return `${I18N.t("channel")} ${ch.number}${note}${v48}`;
    }).join(" · ");
  }
  if (box) {
    box.hidden = channels.length === 0;
    box.innerHTML = channels.length
      ? `<p class="brand">${I18N.t("channels")}</p>` + channels.map((ch) => {
        const draft = Object.hasOwn(dirty, String(ch.number));
        const value = draft ? dirty[String(ch.number)] : (ch.comment || "");
        return `
          <form class="my-channel" data-channel="${ch.number}">
            <strong>${I18N.t("channel")} ${ch.number}</strong>
            <label>
              <span class="visually-hidden">${I18N.t("channelComment")}</span>
              <input name="comment" maxlength="200" value="${escapeHtml(value)}" placeholder="${escapeHtml(I18N.t("channelComment"))}" class="${draft ? "dirty" : ""}" />
            </label>
            <button type="button" class="btn ghost v48-btn${ch.v48 ? " on" : ""}" data-channel-v48="${ch.number}" aria-pressed="${ch.v48 ? "true" : "false"}">${I18N.t("channel48v")}</button>
          </form>`;
      }).join("") + `<p id="channel-error" class="error" hidden></p>`
      : "";
  }
}

function jumpToDate(id) {
  const card = document.getElementById(`date-${id}`);
  if (!card) return;
  card.closest(".card")?.classList.add("flash");
  card.scrollIntoView({ behavior: "smooth", block: "start" });
  setTimeout(() => card.closest(".card")?.classList.remove("flash"), 1600);
}

function renderNextUp() {
  const box = document.getElementById("next-up");
  const next = nextParticipatingDate();
  if (!next) {
    box.hidden = true;
    box.innerHTML = "";
    return;
  }
  const when = pollOpen(next)
    ? I18N.t("severalTimes") + (next.location ? " · " + next.location : "")
    : formatRange(next);
  box.hidden = false;
  box.innerHTML = `
    <p class="brand">${I18N.t("nextUp")}</p>
    <div class="next-up-row">
      <div>
        <strong>${escapeHtml(next.title)}</strong>
        <p class="when">${escapeHtml(when)}</p>
        <p class="muted">${escapeHtml(I18N.category(next.category))}</p>
      </div>
      <div class="next-up-actions">
        <button type="button" class="btn ghost" data-jump="${next.id}">${I18N.t("toDate")}</button>
        ${next.location ? `<a class="btn ghost" href="${mapsSearchURL(next.location)}" target="_blank" rel="noopener noreferrer">${I18N.t("directions")}</a>` : ""}
        <button type="button" class="btn ghost" data-comments="${next.id}">${I18N.t("comments")}</button>
        <button type="button" class="btn ghost" data-titles="${next.id}">${I18N.t("titles")}</button>
        <button type="button" class="btn ghost" data-event-chat="${next.id}">${I18N.t("chatBrand")}</button>
      </div>
    </div>
    ${next.schedule ? `<div class="schedule-box">
      <p class="label">${I18N.t("schedule")}</p>
      <p class="schedule-text">${escapeHtml(next.schedule)}</p>
    </div>` : ""}`;
}

function renderOverview() {
  const overview = document.getElementById("overview");
  const list = document.getElementById("overview-list");
  const empty = document.getElementById("overview-empty");
  overview.hidden = false;
  const rows = upcomingDates();
  empty.hidden = rows.length > 0;
  list.hidden = rows.length === 0;
  list.innerHTML = rows.map((d) => {
    const when = pollOpen(d) ? `${formatWhen(d.startsAt)} · ${I18N.t("poll")}` : formatWhen(d.startsAt);
    const vote = pollOpen(d)
      ? `<span class="badge voting">${escapeHtml(overviewPollVote(d))}</span>`
      : `<span class="badge ${d.myChoice}">${voteLabel(d.myChoice)}</span>`;
    return `<button type="button" class="overview-item" data-jump="${d.id}">
      <div>
        <strong>${escapeHtml(d.title)}</strong>
        <p>${escapeHtml(when)} · ${escapeHtml(I18N.category(d.category))}</p>
      </div>
      <div class="overview-item-meta">
        <span class="badge ${d.status}">${I18N.status(d.status)}</span>
        ${vote}
      </div>
    </button>`;
  }).join("");
}

function spiritLeaders(rank) {
  if (rank?.leaders?.length) return rank.leaders;
  return rank?.leader ? [rank.leader] : [];
}

function renderSpirit() {
  const box = document.getElementById("spirit");
  const leaders = spiritLeaders(ranking);
  if (!leaders.length) {
    box.hidden = true;
    box.innerHTML = "";
    return;
  }
  const mine = typeof ranking.myScore === "number"
    ? `<div class="spirit-mine">
        <span class="spirit-mine-label">${I18N.t("myPoints")}</span>
        <strong class="spirit-score">${ranking.myScore}</strong>
      </div>`
    : "";
  const people = leaders.map((leader) => `
      <div class="spirit-person">
        <strong>${escapeHtml(leader.nickname)}</strong>
        <span>${escapeHtml(I18N.subrole(leader.subrole))}</span>
      </div>`).join("");
  box.hidden = false;
  box.innerHTML = `
    <p class="brand" data-i18n="spiritOfTheYear">${I18N.t("spiritOfTheYear")}</p>
    <div class="spirit-row">
      <div class="spirit-leader">
        ${people}
        <span class="spirit-score">${leaders[0].score} ${I18N.t("spiritPoints")}</span>
      </div>
      ${mine}
    </div>`;
}

function render() {
  renderSpirit();
  renderNextUp();
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

function archiveFileURL(id, file) {
  if (!file?.id) return "#";
  const q = new URLSearchParams();
  if (file.updatedAt) q.set("v", file.updatedAt);
  const qs = q.toString();
  return `/api/archive/${encodeURIComponent(id)}/files/${encodeURIComponent(file.id)}${qs ? `?${qs}` : ""}`;
}

function archiveRoleLabel(role) {
  const id = String(role || "").trim();
  if (!id) return "";
  if (["choir", "band", "orchestra", "technician"].includes(id)) return I18N.role(id);
  return id;
}

function archiveFileLabel(file, kindLabel) {
  const bits = [kindLabel];
  const role = archiveRoleLabel(file.role);
  if (role) bits.push(role);
  if (file.name) bits.push(file.name);
  return bits.join(" · ");
}

function titleMaterialHTML(item) {
  const files = item.files || [];
  const audios = files.filter((f) => f.kind === "audio");
  const lyrics = files.filter((f) => f.kind === "lyrics");
  const sheets = files.filter((f) => f.kind === "sheet");
  const preferred = sheets.filter((f) => f.role && f.role === me?.role);
  const rest = sheets.filter((f) => !preferred.includes(f));
  let html = `
    <button type="button" class="btn ghost" data-titles-back>${I18N.t("backToTitles")}</button>
    <div>
      <strong>${escapeHtml(item.title)}</strong>
      ${item.composer ? `<p class="muted">${escapeHtml(item.composer)}</p>` : ""}
    </div>`;
  audios.forEach((audio) => {
    html += `<div><p class="label">${escapeHtml(archiveFileLabel(audio, I18N.t("archiveAudio")))}</p><audio controls src="${archiveFileURL(item.id, audio)}"></audio></div>`;
  });
  lyrics.forEach((f) => {
    html += filePreviewHTML(item.id, f, archiveFileLabel(f, I18N.t("archiveLyrics")));
  });
  [...preferred, ...rest].forEach((f) => {
    html += filePreviewHTML(item.id, f, archiveFileLabel(f, I18N.t("archiveSheet")));
  });
  if (!audios.length && !lyrics.length && !sheets.length) {
    html += `<p class="muted">${I18N.t("archiveNoFile")}</p>`;
  }
  return html;
}

function filePreviewHTML(id, file, label) {
  const url = archiveFileURL(id, file);
  let body = "";
  if ((file.mime || "").startsWith("image/")) {
    body = `<img class="title-preview-img" src="${url}" alt="" />`;
  } else if ((file.mime || "").startsWith("text/")) {
    body = `<pre class="title-preview-text" data-text-src="${url}"></pre>`;
  } else {
    body = `<iframe class="title-preview" src="${url}" title="${escapeHtml(label)}"></iframe>`;
  }
  return `<div>
    <p class="label">${escapeHtml(label)}</p>
    ${body}
    <p><a class="btn ghost" href="${url}" target="_blank" rel="noopener">${I18N.t("fileOpen")}</a></p>
  </div>`;
}

function showTitlesList() {
  const date = dates.find((d) => d.id === titlesDateId);
  const list = document.getElementById("titles-list");
  const detail = document.getElementById("title-detail");
  detail.hidden = true;
  detail.innerHTML = "";
  list.hidden = false;
  const items = date?.titles || [];
  list.innerHTML = items.length
    ? items.map((item) => `
      <button type="button" class="title-item" data-title="${item.id}">
        <div>
          <strong>${escapeHtml(item.title)}</strong>
          ${item.composer ? `<p>${escapeHtml(item.composer)}</p>` : ""}
        </div>
      </button>`).join("")
    : `<p class="muted">${I18N.t("noTitles")}</p>`;
}

function openTitles(id) {
  const date = dates.find((d) => d.id === id);
  if (!date) return;
  titlesDateId = id;
  document.getElementById("titles-heading").textContent = date.title;
  showTitlesList();
  document.getElementById("titles-dialog").showModal();
}

async function openTitleDetail(itemId) {
  const list = document.getElementById("titles-list");
  const detail = document.getElementById("title-detail");
  try {
    const data = await api(`/api/archive/${encodeURIComponent(itemId)}`);
    list.hidden = true;
    detail.hidden = false;
    detail.innerHTML = titleMaterialHTML(data.item);
    for (const el of detail.querySelectorAll("[data-text-src]")) {
      try {
        const res = await fetch(el.dataset.textSrc, { credentials: "same-origin" });
        el.textContent = await res.text();
      } catch {
        el.textContent = "";
      }
    }
  } catch (err) {
    alert(err.message);
  }
}

function openSchedule(id) {
  const date = dates.find((d) => d.id === id);
  if (!date) return;
  document.getElementById("schedule-heading").textContent = date.title;
  const body = document.getElementById("schedule-body");
  body.innerHTML = date.schedule
    ? `<p class="schedule-text">${escapeHtml(date.schedule)}</p>`
    : `<p class="muted">${I18N.t("noSchedule")}</p>`;
  document.getElementById("schedule-dialog").showModal();
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
  ranking = data.ranking || { year: 0, leaders: [] };
  render();
}

function showGate(which) {
  loginView.hidden = which !== "login";
  pwView.hidden = which !== "password";
  appView.hidden = which !== "app";
}

async function enterApp() {
  showGate("app");
  document.getElementById("who-name").textContent = me.nickname;
  document.getElementById("who-meta").textContent = `${I18N.role(me.role)} · ${I18N.subrole(me.subrole)} · ${me.email}`;
  Photo.paint(document.getElementById("who-photo"), me);
  paintInfoButton(document.getElementById("who-info"), me);
  paintMyChannels();
  renderChatTabs();
  await loadDates();
  connectWS();
}

async function boot() {
  try {
    const data = await api("/api/me");
    me = data.user;
    if (me.mustChangePassword) {
      showGate("password");
      document.getElementById("pw-new").value = "";
      document.getElementById("pw-confirm").value = "";
      showError(document.getElementById("pw-error"), "");
      return;
    }
    await enterApp();
  } catch {
    me = null;
    showGate("login");
  }
}

document.getElementById("pw-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const errEl = document.getElementById("pw-error");
  showError(errEl, "");
  const next = document.getElementById("pw-new").value;
  const confirm = document.getElementById("pw-confirm").value;
  if (next !== confirm) {
    showError(errEl, I18N.t("errPasswordMismatch"));
    return;
  }
  try {
    const data = await api("/api/me/password", {
      method: "PATCH",
      body: JSON.stringify({ password: next }),
    });
    me = data.user;
    await enterApp();
  } catch (err) {
    showError(errEl, err.message);
  }
});

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

async function saveMyChannel(n, comment, v48) {
  const errEl = document.getElementById("channel-error");
  showError(errEl, "");
  const data = await api(`/api/me/channels/${n}`, {
    method: "PATCH",
    body: JSON.stringify({ comment, v48: !!v48 }),
  });
  me = data.user;
  paintMyChannels();
}

function savedChannelComment(n) {
  return (me?.channels || []).find((ch) => ch.number === n)?.comment || "";
}

function paintChannelDirty(input) {
  if (!input) return;
  const form = input.closest("form[data-channel]");
  if (!form) return;
  input.classList.toggle("dirty", input.value !== savedChannelComment(Number(form.dataset.channel)));
}

document.getElementById("my-channels").addEventListener("input", (e) => {
  const input = e.target.closest("input[name=comment]");
  if (input) paintChannelDirty(input);
});

document.getElementById("my-channels").addEventListener("submit", async (e) => {
  const form = e.target.closest("form[data-channel]");
  if (!form) return;
  e.preventDefault();
  const n = Number(form.dataset.channel);
  const comment = form.querySelector("[name=comment]")?.value || "";
  const v48 = form.querySelector("[data-channel-v48]")?.getAttribute("aria-pressed") === "true";
  try {
    await saveMyChannel(n, comment, v48);
  } catch (err) {
    showError(document.getElementById("channel-error"), err.message);
  }
});

document.getElementById("my-channels").addEventListener("click", async (e) => {
  const btn = e.target.closest("[data-channel-v48]");
  if (!btn) return;
  const form = btn.closest("form[data-channel]");
  if (!form) return;
  const n = Number(form.dataset.channel);
  try {
    await saveMyChannel(n, savedChannelComment(n), btn.getAttribute("aria-pressed") !== "true");
  } catch (err) {
    showError(document.getElementById("channel-error"), err.message);
  }
});

document.getElementById("overview-list").addEventListener("click", (e) => {
  const row = e.target.closest("[data-jump]");
  if (!row) return;
  jumpToDate(row.dataset.jump);
});

document.getElementById("next-up").addEventListener("click", async (e) => {
  const jump = e.target.closest("[data-jump]");
  if (jump) {
    jumpToDate(jump.dataset.jump);
    return;
  }
  const commentsBtn = e.target.closest("[data-comments]");
  if (commentsBtn) {
    openComments(commentsBtn.dataset.comments);
    return;
  }
  const scheduleBtn = e.target.closest("[data-schedule]");
  if (scheduleBtn) {
    openSchedule(scheduleBtn.dataset.schedule);
    return;
  }
  const titlesBtn = e.target.closest("[data-titles]");
  if (titlesBtn) {
    openTitles(titlesBtn.dataset.titles);
    return;
  }
  const eventChat = e.target.closest("[data-event-chat]");
  if (!eventChat) return;
  const date = dates.find((d) => d.id === eventChat.dataset.eventChat);
  if (!date) return;
  try {
    await openChat(`event:${date.id}`, date.title);
  } catch (err) {
    alert(err.message);
  }
});

datesEl.addEventListener("click", async (e) => {
  const commentsBtn = e.target.closest("button[data-comments]");
  if (commentsBtn) {
    openComments(commentsBtn.dataset.comments);
    return;
  }
  const scheduleBtn = e.target.closest("button[data-schedule]");
  if (scheduleBtn) {
    openSchedule(scheduleBtn.dataset.schedule);
    return;
  }
  const titlesBtn = e.target.closest("button[data-titles]");
  if (titlesBtn) {
    openTitles(titlesBtn.dataset.titles);
    return;
  }
  const eventChat = e.target.closest("button[data-event-chat]");
  if (eventChat) {
    const date = dates.find((d) => d.id === eventChat.dataset.eventChat);
    if (!date) return;
    try {
      await openChat(`event:${date.id}`, date.title);
    } catch (err) {
      alert(err.message);
    }
    return;
  }
  const btn = e.target.closest("button[data-choice]");
  if (!btn || btn.disabled) return;
  try {
    if (btn.dataset.option) {
      await api(`/api/dates/${btn.dataset.id}/poll`, {
        method: "POST",
        body: JSON.stringify({ optionId: btn.dataset.option, choice: btn.dataset.choice }),
      });
    } else {
      await api(`/api/dates/${btn.dataset.id}/vote`, {
        method: "POST",
        body: JSON.stringify({ choice: btn.dataset.choice }),
      });
    }
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

function memberColor(id) {
  let h = 0;
  for (const ch of String(id || "")) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  return MEMBER_COLORS[h % MEMBER_COLORS.length];
}

function formatChatWhen(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  const now = new Date();
  const sameDay = d.toDateString() === now.toDateString();
  return d.toLocaleString(I18N.locale(), sameDay
    ? { hour: "2-digit", minute: "2-digit" }
    : { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
}

function renderChatTabs() {
  const box = document.getElementById("chat-tabs");
  let any = false;
  box.querySelectorAll("[data-chat]").forEach((btn) => {
    const show = !!(me && CHAT_ROOMS.includes(me.role) && btn.dataset.chat === me.role);
    btn.hidden = !show;
    btn.classList.toggle("on", show && chatRoom === me.role);
    btn.textContent = I18N.role(btn.dataset.chat);
    if (show) any = true;
  });
  box.hidden = !any;
}

function chatAuthorKey(m) {
  return m.isAdmin ? "admin" : m.userId;
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
    <button type="button" class="chat-react-add" data-pick="${m.id}" aria-label="${escapeHtml(I18N.t("chatReact"))}">😊</button>
    <div class="chat-picker" hidden data-picker="${m.id}">${picks}</div>
  </div>`;
}

function chatMessageHTML(m, stacked) {
  const mine = !!(me && !m.isAdmin && m.userId === me.id);
  const color = m.isAdmin ? "#f0a35e" : memberColor(m.userId);
  const face = mine ? "" : Photo.html({
    id: m.isAdmin ? "" : m.userId,
    nickname: m.nickname,
    hasPhoto: !m.isAdmin && m.hasPhoto,
    photoUpdatedAt: m.photoUpdatedAt,
  }, "sm");
  return `<article class="chat-row ${mine ? "mine" : "theirs"}${stacked ? " stack" : ""}" data-msg="${m.id}">
    ${face}
    <div class="chat-col">
      <div class="chat-bubble" data-msg="${m.id}">
        <p class="chat-name" style="color:${color}">${escapeHtml(m.nickname)}</p>
        <p class="chat-text">${escapeHtml(m.text)}</p>
        <span class="chat-time">${escapeHtml(formatChatWhen(m.createdAt))}</span>
      </div>
      ${chatReactionsHTML(m)}
    </div>
  </article>`;
}

function renderChat(keepTop) {
  const list = document.getElementById("chat-list");
  const atBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 48;
  list.innerHTML = chatMessages.length
    ? chatMessages.map((m, i) => {
      const prev = chatMessages[i - 1];
      return chatMessageHTML(m, !!(prev && chatAuthorKey(prev) === chatAuthorKey(m)));
    }).join("")
    : `<p class="muted">${I18N.t("noMessages")}</p>`;
  list.scrollTop = keepTop != null && !atBottom ? keepTop : list.scrollHeight;
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

function removeChat(id) {
  const next = chatMessages.filter((m) => m.id !== id);
  if (next.length === chatMessages.length) return;
  const list = document.getElementById("chat-list");
  chatMessages = next;
  renderChat(list.scrollTop);
}

function canDeleteChat(m) {
  return !!(me && m && !m.isAdmin && m.userId === me.id);
}

function chatTitle(room, title) {
  if (title) return title;
  if (CHAT_ROOMS.includes(room)) return I18N.t(`chat.${room}`);
  return I18N.t("eventChat");
}

async function openChat(room, title) {
  const event = room.startsWith("event:");
  if (!me || (!event && me.role !== room)) return;
  chatRoom = room;
  renderChatTabs();
  document.getElementById("chat-title").textContent = chatTitle(room, title);
  document.getElementById("chat-text").value = "";
  showError(document.getElementById("chat-error"), "");
  const data = await api(`/api/chats/${encodeURIComponent(room)}`);
  chatMessages = data.messages || [];
  renderChat();
  const input = document.getElementById("chat-text");
  input.placeholder = I18N.t("chatWrite");
  document.getElementById("chat-dialog").showModal();
  input.focus();
}

function connectWS() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${proto}://${location.host}/ws/client`);
  ws.onmessage = (ev) => {
    let msg = {};
    try { msg = JSON.parse(ev.data); } catch { return; }
    if (msg.type === "chat") {
      if (chatRoom && msg.data?.room === chatRoom) appendChat(msg.data);
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
    if (msg.type === "changed") {
      api("/api/me").then((data) => {
        me = data.user;
        if (me.mustChangePassword) {
          showGate("password");
          return;
        }
        paintMyChannels();
      }).catch(() => {});
      if (!me?.mustChangePassword) {
        loadDates().catch(() => {});
        if (document.getElementById("proposals-dialog").open) loadProposals().catch(() => {});
      }
    }
  };
  ws.onclose = () => setTimeout(connectWS, 2000);
}

I18N.onChange(() => {
  I18N.apply();
  if (me && !me.mustChangePassword) {
    document.getElementById("who-name").textContent = me.nickname;
    document.getElementById("who-meta").textContent = `${I18N.role(me.role)} · ${I18N.subrole(me.subrole)} · ${me.email}`;
    Photo.paint(document.getElementById("who-photo"), me);
    paintInfoButton(document.getElementById("who-info"), me);
    paintMyChannels();
    renderChatTabs();
    if (document.getElementById("chat-dialog").open && chatRoom) {
      const date = chatRoom.startsWith("event:")
        ? dates.find((d) => d.id === chatRoom.slice("event:".length))
        : null;
      document.getElementById("chat-title").textContent = chatTitle(chatRoom, date?.title);
      document.getElementById("chat-text").placeholder = I18N.t("chatWrite");
      renderChat(document.getElementById("chat-list").scrollTop);
    }
    if (document.getElementById("proposals-dialog").open) renderProposalList();
    render();
  }
});

document.getElementById("chat-tabs").addEventListener("click", async (e) => {
  const btn = e.target.closest("[data-chat]");
  if (!btn || btn.hidden) return;
  try {
    await openChat(btn.dataset.chat);
  } catch (err) {
    alert(err.message);
  }
});

document.getElementById("chat-close").addEventListener("click", () => {
  document.getElementById("chat-dialog").close();
  chatRoom = "";
  chatMessages = [];
  renderChatTabs();
});

document.getElementById("chat-dialog").addEventListener("close", () => {
  chatRoom = "";
  chatMessages = [];
  renderChatTabs();
});

async function sendChat() {
  if (!chatRoom) return;
  const input = document.getElementById("chat-text");
  const errEl = document.getElementById("chat-error");
  showError(errEl, "");
  const text = input.value.trim();
  if (!text) return;
  try {
    const data = await api(`/api/chats/${encodeURIComponent(chatRoom)}`, {
      method: "POST",
      body: JSON.stringify({ text }),
    });
    input.value = "";
    input.style.height = "";
    appendChat(data.message);
  } catch (err) {
    showError(errEl, err.message);
  }
}

document.getElementById("chat-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  await sendChat();
});

document.getElementById("chat-text").addEventListener("keydown", (e) => {
  if (e.key !== "Enter" || e.shiftKey) return;
  e.preventDefault();
  sendChat();
});

document.getElementById("schedule-close").addEventListener("click", () => {
  document.getElementById("schedule-dialog").close();
});

document.getElementById("titles-close").addEventListener("click", () => {
  document.getElementById("titles-dialog").close();
});

document.getElementById("titles-dialog").addEventListener("close", () => {
  titlesDateId = "";
  document.getElementById("title-detail").innerHTML = "";
});

document.getElementById("titles-list").addEventListener("click", async (e) => {
  const btn = e.target.closest("[data-title]");
  if (!btn) return;
  await openTitleDetail(btn.dataset.title);
});

document.getElementById("title-detail").addEventListener("click", (e) => {
  if (!e.target.closest("[data-titles-back]")) return;
  showTitlesList();
});

document.getElementById("chat-list").addEventListener("dblclick", async (e) => {
  if (e.target.closest(".chat-reacts")) return;
  const bubble = e.target.closest("[data-msg]");
  if (!bubble || !chatRoom) return;
  const m = chatMessages.find((x) => x.id === bubble.dataset.msg);
  if (!canDeleteChat(m)) return;
  if (!confirm(I18N.t("confirmDeleteMessage"))) return;
  try {
    await api(`/api/chats/${encodeURIComponent(chatRoom)}/messages/${encodeURIComponent(m.id)}`, { method: "DELETE" });
    removeChat(m.id);
  } catch (err) {
    showError(document.getElementById("chat-error"), err.message);
  }
});

document.getElementById("chat-list").addEventListener("click", async (e) => {
  const pick = e.target.closest("[data-pick]");
  if (pick) {
    const box = document.querySelector(`[data-picker="${pick.dataset.pick}"]`);
    if (box) box.hidden = !box.hidden;
    return;
  }
  const btn = e.target.closest("[data-react]");
  if (!btn || !chatRoom) return;
  try {
    const data = await api(`/api/chats/${encodeURIComponent(chatRoom)}/messages/${encodeURIComponent(btn.dataset.id)}/react`, {
      method: "POST",
      body: JSON.stringify({ emoji: btn.dataset.react }),
    });
    applyReactions(data.message.id, data.message.reactions);
  } catch (err) {
    showError(document.getElementById("chat-error"), err.message);
  }
});

function hasInfo(user) {
  return !!(user?.address || user?.phone || user?.birthday);
}

function paintInfoButton(btn, user) {
  if (!btn) return;
  btn.textContent = I18N.t(user?.birthday ? "modifyInfo" : "addInfo");
  btn.classList.toggle("has-info", hasInfo(user));
}

function fillInfoForm(user = {}) {
  document.getElementById("info-address").value = user.address || "";
  document.getElementById("info-phone").value = user.phone || "";
  document.getElementById("info-birthday").value = user.birthday || "";
  showError(document.getElementById("info-error"), "");
}

function readInfoForm() {
  return {
    address: document.getElementById("info-address").value,
    phone: document.getElementById("info-phone").value,
    birthday: document.getElementById("info-birthday").value,
  };
}

document.getElementById("who-info").addEventListener("click", () => {
  fillInfoForm(me || {});
  document.getElementById("info-dialog").showModal();
});

document.getElementById("info-close").addEventListener("click", () => {
  document.getElementById("info-dialog").close();
});

document.getElementById("info-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const errEl = document.getElementById("info-error");
  showError(errEl, "");
  try {
    const data = await api("/api/me/info", { method: "PATCH", body: JSON.stringify(readInfoForm()) });
    me = data.user;
    Photo.paint(document.getElementById("who-photo"), me);
    paintInfoButton(document.getElementById("who-info"), me);
    document.getElementById("info-dialog").close();
  } catch (err) {
    showError(errEl, err.message);
  }
});

function proposalBadge(status) {
  if (status === "accepted") return "accepted";
  if (status === "declined") return "cancelled";
  return "voting";
}

function proposalLink(url) {
  if (!url) return "";
  return `<a href="${escapeHtml(url)}" target="_blank" rel="noopener noreferrer">${escapeHtml(url)}</a>`;
}

async function loadProposals() {
  const data = await api("/api/proposals");
  proposals = data.proposals || [];
  renderProposalList();
}

function renderProposalList() {
  const list = document.getElementById("proposals-list");
  if (!proposals.length) {
    list.innerHTML = `<p class="muted">${I18N.t("noProposals")}</p>`;
    return;
  }
  list.innerHTML = proposals.map((p) => `
    <article class="proposal-card">
      <div class="proposal-head">
        <strong>${escapeHtml(p.title)}</strong>
        <span class="badge ${proposalBadge(p.status)}">${escapeHtml(I18N.status(p.status))}</span>
      </div>
      <p class="meta">${escapeHtml(I18N.t("proposedBy"))} ${escapeHtml(p.nickname)} · ${escapeHtml(formatWhen(p.createdAt))}</p>
      ${p.url ? `<p class="proposal-url">${proposalLink(p.url)}</p>` : ""}
      ${p.comment ? `<p class="proposal-comment"><span class="label">${escapeHtml(I18N.t("adminComment"))}</span>${escapeHtml(p.comment)}</p>` : ""}
    </article>
  `).join("");
}

document.getElementById("btn-propose").addEventListener("click", () => {
  document.getElementById("propose-form").reset();
  showError(document.getElementById("propose-error"), "");
  document.getElementById("propose-dialog").showModal();
  document.getElementById("propose-title").focus();
});

document.getElementById("propose-close").addEventListener("click", () => {
  document.getElementById("propose-dialog").close();
});

document.getElementById("propose-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const errEl = document.getElementById("propose-error");
  showError(errEl, "");
  try {
    await api("/api/proposals", {
      method: "POST",
      body: JSON.stringify({
        title: document.getElementById("propose-title").value,
        url: document.getElementById("propose-url").value,
      }),
    });
    document.getElementById("propose-dialog").close();
  } catch (err) {
    showError(errEl, err.message);
  }
});

document.getElementById("btn-proposals").addEventListener("click", async () => {
  try {
    await loadProposals();
    document.getElementById("proposals-dialog").showModal();
  } catch (err) {
    alert(err.message);
  }
});

document.getElementById("proposals-close").addEventListener("click", () => {
  document.getElementById("proposals-dialog").close();
});

Photo.bind({
  canRemove: () => !!(me && me.hasPhoto),
  onFile: async (blob) => {
    const data = await Photo.upload("/api/me/photo", blob);
    me = data.user;
    Photo.paint(document.getElementById("who-photo"), me);
    paintInfoButton(document.getElementById("who-info"), me);
  },
  onRemove: async () => {
    const data = await Photo.remove("/api/me/photo");
    me = data.user;
    Photo.paint(document.getElementById("who-photo"), me);
    paintInfoButton(document.getElementById("who-info"), me);
  },
});

boot();
