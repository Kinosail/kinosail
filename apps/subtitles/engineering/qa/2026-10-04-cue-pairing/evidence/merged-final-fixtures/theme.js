// Reserve the fixed navigation's actual height before fragment alignment.
(() => {
  const mobile = matchMedia("(max-width: 900px)");
  let nav, bootstrap;
  const update = () => {
    const root = document.documentElement;
    if (!mobile.matches) { root.style.removeProperty("--subtitle-dock-height"); root.style.removeProperty("--subtitle-header-height"); return; }
    const height = `${nav.getBoundingClientRect().height}px`;
    if (root.style.getPropertyValue("--subtitle-dock-height") !== height) root.style.setProperty("--subtitle-dock-height", height);
    const headerHeight = `${nav.closest("header").getBoundingClientRect().height}px`;
    if (root.style.getPropertyValue("--subtitle-header-height") !== headerHeight) root.style.setProperty("--subtitle-header-height", headerHeight);
  };
  const revealFocus = event => {
    if (!mobile.matches || event.key !== "Tab" || event.altKey || event.ctrlKey || event.metaKey) return;
    const previous = document.activeElement;
    requestAnimationFrame(() => {
      const target = document.activeElement;
      if (event.defaultPrevented || target === previous || !target?.closest(".subtitle-dashboard main")) return;
      const rect = target.getBoundingClientRect();
      const top = nav.closest("header").getBoundingClientRect().bottom + 8;
      const bottom = nav.getBoundingClientRect().top - 8;
      const delta = rect.top < top ? rect.top - top : rect.bottom > bottom ? (rect.height <= bottom - top ? rect.bottom - bottom : rect.top - top) : 0;
      if (delta) scrollBy({top: delta, behavior: "instant"});
    });
  };
  const initialize = () => {
    const candidate = document.querySelector("[data-subtitle-dock]");
    if (nav || !candidate || !(candidate.nextElementSibling || candidate.closest("header")?.nextElementSibling || document.readyState !== "loading")) return;
    nav = candidate;
    update();
    new ResizeObserver(update).observe(nav);
    new ResizeObserver(update).observe(nav.closest("header"));
    mobile.addEventListener("change", update);
    document.addEventListener("keydown", revealFocus);
    bootstrap?.disconnect();
  };
  initialize();
  if (!nav) {
    bootstrap = new MutationObserver(initialize);
    bootstrap.observe(document, {subtree: true, childList: true});
    document.addEventListener("DOMContentLoaded", () => { initialize(); bootstrap.disconnect(); }, {once: true});
  }
})();
const themeKey = "kinosail-theme";
const themes = ["dark", "light", "system"];
const systemTheme = matchMedia("(prefers-color-scheme: dark)");
let theme;
try { theme = localStorage.getItem(themeKey) || "dark"; } catch { theme = "dark"; }
if (!themes.includes(theme)) theme = "dark";

function applyTheme() {
  const effective = theme === "system" ? systemTheme.matches ? "dark" : "light" : theme;
  document.documentElement.dataset.theme = effective;
  document.querySelector('meta[name="theme-color"]')?.setAttribute("content", effective === "light" ? "#f4f8ef" : "#0b0d0b");
  document.querySelectorAll("[data-theme-choice]").forEach((control) => {
    if (control instanceof HTMLInputElement && control.type === "radio") control.checked = control.value === theme;
    else control.value = theme;
  });
}

function bindThemeControls() {
  document.querySelectorAll("[data-theme-choice]").forEach((control) => control.addEventListener("change", () => {
    if (control instanceof HTMLInputElement && control.type === "radio" && !control.checked) return;
    if (!themes.includes(control.value)) return;
    theme = control.value;
    try { localStorage.setItem(themeKey, theme); } catch {}
    applyTheme();
  }));
  applyTheme();
}


applyTheme();
if (document.readyState === "loading") addEventListener("DOMContentLoaded", bindThemeControls, { once: true });
else bindThemeControls();
systemTheme.addEventListener?.("change", () => { if (theme === "system") applyTheme(); });
document.documentElement.classList.add("js");

