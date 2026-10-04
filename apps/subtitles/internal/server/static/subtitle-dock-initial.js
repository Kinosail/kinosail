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
  const initialize = () => {
    const candidate = document.querySelector("[data-subtitle-dock]");
    if (nav || !candidate || !(candidate.nextElementSibling || candidate.closest("header")?.nextElementSibling || document.readyState !== "loading")) return;
    nav = candidate;
    update();
    new ResizeObserver(update).observe(nav);
    mobile.addEventListener("change", update);
    bootstrap?.disconnect();
  };
  initialize();
  if (!nav) {
    bootstrap = new MutationObserver(initialize);
    bootstrap.observe(document, {subtree: true, childList: true});
    document.addEventListener("DOMContentLoaded", () => { initialize(); bootstrap.disconnect(); }, {once: true});
  }
})();
