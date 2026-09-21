document.documentElement.classList.add("js");

const installs = [...document.querySelectorAll("[data-install]")];
const help = document.querySelector("[data-install-help]");
const appleInstallAction = "Add to Home Screen";
const standalone = matchMedia("(display-mode: standalone)").matches || navigator.standalone === true;
const appleMobile = /iPhone|iPad|iPod/.test(navigator.userAgent) || navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1;
let installPrompt;
let installComplete = standalone;

if ("serviceWorker" in navigator && window.isSecureContext) {
  const identify = (registration) => {
    const worker = registration.active || registration.waiting || registration.installing;
    const identity = window.kinosailOfflineIdentity?.state();
    if (identity) worker?.postMessage({type: identity.profile ? "profile" : "logout", ...identity});
    return registration;
  };
  navigator.serviceWorker.addEventListener("controllerchange", () => identify({active: navigator.serviceWorker.controller}));
  navigator.serviceWorker.register("/service-worker.js?v=52").then(identify).then(() => navigator.serviceWorker.ready).then(identify).catch(() => {});
}
if (appleMobile && !standalone && installs.length) {
  for (const install of installs) {
    install.hidden = false;
    install.title = appleInstallAction;
  }
}
window.addEventListener("beforeinstallprompt", (event) => {
  event.preventDefault();
  if (installComplete) return;
  installPrompt = event;
  for (const install of installs) install.hidden = false;
});
for (const install of installs) install.addEventListener("click", async () => {
  if (installPrompt) {
    await installPrompt.prompt();
    const choice = await installPrompt.userChoice;
    installPrompt = undefined;
    if (choice.outcome === "accepted") {
      installComplete = true;
      for (const action of installs) action.hidden = true;
    }
  } else if (help) help.hidden = false;
});
document.querySelector("[data-install-close]")?.addEventListener("click", () => { help.hidden = true; });
window.addEventListener("appinstalled", () => { installComplete = true; for (const install of installs) install.hidden = true; });

const prototypeSwitcher = document.querySelector("[data-prototype-switcher]");
window.addEventListener("keydown", (event) => {
  if (!prototypeSwitcher || !["ArrowLeft", "ArrowRight"].includes(event.key) || event.target.matches("input, textarea, select, [contenteditable]")) return;
  location.assign(prototypeSwitcher.querySelector(event.key === "ArrowLeft" ? "[data-variant-prev]" : "[data-variant-next]").href);
});
document.querySelector("[data-copy-managed-url]")?.addEventListener("click", async (event) => {
  await navigator.clipboard.writeText(event.currentTarget.dataset.copyManagedUrl);
  event.currentTarget.textContent = "Copied";
});

const prefetchedWatchURLs = new Set(), watchPrefetchPattern = /^\/watch\/[A-Za-z0-9_-]{1,256}$/;
const prefetchWatch = (link) => {
  const url = new URL(link.href, location.href);
  if (prefetchedWatchURLs.size >= 8 || url.origin !== location.origin || !watchPrefetchPattern.test(url.pathname) || prefetchedWatchURLs.has(url.href)) return;
  prefetchedWatchURLs.add(url.href);
  const hint = document.createElement("link"); hint.rel = "prefetch"; hint.as = "document"; hint.href = url.href; document.head.append(hint);
};
const scheduleWatchPrefetch = (event) => {
  const link = event.target.closest?.('a[href^="/watch/"]');
  if (!link || link.dataset.watchPrefetchScheduled) return;
  link.dataset.watchPrefetchScheduled = "true";
  window.setTimeout(() => { if (document.contains(link)) prefetchWatch(link); }, 160);
};
for (const event of ["pointerover", "focusin", "touchstart"]) document.addEventListener(event, scheduleWatchPrefetch, {passive: true});

for (const reel of document.querySelectorAll("[data-season-reel]")) {
  const preview = reel.querySelector("[data-preview-link]");
  const image = preview?.querySelector("[data-preview-image]");
  const missing = preview?.querySelector("[data-preview-missing]");
  const selectEpisode = (row) => {
    if (!preview || !row) return;
    preview.setAttribute("href", row.getAttribute("href"));
    preview.querySelector("[data-preview-action]").textContent = row.dataset.episodeAction;
    preview.querySelector("[data-preview-title]").textContent = row.dataset.episodeTitle;
    const plot = preview.querySelector("[data-preview-plot]");
    plot.textContent = row.dataset.episodePlot;
    plot.hidden = !row.dataset.episodePlot;
    if (row.dataset.episodeArt) image.src = row.dataset.episodeArt;
    else image.removeAttribute("src");
    image.hidden = !row.dataset.episodeArt;
    missing.hidden = Boolean(row.dataset.episodeArt);
  };
  for (const event of ["focusin", "pointerover"]) reel.addEventListener(event, ({ target }) => selectEpisode(target.closest?.("[data-episode-row]")));
}