function bindPasswordControls() {
  document.querySelectorAll('input[type="password"]:not([data-password-toggle])').forEach((input) => {
    input.dataset.passwordToggle = "";
    const label = input.closest("label");
    const wrapper = document.createElement("span");
    wrapper.className = "password-control";
    if (label) {
      wrapper.classList.add("has-label");
      label.before(wrapper);
      wrapper.append(label);
    } else {
      input.before(wrapper);
      wrapper.append(input);
    }
    const button = document.createElement("button");
    button.type = "button";
    button.className = "password-toggle";
    button.setAttribute("aria-label", "Show secret");
    button.setAttribute("aria-pressed", "false");
    button.disabled = input.disabled;
    button.addEventListener("click", () => {
      const shown = input.type === "text";
      input.type = shown ? "password" : "text";
      button.setAttribute("aria-label", shown ? "Show secret" : "Hide secret");
      button.setAttribute("aria-pressed", String(!shown));
    });
    wrapper.append(button);
  });
}

function bindCopyControls() {
  document.querySelectorAll("[data-copy-target]").forEach((button) => button.addEventListener("click", async () => {
    const input = document.getElementById(button.dataset.copyTarget);
    const status = button.parentElement?.querySelector(".copy-status");
    if (!input || !status) return;
    try {
      await navigator.clipboard.writeText(input.value);
      status.textContent = button.parentElement.querySelector("[data-copy-success]")?.textContent || "Copied.";
    } catch {
      input.focus();
      input.select();
      status.textContent = button.parentElement.querySelector("[data-copy-fallback]")?.textContent || "Select Copy in your browser.";
    }
  }));
}

function bindDisclosureControls() {
  const openHashDisclosure = () => {
    let id;
    try { id = decodeURIComponent(location.hash.slice(1)); } catch { return; }
    const disclosure = document.getElementById(id);
    if (disclosure instanceof HTMLDetailsElement) disclosure.open = true;
  };
  openHashDisclosure();
  addEventListener("hashchange", openHashDisclosure);
  document.querySelectorAll("[data-open-disclosure]").forEach((link) => link.addEventListener("click", (event) => {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || !(link instanceof HTMLAnchorElement)) return;
    const disclosure = document.getElementById(link.dataset.openDisclosure);
    const summary = disclosure?.querySelector(":scope > summary");
    if (!(disclosure instanceof HTMLDetailsElement) || !summary) return;
    event.preventDefault();
    disclosure.open = true;
    history[location.hash === link.hash ? "replaceState" : "pushState"](null, "", link.hash);
    disclosure.scrollIntoView({ block: "start" });
    summary.focus({ preventScroll: true });
  }));
}

