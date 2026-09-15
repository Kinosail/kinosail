(() => {
  const validID = (value) => typeof value === "string" && /^[a-z0-9_-]{1,128}$/i.test(value);
  function bind() {
    document.querySelectorAll("[data-watch-progress]").forEach((container) => {
      if (container.dataset.watchBound) return;
      container.dataset.watchBound = "true";
      const main = container.closest("main");
      const bar = container.querySelector("progress");
      const label = container.querySelector("[data-watch-remaining]");
      let active;
      async function load(id) {
        active?.abort();
        container.hidden = true;
        if (!validID(id)) return;
        const controller = new AbortController();
        active = controller;
        const timeout = setTimeout(() => controller.abort(), 10000);
        try {
          const response = await fetch(`/api/v1/items/${id}/watch-progress`, { credentials: "same-origin", redirect: "error", signal: controller.signal });
          if (!response.ok || Number(response.headers.get("content-length")) > 256) return;
          const body = await response.text();
          if (body.length > 256 || controller.signal.aborted || !container.isConnected) return;
          const value = JSON.parse(body);
          if (!value || Array.isArray(value) || typeof value !== "object" || Object.keys(value).length !== 2) return;
          if (![value.seconds, value.duration].every((n) => Number.isFinite(n) && n >= 0 && n <= 315360000) || (value.duration > 0 && value.seconds > value.duration)) return;
          if (value.seconds === 0) return;
          label.textContent = value.duration > 0 ? `${Math.ceil((value.duration - value.seconds) / 60)} min left` : `${Math.floor(value.seconds / 60)} min watched`;
          bar.hidden = value.duration === 0;
          bar.value = value.duration > 0 ? value.seconds / value.duration * 100 : 0;
          bar.setAttribute("aria-valuetext", label.textContent);
          container.hidden = false;
        } catch {
          // Unknown runtime never becomes a fabricated percentage.
        } finally { clearTimeout(timeout); }
      }
      main?.addEventListener("kinosail:feature", (event) => load(event.detail?.id));
      load(container.dataset.watchProgress);
    });
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", bind);
  else bind();
  document.addEventListener("htmx:afterSwap", bind);
})();
