const CHOICES = ["yes", "maybe", "no", "unknown"];
const CHOIR_VOICES = ["Sopran", "Alt", "Tenor/Bass"];

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
let directory = [];
let archiveItems = [];
let commentDateId = "";
let titlesDateId = "";
let galleryDateId = "";
let galleryItems = [];
let chatRoom = "";
let chatMessages = [];
let chatUnread = {};
let mixer = { channels: [], people: [] };
let memberWS = null;
let liveStream = null;
let pubStream = null;
let pubPeers = {};
let subPeer = null;

const CHAT_ROOMS = ["choir", "band", "orchestra"];
const CHAT_API = "/api/chats";
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

function sameCalendarDay(a, b) {
  return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
}

function formatDay(d) {
  return d.toLocaleDateString(I18N.locale(), { weekday: "short", day: "numeric", month: "short", year: "numeric" });
}

function formatClock(d) {
  const m = d.getMinutes();
  return m ? `${d.getHours()}:${String(m).padStart(2, "0")}` : String(d.getHours());
}

function formatStartEnd(startIso, endIso) {
  if (!startIso) return "";
  const start = new Date(startIso);
  if (!endIso) return formatWhen(startIso);
  const end = new Date(endIso);
  if (sameCalendarDay(start, end)) {
    return `${formatDay(start)}, ${formatClock(start)}–${formatClock(end)}${I18N.t("clockSuffix")}`;
  }
  return `${formatWhen(startIso)} – ${formatWhen(endIso)}`;
}

function formatRange(date) {
  let s = formatStartEnd(date.startsAt, date.endsAt);
  if (date.location) s += " · " + date.location;
  return s;
}

