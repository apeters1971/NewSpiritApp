const CHOICES = ["yes", "maybe", "no", "unknown"];

const loginView = document.getElementById("login-view");
const appView = document.getElementById("app-view");
const loginForm = document.getElementById("login-form");
const loginError = document.getElementById("login-error");
const datesEl = document.getElementById("dates");
const emptyEl = document.getElementById("empty");

let me = null;
let dates = [];
let ranking = { year: 0, leader: null };
let commentDateId = "";
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
      <td>${escapeHtml(pollOpen(d) ? `${formatWhen(d.startsAt)} · ${I18N.t("poll")}` : formatWhen(d.startsAt))}</td>
      <td>${escapeHtml(I18N.category(d.category))}</td>
      <td>${escapeHtml(d.title)}</td>
      <td>${escapeHtml(bringList(d.bring).join(", ") || "—")}</td>
      <td><span class="badge ${d.status}">${I18N.status(d.status)}</span></td>
      <td>${pollOpen(d) ? escapeHtml(overviewPollVote(d)) : `<span class="badge ${d.myChoice}">${voteLabel(d.myChoice)}</span>`}</td>
    </tr>`).join("");
}

function renderSpirit() {
  const box = document.getElementById("spirit");
  const leader = ranking.leader;
  if (!leader) {
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
  box.hidden = false;
  box.innerHTML = `
    <p class="brand" data-i18n="spiritOfTheYear">${I18N.t("spiritOfTheYear")}</p>
    <div class="spirit-row">
      <div class="spirit-leader">
        <strong>${escapeHtml(leader.nickname)}</strong>
        <span>${escapeHtml(I18N.subrole(leader.subrole))}</span>
        <span class="spirit-score">${leader.score} ${I18N.t("spiritPoints")}</span>
      </div>
      ${mine}
    </div>`;
}

function render() {
  renderSpirit();
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
  ranking = data.ranking || { year: 0, leader: null };
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
    Photo.paint(document.getElementById("who-photo"), me);
    paintInfoButton(document.getElementById("who-info"), me);
    renderChatTabs();
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
    nickname: m.isAdmin ? I18N.t("chatAdmin") : m.nickname,
    hasPhoto: !m.isAdmin && m.hasPhoto,
    photoUpdatedAt: m.photoUpdatedAt,
  }, "sm");
  return `<article class="chat-row ${mine ? "mine" : "theirs"}${stacked ? " stack" : ""}">
    ${face}
    <div class="chat-col">
      <div class="chat-bubble">
        <p class="chat-name" style="color:${color}">${escapeHtml(m.isAdmin ? I18N.t("chatAdmin") : m.nickname)}</p>
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
    if (msg.type === "changed") loadDates().catch(() => {});
  };
  ws.onclose = () => setTimeout(connectWS, 2000);
}

I18N.onChange(() => {
  I18N.apply();
  if (me) {
    document.getElementById("who-name").textContent = me.nickname;
    document.getElementById("who-meta").textContent = `${I18N.role(me.role)} · ${I18N.subrole(me.subrole)} · ${me.email}`;
    Photo.paint(document.getElementById("who-photo"), me);
    paintInfoButton(document.getElementById("who-info"), me);
    renderChatTabs();
    if (document.getElementById("chat-dialog").open && chatRoom) {
      const date = chatRoom.startsWith("event:")
        ? dates.find((d) => d.id === chatRoom.slice("event:".length))
        : null;
      document.getElementById("chat-title").textContent = chatTitle(chatRoom, date?.title);
      document.getElementById("chat-text").placeholder = I18N.t("chatWrite");
      renderChat(document.getElementById("chat-list").scrollTop);
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
