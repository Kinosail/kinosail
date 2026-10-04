if (location.pathname === "/settings") (() => {
  // The Server replaces these JSON strings from the rendered category catalog.
  const anchors = JSON.parse("KINOSAIL_SETTINGS_ANCHORS"), levels = JSON.parse("KINOSAIL_SETTINGS_LEVELS");
  let hash = "";
  try { hash = decodeURIComponent(location.hash.slice(1)); } catch (_) {}
  const category = anchors[hash] || "playback";
  document.documentElement.dataset.settingsCategory = category;
  document.documentElement.dataset.settingsLevel = levels[category];
})();
