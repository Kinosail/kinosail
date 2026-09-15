(() => {
  if (!document.querySelector(".subtitle-dashboard")) return;
  let navigation, pollRequest, pollTimer, searchTimer;
  let currentURL = location.href;
  let actionPending = false;
  const number = new Intl.NumberFormat(document.documentElement.lang || navigator.language);
  const main = () => document.getElementById("main");
  const label = name => main()?.dataset[name] || "";

  function formatContent() {
    document.querySelectorAll("[data-number]").forEach(element => {
      const value = Number(element.textContent);
      if (Number.isSafeInteger(value) && value >= 0) element.textContent = number.format(value);
    });
    document.querySelectorAll("time:not([datetime])").forEach(element => {
      const value = element.textContent.trim();
      const date = new Date(value);
      if (!Number.isNaN(date.getTime())) {
        element.dateTime = value;
        element.title = value;
        element.textContent = new Intl.DateTimeFormat(document.documentElement.lang || navigator.language, {dateStyle: "medium", timeStyle: "short"}).format(date);
      }
    });
    const loading = document.getElementById("subtitle-loading");
    if (loading && document.getElementById("subtitle-content")) loading.hidden = true;
  }

  function feedback(message, error = false, retryURL = "") {
    const region = document.getElementById("subtitle-feedback");
    if (!region) return;
    region.textContent = message;
    region.hidden = !message;
    region.toggleAttribute("data-error", error);
    if (retryURL) {
      const retry = document.createElement("a");
      retry.href = retryURL;
      retry.dataset.subtitleNav = "";
      retry.textContent = label("retryLabel");
      region.append(retry);
    }
  }

  function rememberScroll() {
    history.replaceState({...history.state, subtitleScroll: scrollY}, "", currentURL);
  }

  async function loadPage(url, {mode = "push", focus = "list", preserve = false, scroll = 0} = {}) {
    const target = new URL(url, location.href);
    if (target.origin !== location.origin || target.pathname !== "/") return;
    navigation?.abort();
    pollRequest?.abort();
    clearTimeout(pollTimer);
    const controller = new AbortController();
    navigation = controller;
    const timeout = setTimeout(() => controller.abort("timeout"), 30000);
    const openFiles = preserve ? [...document.querySelectorAll(".subtitle-file[open]")].map(element => element.id) : [];
    const search = document.getElementById("subtitle-search");
    const selection = focus === "search" ? [search?.selectionStart, search?.selectionEnd] : null;
    const oldScroll = scrollY;
    main()?.setAttribute("aria-busy", "true");
    feedback(label("loadingLabel"));
    try {
      const response = await fetch(target, {headers: {Accept: "text/html"}, signal: controller.signal});
      if (response.redirected && new URL(response.url).pathname !== "/") {
        location.assign(response.url);
        return false;
      }
      if (!response.ok) throw new Error("page unavailable");
      const text = await response.text();
      if (controller !== navigation || controller.signal.aborted) return false;
      const page = new DOMParser().parseFromString(text, "text/html");
      const replacement = page.querySelector(".subtitle-dashboard #main");
      if (!replacement || !replacement.querySelector("#subtitle-content") || replacement.querySelector("#subtitle-content > [role=alert]")) throw new Error("incomplete page");
      replacement.querySelectorAll("script").forEach(script => script.remove());
      if (mode !== "pop") {
        rememberScroll();
        const method = mode === "replace" || target.href === currentURL ? "replaceState" : "pushState";
        history[method]({subtitleScroll: preserve ? oldScroll : 0}, "", target);
      }
      currentURL = target.href;
      main().replaceWith(document.importNode(replacement, true));
      document.title = page.title;
      document.querySelectorAll(".app-header nav a[data-subtitle-nav]").forEach(link => {
        const active = new URL(link.href).searchParams.get("view") === main().dataset.view;
        link.classList.toggle("active", active);
        if (active) link.setAttribute("aria-current", "page");
        else link.removeAttribute("aria-current");
      });
      openFiles.forEach(id => { const item = document.getElementById(id); if (item) item.open = true; });
      formatContent();
      if (actionPending) feedback(label("actionLabel"));
      if (focus === "search") {
        const input = document.getElementById("subtitle-search");
        input?.focus({preventScroll: true});
        if (selection?.[0] !== null) input?.setSelectionRange(...selection);
        window.scrollTo({top: oldScroll, behavior: "instant"});
      } else if (mode === "pop" || preserve) {
        window.scrollTo({top: preserve ? oldScroll : scroll, behavior: "instant"});
      } else {
        const heading = document.getElementById(focus === "page" ? "main" : "subtitle-list-title");
        heading?.focus({preventScroll: true});
        if (focus === "page") window.scrollTo({top: 0, behavior: "instant"});
        else heading?.scrollIntoView({block: "start", behavior: "instant"});
      }
      return true;
    } catch (error) {
      if (controller === navigation && (!controller.signal.aborted || controller.signal.reason === "timeout")) feedback(label("errorLabel"), true, target.href);
      return false;
    } finally {
      clearTimeout(timeout);
      if (controller === navigation) {
        main()?.removeAttribute("aria-busy");
        navigation = null;
        schedulePoll();
      }
    }
  }

  function searchURL(form) {
    const url = new URL("/", location.origin);
    url.search = new URLSearchParams(new FormData(form)).toString();
    return url;
  }

  document.addEventListener("click", event => {
    const link = event.target.closest("a[data-subtitle-nav], a[data-subtitle-page], a[data-subtitle-refresh]");
    if (!link || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    clearTimeout(searchTimer);
    if (link.hasAttribute("data-subtitle-refresh")) loadPage(location.href, {mode: "replace", preserve: true});
    else loadPage(link.href, {focus: link.hasAttribute("data-subtitle-page") ? "list" : "page"});
  });

  document.addEventListener("submit", event => {
    const form = event.target;
    if (form.matches("[data-subtitle-search]")) {
      event.preventDefault();
      clearTimeout(searchTimer);
      loadPage(searchURL(form));
    } else if (form.matches("[data-subtitle-action]")) {
      event.preventDefault();
      runAction(form, event.submitter);
    }
  });

  document.addEventListener("input", event => {
    if (event.target.id !== "subtitle-search" || event.isComposing) return;
    clearTimeout(searchTimer);
    navigation?.abort();
    const form = event.target.form;
    searchTimer = setTimeout(() => loadPage(searchURL(form), {mode: "replace", focus: "search"}), 350);
  });
  document.addEventListener("change", event => {
    if (event.target.matches(".subtitle-filters select")) event.target.form.requestSubmit();
  });
  addEventListener("popstate", event => {
    clearTimeout(searchTimer);
    loadPage(location.href, {mode: "pop", scroll: event.state?.subtitleScroll || 0});
  });

  async function runAction(form, button) {
    if (actionPending || !button) { feedback(label("actionLabel")); return; }
    const endpoint = new URL(form.dataset.api, location.origin);
    if (endpoint.origin !== location.origin || !/^\/api\/v1\/subtitle-library\/(maintain|[a-f0-9]{16}\/(fetch|restore|replacement))$/.test(endpoint.pathname)) return;
    const payload = {};
    if (form.dataset.language) payload.language = form.dataset.language;
    if (button.name === "replaceable") payload.replaceable = button.value === "true";
    const fileID = form.closest(".subtitle-file")?.id;
    actionPending = true;
    button.disabled = true;
    form.setAttribute("aria-busy", "true");
    feedback(label("actionLabel"));
    try {
      const response = await fetch(endpoint, {method: "POST", headers: {"Content-Type": "application/json", Accept: "application/json", "X-Kinosail-CSRF": document.querySelector('meta[name="kinosail-csrf"]')?.content || ""}, body: JSON.stringify(payload)});
      if (!response.ok) throw new Error("action unavailable");
      const result = response.status === 200 ? await response.json() : null;
      let message = label("actionDone");
      if (result && [result.attempted, result.added, result.upgraded, result.failed].every(value => Number.isSafeInteger(value) && value >= 0)) {
        message = `${number.format(result.attempted)} ${label("checkedLabel")} · ${number.format(result.added)} ${label("addedLabel")} · ${number.format(result.upgraded)} ${label("improvedLabel")} · ${number.format(result.failed)} ${label("failedLabel")}`;
      }
      const loaded = await loadPage(location.href, {mode: "replace", preserve: true});
      if (loaded) {
        feedback(message, result?.failed > 0);
        const file = fileID && document.getElementById(fileID);
        file?.querySelector("summary")?.focus({preventScroll: true});
      } else feedback(message + " " + label("errorLabel"), true, location.href);
    } catch {
      feedback(label("actionError"), true, location.href);
    } finally {
      actionPending = false;
      button.disabled = false;
      form.removeAttribute("aria-busy");
    }
  }

  function schedulePoll() {
    clearTimeout(pollTimer);
    const pending = Number(document.getElementById("subtitle-content")?.dataset.pending);
    pollTimer = setTimeout(checkCoverage, pending > 0 ? 15000 : 60000);
  }
  async function checkCoverage() {
    if (document.hidden || navigation || actionPending) { schedulePoll(); return; }
    pollRequest = new AbortController();
    const timeout = setTimeout(() => pollRequest?.abort(), 15000);
    try {
      const response = await fetch("/api/v1/subtitle-library?view=summary", {headers: {Accept: "application/json"}, signal: pollRequest.signal});
      if (!response.ok) return;
      const data = await response.json();
      const content = document.getElementById("subtitle-content");
      const fields = ["total", "ready", "wanted", "pending", "unavailable"];
      if (content && fields.every(key => Number.isSafeInteger(data[key]) && data[key] >= 0) && fields.some(key => data[key] !== Number(content.dataset[key]))) {
        const update = document.getElementById("subtitle-update");
        if (update) update.hidden = false;
      }
    } catch { /* A failed background check never replaces the current library. */ }
    finally { clearTimeout(timeout); pollRequest = null; schedulePoll(); }
  }
  addEventListener("pageshow", event => { if (event.persisted) schedulePoll(); });
  addEventListener("pagehide", () => { navigation?.abort(); pollRequest?.abort(); clearTimeout(pollTimer); clearTimeout(searchTimer); });
  formatContent();
  schedulePoll();
})();