let libraryObserver;
let libraryAbortController;
let mainRequestGeneration = 0;
const mainRequests = new WeakMap();
const loadingRequests = new WeakMap();
const loadingTargets = new WeakMap();
document.body.addEventListener("htmx:beforeRequest", ({ detail }) => {
  const target = detail?.target;
  if (!target || !detail.xhr) return;
  let state = loadingTargets.get(target);
  if (!state) {
    state = { count: 0, busy: target.getAttribute("aria-busy"), inert: target.inert };
    loadingTargets.set(target, state);
    target.setAttribute("aria-busy", "true");
    if (!target.matches("button, input, select, textarea")) {
      target.classList.add("request-skeleton");
      target.inert = true;
    }
  }
  state.count++;
  loadingRequests.set(detail.xhr, target);
});
function finishLoadingRequest({ detail }) {
  const target = loadingRequests.get(detail?.xhr);
  if (!target) return;
  loadingRequests.delete(detail.xhr);
  const state = loadingTargets.get(target);
  if (--state.count) return;
  target.classList.remove("request-skeleton");
  target.inert = state.inert;
  if (state.busy === null) target.removeAttribute("aria-busy");
  else target.setAttribute("aria-busy", state.busy);
  loadingTargets.delete(target);
}
for (const event of ["htmx:afterRequest", "htmx:sendError", "htmx:timeout", "htmx:sendAbort"]) {
  document.body.addEventListener(event, finishLoadingRequest);
}

document.body.addEventListener("htmx:beforeRequest", (event) => {
  if (event.detail?.target?.id === "main") mainRequests.set(event.detail.xhr, ++mainRequestGeneration);
});
document.body.addEventListener("htmx:beforeSwap", (event) => {
  const generation = mainRequests.get(event.detail?.xhr);
  if (generation && generation !== mainRequestGeneration) event.detail.shouldSwap = false;
});
function motionAllowed() {
  return !window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}
function animateMotion(element, className, duration = 240) {
  if (!element || !motionAllowed()) return;
  element.classList.remove(className);
  window.requestAnimationFrame(() => {
    element.classList.add(className);
    window.setTimeout(() => element.classList.remove(className), duration);
  });
}
function bindInfiniteLibrary() {
  libraryObserver?.disconnect();
  libraryAbortController?.abort();
  libraryAbortController = undefined;
  const next = document.querySelector("[data-library-next]");
  if (!next || !("IntersectionObserver" in window)) return;
  libraryObserver = new IntersectionObserver(async (entries) => {
    if (!entries.some(({ isIntersecting }) => isIntersecting) || next.dataset.loading) return;
    const navigation = next.closest("[data-library-pagination]");
    const status = document.querySelector("[data-library-status]");
    const controller = new AbortController();
    libraryAbortController = controller;
    next.dataset.loading = "true";
    navigation?.setAttribute("aria-busy", "true");
    try {
      const response = await fetch(next.href, { headers: { "X-Kinosail-Library-Page": "1" }, signal: controller.signal });
      if (!response.ok) throw new Error(`Library page failed with ${response.status}`);
      const incoming = new DOMParser().parseFromString(await response.text(), "text/html");
      const library = document.querySelector("#library");
      const appended = [];
      let added = 0;
      for (const group of incoming.querySelectorAll("[data-library-group]")) {
        const current = [...library.querySelectorAll("[data-library-group]")].find(({ dataset }) => dataset.libraryGroup === group.dataset.libraryGroup);
        if (!current) {
          added += group.querySelectorAll(".card").length;
          library.append(group);
          appended.push(group);
          continue;
        }
        const grid = current.querySelector(".grid");
        const seen = new Set([...grid.querySelectorAll(".card")].map((card) => card.getAttribute("href")));
        for (const card of group.querySelectorAll(".card")) {
          if (seen.has(card.getAttribute("href"))) continue;
          grid.append(card);
          appended.push(card);
          added++;
        }
      }
      for (const element of appended) animateMotion(element, "motion-append", 180);
      const incomingNext = incoming.querySelector("[data-library-next]");
      if (incomingNext) next.replaceWith(incomingNext);
      else {
        next.remove();
        if (!navigation?.querySelector("a")) navigation?.remove();
      }
      if (status) status.textContent = incomingNext ? `${added} more titles loaded.` : "All titles are loaded.";
      navigation?.removeAttribute("aria-busy");
      libraryAbortController = undefined;
      bindInfiniteLibrary();
    } catch (error) {
      if (error.name === "AbortError") return;
      delete next.dataset.loading;
      navigation?.removeAttribute("aria-busy");
      if (status) status.textContent = "Could not load more. Use the link to try again.";
    } finally {
      if (libraryAbortController === controller) libraryAbortController = undefined;
    }
  }, { rootMargin: "600px 0px" });
  libraryObserver.observe(next);
}
bindInfiniteLibrary();

