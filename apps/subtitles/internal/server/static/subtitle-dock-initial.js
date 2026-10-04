// Reserve the fixed navigation's actual height before fragment alignment.
(() => {
  const mobile = matchMedia("(max-width: 900px)");
  let nav, bootstrap;
  const update = () => {
    const root = document.documentElement;
    if (!mobile.matches) { root.style.removeProperty("--subtitle-dock-height"); return; }
    const height = `${nav.getBoundingClientRect().height}px`;
    if (root.style.getPropertyValue("--subtitle-dock-height") !== height) root.style.setProperty("--subtitle-dock-height", height);
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