function bindTrustedHTTPSControls() {
  document.querySelectorAll("[data-trusted-https-test]").forEach((button) => {
    const form = button.closest("form");
    const status = form?.querySelector("[data-trusted-https-test-status]");
    if (!form || !status) return;
    let generation = 0;
    let validationTimer;
    const csrf = document.querySelector('meta[name="kinosail-csrf"]')?.content;
    const fields = () => {
      const data = new FormData(form);
      return { provider: data.get("provider"), domain: data.get("domain"), token: data.get("token"), address: data.get("address"), termsAccepted: data.get("termsAccepted") === "true" };
    };
    const request = async (path, method = "POST") => {
      const response = await fetch(path, {
        method,
        headers: { "Content-Type": "application/json", ...(csrf ? { "X-Kinosail-CSRF": csrf } : {}) },
        body: JSON.stringify(fields()),
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.error?.replaceAll("\n", " — ") || "Trusted HTTPS validation failed");
      return result;
    };
    const complete = () => form.checkValidity() && [...form.querySelectorAll("input[required], select[required]")].every((field) => field.type === "checkbox" ? field.checked : field.value.trim());
    const show = (message, state) => {
      status.textContent = message;
      if (state) status.dataset.state = state;
      else delete status.dataset.state;
    };
    const validate = async () => {
      if (!complete()) return;
      const requestGeneration = generation;
      show("Checking these details…");
      try {
        const result = await request("/api/v1/settings/trusted-https/validate");
        if (requestGeneration === generation) show(`Details match this deployment for ${result.trustedHttps.hostname}. DNS has not been tested.`);
      } catch (error) {
        if (requestGeneration === generation) show(error instanceof Error ? error.message : "Trusted HTTPS validation failed", "failed");
      }
    };
    form.addEventListener("input", () => {
      generation++;
      clearTimeout(validationTimer);
      show(status.dataset.state === "passed" ? "Details changed. Test again." : "");
      if (complete()) validationTimer = setTimeout(validate, 450);
    });
    button.addEventListener("click", async () => {
      if (!form.reportValidity()) return;
      clearTimeout(validationTimer);
      const requestGeneration = ++generation;
      button.disabled = true;
      form.setAttribute("aria-busy", "true");
      show("Testing DNS connection…");
      try {
        const result = await request("/api/v1/settings/trusted-https/test");
        if (!result.trustedHttps?.hostname) throw new Error("DNS provider test failed");
        if (requestGeneration !== generation) return;
        show(`Connection verified for ${result.trustedHttps.hostname}. Nothing was saved.`, "passed");
      } catch (error) {
        if (requestGeneration !== generation) return;
        show(error instanceof Error ? error.message : "DNS provider test failed", "failed");
      } finally {
        button.disabled = false;
        form.removeAttribute("aria-busy");
      }
    });
    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      if (!form.reportValidity()) return;
      clearTimeout(validationTimer);
      generation++;
      const submit = event.submitter;
      if (submit instanceof HTMLButtonElement) submit.disabled = true;
      button.disabled = true;
      form.setAttribute("aria-busy", "true");
      show("Checking and saving…");
      try {
        await request("/api/v1/settings/trusted-https", "PUT");
        location.assign(form.action.includes("/onboarding/") ? "/onboarding/connection" : "/settings#trusted-https");
      } catch (error) {
        show(error instanceof Error ? error.message : "Trusted HTTPS could not be saved", "failed");
        if (submit instanceof HTMLButtonElement) submit.disabled = false;
        button.disabled = false;
        form.removeAttribute("aria-busy");
      }
    });
  });
}

function bindDocumentControls() {
  bindPasswordControls();
  bindCopyControls();
  bindDisclosureControls();
  bindTrustedHTTPSControls();
  document.querySelectorAll("[data-curation-menu]").forEach((menu) => {
    const search = menu.querySelector("[data-curation-search]");
    const empty = menu.querySelector("[data-curation-empty]");
    search?.addEventListener("input", () => {
      const query = search.value.trim().toLocaleLowerCase();
      let matches = 0;
      menu.querySelectorAll("[data-curation-option]").forEach((option) => {
        option.hidden = !option.querySelector("button span").textContent.toLocaleLowerCase().includes(query);
        if (!option.hidden) matches++;
      });
      menu.querySelectorAll(".curation-options section").forEach((section) => { section.hidden = !section.querySelector("[data-curation-option]:not([hidden])"); });
      empty.hidden = matches > 0;
    });
  });
}

if (document.readyState === "loading") addEventListener("DOMContentLoaded", bindDocumentControls, { once: true });
else bindDocumentControls();

