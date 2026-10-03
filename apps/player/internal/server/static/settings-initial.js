if (location.pathname === "/settings") (() => {
  // The Server replaces these JSON strings from the rendered category catalog.
  const anchors = JSON.parse("KINOSAIL_SETTINGS_ANCHORS"), levels = JSON.parse("KINOSAIL_SETTINGS_LEVELS");
  let hash = "";
  try { hash = decodeURIComponent(location.hash.slice(1)); } catch (_) {}
  const category = anchors[hash] || "playback";
  document.documentElement.dataset.settingsCategory = category;
  document.documentElement.dataset.settingsLevel = levels[category];
  if (hash) document.addEventListener("readystatechange", () => {
    const target = document.getElementById(hash);
    if (!target) return;
    for (let ancestor = target; ancestor; ancestor = ancestor.parentElement) if (ancestor instanceof HTMLDetailsElement) ancestor.open = true;
    target.scrollIntoView({block: "start", behavior: "instant"});
  }, {once: true});
})();
