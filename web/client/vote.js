(() => {
  const token = location.pathname.split("/").filter(Boolean).pop() || "";
  const box = document.getElementById("vote-box");
  const errEl = document.getElementById("vote-error");
  const okEl = document.getElementById("vote-ok");
  const helloEl = document.getElementById("vote-hello");
  const titleEl = document.getElementById("vote-title");
  const whenEl = document.getElementById("vote-when");
  const locEl = document.getElementById("vote-location");
  const CHOICES = ["yes", "maybe", "no"];

  function showError(msg) {
    errEl.hidden = !msg;
    errEl.textContent = msg || "";
  }

  function formatWhen(iso) {
    if (!iso) return "";
    return new Date(iso).toLocaleString(I18N.locale(), {
      weekday: "short",
      day: "numeric",
      month: "short",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  function formatRange(start, end) {
    const a = formatWhen(start);
    const b = formatWhen(end);
    if (a && b && a !== b) return `${a} – ${b}`;
    return a;
  }

  function voteButtons(current, extra = "") {
    return CHOICES.map((c) => `
      <button type="button" data-choice="${c}" ${extra} class="${c}${current === c ? " on" : ""}">${I18N.t(c)}</button>
    `).join("");
  }

  function render(data) {
    helloEl.textContent = I18N.t("voteHello").replace("{name}", data.nickname || "");
    titleEl.textContent = data.title || "";
    whenEl.textContent = data.pollOpen && !data.locationOwner ? I18N.t("severalTimes") : formatRange(data.startsAt, data.endsAt);
    locEl.textContent = data.location || "";
    if (data.pollOpen && !data.locationOwner) {
      box.innerHTML = (data.options || []).map((o) => `
        <div class="vote-block">
          <p class="label">${formatRange(o.startsAt, o.endsAt)}</p>
          <div class="vote-row">${voteButtons(o.myChoice, `data-option="${o.id}"`)}</div>
        </div>
      `).join("");
    } else {
      box.innerHTML = `<div class="vote-row">${voteButtons(data.myChoice)}</div>`;
    }
  }

  async function load() {
    showError("");
    try {
      const res = await fetch(`/api/vote/${encodeURIComponent(token)}`, { credentials: "same-origin" });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(I18N.error(data.error || res.statusText));
      render(data);
    } catch (err) {
      box.innerHTML = "";
      showError(err.message || I18N.t("errVoteLink"));
    }
  }

  box.addEventListener("click", async (e) => {
    const btn = e.target.closest("[data-choice]");
    if (!btn) return;
    showError("");
    okEl.hidden = true;
    try {
      const res = await fetch(`/api/vote/${encodeURIComponent(token)}`, {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ choice: btn.dataset.choice, optionId: btn.dataset.option || "" }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(I18N.error(data.error || res.statusText));
      render(data);
      okEl.hidden = false;
    } catch (err) {
      showError(err.message || I18N.t("errVoteLink"));
    }
  });

  document.querySelectorAll("[data-lang]").forEach((btn) => {
    btn.addEventListener("click", () => {
      I18N.setLang(btn.dataset.lang);
      load();
    });
  });
  I18N.apply();
  load();
})();
