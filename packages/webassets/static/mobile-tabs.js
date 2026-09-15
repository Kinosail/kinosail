/* Personal mobile destinations; More always preserves access to the full library. */
const mobileTabCatalog = [
  ["all", "Home"], ["shows", "TV Shows"], ["movies", "Movies"], ["search", "Search"], ["list", "My List"],
  ["music", "Music"], ["audiobooks", "Audiobooks"], ["books", "Books"], ["photos", "Photos"],
  ["collections", "Collections"], ["playlists", "Playlists"], ["unwatched", "Unwatched"], ["history", "History"],
];
const mobileTabDefaults = ["all", "shows", "movies", "search"];
function parseMobileTabs(raw) {
  if (typeof raw !== "string" || raw.length > 256) throw new Error("Invalid tabs");
  const items = JSON.parse(raw);
  if (!Array.isArray(items) || items.length < 1 || items.length > 4 || new Set(items).size !== items.length ||
      items.some(id => typeof id !== "string" || !mobileTabCatalog.some(item => item[0] === id))) throw new Error("Invalid tabs");
  return items;
}
function restoreMobileTabs(raw, legacy) {
  if (raw !== null) return parseMobileTabs(raw);
  if (legacy !== null) {
    const items = parseMobileTabs(legacy);
    if (JSON.stringify(items) !== '["movies","shows"]') return items;
  }
  return [...mobileTabDefaults];
}
(() => {
  const nav = document.querySelector("[data-mobile-tabs]");
  if (!nav) return;
  const profile = nav.dataset.navProfile;
  if (!profile || profile.length > 128 || !/^[a-zA-Z0-9_-]+$/.test(profile)) return;
  const key = `kinosail:tabs:v2:${profile}`;
  let pinned = [...mobileTabDefaults];
  try { pinned = restoreMobileTabs(localStorage.getItem(key), localStorage.getItem(`kinosail:tabs:${profile}`)); } catch (_) {}
  const more = nav.querySelector(".nav-more");
  const menu = more?.querySelector(".nav-more-menu");
  if (!menu) return;
  const group = document.createElement("section");
  group.className = "nav-personal-more nav-more-section";
  const customize = document.createElement("button");
  customize.type = "button"; customize.textContent = "Customize tabs";
  const dialog = document.createElement("dialog");
  dialog.className = "tab-editor"; dialog.setAttribute("aria-labelledby", "tab-editor-title");
  const heading = document.createElement("h2"); heading.id = "tab-editor-title"; heading.textContent = "Customize tabs";
  const copy = document.createElement("p"); copy.textContent = "Choose up to four tabs for this Viewer Profile in this browser. Find the remaining sections in More.";
  const choices = document.createElement("div"); choices.className = "tab-editor-choices";
  const status = document.createElement("p"); status.setAttribute("role", "status");
  const done = document.createElement("button"); done.type = "button"; done.textContent = "Done";
  done.addEventListener("click", () => dialog.close());
  const reset = document.createElement("button"); reset.type = "button"; reset.className = "quiet"; reset.textContent = "Reset to default tabs";
  reset.addEventListener("click", () => save([...mobileTabDefaults]));
  const actions = document.createElement("footer"); actions.append(reset, done);
  dialog.append(heading, copy, choices, status, actions); document.body.append(dialog);
  let returnFocus;
  for (const action of [customize, ...document.querySelectorAll("[data-customize-tabs]")]) {
    action.addEventListener("click", () => {
      returnFocus = action === customize ? more.querySelector("summary") : action;
      more.open = false; renderChoices(); dialog.showModal();
    });
  }
  dialog.addEventListener("close", () => returnFocus?.focus());
  menu.append(group, customize);
  const title = id => mobileTabCatalog.find(item => item[0] === id)[1];
  const currentView = () => location.pathname !== "/" ? null : new URLSearchParams(location.search).get("q")?.trim() ? "search" : new URLSearchParams(location.search).get("view") || "all";
  const link = id => {
    const a = document.createElement("a"); a.href = id === "search" ? "#library-search" : `/?view=${id}`; a.textContent = title(id);
    if (id === "search") a.addEventListener("click", event => {
      const input = document.getElementById("library-search");
      if (!input) return;
      event.preventDefault(); more.open = false; input.focus(); input.select();
    });
    const view = currentView();
    if (location.pathname === "/" && view === id) { a.classList.add("active"); a.setAttribute("aria-current", "page"); }
    return a;
  };
  function renderNavigation() {
    nav.querySelectorAll(":scope > .nav-personal-tab").forEach(item => item.remove());
    for (const id of pinned) { const a = link(id); a.classList.add("nav-personal-tab"); nav.insertBefore(a, more); }
    group.replaceChildren(...mobileTabCatalog.filter(item => !pinned.includes(item[0])).map(item => link(item[0])));
    nav.style.setProperty("--personal-tab-count", String(pinned.length + 1));
    nav.classList.add("has-personal-tabs");
    const view = currentView();
    more.querySelector("summary").classList.toggle("active", !pinned.includes(view));
  }
  function save(items, focusLabel) {
    try {
      const raw = JSON.stringify(items); const next = parseMobileTabs(raw);
      localStorage.setItem(key, raw);
      pinned = next;
      status.textContent = "Tabs saved.";
    } catch (_) { status.textContent = "Couldn’t save tabs in this browser. Your previous saved layout is unchanged."; return; }
    renderNavigation(); renderChoices();
    if (focusLabel) {
      const target = choices.querySelector(`[aria-label="${focusLabel}"]`);
      (target?.disabled ? target.parentElement.querySelector("button:not(:disabled)") : target)?.focus();
    }
  }
  function renderChoices() {
    choices.replaceChildren();
    const order = [...pinned, ...mobileTabCatalog.map(item => item[0]).filter(id => !pinned.includes(id))];
    for (const id of order) {
      const row = document.createElement("div"); row.className = "tab-editor-row";
      const name = document.createElement("span"); name.textContent = title(id); row.append(name);
      const index = pinned.indexOf(id);
      const button = (label, text, action, disabled, iconDirection) => {
        const b = document.createElement("button"); b.type = "button"; b.className = "quiet";
        b.setAttribute("aria-label", label); b.textContent = text; b.disabled = disabled;
        if (iconDirection) {
          const icon = document.createElement("i"); icon.className = `i-back tab-move-${iconDirection}`;
          icon.setAttribute("aria-hidden", "true"); b.append(icon);
        }
        b.addEventListener("click", action); row.append(b);
      };
      if (index >= 0) {
        for (const [offset, direction] of [[-1, "earlier"], [1, "later"]]) {
          const label = `Move ${title(id)} ${direction}`;
          button(label, "", () => { const next = [...pinned]; [next[index], next[index + offset]] = [next[index + offset], next[index]]; save(next, label); }, index + offset < 0 || index + offset >= pinned.length, direction);
        }
        button(`Remove ${title(id)} from tabs`, "Remove", () => save(pinned.filter(item => item !== id), `Add ${title(id)} to tabs`), pinned.length === 1);
      } else {
        button(`Add ${title(id)} to tabs`, "Add", () => save([...pinned, id], `Remove ${title(id)} from tabs`), pinned.length === 4);
      }
      choices.append(row);
    }
  }
  renderNavigation();
  document.addEventListener("htmx:afterSettle", renderNavigation);
})();
