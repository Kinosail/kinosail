const settingsNav = document.querySelector("[data-settings-nav]");
const settingsFlow = document.querySelector("[data-settings-flow]");
if (settingsNav && settingsFlow) {
  const settingsSearch = document.querySelector("[data-settings-search]");
  const settingsSearchInput = settingsSearch?.querySelector("[data-settings-search-input]");
  const settingsSearchResults = settingsSearch?.querySelector("[data-settings-search-results]");
  const settingsSearchStatus = settingsSearch?.querySelector("[data-settings-search-status]");
  const links = [...settingsNav.querySelectorAll("[data-settings-group]")];
  const sections = [...settingsFlow.querySelectorAll(":scope > [data-settings-group]")];
  const searchSections = [...sections, ...document.querySelectorAll(".settings-shell > .onboarding-settings")];
  const organized = settingsNav.hasAttribute("data-settings-organized");
  const levelLinks = [...document.querySelectorAll("[data-settings-levels] a")];
  const categoryOf = (section) => organized ? section?.dataset.settingsCategory || section?.dataset.settingsGroup : section?.dataset.settingsGroup;
  const aliases = new Map([
    ["mfa require extra sign in protection for every viewer profile", "mfa two factor authenticator security login passkey protected automatically"],
    ["automatic sign-out", "session timeout sessions"],
    ["playback", "direct original compatible transcode autoplay intro recap credits"],
    ["subtitles", "captions closed caption language"],
    ["metadata provider", "tmdb artwork"],
    ["transcoder", "conversion codec hardware hdr"],
    ["video conversion", "transcoder codec hardware hdr"],
    ["trusted https on this network", "certificate ssl tls"],
    ["remote access", "internet vpn wireguard"],
    ["integrations", "oidc saml scim webhook home assistant jellyfin"],
    ["profiles", "users accounts family password"],
    ["move viewing activity", "import migrate plex jellyfin history watched"],
    ["library folders", "media directory"],
    ["library discovery", "scan watch folder"],
    ["playback segment analysis", "markers"],
    ["api keys", "tokens developer"],
    ["software updates", "update release"],
    ["automatic maintenance", "backups recovery cache"],
    ["language", "appearance theme"],
  ]);
  const normalize = (value) => value.normalize("NFKD").replace(/[^\p{L}\p{N}]+/gu, " ").toLowerCase().trim();
  const groupLabel = (group) => links.find((link) => link.dataset.settingsGroup === group)?.textContent.trim() || group;
  const searchableText = (section) => {
    const heading = normalize(section.querySelector("h2,h3")?.textContent || "");
    return normalize([section.textContent, groupLabel(categoryOf(section)), aliases.get(heading)].join(" "));
  };
  const groupForHash = () => {
    const target = document.getElementById(location.hash.slice(1));
    return categoryOf(target?.closest("[data-settings-category], [data-settings-group]")) || links.find((link) => link.hash === location.hash)?.dataset.settingsGroup || links[0]?.dataset.settingsGroup || "general";
  };
  const selectSettingsGroup = (group) => {
    const level = links.find((link) => link.dataset.settingsGroup === group)?.dataset.settingsLevel;
    for (const link of levelLinks) {
      if (link.dataset.settingsLevel === level) link.setAttribute("aria-current", "page");
      else link.removeAttribute("aria-current");
    }
    if (organized) {
      for (const section of searchSections) {
        section.hidden = section.getAttribute("role") !== "alert" && !!categoryOf(section) && categoryOf(section) !== group;
      }
    }
    for (const link of links) {
      if (organized) link.hidden = link.dataset.settingsLevel !== level;
      if (link.dataset.settingsGroup === group) link.setAttribute("aria-current", "page");
      else link.removeAttribute("aria-current");
    }
  };
  const searchSettings = () => {
    const query = normalize(settingsSearchInput?.value || "");
    const terms = query ? query.split(/\s+/) : [];
    if (!terms.length) {
      if (settingsSearchResults) {
        settingsSearchResults.hidden = true;
        settingsSearchResults.replaceChildren();
      }
      if (settingsSearchStatus) settingsSearchStatus.textContent = "";
      selectSettingsGroup(groupForHash());
      return;
    }
    const matches = searchSections.filter((section) => {
      const haystack = searchableText(section);
      return terms.every((term) => haystack.includes(term));
    }).sort((first, second) => {
      const firstHeading = normalize(first.querySelector("h2,h3")?.textContent || "");
      const secondHeading = normalize(second.querySelector("h2,h3")?.textContent || "");
      return Number(terms.every((term) => secondHeading.includes(term))) - Number(terms.every((term) => firstHeading.includes(term)));
    });
    settingsSearchResults?.replaceChildren();
    for (const section of matches) {
      const result = document.createElement("a");
      result.href = section.id ? `#${section.id}` : links.find((link) => link.dataset.settingsGroup === categoryOf(section))?.hash || "#";
      result.dataset.settingsSearchResult = "";
      const title = document.createElement("span");
      title.textContent = section.querySelector("h2,h3")?.textContent.trim() || "Setting";
      const group = document.createElement("span");
      group.className = "settings-search-result-group";
      const categoryLink = links.find((link) => link.dataset.settingsGroup === categoryOf(section));
      group.textContent = `${organized && categoryLink?.dataset.settingsLevel === "advanced" ? "Advanced · " : ""}${groupLabel(categoryOf(section))}`;
      result.append(title, group);
      settingsSearchResults?.append(result);
    }
    if (settingsSearchResults) settingsSearchResults.hidden = !matches.length;
    if (settingsSearchStatus) settingsSearchStatus.textContent = matches.length ? `${matches.length} matching settings.` : "No settings match that search.";
  };
  selectSettingsGroup(groupForHash());
  const revealHash = () => {
    selectSettingsGroup(groupForHash());
    if (!organized) return;
    const target = document.getElementById(location.hash.slice(1));
    if (!target) return;
    for (let parent = target.parentElement; parent; parent = parent.parentElement) {
      if (parent.tagName === "DETAILS") parent.open = true;
    }
    target.scrollIntoView({ block: "start" });
  };
  revealHash();
  window.addEventListener("hashchange", revealHash);
  if (organized) settingsSearchResults?.addEventListener("click", (event) => {
    const result = event.target.closest("[data-settings-search-result]");
    if (!result) return;
    const target = document.getElementById(result.hash.slice(1));
    if (!target) return;
    settingsSearchInput.value = "";
    searchSettings();
    selectSettingsGroup(categoryOf(target));
    target.setAttribute("tabindex", "-1");
    target.focus({ preventScroll: true });
  });
  settingsNav.addEventListener("click", (event) => {
    const link = event.target.closest("[data-settings-group]");
    if (!link) return;
    selectSettingsGroup(link.dataset.settingsGroup);
  });
  settingsSearchInput?.addEventListener("input", searchSettings);
  settingsSearchInput?.addEventListener("search", searchSettings);
  settingsSearchInput?.addEventListener("keydown", (event) => {
    if (event.key !== "Escape" || !settingsSearchInput.value) return;
    settingsSearchInput.value = "";
    searchSettings();
    event.preventDefault();
  });
}

