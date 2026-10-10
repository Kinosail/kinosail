(() => {
  let generation = 0;
  let timer;
  let controller;
  let signingOut = false;
  let departing = false;
  const prepared = new Set();
  const active = new Set();
  const idFor = (link) => /^\/watch\/([a-f0-9]{16})$/.exec(link?.getAttribute("href") || "")?.[1];
  const headers = () => ({"Content-Type": "application/json", "X-Kinosail-CSRF": document.querySelector('meta[name="kinosail-csrf"]')?.content || ""});
  const eligible = () => !signingOut && !departing && !document.hidden && !document.querySelector("video,audio") && navigator.onLine && !navigator.connection?.saveData;
  async function boundedJSON(response) {
    if (!response.ok || !response.body) return;
    const reader = response.body.getReader();
    const chunks = []; let length = 0;
    try {
      while (true) {
        const {done, value} = await reader.read();
        if (done) break;
        length += value.byteLength;
        if (length > 1048576) { await reader.cancel(); return; }
        chunks.push(value);
      }
    } finally { reader.releaseLock(); }
    const bytes = new Uint8Array(length); let offset = 0;
    for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength; }
    return JSON.parse(new TextDecoder().decode(bytes));
  }
  async function prepare(id, current, signal) {
    if (!eligible() || current !== generation || signal.aborted || prepared.has(id) || prepared.size >= 3) return;
    prepared.add(id);
    const video = document.createElement("video");
    const capabilities = window.kinosailPlaybackCapabilities;
    try {
      const api = `/api/v1/items/${id}/playback`;
      let result = await boundedJSON(await fetch(api, {signal, credentials: "same-origin", redirect: "error"}));
      if (!eligible() || current !== generation || signal.aborted) return;
      let savedPolicy;
      try { savedPolicy = localStorage.getItem("kinosail.playback-policy-v2"); } catch {}
      const policy = capabilities.policy(savedPolicy, result?.policy, Boolean(result?.compatible));
      const planned = result?.plan?.mode !== "direct";
      const initialMode = planned ? result?.plan?.mode : result?.compatiblePlan?.mode;
      const compatible = capabilities.initialCompatible(policy, initialMode, result?.directType, result?.direct, planned, result?.plan?.audioCompatibilityRequired);
      if (compatible && result?.compatible && !["remux", "audio-transcode"].includes(initialMode)) {
        const media = result.media;
        const facts = {width: media?.video?.Width, height: media?.video?.Height, bitrate: media?.bitrate, framerate: media?.video?.FrameRate};
        const attempt = new AbortController();
        const abort = () => attempt.abort();
        signal.addEventListener("abort", abort, {once: true});
        let expire;
        const budget = new Promise(resolve => { expire = setTimeout(() => { attempt.abort(); resolve(null); }, 1500); });
        try {
          const codecs = await Promise.race([Promise.all(capabilities.codecs.map(async codec => await capabilities.supports(video, facts, codec) ? codec[0] : "")), budget]);
          if (!attempt.signal.aborted && codecs?.some(codec => codec && codec !== "h264")) {
            const negotiated = await boundedJSON(await fetch(`${api}?videoCodecs=${encodeURIComponent(codecs.filter(Boolean).join(","))}`, {signal: attempt.signal, credentials: "same-origin", redirect: "error"}));
            if (!attempt.signal.aborted) result = negotiated || result;
          }
        } catch { /* Use the same default rendition as playback after its deadline. */ }
        finally { clearTimeout(expire); signal.removeEventListener("abort", abort); }
      }
      if (!eligible() || current !== generation || signal.aborted || !result?.plan?.allowed) return;
      let source = compatible ? result.compatible : result.direct;
      if (typeof source !== "string" || source.length > 2048) return;
      const url = new URL(source, location.href);
      if (url.origin !== location.origin || url.search || url.hash || url.username || url.password) return;
      if (source.startsWith(`/hls/${id}/p/`)) {
        const resume = result.start;
        if (Number.isFinite(resume) && resume >= 0.1 && resume < result.duration) source = source.replace(/\/index\.m3u8$/, `-o${Math.floor(resume * 10) * 100}/index.m3u8`);
        if (capabilities.needsAdapter(video)) fetch("/static/hls.min.js?v=1.7.1", {signal, cache: "force-cache"}).catch(() => {});
      }
      active.add(id);
      await fetch(`/api/v1/items/${id}/playback-prepare`, {method: "POST", headers: headers(), body: JSON.stringify({source}), signal, credentials: "same-origin", redirect: "error"});
    } catch { /* Cold playback remains available after preparation failure. */ }
  }
  function schedule(ids) {
    if (!eligible()) return;
    clearTimeout(timer);
    timer = setTimeout(async () => {
      controller?.abort();
      const job = new AbortController();
      controller = job;
      const current = generation;
      const expire = setTimeout(() => job.abort(), 10000);
      try { for (const id of ids.filter(Boolean).slice(0, 3)) await prepare(id, current, job.signal); }
      finally { clearTimeout(expire); }
    }, 600);
  }
  function stop() {
    generation++;
    clearTimeout(timer);
    controller?.abort();
  }
  function cancel(notify = true) {
    stop();
    if (notify) for (const id of active) fetch(`/api/v1/items/${id}/playback-prepare`, {method: "DELETE", headers: headers(), credentials: "same-origin", keepalive: true, redirect: "error"}).catch(() => {});
    active.clear();
    prepared.clear();
  }
  function bind() {
    if (!eligible()) return;
    schedule([...document.querySelectorAll(".home-feature [data-feature-action], .continue-shelf .resume-link, .hero-actions .hero-action")].map(idFor));
  }
  for (const event of ["pointerover", "focusin"]) document.addEventListener(event, ({target}) => {
    const link = target instanceof Element ? target.closest('a[href^="/watch/"]') : null;
    const id = idFor(link);
    if (id) schedule([id]);
  }, {passive: true});
  document.addEventListener("click", event => {
    if (event.defaultPrevented || event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
    const link = event.target instanceof Element ? event.target.closest("a") : null;
    if (!(link instanceof HTMLAnchorElement) || !idFor(link) || link.origin !== location.origin || link.username || link.password || link.hasAttribute("download")) return;
    const target = link.getAttribute("target") ?? document.querySelector("base[target]")?.getAttribute("target") ?? "";
    if (target && target.toLowerCase() !== "_self") return;
    departing = true;
    stop();
    const current = generation;
    setTimeout(() => {
      if (event.defaultPrevented && current === generation) { departing = false; prepared.clear(); bind(); }
    }, 0);
  });
  document.addEventListener("visibilitychange", () => { if (document.hidden) cancel(); else bind(); });
  document.addEventListener("submit", ({target}) => {
    if (!(target instanceof HTMLFormElement)) return;
    const action = new URL(target.action, location.href);
    if (action.origin !== location.origin || action.pathname !== "/logout") return;
    // A fresh document recovers after a failed logout, cancelled submit, or retry.
    signingOut = true;
    cancel(false);
  }, true);
  window.addEventListener("pageshow", ({persisted}) => {
    if (!persisted) return;
    if (signingOut) location.reload();
    else { departing = false; bind(); }
  });
  window.addEventListener("pagehide", cancel);
  document.addEventListener("htmx:before:swap", () => { cancel(); prepared.clear(); });
  document.addEventListener("htmx:after:swap", bind);
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", bind);
  else bind();
})();
