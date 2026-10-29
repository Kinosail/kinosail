function revealDockFocus(event, nav, mobile, scope) {
  if (!mobile.matches || event.key !== "Tab" || event.altKey || event.ctrlKey || event.metaKey) return;
  const previous = document.activeElement;
  requestAnimationFrame(() => {
    const target = document.activeElement;
    if (event.defaultPrevented || target === previous || !target?.closest(scope)) return;
    const rect = target.getBoundingClientRect();
    const top = nav.closest("header").getBoundingClientRect().bottom + 8;
    const bottom = nav.getBoundingClientRect().top - 8;
    const delta = rect.top < top ? rect.top - top : rect.bottom > bottom ? (rect.height <= bottom - top ? rect.bottom - bottom : rect.top - top) : 0;
    if (delta) scrollBy({top: delta, behavior: "instant"});
  });
}