// Project capability-dependent Apple controls while the HTML is being parsed,
// before the deferred player bundle. Keep one toolbar and one settings panel.
if (location.pathname.startsWith("/watch/")) {
  const appleTouch = /iPhone|iPad|iPod/.test(navigator.userAgent) || navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1;
  if (appleTouch && typeof document.createElement("video").webkitEnterFullscreen === "function") {
    const project = () => {
      const video = document.querySelector(".media-stage video");
      if (!video) return;
      video.autoplay = false;
      delete video.dataset.autoplay;
      video.controls = false;
      const stage = video.closest(".media-stage");
      stage.querySelector("[data-player-controls]")?.classList.add("player-native-controls");
      const existing = stage.nextElementSibling;
      if (existing?.classList.contains("player-native-options")) {
        if (existing.querySelector(".player-settings")) observer.disconnect();
        return;
      }
      const toolbar = stage.querySelector(".player-stage-toolbar");
      const settings = stage.querySelector(".player-settings");
      // The buffer follows the complete settings subtree in both app templates.
      if (!toolbar || !settings || !stage.querySelector("[data-player-status]")) return;
      const button = stage.querySelector("[data-player-settings]");
      if (button && !toolbar.contains(button)) toolbar.append(button);
      const options = document.createElement("div");
      options.className = "player-native-options";
      stage.after(options);
      options.append(toolbar, settings);
      observer.disconnect();
    };
    const observer = new MutationObserver(project);
    observer.observe(document, {childList: true, subtree: true});
    document.addEventListener("DOMContentLoaded", () => { project(); observer.disconnect(); }, {once: true});
  }
}
// Align a bookmarked section before the first body paint. Native fragment
// scrolling can otherwise wait for unrelated deferred scripts to finish.
if (location.hash) (() => {
  let id;
  try { id = decodeURIComponent(location.hash.slice(1)); } catch (_) { return; }
  if (!id) return;
  let completed = false;
  let headerObserver;
  const waitingStyles = new WeakSet();
  const project = () => {
    if (completed) return;
    const target = document.getElementById(id);
    if (!target) return;
    const pendingStyles = [...document.querySelectorAll('link[rel~="stylesheet"]')].filter(link => !link.sheet);
    for (const link of pendingStyles) if (!waitingStyles.has(link)) { waitingStyles.add(link); link.addEventListener("load", project, {once: true}); }
    if (pendingStyles.length) return;
    const dock = document.querySelector("[data-subtitle-dock]");
    const header = document.querySelector("body.settings-page>.app-header");
    if (!dock && header && !headerObserver) {
      const update = () => document.documentElement.style.setProperty("--settings-header-height", `${header.getBoundingClientRect().height}px`);
      update();
      headerObserver = new ResizeObserver(update);
      headerObserver.observe(header);
    }
    if (matchMedia("(max-width: 900px)").matches && dock) {
      const root = document.documentElement.style;
      const matches = (name, node) => Math.abs(Number.parseFloat(root.getPropertyValue(name)) - node.getBoundingClientRect().height) <= 1;
      if (!matches("--subtitle-dock-height", dock) || !matches("--subtitle-header-height", dock.closest("header"))) return;
    }
    for (let ancestor = target; ancestor; ancestor = ancestor.parentElement) if (ancestor instanceof HTMLDetailsElement) ancestor.open = true;
    if (!target.getClientRects().length) return;
    const number = value => Number.parseFloat(value) || 0;
    const desired = target.getBoundingClientRect().top + scrollY - number(getComputedStyle(target).scrollMarginTop) - number(getComputedStyle(document.documentElement).scrollPaddingTop);
    if (document.readyState === "loading" && desired > document.documentElement.scrollHeight - innerHeight + 1) return;
    target.scrollIntoView({block: "start", behavior: "instant"});
    completed = true;
    observer.disconnect();
  };
  const observer = new MutationObserver(project);
  observer.observe(document, {childList: true, subtree: true});
  observer.observe(document.documentElement, {attributes: true, attributeFilter: ["style"]});
  project();
  document.addEventListener("readystatechange", () => { if (document.readyState === "interactive") project(); });
  document.addEventListener("DOMContentLoaded", () => {
    project();
    if (completed || !document.getElementById(id)) observer.disconnect();
  }, {once: true});
  addEventListener("pagehide", event => {observer.disconnect(); if (!event.persisted) headerObserver?.disconnect();});
})();
(() => {
  function bind() {
    const main = document.querySelector("main.last-light");
    if (!main || main.dataset.paletteBound) return;
    main.dataset.paletteBound = "true";
    // Keep the featured title and its actions stable while people browse a shelf.
    main.querySelectorAll(".media-hero > .media-backdrop, .home-feature > img").forEach((image) => {
      image.addEventListener("error", () => {
        image.hidden = true;
        image.parentElement?.classList.remove("has-media-backdrop");
      });
    });
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", bind);
  else bind();
  document.addEventListener("htmx:after:swap", bind);
})();
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
  document.addEventListener("htmx:after:swap", bind);
})();