const focusTargets = () => [...document.querySelectorAll("#library a.card, .home-shelf .card > a, .destination-card, .letter-jump a")].filter((target) => target.getClientRects().length);
function moveLibraryFocus(event) {
  const current = event.target.closest?.("#library a.card, .home-shelf .card > a, .destination-card, .letter-jump a");
  if (!current || !["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown"].includes(event.key)) return false;
  const from = current.getBoundingClientRect();
  const horizontal = event.key === "ArrowLeft" || event.key === "ArrowRight";
  const direction = event.key === "ArrowLeft" || event.key === "ArrowUp" ? -1 : 1;
  const candidates = focusTargets().filter((target) => {
    if (target === current) return false;
    const rect = target.getBoundingClientRect();
    return direction * (horizontal ? rect.left - from.left : rect.top - from.top) > 1;
  });
  candidates.sort((left, right) => {
    const score = (target) => {
      const rect = target.getBoundingClientRect();
      const primary = Math.abs(horizontal ? rect.left - from.left : rect.top - from.top);
      const secondary = Math.abs(horizontal ? rect.top - from.top : rect.left - from.left);
      return primary + secondary * 2;
    };
    return score(left) - score(right);
  });
  if (!candidates[0]) return false;
  candidates[0].focus({ preventScroll: true });
  candidates[0].scrollIntoView({ block: "nearest", inline: "nearest" });
  event.preventDefault();
  return true;
}

window.addEventListener("keydown", (event) => {
  if (commandMenu?.open || moveLibraryFocus(event) || editable(event.target)) return;
  if (commandMenu && (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
    event.preventDefault();
    openCommands();
  } else if (event.key === "/") {
    const search = document.querySelector("#library-search, #settings-search-input");
    if (!search) return;
    event.preventDefault();
    search?.focus();
    search?.select();
  } else if (commandMenu && event.key === "?") {
    event.preventDefault();
    openCommands();
  }
});