const boundTitleJumps = new WeakSet();
function bindTitleJump() {
  const jump = document.querySelector("[data-title-jump]");
  if (!jump || boundTitleJumps.has(jump)) return;
  boundTitleJumps.add(jump);
  const open = jump.querySelector("[data-title-jump-open]");
  const dialog = jump.querySelector("[data-title-jump-dialog]");
  const index = jump.querySelector("[data-title-jump-index]");
  const preview = jump.querySelector("[data-title-jump-preview]");
  const links = [...index.querySelectorAll("[data-title-letter]")];
  const canScrub = () => links.length <= 27 && window.matchMedia("(max-width: 700px) and (min-height: 820px)").matches;
  jump.classList.toggle("title-jump-scrubbable", links.length <= 27);
  open?.addEventListener("click", () => {
    dialog.showModal();
    animateMotion(dialog, "motion-panel-open");
  });
  dialog?.addEventListener("click", (event) => {
    if (event.target === dialog || event.target.closest?.("[data-title-letter]")) dialog.close();
  });
  if (!links.length) return;
  let dragging = false;
  let selected;
  let suppressClick = false;
  const nearest = (y) => links.reduce((closest, link) => {
    const center = link.getBoundingClientRect().top + link.getBoundingClientRect().height / 2;
    return !closest || Math.abs(center - y) < closest.distance ? { link, distance: Math.abs(center - y) } : closest;
  }, undefined)?.link;
  const select = (event) => {
    selected = nearest(event.clientY);
    if (!selected) return;
    for (const link of links) link.toggleAttribute("data-title-preview", link === selected);
    preview.hidden = false;
    preview.textContent = selected.getAttribute("aria-label");
  };
  const finish = (event, commit) => {
    if (!dragging) return;
    select(event);
    dragging = false;
    index.releasePointerCapture?.(event.pointerId);
    if (commit && selected) {
      suppressClick = true;
      selected.click();
      setTimeout(() => { suppressClick = false; }, 0);
    }
    setTimeout(() => { preview.hidden = true; }, 300);
    event.preventDefault();
  };
  index.addEventListener("pointerdown", (event) => {
    if (!canScrub() || event.button !== 0) return;
    dragging = true;
    index.setPointerCapture?.(event.pointerId);
    select(event);
    event.preventDefault();
  });
  index.addEventListener("pointermove", (event) => { if (dragging) select(event); });
  index.addEventListener("pointerup", (event) => finish(event, true));
  index.addEventListener("pointercancel", (event) => finish(event, false));
  index.addEventListener("click", (event) => {
    if (suppressClick && event.isTrusted) event.preventDefault();
  });
}

bindTitleJump();
document.body.addEventListener("htmx:afterSwap", (event) => {
  if (event.detail?.target?.id === "main") animateMotion(event.detail.target, "motion-enter");
  bindInfiniteLibrary();
  bindTitleJump();
});
document.body.addEventListener("htmx:historyRestore", () => {
  bindInfiniteLibrary();
  bindTitleJump();
});
document.body.addEventListener("htmx:afterSettle", () => {
  const active = document.querySelector('[data-title-jump-index] [aria-current="true"]');
  if (!active) return;
  const first = document.querySelector("#library a.card");
  const status = document.querySelector("[data-library-status]");
  first?.focus({ preventScroll: true });
  if (status && active) status.textContent = `${active.getAttribute("aria-label")}.`;
});

// Error recovery and title details preserve the preceding browse/form position.
for (const control of document.querySelectorAll('[data-return-to-form], [data-browse-back]')) {
  let previous;
  try { previous = new URL(document.referrer); } catch (_) { continue; }
  if (previous.origin !== location.origin || history.length < 2) continue;
  control.hidden = false;
  control.addEventListener('click', event => { event.preventDefault(); history.back(); });
}
