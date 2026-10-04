let libraryObserver;
let libraryAbortController;

function libraryCardKey(card) {
  const link = card.matches("a[href]") ? card : card.querySelector("a.show-details[href]");
  const key = link?.getAttribute("href");
  if (!/^\/(?:show|watch|item|album|book)\/[A-Za-z0-9_-]{1,256}$/.test(key || "")) throw new Error("Invalid library card identity");
  return key;
}
function bindInfiniteLibrary() {
  libraryObserver?.disconnect();
  libraryAbortController?.abort();
  libraryAbortController = undefined;
  const next = document.querySelector("[data-library-next]");
  if (!next || typeof window.IntersectionObserver !== "function") return;
  const status = document.querySelector("[data-library-status]");
  const library = document.querySelector("#library");
  if (!status || !library) return;
  next.hidden = true; next.addEventListener("click", event => { event.preventDefault(); bindInfiniteLibrary(); }, { once: true });
  libraryObserver = new IntersectionObserver(async (entries) => {
    if (!next.isConnected || !entries.some(({ isIntersecting }) => isIntersecting) || next.dataset.loading) return;
    const navigation = next.closest("[data-library-pagination]");
    const controller = new AbortController();
    libraryAbortController = controller;
    next.dataset.loading = "true";
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
      if (controller.signal.aborted || libraryAbortController !== controller || !next.isConnected || !library.isConnected) return;
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
      bindInfiniteLibrary();
    } catch (error) {
      if (error.name === "AbortError" || controller.signal.aborted || !next.isConnected) return;
      status.dataset.failure = failure;
      if (requestID) status.dataset.requestId = requestID;
      delete next.dataset.loading;
      navigation?.removeAttribute("aria-busy");
      libraryObserver?.disconnect(); next.hidden = false;
      next.textContent = "Retry loading"; status.textContent = "Could not load more titles.";
    } finally {
      if (libraryAbortController === controller) libraryAbortController = undefined;
    }
  }, { rootMargin: "600px 0px" });
  libraryObserver.observe(status);
}
