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
  const revealFocus = event => revealDockFocus(event, nav, mobile, ".subtitle-dashboard main");
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
