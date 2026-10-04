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