function formatOptionRange(opt) {
  return formatStartEnd(opt.startsAt, opt.endsAt);
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

function choirVoiceYes(date) {
  const counts = Object.fromEntries(CHOIR_VOICES.map((v) => [v, 0]));
  const present = (e) => e.role === "choir" && e.choice === "yes" && !e.attendance && counts[e.subrole] !== undefined;
  if (pollOpen(date) && (date.options || []).length) {
    for (const voice of CHOIR_VOICES) {
      counts[voice] = Math.max(0, ...(date.options || []).map((o) =>
        (o.roster || []).filter((e) => e.role === "choir" && e.subrole === voice && e.choice === "yes" && !e.attendance).length));
    }
    return counts;
  }
  for (const e of date.roster || []) {
    if (present(e)) counts[e.subrole]++;
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

function moodHTML(date, extraClass = "") {
  const mood = participationMood(date);
  if (!mood) return "";
  const counts = choirVoiceYes(date);
  const detail = CHOIR_VOICES.map((v) => `${I18N.subrole(v)} ${counts[v]}`).join(" · ");
  return `<span class="mood ${extraClass}" title="${escapeHtml(detail)}" aria-label="${escapeHtml(I18N.t(mood.key))}">${mood.emoji}</span>`;
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
    const buttons = canVote() ? CHOICES.map((c) => `
      <button type="button" data-id="${date.id}" data-option="${o.id}" data-choice="${c}" class="${c}${o.myChoice === c ? " on" : ""}" ${locked ? "disabled" : ""}>${voteLabel(c)}</button>
    `).join("") : "";
    return `<div class="poll-option ${o.frozen ? "frozen" : ""}">
      <p class="when">${escapeHtml(formatOptionRange(o))}${o.frozen ? ` · ${I18N.t("chosenTime")}` : ""}</p>
      ${buttons ? `<div class="vote-row">${buttons}</div>` : ""}
      ${canVote() ? mine : ""}
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
  const buttons = canVote() ? CHOICES.map((c) => `
    <button type="button" data-id="${date.id}" data-choice="${c}" class="${c}${date.myChoice === c ? " on" : ""}" ${locked ? "disabled" : ""}>${voteLabel(c)}</button>
  `).join("") : "";
  const mine = canVote() && date.myInitial && date.myInitial !== date.myChoice
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
  return `<article class="card${needsVote(date) ? " needs-vote" : ""}">
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
        ${moodHTML(date)}
      </div>
    </div>
    ${isPoll ? renderPoll(date) : ""}
    ${pollOpen(date) || !buttons ? "" : `<div class="vote-row">${buttons}</div>${mine}`}
    <div class="card-actions">
      <button type="button" class="btn ghost" data-comments="${date.id}">${I18N.t("comments")}${commentCount ? ` (${commentCount})` : ""}</button>
      ${date.schedule ? `<button type="button" class="btn ghost" data-schedule="${date.id}">${I18N.t("schedule")}</button>` : ""}
      <button type="button" class="btn ghost" data-titles="${date.id}">${I18N.t("titles")}${(date.titles || []).length ? ` (${date.titles.length})` : ""}</button>
      ${date.chatOpen ? `<button type="button" class="btn ghost" data-event-chat="${date.id}">${I18N.t("eventChat")}</button>` : ""}
      <button type="button" class="btn ghost" data-gallery="${date.id}">${I18N.t("gallery")}${date.galleryCount ? ` (${date.galleryCount})` : ""}</button>
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

function hasAnswered(d) {
  if (pollOpen(d)) {
    const options = d.options || [];
    return options.length > 0 && options.every((o) => o.myChoice === "yes" || o.myChoice === "maybe" || o.myChoice === "no");
  }
  return d.myChoice === "yes" || d.myChoice === "maybe" || d.myChoice === "no";
}

function canVote() {
  return me?.role !== "ehemalige";
}

function canUseChatRoom(room) {
  if (!me || !CHAT_ROOMS.includes(room)) return false;
  if (me.role === "chorleiter") return true;
  if (me.role === "ehemalige") return room === "choir";
  return me.role === room;
}

function primaryChatRoom() {
  return CHAT_ROOMS.find((room) => canUseChatRoom(room)) || "";
}

function needsVote(d) {
  return !!(canVote() && d && d.status !== "cancelled" && !hasAnswered(d));
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

function canAssignChannels() {
  return me?.role === "technician";
}

function paintMyChannels() {
  const box = document.getElementById("my-channels");
  if (box) box.classList.toggle("mixer-desk", canAssignChannels());
  if (canAssignChannels()) {
    paintMixerDesk();
    return;
  }
  const header = document.getElementById("who-channels");
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

function mixerPeople() {
  return mixer.people || [];
}

function savedMixerComment(n) {
  return (mixer.channels || []).find((ch) => ch.number === n)?.comment || "";
}

function paintMixerDesk() {
  const header = document.getElementById("who-channels");
  const box = document.getElementById("my-channels");
  if (!box) return;
  const channels = mixer.channels || [];
  const assigned = channels.filter((ch) => ch.userId).length;
  const q = (document.getElementById("mixer-search")?.value || "").trim().toLowerCase();
  const dirty = {};
  box.querySelectorAll("[data-mixer-comment].dirty").forEach((input) => {
    const n = Number(input.dataset.mixerComment);
    if (input.value !== savedMixerComment(n)) dirty[String(n)] = input.value;
  });
  if (header) {
    header.hidden = false;
    header.textContent = `${I18N.t("channels")} · ${assigned}/${channels.length || 96}`;
  }
  const people = mixerPeople();
  const rows = channels.filter((ch) => {
    if (!q) return true;
    const hay = `${ch.number} ${ch.nickname || ""} ${ch.comment || ""} ${I18N.role(ch.role || "")}${ch.v48 ? " 48v" : ""}`.toLowerCase();
    return hay.includes(q);
  }).map((ch) => {
    const draft = Object.hasOwn(dirty, String(ch.number));
    const value = draft ? dirty[String(ch.number)] : (ch.comment || "");
    const opts = [`<option value="">${I18N.t("channelNone")}</option>`]
      .concat(people.map((u) => `<option value="${u.id}" ${u.id === ch.userId ? "selected" : ""}>${escapeHtml(u.nickname)} · ${escapeHtml(I18N.role(u.role))}</option>`))
      .join("");
    return `<tr>
      <td>${ch.number}</td>
      <td><select data-mixer-user="${ch.number}">${opts}</select></td>
      <td><input data-mixer-comment="${ch.number}" maxlength="200" value="${escapeHtml(value)}" placeholder="${escapeHtml(I18N.t("channelComment"))}" class="${draft ? "dirty" : ""}" /></td>
      <td><button type="button" class="btn ghost v48-btn${ch.v48 ? " on" : ""}" data-mixer-v48="${ch.number}" aria-pressed="${ch.v48 ? "true" : "false"}">${I18N.t("channel48v")}</button></td>
    </tr>`;
  }).join("");
  box.hidden = false;
  box.innerHTML = `
    <p class="brand">${I18N.t("channels")}</p>
    <p class="muted mixer-lede">${I18N.t("channelsAssignHint")}</p>
    <label class="mixer-search-label" for="mixer-search">${I18N.t("search")}</label>
    <input id="mixer-search" type="search" autocomplete="off" value="${escapeHtml(q)}" />
    <div class="mixer-wrap">
      <table class="roster mixer-table">
        <thead><tr>
          <th>${I18N.t("channel")}</th>
          <th>${I18N.t("person")}</th>
          <th>${I18N.t("channelComment")}</th>
          <th>${I18N.t("channel48v")}</th>
        </tr></thead>
        <tbody>${rows || `<tr><td colspan="4" class="muted">${I18N.t("archivePickEmpty")}</td></tr>`}</tbody>
      </table>
    </div>
    <p id="channel-error" class="error" hidden></p>`;
}

async function loadMixer() {
  if (!canAssignChannels()) {
    mixer = { channels: [], people: [] };
    return;
  }
  const data = await api("/api/channels");
  mixer.channels = data.channels || [];
  mixer.people = data.people || [];
  paintMyChannels();
}

async function saveMixerChannel(number, v48) {
  const userEl = document.querySelector(`[data-mixer-user="${number}"]`);
  const commentEl = document.querySelector(`[data-mixer-comment="${number}"]`);
  const v48El = document.querySelector(`[data-mixer-v48="${number}"]`);
  if (!userEl || !commentEl) return;
  const nextV48 = typeof v48 === "boolean" ? v48 : v48El?.getAttribute("aria-pressed") === "true";
  showError(document.getElementById("channel-error"), "");
  const data = await api(`/api/channels/${number}`, {
    method: "PATCH",
    body: JSON.stringify({ userId: userEl.value, comment: commentEl.value, v48: !!nextV48 }),
  });
  const idx = (mixer.channels || []).findIndex((c) => c.number === number);
  if (idx >= 0) mixer.channels[idx] = data.channel;
  if (v48El) {
    v48El.classList.toggle("on", !!data.channel.v48);
    v48El.setAttribute("aria-pressed", data.channel.v48 ? "true" : "false");
  }
  commentEl.classList.remove("dirty");
  const header = document.getElementById("who-channels");
  if (header) {
    const assigned = (mixer.channels || []).filter((ch) => ch.userId).length;
    header.textContent = `${I18N.t("channels")} · ${assigned}/${(mixer.channels || []).length || 96}`;
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
    ${moodHTML(next)}
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
        <button type="button" class="btn ghost" data-gallery="${next.id}">${I18N.t("gallery")}${next.galleryCount ? ` (${next.galleryCount})` : ""}</button>
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
    const vote = !canVote()
      ? ""
      : pollOpen(d)
        ? `<span class="badge voting">${escapeHtml(overviewPollVote(d))}</span>`
        : `<span class="badge ${d.myChoice}">${voteLabel(d.myChoice)}</span>`;
    const pending = needsVote(d);
    return `<button type="button" class="overview-item${pending ? " needs-vote" : ""}" data-jump="${d.id}"${pending ? ` title="${escapeHtml(I18N.t("voteNeeded"))}"` : ""}">
      <div>
        <strong>${escapeHtml(d.title)}</strong>
        <p>${escapeHtml(when)} · ${escapeHtml(I18N.category(d.category))}</p>
      </div>
      <div class="overview-item-meta">
        ${moodHTML(d)}
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

function canSeeSpirit() {
  return me?.role === "choir" || me?.role === "chorleiter" || me?.role === "ehemalige";
}

function spiritPresencePct(yes, events) {
  if (typeof yes === "number" && events > 0) {
    return Math.round((yes / events) * 100);
  }
  return null;
}

function spiritMinePct(rank) {
  if (typeof rank?.myYes === "number") {
    return spiritPresencePct(rank.myYes, rank.events);
  }
  if (typeof rank?.participation === "number" && rank.events > 0) {
    return Math.round(rank.participation * 100);
  }
  return null;
}

function spiritStatHTML(pct, extraClass = "") {
  if (pct === null) return "";
  return `<div class="spirit-stat${extraClass ? ` ${extraClass}` : ""}">
        <span class="spirit-mine-label">${I18N.t("presence")}</span>
        <strong class="spirit-score">${pct}%</strong>
      </div>`;
}

function renderSpirit() {
  const box = document.getElementById("spirit");
  const leaders = spiritLeaders(ranking);
  if (!canSeeSpirit() || !leaders.length) {
    box.hidden = true;
    box.innerHTML = "";
    return;
  }
  const events = ranking.events || leaders[0].events || 0;
  const leaderPcts = leaders.map((leader) => spiritPresencePct(leader.yes, leader.events || events));
  const sharedPct = leaderPcts.length && leaderPcts.every((pct) => pct === leaderPcts[0]) ? leaderPcts[0] : null;
  const people = leaders.map((leader, i) => `
      <div class="spirit-person">
        <strong>${escapeHtml(leader.nickname)}</strong>
        <span>${escapeHtml(I18N.subrole(leader.subrole))}</span>
        ${sharedPct === null ? spiritStatHTML(leaderPcts[i]) : ""}
      </div>`).join("");
  box.hidden = false;
  box.innerHTML = `
    <p class="brand" data-i18n="spiritOfTheYear">${I18N.t("spiritOfTheYear")}</p>
    <div class="spirit-row">
      <div class="spirit-leader">
        ${people}
        ${spiritStatHTML(sharedPct, "spirit-lead-stat")}
      </div>
      <div class="spirit-mine">${spiritStatHTML(spiritMinePct(ranking))}</div>
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
  if (["choir", "chorleiter", "band", "orchestra", "technician", "ehemalige"].includes(id)) return I18N.role(id);
  return id;
}

function archiveFileLabel(file, kindLabel) {
  const bits = [kindLabel];
  const role = archiveRoleLabel(file.role);
  if (role) bits.push(role);
  if (file.name) bits.push(file.name);
  return bits.join(" · ");
}

function archiveKindAccept(kind) {
  if (kind === "audio" || kind === "tracks") return "audio/*";
  if (kind === "lyrics") return "application/pdf,text/plain,image/*";
  return "application/pdf,image/*";
}

function archiveManageHTML(item) {
  const kinds = [
    { kind: "audio", label: I18N.t("archiveAudio") },
    { kind: "tracks", label: I18N.t("archiveTracks") },
    { kind: "lyrics", label: I18N.t("archiveLyrics") },
    { kind: "sheet", label: I18N.t("archiveSheet") },
  ];
  const slots = kinds.map((slot) => `
    <section class="archive-kind">
      <p class="label">${escapeHtml(slot.label)}</p>
      <div class="archive-upload-row">
        <input data-archive-role="${slot.kind}" maxlength="40" placeholder="${escapeHtml(I18N.t("archiveRoleHint"))}" />
        <button type="button" class="btn ghost" data-archive-upload="${slot.kind}" data-accept="${archiveKindAccept(slot.kind)}">${I18N.t("archiveAddFile")}</button>
      </div>
    </section>`).join("");
  return `<div class="archive-manage" data-archive-id="${item.id}">
    <p class="muted">${I18N.t("archiveAttachHint")}</p>
    ${slots}
    <section class="archive-kind">
      <p class="label">${escapeHtml(I18N.t("archiveShareURL"))}</p>
      <div class="archive-upload-row">
        <input data-archive-link-name maxlength="120" placeholder="${escapeHtml(I18N.t("archiveURLName"))}" />
        <input data-archive-link-url maxlength="2000" placeholder="https://" />
        <button type="button" class="btn ghost" data-archive-add-link>${I18N.t("archiveAddURL")}</button>
      </div>
    </section>
  </div>`;
}

function titleMaterialHTML(item, back) {
  const files = item.files || [];
  const audios = files.filter((f) => f.kind === "audio");
  const tracks = files.filter((f) => f.kind === "tracks");
  const lyrics = files.filter((f) => f.kind === "lyrics");
  const sheets = files.filter((f) => f.kind === "sheet");
  const links = files.filter((f) => f.kind === "link" && f.url);
  const preferred = sheets.filter((f) => f.role && f.role === me?.role);
  const rest = sheets.filter((f) => !preferred.includes(f));
  const backAttr = back === "archive" ? "data-archive-back" : "data-titles-back";
  const backLabel = back === "archive" ? I18N.t("backToArchive") : I18N.t("backToTitles");
  let html = `
    <button type="button" class="btn ghost" ${backAttr}>${backLabel}</button>
    <div>
      <strong>${escapeHtml(item.title)}</strong>
      ${item.composer ? `<p class="muted">${escapeHtml(item.composer)}</p>` : ""}
    </div>`;
  audios.forEach((audio) => {
    html += `<div><p class="label">${escapeHtml(archiveFileLabel(audio, I18N.t("archiveAudio")))}</p><audio controls src="${archiveFileURL(item.id, audio)}"></audio></div>`;
  });
  tracks.forEach((track) => {
    html += `<div><p class="label">${escapeHtml(archiveFileLabel(track, I18N.t("archiveTracks")))}</p><audio controls src="${archiveFileURL(item.id, track)}"></audio></div>`;
  });
  lyrics.forEach((f) => {
    html += filePreviewHTML(item.id, f, archiveFileLabel(f, I18N.t("archiveLyrics")));
  });
  [...preferred, ...rest].forEach((f) => {
    html += filePreviewHTML(item.id, f, archiveFileLabel(f, I18N.t("archiveSheet")));
  });
  links.forEach((f) => {
    const label = f.name || I18N.t("archiveShareURL");
    html += `<div><p class="label">${escapeHtml(label)}</p><a class="btn ghost" href="${escapeHtml(f.url)}" target="_blank" rel="noopener">${escapeHtml(f.url)}</a></div>`;
  });
  if (!audios.length && !tracks.length && !lyrics.length && !sheets.length && !links.length) {
    html += `<p class="muted">${I18N.t("archiveNoFile")}</p>`;
  }
  return html;
}

function filePreviewHTML(id, file, label) {
  const url = archiveFileURL(id, file);
  const mime = file.mime || "";
  let body = "";
  if (mime.startsWith("image/")) {
    body = `<img class="title-preview-img" src="${url}" alt="" />`;
  } else if (mime.startsWith("text/")) {
    body = `<pre class="title-preview-text" data-text-src="${url}"></pre>`;
  }
  // Do not iframe PDFs or other binaries: many browsers download them on load.
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
  const copyBtn = document.getElementById("titles-copy");
  detail.hidden = true;
  detail.innerHTML = "";
  list.hidden = false;
  const items = date?.titles || [];
  if (copyBtn) copyBtn.disabled = items.length === 0;
  list.innerHTML = items.length
    ? items.map((item, i) => `
      <button type="button" class="title-item" data-title="${item.id}">
        <span class="title-num">${i + 1}</span>
        <div class="title-item-text">
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
    detail.innerHTML = titleMaterialHTML(data.item, "titles");
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

function galleryFileURL(dateId, item) {
  return `/api/dates/${encodeURIComponent(dateId)}/gallery/${encodeURIComponent(item.id)}`;
}

function renderGallery() {
  const list = document.getElementById("gallery-list");
  const date = dates.find((d) => d.id === galleryDateId);
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
    </article>`;
  }).join("");
}

async function openGallery(id) {
  const date = dates.find((d) => d.id === id);
  if (!date) return;
  galleryDateId = id;
  showError(document.getElementById("gallery-error"), "");
  try {
    const data = await api(`/api/dates/${encodeURIComponent(id)}/gallery`);
    galleryItems = data.gallery || [];
    renderGallery();
    const dialog = document.getElementById("gallery-dialog");
    if (!dialog.open) dialog.showModal();
    paintGallerySize();
  } catch (err) {
    alert(err.message);
  }
}

let knownDateIds = null;
const NOTICES_KEY = "spirit-notices";

function noticesWanted() {
  return localStorage.getItem(NOTICES_KEY) !== "off";
}

function setNoticesWanted(on) {
  localStorage.setItem(NOTICES_KEY, on ? "on" : "off");
}

function paintNoticesButton() {
  const btn = document.getElementById("btn-header-notices");
  if (!btn) return;
  const on = noticesWanted();
  btn.classList.toggle("on", on);
  btn.classList.toggle("off", !on);
  btn.setAttribute("aria-pressed", on ? "true" : "false");
  const label = I18N.t(on ? "noticesOn" : "noticesOff");
  btn.setAttribute("aria-label", label);
}

async function enableDesktopNotices() {
  paintNoticesButton();
  if (!noticesWanted() || !("Notification" in window)) return;
  if (Notification.permission === "default") {
    await Notification.requestPermission().catch(() => "denied");
    paintNoticesButton();
  }
}

let noticeToastTimer = 0;
let noticeToastData = null;

function hideInAppNotice() {
  const box = document.getElementById("notice-toast");
  if (box) box.hidden = true;
  clearTimeout(noticeToastTimer);
}

function showInAppNotice(title, body, data) {
  const box = document.getElementById("notice-toast");
  if (!box) return;
  noticeToastData = data || {};
  document.getElementById("notice-toast-title").textContent = title;
  document.getElementById("notice-toast-body").textContent = body || "";
  box.hidden = false;
  clearTimeout(noticeToastTimer);
  noticeToastTimer = setTimeout(hideInAppNotice, 8000);
}

function swRegistration() {
  if (!("serviceWorker" in navigator)) return Promise.resolve(null);
  return Promise.race([
    navigator.serviceWorker.ready,
    new Promise((resolve) => setTimeout(() => resolve(null), 800)),
  ]).catch(() => null);
}

async function showDesktopNotice(title, body, data = {}) {
  if (!noticesWanted()) return;
  showInAppNotice(title, body, data);
  if (!("Notification" in window) || Notification.permission !== "granted") return;
  const opts = {
    body: body || "",
    icon: "/static/nsgc-symbol.png",
    tag: data.tag || "spirit",
    renotify: true,
    data,
  };
  try {
    const n = new Notification(title, opts);
    n.onclick = () => {
      window.focus();
      handleNotifyClick(data);
      n.close();
    };
    return;
  } catch {}
  try {
    const reg = await swRegistration();
    if (reg) await reg.showNotification(title, opts);
  } catch {}
}

function chatNoticeBody(m) {
  if (m.kind === "voice") return I18N.t("chatVoice");
  if (m.kind === "image" || m.kind === "video") return I18N.t("chatMedia");
  return String(m.text || "").trim() || I18N.t("chatBrand");
}

function notifyChatMessage(m) {
  if (!m?.room) return;
  if (me && !m.isAdmin && m.userId === me.id) return;
  const openHere = document.getElementById("chat-dialog")?.open && chatRoom === m.room;
  if (openHere && document.visibilityState === "visible") return;
  const date = m.room.startsWith("event:")
    ? dates.find((d) => d.id === m.room.slice("event:".length))
    : null;
  const roomLabel = chatTitle(m.room, date?.title);
  const who = m.nickname || I18N.t("chatBrand");
  showDesktopNotice(`${who} · ${roomLabel}`, chatNoticeBody(m), {
    kind: "chat",
    room: m.room,
    tag: `chat:${m.room}`,
  });
}

function notifyNewDate(d) {
  if (!d) return;
  showDesktopNotice(I18N.t("newDate"), [d.title, formatWhen(d.startsAt)].filter(Boolean).join(" · "), {
    kind: "date",
    dateId: d.id,
    tag: `date:${d.id}`,
  });
}

async function handleNotifyClick(data) {
  if (!me || me.mustChangePassword) return;
  try {
    if (data?.kind === "chat" && data.room) {
      const title = data.room.startsWith("event:")
        ? dates.find((d) => d.id === data.room.slice("event:".length))?.title
        : "";
      await openChat(data.room, title);
      return;
    }
    if (data?.kind === "date" && data.dateId) jumpToDate(data.dateId);
    if (data?.kind === "stream") await startWatch();
  } catch (err) {
    alert(err.message);
  }
}

async function loadDates() {
  const data = await api("/api/dates");
  const next = data.dates || [];
  if (knownDateIds) {
    next.forEach((d) => {
      if (!knownDateIds.has(d.id)) notifyNewDate(d);
    });
  }
  knownDateIds = new Set(next.map((d) => d.id));
  dates = next;
  ranking = data.ranking || { year: 0, leaders: [] };
  applyUnread(data.unread);
  renderChatTabs();
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
  paintStreamButtons();
  await loadDates();
  await loadMixer().catch(() => {});
  connectWS();
  enableDesktopNotices();
}

let installPrompt = null;

function isStandaloneApp() {
  return window.matchMedia("(display-mode: standalone)").matches || window.navigator.standalone === true;
}

function paintInstallButtons() {
  document.querySelectorAll(".install-open").forEach((btn) => {
    btn.hidden = isStandaloneApp();
  });
}

function openInstallDialog() {
  const installed = document.getElementById("install-installed");
  const now = document.getElementById("install-now");
  installed.hidden = !isStandaloneApp();
  now.hidden = !installPrompt;
  document.getElementById("install-dialog").showModal();
}

function registerInstall() {
  paintInstallButtons();
  if ("serviceWorker" in navigator) {
    navigator.serviceWorker.register("/sw.js", { scope: "/" }).catch(() => {});
    navigator.serviceWorker.addEventListener("message", (e) => {
      if (e.data?.type !== "notify-click") return;
      handleNotifyClick(e.data);
    });
  }
  window.addEventListener("beforeinstallprompt", (e) => {
    e.preventDefault();
    installPrompt = e;
    paintInstallButtons();
  });
  window.addEventListener("appinstalled", () => {
    installPrompt = null;
    paintInstallButtons();
    document.getElementById("install-dialog").close();
  });
  document.querySelectorAll(".install-open").forEach((btn) => {
    btn.addEventListener("click", openInstallDialog);
  });
  document.getElementById("install-close").addEventListener("click", () => {
    document.getElementById("install-dialog").close();
  });
  document.getElementById("install-now").addEventListener("click", async () => {
    if (!installPrompt) return;
    installPrompt.prompt();
    const choice = await installPrompt.userChoice.catch(() => null);
    if (choice?.outcome === "accepted") installPrompt = null;
    paintInstallButtons();
    document.getElementById("install-dialog").close();
  });
}

async function boot() {
  registerInstall();
  try {
    const data = await api("/api/me");
    me = data.user;
    applyUnread(data.unread);
    applyLiveStream(data.stream, false);
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

async function signOut() {
  setMenuOpen(false);
  await api("/api/logout", { method: "POST" });
  me = null;
  knownDateIds = null;
  await boot();
}

document.getElementById("btn-logout").addEventListener("click", signOut);
document.getElementById("btn-logout-menu").addEventListener("click", signOut);

function setMenuOpen(open) {
  const nav = document.getElementById("top-actions");
  const btn = document.getElementById("btn-menu");
  if (open) setAvatarMenuOpen(false);
  nav.classList.toggle("open", open);
  document.body.classList.toggle("menu-open", open);
  btn.setAttribute("aria-expanded", open ? "true" : "false");
}

function setAvatarMenuOpen(open) {
  const menu = document.getElementById("avatar-menu");
  const btn = document.getElementById("who-photo");
  if (!menu || !btn) return;
  if (open) setMenuOpen(false);
  menu.hidden = !open;
  btn.setAttribute("aria-expanded", open ? "true" : "false");
}

document.getElementById("btn-menu").addEventListener("click", () => {
  setMenuOpen(!document.getElementById("top-actions").classList.contains("open"));
});
document.getElementById("menu-close").addEventListener("click", () => setMenuOpen(false));
document.getElementById("top-actions").addEventListener("click", (e) => {
  if (e.target.closest("button, a")) setMenuOpen(false);
});
document.getElementById("who-photo").addEventListener("click", (e) => {
  e.preventDefault();
  const menu = document.getElementById("avatar-menu");
  setAvatarMenuOpen(!!menu?.hidden);
});
document.getElementById("avatar-menu").addEventListener("click", (e) => {
  if (e.target.closest("button")) setAvatarMenuOpen(false);
});
document.getElementById("btn-header-notices").addEventListener("click", async () => {
  setNoticesWanted(!noticesWanted());
  if (noticesWanted()) {
    await enableDesktopNotices();
    if ("Notification" in window && Notification.permission === "denied") {
      alert(I18N.t("noticesDenied"));
    }
  } else {
    hideInAppNotice();
    paintNoticesButton();
  }
});
document.getElementById("notice-toast-open").addEventListener("click", () => {
  hideInAppNotice();
  handleNotifyClick(noticeToastData || {});
});
document.getElementById("notice-toast-close").addEventListener("click", hideInAppNotice);
document.getElementById("btn-header-record").addEventListener("click", () => {
  if (pubStream) stopPublish();
  else startPublish();
});
document.getElementById("btn-header-play").addEventListener("click", () => {
  startWatch();
});
document.getElementById("stream-close").addEventListener("click", () => {
  closeStreamDialog();
});
document.getElementById("stream-dialog").addEventListener("close", () => {
  const video = document.getElementById("stream-video");
  if (!pubStream && video) video.srcObject = null;
  if (!pubStream) stopWatch(false);
});
document.getElementById("btn-header-chat").addEventListener("click", async () => {
  const room = primaryChatRoom();
  if (!room) return;
  try {
    await openChat(room);
  } catch (err) {
    alert(err.message);
  }
});
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") {
    setMenuOpen(false);
    setAvatarMenuOpen(false);
  }
});
document.addEventListener("pointerdown", (e) => {
  if (document.body.classList.contains("menu-open") && !e.target.closest("#top-actions, #btn-menu")) {
    setMenuOpen(false);
  }
  if (!document.getElementById("avatar-menu")?.hidden && !e.target.closest(".face-cluster")) {
    setAvatarMenuOpen(false);
  }
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
  if (e.target.id === "mixer-search") {
    paintMixerDesk();
    document.getElementById("mixer-search")?.focus();
    return;
  }
  const input = e.target.closest("input[name=comment]");
  if (input) paintChannelDirty(input);
  const mixerComment = e.target.closest("[data-mixer-comment]");
  if (mixerComment) {
    mixerComment.classList.toggle("dirty", mixerComment.value !== savedMixerComment(Number(mixerComment.dataset.mixerComment)));
  }
});

document.getElementById("my-channels").addEventListener("change", async (e) => {
  const userEl = e.target.closest("[data-mixer-user]");
  const commentEl = e.target.closest("[data-mixer-comment]");
  const n = Number(userEl?.dataset.mixerUser || commentEl?.dataset.mixerComment || 0);
  if (!n) return;
  try {
    await saveMixerChannel(n);
  } catch (err) {
    showError(document.getElementById("channel-error"), err.message);
  }
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
  const mixerBtn = e.target.closest("[data-mixer-v48]");
  if (mixerBtn) {
    try {
      await saveMixerChannel(Number(mixerBtn.dataset.mixerV48), mixerBtn.getAttribute("aria-pressed") !== "true");
    } catch (err) {
      showError(document.getElementById("channel-error"), err.message);
    }
    return;
  }
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
  const galleryBtn = e.target.closest("[data-gallery]");
  if (galleryBtn) {
    await openGallery(galleryBtn.dataset.gallery);
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
  const galleryBtn = e.target.closest("button[data-gallery]");
  if (galleryBtn) {
    await openGallery(galleryBtn.dataset.gallery);
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
  if (!btn || btn.disabled || !canVote()) return;
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
      `/api/dates/${encodeURIComponent(galleryDateId)}/gallery`,
      files,
      (name, loaded, total) => setGalleryProgress(true, name, loaded, total),
    );
    await loadDates();
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

function memberColor(id) {
  let h = 0;
  for (const ch of String(id || "")) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  return MEMBER_COLORS[h % MEMBER_COLORS.length];
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
  if (!m?.room) return;
  if (CHAT_ROOMS.includes(m.room)) {
    if (chatRoom === m.room) {
      chatUnread[m.room] = 0;
      api(`/api/chats/${encodeURIComponent(m.room)}/read`, { method: "POST" }).catch(() => {});
      renderChatTabs();
    } else if (!(me && !m.isAdmin && m.userId === me.id)) {
      chatUnread[m.room] = unreadCount(m.room) + 1;
      renderChatTabs();
    }
  }
  notifyChatMessage(m);
}

function renderChatTabs() {
  const box = document.getElementById("chat-tabs");
  let any = false;
  box.querySelectorAll("[data-chat]").forEach((btn) => {
    const show = canUseChatRoom(btn.dataset.chat);
    btn.hidden = !show;
    btn.classList.toggle("on", show && chatRoom === btn.dataset.chat);
    const n = show ? unreadCount(btn.dataset.chat) : 0;
    const label = I18N.role(btn.dataset.chat);
    btn.innerHTML = `${escapeHtml(label)}${chatBadge(n)}`;
    btn.setAttribute("aria-label", n ? `${label}, ${n}` : label);
    if (show) any = true;
  });
  box.hidden = !any;
  paintMenuUnread();
  paintHeaderChat();
}

function chatUnreadTotal() {
  return CHAT_ROOMS.reduce((sum, room) => sum + (canUseChatRoom(room) ? unreadCount(room) : 0), 0);
}

function paintUnreadBadge(id, n) {
  const badge = document.getElementById(id);
  if (!badge) return;
  badge.hidden = n === 0;
  badge.textContent = n > 99 ? "99+" : String(n);
}

function paintMenuUnread() {
  paintUnreadBadge("menu-unread", chatUnreadTotal());
}

function paintHeaderChat() {
  const btn = document.getElementById("btn-header-chat");
  if (!btn) return;
  const room = primaryChatRoom();
  btn.hidden = !room;
  const n = chatUnreadTotal();
  paintUnreadBadge("header-chat-unread", n);
  const label = room ? I18N.t(`chat.${room}`) : I18N.t("chatBrand");
  btn.setAttribute("aria-label", n ? `${label}, ${n}` : label);
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

function chatMediaDelete(m) {
  if (!canDeleteChat(m)) return "";
  return `<button type="button" class="chat-voice-del" data-voice-delete="${m.id}" aria-label="${escapeHtml(I18N.t("chatMediaDelete"))}"></button>`;
}

function chatBodyHTML(m) {
  if (m.kind === "voice") {
    const src = chatVoiceURL(m.room || chatRoom, m.id);
    const text = m.text ? `<p class="chat-voice-text">${escapeHtml(m.text)}</p>` : "";
    const del = canDeleteChat(m)
      ? `<button type="button" class="chat-voice-del" data-voice-delete="${m.id}" aria-label="${escapeHtml(I18N.t("chatVoiceDelete"))}"></button>`
      : "";
    return `<div class="chat-voice">
      <button type="button" class="chat-voice-play" data-voice="${m.id}" aria-label="${escapeHtml(I18N.t("chatVoicePlay"))}"></button>
      <span class="chat-voice-track" aria-hidden="true"><i data-voice-bar="${m.id}"></i></span>
      <span class="chat-voice-dur" data-voice-dur="${m.id}">${formatVoiceDur(m.durationMs)}</span>
      ${del}
      <audio preload="none" src="${escapeHtml(src)}" data-voice-audio="${m.id}"></audio>
    </div>${text}`;
  }
  if (m.kind === "image" || m.kind === "video") {
    const src = chatMediaURL(m.room || chatRoom, m.id);
    const media = m.kind === "video"
      ? `<video controls playsinline preload="metadata" src="${escapeHtml(src)}"></video>`
      : `<a href="${escapeHtml(src)}" target="_blank" rel="noopener"><img src="${escapeHtml(src)}" alt="" /></a>`;
    return `<div class="chat-media">${media}${chatMediaDelete(m)}</div>`;
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
  return !!(canDeleteChat(m) && isChatText(m));
}

function chatActionsHTML(m) {
  if (editingChatId === m.id) return "";
  const edit = canEditChat(m)
    ? `<button type="button" class="chat-edit-btn" data-chat-edit="${m.id}" aria-label="${escapeHtml(I18N.t("chatEdit"))}"></button>`
    : "";
  const del = canDeleteChat(m) && isChatText(m)
    ? `<button type="button" class="chat-voice-del" data-voice-delete="${m.id}" aria-label="${escapeHtml(I18N.t("chatDelete"))}"></button>`
    : "";
  if (!edit && !del) return "";
  return `<div class="chat-tools">${edit}${del}</div>`;
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
        ${chatBodyHTML(m)}
        <span class="chat-time">${escapeHtml(formatChatWhen(m.createdAt))}</span>
      </div>
      ${chatActionsHTML(m)}
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
      const newDay = !prev || chatDayKey(prev.createdAt) !== chatDayKey(m.createdAt);
      const stacked = !newDay && !!(prev && chatAuthorKey(prev) === chatAuthorKey(m));
      const heading = newDay && formatChatDay(m.createdAt)
        ? `<p class="chat-day">${escapeHtml(formatChatDay(m.createdAt))}</p>`
        : "";
      return heading + chatMessageHTML(m, stacked);
    }).join("")
    : `<p class="muted">${I18N.t("noMessages")}</p>`;
  list.querySelectorAll("audio[data-voice-audio]").forEach(bindVoiceAudio);
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

function removeChat(id) {
  const next = chatMessages.filter((m) => m.id !== id);
  if (next.length === chatMessages.length) return;
  const list = document.getElementById("chat-list");
  chatMessages = next;
  renderChat(list.scrollTop);
}

let editingChatId = "";

function canDeleteChat(m) {
  return !!(me && m && !m.isAdmin && m.userId === me.id);
}

async function saveChatEdit(id, text) {
  if (!chatRoom || !id) return;
  try {
    const data = await api(`/api/chats/${encodeURIComponent(chatRoom)}/messages/${encodeURIComponent(id)}`, {
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
    await api(`/api/chats/${encodeURIComponent(chatRoom)}/messages/${encodeURIComponent(id)}`, { method: "DELETE" });
    removeChat(id);
  } catch (err) {
    showError(document.getElementById("chat-error"), err.message);
  }
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
  if (!me || (!event && !canUseChatRoom(room))) return;
  chatRoom = room;
  renderChatTabs();
  document.getElementById("chat-title").textContent = chatTitle(room, title);
  discardVoiceRecord();
  document.getElementById("chat-text").value = "";
  showError(document.getElementById("chat-error"), "");
  const data = await api(`/api/chats/${encodeURIComponent(room)}`);
  chatMessages = data.messages || [];
  chatUnread[room] = 0;
  renderChatTabs();
  renderChat();
  const input = document.getElementById("chat-text");
  input.placeholder = I18N.t("chatWrite");
  document.getElementById("chat-dialog").showModal();
  if (window.matchMedia("(max-width: 720px)").matches) setChatFull(true);
  paintChatSize();
  input.focus();
}

const STREAM_ICE = { iceServers: [{ urls: "stun:stun.l.google.com:19302" }] };

function wsSend(obj) {
  if (memberWS?.readyState === WebSocket.OPEN) {
    memberWS.send(JSON.stringify(obj));
  }
}

function applyLiveStream(stream, announce) {
  const next = stream && stream.userId ? { userId: stream.userId, nickname: stream.nickname || "" } : null;
  const wasId = liveStream?.userId || "";
  liveStream = next;
  paintStreamButtons();
  if (!next) {
    if (wasId) stopWatch(false);
    return;
  }
  if (announce && next.userId !== me?.id && next.userId !== wasId) {
    showDesktopNotice(`${next.nickname} ${I18N.t("streamLive")}`, I18N.t("streamPlay"), {
      kind: "stream",
      tag: "stream",
    });
  }
}

function paintStreamButtons() {
  const rec = document.getElementById("btn-header-record");
  const play = document.getElementById("btn-header-play");
  if (!rec || !play) return;
  rec.hidden = !me?.streamer;
  rec.classList.toggle("on", !!pubStream);
  rec.setAttribute("aria-label", I18N.t(pubStream ? "streamStop" : "streamStart"));
  rec.setAttribute("aria-pressed", pubStream ? "true" : "false");
  const live = !!liveStream;
  play.disabled = !live;
  play.classList.toggle("live", live);
  play.setAttribute("aria-label", I18N.t("streamPlay"));
}

function openStreamDialog(publishing) {
  const video = document.getElementById("stream-video");
  const who = liveStream?.nickname || me?.nickname || I18N.t("streamBrand");
  document.getElementById("stream-who").textContent = who;
  document.getElementById("stream-status").textContent = I18N.t(publishing ? "streamPublishing" : "streamWatching");
  if (publishing && pubStream) {
    video.srcObject = pubStream;
    video.muted = true;
    video.play().catch(() => {});
  } else {
    video.muted = false;
  }
  const dialog = document.getElementById("stream-dialog");
  if (!dialog.open) dialog.showModal();
}

function closeStreamDialog() {
  const dialog = document.getElementById("stream-dialog");
  if (dialog.open) dialog.close();
  const video = document.getElementById("stream-video");
  if (!pubStream) {
    video.srcObject = null;
    stopWatch(false);
  }
}

function resetLocalStream(ended) {
  Object.values(pubPeers).forEach((pc) => pc.close());
  pubPeers = {};
  if (pubStream) {
    pubStream.getTracks().forEach((t) => t.stop());
    pubStream = null;
  }
  stopWatch(false);
  if (ended) liveStream = null;
  const video = document.getElementById("stream-video");
  if (video) video.srcObject = null;
  const dialog = document.getElementById("stream-dialog");
  if (dialog?.open) dialog.close();
  paintStreamButtons();
}

async function startPublish() {
  if (!me?.streamer) return;
  if (!navigator.mediaDevices?.getUserMedia) {
    alert(I18N.t("streamNeedCamera"));
    return;
  }
  try {
    pubStream = await navigator.mediaDevices.getUserMedia({
      audio: true,
      video: { facingMode: "user", width: { ideal: 1280 }, height: { ideal: 720 } },
    });
  } catch {
    pubStream = null;
    alert(I18N.t("streamNeedCamera"));
    return;
  }
  wsSend({ type: "streamStart" });
  paintStreamButtons();
  openStreamDialog(true);
}

function stopPublish() {
  Object.values(pubPeers).forEach((pc) => pc.close());
  pubPeers = {};
  if (pubStream) {
    pubStream.getTracks().forEach((t) => t.stop());
    pubStream = null;
  }
  wsSend({ type: "streamStop" });
  const video = document.getElementById("stream-video");
  if (video) video.srcObject = null;
  const dialog = document.getElementById("stream-dialog");
  if (dialog?.open) dialog.close();
  paintStreamButtons();
}

function stopWatch(closeDialog) {
  if (subPeer) {
    subPeer.close();
    subPeer = null;
  }
  const video = document.getElementById("stream-video");
  if (video && !pubStream) video.srcObject = null;
  if (closeDialog) {
    const dialog = document.getElementById("stream-dialog");
    if (dialog?.open) dialog.close();
  }
}

async function startWatch() {
  if (!liveStream) {
    alert(I18N.t("streamNone"));
    return;
  }
  if (liveStream.userId === me?.id && pubStream) {
    openStreamDialog(true);
    return;
  }
  openStreamDialog(false);
  wsSend({ type: "streamWatch" });
}

async function onStreamWatch(from) {
  if (!pubStream || !from) return;
  if (pubPeers[from]) pubPeers[from].close();
  const pc = new RTCPeerConnection(STREAM_ICE);
  pubPeers[from] = pc;
  pubStream.getTracks().forEach((t) => pc.addTrack(t, pubStream));
  pc.onicecandidate = (e) => {
    if (e.candidate) wsSend({ type: "streamIce", data: { to: from, candidate: e.candidate } });
  };
  pc.onconnectionstatechange = () => {
    if (["failed", "closed", "disconnected"].includes(pc.connectionState)) {
      pc.close();
      delete pubPeers[from];
    }
  };
  try {
    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);
    wsSend({ type: "streamOffer", data: { to: from, sdp: pc.localDescription } });
  } catch {
    pc.close();
    delete pubPeers[from];
  }
}

async function onStreamOffer(from, sdp) {
  if (!sdp || !from) return;
  if (subPeer) subPeer.close();
  subPeer = new RTCPeerConnection(STREAM_ICE);
  subPeer.ontrack = (e) => {
    const video = document.getElementById("stream-video");
    video.srcObject = e.streams[0] || new MediaStream([e.track]);
    video.muted = false;
    video.play().catch(() => {});
  };
  subPeer.onicecandidate = (e) => {
    if (e.candidate) wsSend({ type: "streamIce", data: { to: from, candidate: e.candidate } });
  };
  try {
    await subPeer.setRemoteDescription(sdp);
    const answer = await subPeer.createAnswer();
    await subPeer.setLocalDescription(answer);
    wsSend({ type: "streamAnswer", data: { to: from, sdp: subPeer.localDescription } });
  } catch {
    stopWatch(false);
  }
}

async function onStreamAnswer(from, sdp) {
  const pc = pubPeers[from];
  if (!pc || !sdp) return;
  try {
    await pc.setRemoteDescription(sdp);
  } catch {}
}

async function onStreamIce(from, candidate) {
  const pc = pubPeers[from] || subPeer;
  if (!pc || !candidate) return;
  try {
    await pc.addIceCandidate(candidate);
  } catch {}
}

function handleStreamMessage(msg) {
  switch (msg.type) {
    case "streamLive":
      applyLiveStream(msg.data, true);
      return true;
    case "streamEnded":
      applyLiveStream(null, false);
      if (pubStream) {
        Object.values(pubPeers).forEach((pc) => pc.close());
        pubPeers = {};
        pubStream.getTracks().forEach((t) => t.stop());
        pubStream = null;
      }
      stopWatch(true);
      paintStreamButtons();
      return true;
    case "streamBusy":
      stopPublish();
      alert(I18N.t("streamBusy"));
      return true;
    case "streamDenied":
      stopPublish();
      alert(I18N.t("streamDenied"));
      return true;
    case "streamWatch":
      onStreamWatch(msg.data?.from);
      return true;
    case "streamOffer":
      onStreamOffer(msg.data?.from, msg.data?.sdp);
      return true;
    case "streamAnswer":
      onStreamAnswer(msg.data?.from, msg.data?.sdp);
      return true;
    case "streamIce":
      onStreamIce(msg.data?.from, msg.data?.candidate);
      return true;
    default:
      return false;
  }
}

function connectWS() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${proto}://${location.host}/ws/client`);
  memberWS = ws;
  ws.onmessage = (ev) => {
    let msg = {};
    try { msg = JSON.parse(ev.data); } catch { return; }
    if (handleStreamMessage(msg)) return;
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
    if (msg.type === "changed") {
      api("/api/me").then((data) => {
        me = data.user;
        applyUnread(data.unread);
        applyLiveStream(data.stream, false);
        renderChatTabs();
        if (me.mustChangePassword) {
          showGate("password");
          return;
        }
        if (!me.streamer && pubStream) stopPublish();
        paintMyChannels();
        paintStreamButtons();
        loadMixer().catch(() => {});
      }).catch(() => {});
      if (!me?.mustChangePassword) {
        loadDates().catch(() => {});
        if (document.getElementById("proposals-dialog").open) loadProposals().catch(() => {});
        if (document.getElementById("directory-dialog").open) loadDirectory().catch(() => {});
        if (document.getElementById("archive-dialog").open) loadArchive().catch(() => {});
        if (document.getElementById("gallery-dialog").open && galleryDateId) openGallery(galleryDateId).catch(() => {});
      }
    }
  };
  ws.onclose = () => {
    if (memberWS === ws) memberWS = null;
    resetLocalStream(true);
    setTimeout(connectWS, 2000);
  };
}

I18N.onChange(() => {
  I18N.apply();
  paintInstallButtons();
  if (me && !me.mustChangePassword) {
    document.getElementById("who-name").textContent = me.nickname;
    document.getElementById("who-meta").textContent = `${I18N.role(me.role)} · ${I18N.subrole(me.subrole)} · ${me.email}`;
    Photo.paint(document.getElementById("who-photo"), me);
    paintInfoButton(document.getElementById("who-info"), me);
    paintMyChannels();
    renderChatTabs();
    paintNoticesButton();
    paintStreamButtons();
    paintChatSize();
    if (document.getElementById("chat-dialog").open && chatRoom) {
      const date = chatRoom.startsWith("event:")
        ? dates.find((d) => d.id === chatRoom.slice("event:".length))
        : null;
      document.getElementById("chat-title").textContent = chatTitle(chatRoom, date?.title);
      document.getElementById("chat-text").placeholder = I18N.t("chatWrite");
      renderChat(document.getElementById("chat-list").scrollTop);
    }
    if (document.getElementById("proposals-dialog").open) renderProposalList();
    if (document.getElementById("directory-dialog").open) renderDirectory();
    if (document.getElementById("archive-dialog").open) renderArchive();
    if (document.getElementById("gallery-dialog").open) {
      paintGallerySize();
      renderGallery();
    }
    if (document.getElementById("calendar-dialog").open) {
      document.getElementById("calendar-copy").textContent = I18N.t("copyLink");
    }
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

document.getElementById("chat-shrink").addEventListener("click", () => setChatFull(false));
document.getElementById("chat-expand").addEventListener("click", () => setChatFull(true));

document.getElementById("chat-close").addEventListener("click", () => {
  discardVoiceRecord();
  editingChatId = "";
  document.getElementById("chat-dialog").close();
  chatRoom = "";
  chatMessages = [];
  renderChatTabs();
});

document.getElementById("chat-dialog").addEventListener("close", () => {
  discardVoiceRecord();
  editingChatId = "";
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
    setChatEmojiOpen(false);
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

document.getElementById("schedule-close").addEventListener("click", () => {
  document.getElementById("schedule-dialog").close();
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

document.getElementById("titles-copy").addEventListener("click", async () => {
  const date = dates.find((d) => d.id === titlesDateId);
  const titles = (date?.titles || []).map((item) => item.title).filter((t) => t);
  if (!titles.length) return;
  try {
    await copyText(titlesClipboardText(date, titles));
    flashCopyBtn(document.getElementById("titles-copy"));
  } catch (err) {
    alert(err.message);
  }
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
    const m = chatMessages.find((x) => x.id === trash.dataset.voiceDelete);
    if (canDeleteChat(m)) await deleteChatMessage(m.id);
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
  return !!(user?.address || user?.phone || user?.birthday || user?.altEmail);
}

function paintInfoButton(btn, user) {
  if (!btn) return;
  btn.textContent = I18N.t(user?.birthday ? "modifyInfo" : "addInfo");
  btn.classList.toggle("has-info", hasInfo(user));
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
        <div>
          <strong>${escapeHtml(p.title)}</strong>
          <p class="proposal-by">${escapeHtml(I18N.t("proposedBy"))} <b>${escapeHtml(p.nickname || "—")}</b></p>
        </div>
        <span class="badge ${proposalBadge(p.status)}">${escapeHtml(I18N.status(p.status))}</span>
      </div>
      <p class="meta">${escapeHtml(formatWhen(p.createdAt))}</p>
      ${p.url ? `<p class="proposal-url">${proposalLink(p.url)}</p>` : ""}
      ${p.comment ? `<p class="proposal-comment"><span class="label">${escapeHtml(I18N.t("adminComment"))}</span>${escapeHtml(p.comment)}</p>` : ""}
    </article>
  `).join("");
}

function phoneHref(phone) {
  const n = String(phone || "").replace(/[^\d+]/g, "");
  return n ? `tel:${n}` : "";
}

async function loadDirectory() {
  const data = await api("/api/directory");
  directory = data.people || [];
  renderDirectory();
}

function renderDirectory() {
  const list = document.getElementById("directory-list");
  const q = document.getElementById("directory-search").value.trim().toLowerCase();
  const rows = directory.filter((p) => {
    if (!q) return true;
    const hay = `${p.nickname} ${p.email} ${p.altEmail || ""} ${p.phone} ${I18N.role(p.role)} ${I18N.subrole(p.subrole)}`.toLowerCase();
    return hay.includes(q);
  });
  if (!rows.length) {
    list.innerHTML = `<p class="muted">${I18N.t("noContacts")}</p>`;
    return;
  }
  list.innerHTML = rows.map((p) => {
    const mail = p.email ? `<p><a href="mailto:${escapeHtml(p.email)}">${escapeHtml(p.email)}</a></p>` : "";
    const alt = p.altEmail ? `<p><a href="mailto:${escapeHtml(p.altEmail)}">${escapeHtml(p.altEmail)}</a></p>` : "";
    const tel = phoneHref(p.phone);
    const phone = p.phone
      ? `<p>${tel ? `<a href="${escapeHtml(tel)}">${escapeHtml(p.phone)}</a>` : escapeHtml(p.phone)}</p>`
      : "";
    return `
    <article class="directory-card">
      ${Photo.html(p, "sm")}
      <div>
        <strong>${escapeHtml(p.nickname)}</strong>
        <p class="meta">${escapeHtml(I18N.role(p.role))} · ${escapeHtml(I18N.subrole(p.subrole))}</p>
        ${mail}${alt}${phone}
      </div>
    </article>
  `;
  }).join("");
}

async function loadArchive() {
  const data = await api("/api/archive");
  archiveItems = data.archive || [];
  renderArchive();
}

function showArchiveCreate(show) {
  document.getElementById("archive-create").hidden = !show;
  document.getElementById("archive-toolbar").hidden = show;
  document.getElementById("archive-list").hidden = show;
  document.getElementById("archive-search").hidden = show;
  document.querySelector("label[for=archive-search]").hidden = show;
  if (show) {
    document.getElementById("archive-detail").hidden = true;
    document.getElementById("archive-create-title").value = "";
    document.getElementById("archive-create-composer").value = "";
    showError(document.getElementById("archive-create-error"), "");
    document.getElementById("archive-create-title").focus();
  }
}

function showArchiveList() {
  document.getElementById("archive-detail").hidden = true;
  document.getElementById("archive-detail").innerHTML = "";
  document.getElementById("archive-create").hidden = true;
  document.getElementById("archive-toolbar").hidden = false;
  document.getElementById("archive-list").hidden = false;
  document.getElementById("archive-search").hidden = false;
  document.querySelector("label[for=archive-search]").hidden = false;
  renderArchive();
}

function renderArchive() {
  const list = document.getElementById("archive-list");
  if (list.hidden) return;
  const q = document.getElementById("archive-search").value.trim().toLowerCase();
  const rows = archiveItems.filter((item) => {
    if (!q) return true;
    const hay = `${item.title} ${item.composer || ""}`.toLowerCase();
    return hay.includes(q);
  });
  if (!rows.length) {
    list.innerHTML = `<p class="muted">${I18N.t("archiveNoItems")}</p>`;
    return;
  }
  list.innerHTML = rows.map((item) => `
    <button type="button" class="title-item" data-archive-item="${item.id}">
      <div>
        <strong>${escapeHtml(item.title)}</strong>
        ${item.composer ? `<p>${escapeHtml(item.composer)}</p>` : ""}
      </div>
    </button>`).join("");
}

async function openArchiveItem(itemId) {
  const list = document.getElementById("archive-list");
  const detail = document.getElementById("archive-detail");
  try {
    const data = await api(`/api/archive/${encodeURIComponent(itemId)}`);
    list.hidden = true;
    detail.hidden = false;
    document.getElementById("archive-create").hidden = true;
    document.getElementById("archive-toolbar").hidden = true;
    document.getElementById("archive-search").hidden = true;
    document.querySelector("label[for=archive-search]").hidden = true;
    detail.innerHTML = titleMaterialHTML(data.item, "archive") + archiveManageHTML(data.item);
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

document.getElementById("btn-archive").addEventListener("click", async () => {
  try {
    document.getElementById("archive-search").value = "";
    await loadArchive();
    showArchiveList();
    document.getElementById("archive-dialog").showModal();
    document.getElementById("archive-search").focus();
  } catch (err) {
    alert(err.message);
  }
});

document.getElementById("archive-close").addEventListener("click", () => {
  document.getElementById("archive-dialog").close();
});

document.getElementById("archive-dialog").addEventListener("close", () => {
  document.getElementById("archive-detail").innerHTML = "";
});

document.getElementById("archive-search").addEventListener("input", renderArchive);

document.getElementById("archive-list").addEventListener("click", async (e) => {
  const btn = e.target.closest("[data-archive-item]");
  if (!btn) return;
  await openArchiveItem(btn.dataset.archiveItem);
});

document.getElementById("archive-detail").addEventListener("click", (e) => {
  if (e.target.closest("[data-archive-back]")) {
    showArchiveList();
    return;
  }
  const addLink = e.target.closest("[data-archive-add-link]");
  if (addLink) {
    const box = document.querySelector("[data-archive-id]");
    if (!box) return;
    const url = document.querySelector("[data-archive-link-url]")?.value || "";
    const name = document.querySelector("[data-archive-link-name]")?.value || "";
    api(`/api/archive/${encodeURIComponent(box.dataset.archiveId)}/links`, {
      method: "POST",
      body: JSON.stringify({ url, name }),
    }).then(async (data) => {
      await loadArchive();
      await openArchiveItem(data.item.id);
    }).catch((err) => alert(err.message));
    return;
  }
  const upload = e.target.closest("[data-archive-upload]");
  if (!upload) return;
  const input = document.getElementById("archive-member-file");
  input.dataset.kind = upload.dataset.archiveUpload;
  input.dataset.accept = upload.dataset.accept || "";
  input.accept = upload.dataset.accept || "";
  input.value = "";
  input.click();
});

document.getElementById("archive-new").addEventListener("click", () => showArchiveCreate(true));

document.getElementById("archive-create-cancel").addEventListener("click", () => showArchiveList());

document.getElementById("archive-create").addEventListener("submit", async (e) => {
  e.preventDefault();
  const errEl = document.getElementById("archive-create-error");
  showError(errEl, "");
  try {
    const data = await api("/api/archive", {
      method: "POST",
      body: JSON.stringify({
        title: document.getElementById("archive-create-title").value,
        composer: document.getElementById("archive-create-composer").value,
      }),
    });
    await loadArchive();
    await openArchiveItem(data.item.id);
  } catch (err) {
    showError(errEl, err.message);
  }
});

document.getElementById("archive-member-file").addEventListener("change", async (e) => {
  const input = e.target;
  const file = input.files?.[0];
  const kind = input.dataset.kind;
  const box = document.querySelector("[data-archive-id]");
  if (!file || !kind || !box) return;
  const role = document.querySelector(`[data-archive-role="${kind}"]`)?.value || "";
  const fd = new FormData();
  fd.append("file", file);
  fd.append("kind", kind);
  if (role) fd.append("role", role);
  try {
    const res = await fetch(`/api/archive/${encodeURIComponent(box.dataset.archiveId)}/files`, {
      method: "POST",
      credentials: "same-origin",
      body: fd,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(I18N.error(data.error || res.statusText));
    await loadArchive();
    await openArchiveItem(data.item.id);
  } catch (err) {
    alert(err.message);
  } finally {
    input.value = "";
  }
});

document.getElementById("btn-directory").addEventListener("click", async () => {
  try {
    document.getElementById("directory-search").value = "";
    await loadDirectory();
    document.getElementById("directory-dialog").showModal();
    document.getElementById("directory-search").focus();
  } catch (err) {
    alert(err.message);
  }
});

document.getElementById("directory-close").addEventListener("click", () => {
  document.getElementById("directory-dialog").close();
});

document.getElementById("directory-search").addEventListener("input", renderDirectory);

async function openCalendar() {
  const errEl = document.getElementById("calendar-error");
  const urlEl = document.getElementById("calendar-url");
  const openEl = document.getElementById("calendar-open");
  const copyEl = document.getElementById("calendar-copy");
  showError(errEl, "");
  urlEl.value = "";
  openEl.removeAttribute("href");
  copyEl.textContent = I18N.t("copyLink");
  try {
    const data = await api("/api/me/calendar");
    urlEl.value = data.url || "";
    if (data.webcalUrl) openEl.href = data.webcalUrl;
    document.getElementById("calendar-dialog").showModal();
    urlEl.focus();
    urlEl.select();
  } catch (err) {
    showError(errEl, err.message);
    document.getElementById("calendar-dialog").showModal();
  }
}

document.getElementById("btn-calendar").addEventListener("click", openCalendar);

document.getElementById("calendar-close").addEventListener("click", () => {
  document.getElementById("calendar-dialog").close();
});

document.getElementById("calendar-copy").addEventListener("click", async () => {
  const url = document.getElementById("calendar-url").value;
  const copyEl = document.getElementById("calendar-copy");
  if (!url) return;
  try {
    await navigator.clipboard.writeText(url);
    copyEl.textContent = I18N.t("copied");
    setTimeout(() => {
      copyEl.textContent = I18N.t("copyLink");
    }, 1600);
  } catch {
    document.getElementById("calendar-url").select();
  }
});

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
