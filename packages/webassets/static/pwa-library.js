let libraryObserver;
let libraryAbortController;
let libraryPendingPage;
let libraryRestorePending = false;
const libraryRetryControls = new WeakSet();

function libraryCardKey(card) {
  const link = card.matches("a[href]") ? card : card.querySelector("a.show-details[href]");
  const key = link?.getAttribute("href");
  if (!/^\/(?:show|watch|item|album|book)\/[A-Za-z0-9_-]{1,256}$/.test(key || "")) throw new Error("Invalid library card identity");
  return key;
}
function stopLibraryPaging() {
  libraryObserver?.disconnect();
  libraryAbortController?.abort();
  if (libraryPendingPage) {
    delete libraryPendingPage.next.dataset.loading;
    libraryPendingPage.next.hidden = false;
    libraryPendingPage.navigation?.removeAttribute("aria-busy");
  }
  libraryPendingPage = undefined;
  libraryAbortController = undefined;
}
function bindLibraryRetry(next) {
  if (typeof window.IntersectionObserver !== "function" || libraryRetryControls.has(next)) return;
  libraryRetryControls.add(next);
  next.addEventListener("click", event => {
    event.preventDefault(); libraryRetryControls.delete(next); bindInfiniteLibrary();
  }, { once: true });
}
function bindInfiniteLibrary() {
  stopLibraryPaging();
  if (libraryRestorePending) return;
  const next = document.querySelector("[data-library-next]");
  if (!next || typeof window.IntersectionObserver !== "function") return;
  const status = document.querySelector("[data-library-status]");
  const library = document.querySelector("#library");
  if (!status || !library) return;
  next.hidden = true; bindLibraryRetry(next);
  libraryObserver = new IntersectionObserver(async (entries) => {
    if (!next.isConnected || !entries.some(({ isIntersecting }) => isIntersecting) || next.dataset.loading) return;
    await loadLibraryPage(next);
  }, { rootMargin: "600px 0px" });
  libraryObserver.observe(status);
}

// Both automatic paging and browse return use the same validated Go fragment.
async function loadLibraryPage(next, current = () => true) {
  const status = document.querySelector("[data-library-status]");
  const library = document.querySelector("#library");
  if (!status || !library || !next.isConnected || next.dataset.loading || !current()) return false;
  bindLibraryRetry(next);
  libraryObserver?.disconnect();
  const navigation = next.closest("[data-library-pagination]");
  const controller = new AbortController();
  libraryAbortController = controller;
  libraryPendingPage = { next, navigation, controller };
  next.dataset.loading = "true"; next.hidden = true;
  navigation?.setAttribute("aria-busy", "true"); status.textContent = "Loading more titles…";
  delete status.dataset.failure; delete status.dataset.requestId;
  let failure = "network", requestID;
  try {
    const response = await fetch(next.href, { headers: { "X-Kinosail-Library-Page": "1" }, signal: controller.signal });
    const correlation = response.headers.get("X-Request-ID");
    if (/^[A-Za-z0-9_-]{1,64}$/.test(correlation || "")) requestID = correlation;
    failure = "http";
    if (!response.ok) throw new Error(`Library page failed with ${response.status}`);
    const incoming = new DOMParser().parseFromString(await response.text(), "text/html");
    if (controller.signal.aborted || libraryAbortController !== controller || !next.isConnected || !library.isConnected || !current()) return false;
    failure = "invalid_fragment";
    if (!incoming.querySelector("#library")) throw new Error("Missing library page");
    const groups = [...incoming.querySelectorAll("#library [data-library-group]")];
    const keys = new Map();
    for (const group of groups) {
      if (!group.querySelector(".grid")) throw new Error("Missing library grid");
      for (const card of group.querySelectorAll(".card")) keys.set(card, libraryCardKey(card));
    }
    for (const card of library.querySelectorAll(".card")) libraryCardKey(card);
    const appended = [];
    let added = 0;
    for (const group of groups) {
      const current = [...library.querySelectorAll("[data-library-group]")].find(({ dataset }) => dataset.libraryGroup === group.dataset.libraryGroup);
      const grid = current?.querySelector(".grid") || group.querySelector(".grid");
      const seen = new Set(current ? [...grid.querySelectorAll(".card")].map(libraryCardKey) : []);
      for (const card of group.querySelectorAll(".card")) {
        const key = keys.get(card);
        if (seen.has(key)) { card.remove(); continue; }
        seen.add(key);
        if (current) { grid.append(card); appended.push(card); }
        added++;
      }
      if (!current) { library.append(group); appended.push(group); }
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
    libraryPendingPage = undefined;
    if (!libraryRestorePending) bindInfiniteLibrary();
    queueMicrotask(() => window.dispatchEvent(new Event("kinosail:library-page")));
    return true;
  } catch (error) {
    if (error.name === "AbortError" || controller.signal.aborted || !next.isConnected || !current()) return false;
    status.dataset.failure = failure;
    if (requestID) status.dataset.requestId = requestID;
    delete next.dataset.loading;
    navigation?.removeAttribute("aria-busy");
    libraryObserver?.disconnect(); next.hidden = false;
    next.textContent = "Retry loading"; status.textContent = "Could not load more titles.";
    return false;
  } finally {
    if (libraryPendingPage?.controller === controller) {
      delete next.dataset.loading;
      navigation?.removeAttribute("aria-busy");
      if (controller.signal.aborted && next.isConnected) next.hidden = false;
      libraryPendingPage = undefined;
    }
    if (libraryAbortController === controller) libraryAbortController = undefined;
  }
}
