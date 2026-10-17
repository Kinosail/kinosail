(() => {
  const region = document.querySelector("[data-library-loading]");
  if (!region) return;
  let timer = 0;
  let probe = null;
  const delay = value => {
    const seconds = Number.parseInt(String(value ?? ""), 10);
    return (Number.isSafeInteger(seconds) && seconds > 0 ? Math.min(seconds, 60) : 5) * 1000;
  };
  const schedule = value => {
    clearTimeout(timer);
    timer = setTimeout(check, delay(value));
  };
  async function check() {
    if (document.hidden) { schedule(region.dataset.retryAfter); return; }
    probe = new AbortController();
    const timeout = setTimeout(() => probe?.abort(), 15000);
    try {
      const response = await fetch(location.href, {headers: {Accept: "text/html"}, cache: "no-store", credentials: "same-origin", signal: probe.signal});
      if (response.status === 503 && response.headers.has("Retry-After")) {
        schedule(response.headers.get("Retry-After"));
        return;
      }
      location.reload();
    } catch {
      schedule(region.dataset.retryAfter);
    } finally {
      clearTimeout(timeout);
      probe = null;
    }
  }
  addEventListener("pagehide", () => { clearTimeout(timer); probe?.abort(); });
  addEventListener("pageshow", event => { if (event.persisted) schedule(region.dataset.retryAfter); });
  schedule(region.dataset.retryAfter);
})();
