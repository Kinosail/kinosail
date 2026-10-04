// Runs inside the real page. Record viewport geometry separately from document
// coordinates: a scroll-only bookmark error is invisible to element-size checks.
export function bookmarkSnapshot() {
  if (location.pathname !== "/settings" || !location.hash) return undefined;
  let id;
  try { id = decodeURIComponent(location.hash.slice(1)); }
  catch { return {malformed: true, resolved: false}; }
  const target = document.getElementById(id);
  if (!target) return {malformed: false, resolved: false};
  const section = target.matches("section") ? target :
    target.closest("section") || (id === "security" ? document.getElementById("session-timeouts") : target);
  const heading = section?.querySelector("h2") || section;
  const rectangle = node => node?.getBoundingClientRect().toJSON();
  const header = document.querySelector(".app-header");
  const dock = [...document.querySelectorAll("[data-subtitle-dock], .mobile-navigation")]
    .find(node => getComputedStyle(node).position === "fixed" && node.getBoundingClientRect().height > 0);
  const headingBox = rectangle(heading), headerBox = rectangle(header), dockBox = rectangle(dock);
  const headerPinned = header && ["fixed", "sticky"].includes(getComputedStyle(header).position);
  const top = headerPinned ? Math.max(0, headerBox.bottom) : 0;
  const bottom = dockBox ? dockBox.top : innerHeight;
  const number = value => Number.parseFloat(value) || 0;
  const margin = number(getComputedStyle(target).scrollMarginTop);
  const padding = number(getComputedStyle(document.documentElement).scrollPaddingTop);
  const targetBox = rectangle(target), documentTop = targetBox.top + scrollY;
  const maximumScroll = Math.max(0, document.documentElement.scrollHeight - innerHeight);
  const visible = Boolean(headingBox?.height && headingBox.top >= top - 1 && headingBox.top < bottom - 1 &&
    (headingBox.height > bottom - top || headingBox.bottom <= bottom + 1));
  return {malformed: false, resolved: true, visible, heading: headingBox, header: headerBox, dock: dockBox,
    visibleTop: top, visibleBottom: bottom, target: targetBox, documentTop, scrollY, maximumScroll,
    scrollMarginTop: margin, scrollPaddingTop: padding, desiredScroll: Math.min(maximumScroll, Math.max(0, documentTop - margin - padding))};
}
