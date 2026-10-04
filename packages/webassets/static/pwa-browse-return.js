(() => {
  const key = "kinosail:browse-return:v1", maximumExtent = 2000;
  const actions = {
    library: "#library a.card, #library a.show-details, #library a.show-play",
    shelf: ".home-shelf a.card, .home-shelf .card > a, .home-shelf .resume-link",
    destination: ".destination-card",
    letter: "[data-title-jump-index] a[data-title-letter]",
  };
  const selector = Object.values(actions).join(", ");
  const profile = () => document.body.dataset.viewerProfile || document.querySelector("[data-nav-profile]")?.dataset.navProfile;
  const here = () => location.pathname + location.search;
  const cardCount = () => document.querySelectorAll("#library .card").length;
  const back = document.querySelector("[data-browse-return]");
  const backLabel = back?.querySelector("[data-browse-return-label]");
  const defaultLabel = backLabel?.textContent;
  let generation = 0, running, historyReturn = false;
  const restored = new WeakMap();

  // Go ParseBrowse remains authoritative; this rejects unsafe stored navigation
  // before any href assignment, network request, focus or history operation.
  function browseURL(value) {
    if (typeof value !== "string" || value.length > 2048 || !/^\/(?:\?|$)/.test(value) || /[\\\x00-\x20]/.test(value)) return;
    const url = new URL(value, location.origin);
    if (url.origin !== location.origin || url.pathname !== "/" || url.hash) return;
    try { decodeURIComponent(url.search.replace(/\+/g, " ")); } catch { return; }
    const limits = { q: 512, view: 32, sort: 16, offset: 10, limit: 10, lang: 64, letter: 16 };
    for (const [name, selected] of url.searchParams) {
      if (!Object.hasOwn(limits, name) || url.searchParams.getAll(name).length !== 1 || new TextEncoder().encode(selected).length > limits[name]) return;
    }
    const values = url.searchParams;
    if (!["", "all", "list", "unwatched", "history", "movies", "shows", "collections", "playlists", "music", "audiobooks", "books", "photos"].includes(values.get("view") || "")) return;
    if (!["", "title", "added", "year"].includes(values.get("sort") || "")) return;
    for (const [name, minimum, maximum] of [["offset", 0, 1000000], ["limit", 1, 200]]) {
      if (values.has(name) && (!/^\d{1,10}$/.test(values.get(name)) || Number(values.get(name)) < minimum || Number(values.get(name)) > maximum)) return;
    }
    const letter = values.get("letter")?.trim();
    if (values.has("letter") && (!letter || letter !== "#" && !/^\p{L}{1,4}$/u.test(letter) || values.get("q") || !["", "title"].includes(values.get("sort") || ""))) return;
    return url;
  }
  const mediaPath = value => typeof value === "string" && /^\/(?:watch|show|item|album|book)\/[A-Za-z0-9_-]{1,256}$/.test(value);
  const actionPath = value => mediaPath(value) || Boolean(browseURL(value));
  function valid(state) {
    if (!state || Array.isArray(state) || typeof state !== "object" || Object.keys(state).sort().join(",") !== "area,at,destination,extent,href,profile,returning,url,version,x,y") return;
    if (state.version !== 1 || typeof state.profile !== "string" || !/^[A-Za-z0-9_-]{1,128}$/.test(state.profile) || state.profile !== profile() || !browseURL(state.url)) return;
    if (typeof state.area !== "string" || !Object.hasOwn(actions, state.area) || !actionPath(state.href) || !actionPath(state.destination) || typeof state.returning !== "boolean") return;
    if (!Number.isSafeInteger(state.extent) || state.extent < 0 || state.extent > maximumExtent) return;
    if (![state.x, state.y].every(value => typeof value === "number" && Number.isFinite(value) && value >= 0 && value <= 10000000)) return;
    if (!Number.isSafeInteger(state.at) || state.at > Date.now() || Date.now() - state.at > 14400000) return;
    const selected = window.kinosailOfflineIdentity?.current();
    if (selected !== undefined && selected !== state.profile) return;
    return state;
  }
  function read() {
    try {
      const raw = sessionStorage.getItem(key);
      if (!raw) return;
      if (new TextEncoder().encode(raw).length > 4096) { clear(); return; }
      const state = valid(JSON.parse(raw));
      if (!state) clear();
      return state;
    } catch { clear(); /* Storage can be disabled; the Go Library fallback still works. */ }
  }
  function write(state) {
    try {
      const raw = JSON.stringify(state);
      if (valid(state) && new TextEncoder().encode(raw).length <= 4096) sessionStorage.setItem(key, raw);
    } catch { /* Return enhancement is optional when session storage is denied. */ }
  }
  function clear() { try { sessionStorage.removeItem(key); } catch {} }
  function updateBack() {
    if (!back || !backLabel) return;
    const state = read();
    back.setAttribute("href", "/"); backLabel.textContent = defaultLabel;
    if (!state || state.destination !== location.pathname || !/^\/watch\//.test(location.pathname)) return;
    const query = browseURL(state.url).searchParams;
    const label = query.get("q") ? back.dataset.returnSearch : query.get("view") === "movies" ? back.dataset.returnMovies : query.get("view") === "shows" ? back.dataset.returnShows : defaultLabel;
    back.setAttribute("href", state.url); backLabel.textContent = label;
  }
  document.addEventListener("click", event => {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const link = event.target.closest?.("a[href]");
    if (!link || link.hasAttribute("download") || link.target && link.target !== "_self") return;
    const href = link.getAttribute("href");
    if (link === back) {
      const state = read();
      if (state && href === state.url && state.destination === location.pathname) write({ ...state, returning: true });
      return;
    }
    if (location.pathname === "/" && browseURL(here()) && link.matches(selector) && actionPath(href)) {
      const area = Object.keys(actions).find(name => link.matches(actions[name]));
      write({ version: 1, profile: profile(), url: here(), href, destination: href, area,
        extent: Math.min(maximumExtent, cardCount()), x: scrollX, y: scrollY, at: Date.now(), returning: false });
    } else if (/^\/show\//.test(location.pathname) && /^\/watch\/[A-Za-z0-9_-]{1,256}$/.test(href || "")) {
      const state = read();
      if (state?.destination === location.pathname) write({ ...state, destination: href });
    }
  }, true);

  function cancel() {
    if (running) {
      window.clearTimeout(running.timer);
      for (const frame of running.frames) cancelAnimationFrame(frame);
      running.settle?.();
    }
    generation++; running = undefined; libraryRestorePending = false; stopLibraryPaging();
  }
  function failed(kind) {
    const status = document.querySelector("[data-library-status]");
    if (status) {
      status.dataset.browseRestoreFailure = kind;
      status.textContent = "Could not return to your previous position. Continue browsing or retry loading.";
    }
    const next = document.querySelector("[data-library-next]");
    if (next) { next.hidden = false; bindLibraryRetry(next); }
  }
  async function restore(force = false) {
    const state = read(), library = document.querySelector("#library");
    if (!state || !library || state.url !== here() || !force && !state.returning || restored.get(library) === state.at || running?.library === library) return;
    stopLibraryPaging(); libraryRestorePending = true;
    const run = { library, generation: ++generation, frames: [] }; running = run;
    let expired = false, complete = false;
    const current = () => generation === run.generation && library.isConnected && library === document.querySelector("#library") && state.url === here() && Boolean(valid(state)) && !expired;
    run.timer = window.setTimeout(() => {
      expired = true; stopLibraryPaging(); run.settle?.();
    }, 20000);
    write({ ...state, returning: true });
    try {
      const seen = new Set();
      while (current() && cardCount() < state.extent) {
        const next = document.querySelector("[data-library-next]");
        if (!next) { failed("extent_changed"); return; }
        const url = browseURL(next.getAttribute("href"));
        const original = browseURL(state.url);
        if (!url || ["q", "view", "sort", "lang", "letter"].some(name => (url.searchParams.get(name) || "") !== (original.searchParams.get(name) || "")) || seen.has(url.href) || seen.size >= 100) {
          failed("invalid_page"); return;
        }
        seen.add(url.href);
        const count = cardCount();
        if (!await loadLibraryPage(next, current)) { if (expired) failed("deadline"); return; }
        if (cardCount() <= count) { failed("extent_changed"); return; }
      }
      if (!current()) return;
      const link = [...document.querySelectorAll(actions[state.area])].find(action => action.getAttribute("href") === state.href);
      if (!link) { failed("title_unavailable"); return; }
      // The real DOM must exist before focus/scroll, including HTMX's settlement.
      await new Promise(resolve => {
        run.settle = resolve;
        run.frames.push(requestAnimationFrame(() => { run.frames.push(requestAnimationFrame(resolve)); }));
      });
      if (!current()) { if (expired && generation === run.generation) failed("deadline"); return; }
      link.focus({ preventScroll: true });
      scrollTo({ left: state.x, top: state.y, behavior: "instant" });
      restored.set(library, state.at); write({ ...state, returning: false });
      delete document.querySelector("[data-library-status]")?.dataset.browseRestoreFailure;
      complete = true;
    } finally {
      window.clearTimeout(run.timer);
      for (const frame of run.frames) cancelAnimationFrame(frame);
      if (generation === run.generation) {
        running = undefined; libraryRestorePending = false;
        if (complete) bindInfiniteLibrary();
      }
    }
  }
  window.addEventListener("pagehide", cancel);
  window.addEventListener("pageshow", event => {
    updateBack();
    if (event.persisted || performance.getEntriesByType("navigation")[0]?.type === "back_forward") void restore(true);
  });
  document.addEventListener("htmx:before:history:restore", () => { historyReturn = true; cancel(); });
  document.body.addEventListener("htmx:before:request", event => {
    if (event.detail?.ctx?.target?.id === "main") cancel();
  });
  document.body.addEventListener("htmx:after:swap", () => {
    if (!historyReturn) return;
    historyReturn = false; queueMicrotask(() => { void restore(true); });
  });
  window.addEventListener("kinosail:library-page", () => { void restore(); });
  window.addEventListener("kinosail:offline-profile", () => {
    if (window.kinosailOfflineIdentity?.current() === profile()) return;
    cancel(); clear(); updateBack();
  });
  document.addEventListener("submit", event => {
    if (event.target instanceof HTMLFormElement && new URL(event.target.action, location.href).pathname === "/logout") { cancel(); clear(); }
  }, true);
  updateBack();
  void restore(performance.getEntriesByType("navigation")[0]?.type === "back_forward");
})();
