(() => {
  const MAX = 512;

  function t(key) {
    return window.I18N ? I18N.t(key) : key;
  }

  function url(user) {
    if (!user?.id || !user.hasPhoto) return "";
    return `/api/photos/${encodeURIComponent(user.id)}?v=${encodeURIComponent(user.photoUpdatedAt || "")}`;
  }

  function initials(name) {
    const parts = String(name || "").trim().split(/\s+/).filter(Boolean);
    if (!parts.length) return "+";
    if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
  }

  function paint(button, user, previewURL) {
    if (!button) return;
    const img = button.querySelector("img");
    const ph = button.querySelector(".avatar-placeholder");
    const src = previewURL || url(user);
    if (src) {
      img.src = src;
      img.hidden = false;
      if (ph) ph.hidden = true;
      button.classList.add("has-photo");
    } else {
      img.removeAttribute("src");
      img.hidden = true;
      if (ph) {
        ph.hidden = false;
        ph.textContent = initials(user?.nickname);
      }
      button.classList.remove("has-photo");
    }
  }

  function html(user, size) {
    const src = url(user);
    const cls = `avatar avatar-${size || "sm"}${src ? " has-photo" : ""}`;
    if (src) return `<span class="${cls}"><img src="${src}" alt="" /></span>`;
    return `<span class="${cls}"><span class="avatar-placeholder">${escapeText(initials(user?.nickname))}</span></span>`;
  }

  function escapeText(s) {
    return String(s || "")
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function fileToJPEG(file) {
    return new Promise((resolve, reject) => {
      if (!file || !String(file.type || "").startsWith("image/")) {
        reject(new Error(t("errPictureInvalid")));
        return;
      }
      const img = new Image();
      const obj = URL.createObjectURL(file);
      img.onload = () => {
        URL.revokeObjectURL(obj);
        let w = img.naturalWidth || img.width;
        let h = img.naturalHeight || img.height;
        if (!w || !h) {
          reject(new Error(t("errPictureInvalid")));
          return;
        }
        if (w > MAX || h > MAX) {
          if (w > h) {
            h = Math.round(h * MAX / w);
            w = MAX;
          } else {
            w = Math.round(w * MAX / h);
            h = MAX;
          }
        }
        const canvas = document.createElement("canvas");
        canvas.width = w;
        canvas.height = h;
        canvas.getContext("2d").drawImage(img, 0, 0, w, h);
        canvas.toBlob((blob) => {
          if (!blob) reject(new Error(t("errPictureInvalid")));
          else resolve(blob);
        }, "image/jpeg", 0.85);
      };
      img.onerror = () => {
        URL.revokeObjectURL(obj);
        reject(new Error(t("errPictureInvalid")));
      };
      img.src = obj;
    });
  }

  async function upload(path, blob) {
    const body = new FormData();
    body.append("photo", blob, "photo.jpg");
    const res = await fetch(path, { method: "POST", credentials: "same-origin", body });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(I18N.error(data.error || res.statusText));
    return data;
  }

  async function remove(path) {
    const res = await fetch(path, { method: "DELETE", credentials: "same-origin" });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(I18N.error(data.error || res.statusText));
    return data;
  }

  function stopCamera(video) {
    const stream = video?.srcObject;
    if (stream) stream.getTracks().forEach((track) => track.stop());
    if (video) video.srcObject = null;
  }

  function captureFrame(video) {
    const w = video.videoWidth;
    const h = video.videoHeight;
    if (!w || !h) return Promise.reject(new Error(t("errPictureInvalid")));
    const canvas = document.createElement("canvas");
    let cw = w;
    let ch = h;
    if (cw > MAX || ch > MAX) {
      if (cw > ch) {
        ch = Math.round(ch * MAX / cw);
        cw = MAX;
      } else {
        cw = Math.round(cw * MAX / ch);
        ch = MAX;
      }
    }
    canvas.width = cw;
    canvas.height = ch;
    canvas.getContext("2d").drawImage(video, 0, 0, cw, ch);
    return new Promise((resolve, reject) => {
      canvas.toBlob((blob) => {
        if (!blob) reject(new Error(t("errPictureInvalid")));
        else resolve(blob);
      }, "image/jpeg", 0.85);
    });
  }

  function setCameraMode(on) {
    const video = document.getElementById("photo-video");
    const snap = document.getElementById("photo-snap");
    const take = document.getElementById("photo-take");
    const choose = document.getElementById("photo-choose");
    if (video) video.hidden = !on;
    if (snap) snap.hidden = !on;
    if (take) take.hidden = on;
    if (choose) choose.hidden = on;
  }

  let handlers = null;

  function showError(msg) {
    const el = document.getElementById("photo-error");
    if (!el) return;
    el.hidden = !msg;
    el.textContent = msg || "";
  }

  function closeDialog() {
    const dialog = document.getElementById("photo-dialog");
    const video = document.getElementById("photo-video");
    stopCamera(video);
    setCameraMode(false);
    showError("");
    dialog?.close();
  }

  function openDialog() {
    const dialog = document.getElementById("photo-dialog");
    if (!dialog || !handlers) return;
    const removeBtn = document.getElementById("photo-remove");
    if (removeBtn) removeBtn.hidden = !handlers.canRemove();
    const take = document.getElementById("photo-take");
    if (take) take.hidden = !(navigator.mediaDevices && navigator.mediaDevices.getUserMedia);
    setCameraMode(false);
    showError("");
    dialog.showModal();
  }

  function bind(next) {
    handlers = next;
    const dialog = document.getElementById("photo-dialog");
    const file = document.getElementById("photo-file");
    const video = document.getElementById("photo-video");
    if (!dialog || !file) return;

    document.addEventListener("click", (e) => {
      if (e.target.closest("[data-photo-open]")) {
        e.preventDefault();
        openDialog();
      }
    });

    document.getElementById("photo-close")?.addEventListener("click", closeDialog);
    dialog.addEventListener("close", () => {
      stopCamera(video);
      setCameraMode(false);
      showError("");
    });

    document.getElementById("photo-choose")?.addEventListener("click", () => file.click());
    file.addEventListener("change", async () => {
      const picked = file.files[0];
      file.value = "";
      if (!picked) return;
      showError("");
      try {
        await handlers.onFile(await fileToJPEG(picked));
        closeDialog();
      } catch (err) {
        showError(err.message);
      }
    });

    document.getElementById("photo-take")?.addEventListener("click", async () => {
      showError("");
      try {
        const stream = await navigator.mediaDevices.getUserMedia({ video: { facingMode: "user" }, audio: false });
        video.srcObject = stream;
        await video.play();
        setCameraMode(true);
      } catch {
        showError(t("photoCameraOff"));
      }
    });

    document.getElementById("photo-snap")?.addEventListener("click", async () => {
      showError("");
      try {
        await handlers.onFile(await captureFrame(video));
        closeDialog();
      } catch (err) {
        showError(err.message);
      }
    });

    document.getElementById("photo-remove")?.addEventListener("click", async () => {
      showError("");
      try {
        await handlers.onRemove();
        closeDialog();
      } catch (err) {
        showError(err.message);
      }
    });
  }

  window.Photo = { url, initials, paint, html, fileToJPEG, upload, remove, bind };
})();
