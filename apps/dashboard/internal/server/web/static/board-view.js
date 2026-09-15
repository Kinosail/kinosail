import { brandMark, button, clear, element, field, formatAge, healthText, icon } from "./dom.js";
import { enableDrag } from "./drag.js";

export function createBoardView(state, editor, toast, refresh, persist) {
  function catalogEntryFor(app) {
    const name = app.name.toLocaleLowerCase();
    const exact = state.catalog.find(entry => entry.name.toLocaleLowerCase() === name);
    if (exact) return exact;
    let hostname = "";
    try { hostname = new URL(app.url).hostname.toLocaleLowerCase(); } catch { /* Persisted URLs are validated server-side. */ }
    return state.catalog.find(entry => hostname.includes(entry.id) || name.includes(entry.name.toLocaleLowerCase())) || null;
  }

  function appCategory(app) {
    return app.category || catalogEntryFor(app)?.category || "Other";
  }

  function appSearchText(app) {
    const entry = catalogEntryFor(app);
    return [app.name, app.category, app.description, app.url, entry?.id, entry?.name, entry?.description, ...(entry?.aliases || [])].filter(Boolean).join(" ").toLocaleLowerCase();
  }

  function fuzzyMatch(text, query) {
    let position = 0;
    for (const character of query) {
      position = text.indexOf(character, position);
      if (position < 0) return false;
      position++;
    }
    return true;
  }

  function searchScore(app, query) {
    const text = appSearchText(app);
    const terms = query.trim().toLocaleLowerCase().split(/\s+/u).filter(Boolean);
    if (!terms.length) return 0;
    let score = 0;
    for (const term of terms) {
      const index = text.indexOf(term);
      if (index >= 0) score += index === 0 ? 100 : 50;
      else if (term.length >= 4 && fuzzyMatch(text, term)) score += 10;
      else return -1;
    }
    return score;
  }

  function renderFilters() {
    const filters = field("category-filters");
    clear(filters);
    const categories = [...new Set(state.board.apps.map(appCategory))];
    const options = [{ key: "all", label: "All apps" }, { key: "recent", label: "Recent" }, { key: "favorites", label: "Favorites" }, ...categories.map(category => ({ key: category, label: category }))];
    options.forEach(option => {
      const control = button("filter-button", option.label);
      control.setAttribute("aria-pressed", String(state.category === option.key));
      control.disabled = state.editing;
      control.addEventListener("click", () => { state.category = option.key; renderFilters(); renderApps(); });
      filters.append(control);
    });
  }

  function renderPulse() {
    const summary = state.board.summary;
    const copy = field("pulse-copy");
    const summaryKey = JSON.stringify(summary);
    if (copy.dataset.summaryKey === summaryKey) return;
    copy.dataset.summaryKey = summaryKey;
    clear(copy);
    if (summary.total === 0) {
      copy.textContent = "No services configured yet.";
      return;
    }
    const parts = [];
    if (summary.reachable) parts.push([summary.reachable, "reachable", "reachable"]);
    if (summary.slow + summary.degraded) parts.push([summary.slow + summary.degraded, "needing attention", "warning"]);
    if (summary.unavailable) parts.push([summary.unavailable, "unavailable", "unavailable"]);
    if (summary.unchecked) parts.push([summary.unchecked, "not checked", "warning"]);
    if (summary.disabled) parts.push([summary.disabled, "checks off", "warning"]);
    parts.forEach(([count, label, status], index) => {
      if (index) copy.append(document.createTextNode(" · "));
      const value = element("span");
      value.dataset.state = status;
      value.append(element("strong", "", String(count)), document.createTextNode(` ${label}`));
      copy.append(value);
    });
  }

  function renderApps() {
    const grid = field("app-grid");
    const requestedFocus = state.focusAfterRender;
    const focusedID = requestedFocus?.id || document.activeElement?.closest?.(".app-tile")?.dataset.id;
    const focusedRole = requestedFocus?.role || document.activeElement?.dataset.focusRole;
    state.focusAfterRender = null;
    const content = document.createDocumentFragment();
    const query = state.filter.trim().toLocaleLowerCase();
    const category = state.category;
    const eligible = state.board.apps.filter(app => category === "all" || category === "recent" && state.recentIDs.includes(app.id) || category === "favorites" && app.favorite || appCategory(app) === category);
    const termCount = query ? query.split(/\s+/u).filter(Boolean).length : 0;
    let apps = query ? eligible.filter(app => searchScore(app, query) >= termCount * 50) : eligible;
    if (query && !apps.length && termCount === 1) apps = eligible.filter(app => searchScore(app, query) >= 0);
    if (category === "recent") apps.sort((left, right) => state.recentIDs.indexOf(left.id) - state.recentIDs.indexOf(right.id));
    apps.forEach(app => content.append(renderApp(app, state.editing ? state.board.apps.indexOf(app) : 0)));
    grid.replaceChildren(content);
    field("empty-state").hidden = state.board.apps.length !== 0;
    field("no-results").hidden = (query === "" && category === "all") || apps.length !== 0 || state.board.apps.length === 0;
    field("no-results-title").textContent = category === "favorites" ? "No favorites yet" : category === "recent" ? "No recent launches" : "No matching applications";
    field("no-results-copy").textContent = category === "favorites" ? "Edit an application and select Mark as a favorite to add it here." : category === "recent" ? "Open an application and it will appear here." : "Try another name, category, alias, or address.";
    field("search-count").textContent = query ? `${apps.length} of ${state.board.apps.length}` : `Applications: ${apps.length}`;
    field("directory-summary").textContent = `${apps.length} ${apps.length === 1 ? "application" : "applications"}${category === "all" ? "" : ` · ${category === "favorites" ? "favorites" : category}`}`;
    field("edit-bar").hidden = !state.editing;
    field("edit-button").setAttribute("aria-pressed", String(state.editing));
    field("edit-button").querySelector("span").textContent = state.editing ? "Stop editing" : "Edit board";
    field("board-search").disabled = state.editing;
    field("usage-summary").textContent = `Applications opened: ${state.usage.launches || 0}. This history is saved only in this browser.`;
    restoreFocus(grid, focusedID, focusedRole);
  }

  function restoreFocus(grid, focusedID, focusedRole) {
    if (!focusedID) return;
    const tile = grid.querySelector(`[data-id="${focusedID}"]`);
    let target = focusedRole ? tile?.querySelector(`[data-focus-role="${focusedRole}"]`) : tile?.querySelector(state.editing ? ".tile-edit-button" : ".app-link");
    if (target?.disabled && focusedRole?.startsWith("move-")) {
      const fallbackRole = focusedRole === "move-earlier" ? "move-later" : "move-earlier";
      target = tile?.querySelector(`[data-focus-role="${fallbackRole}"]`);
    }
    if (target && !target.disabled) target.focus({ preventScroll: true });
  }

  function renderApp(app, index) {
    const tile = element("article", "app-tile");
    tile.dataset.id = app.id;
    tile.dataset.accent = app.accent;
    tile.dataset.health = app.checkEnabled ? app.health.state : "disabled";
    if (state.editing) tile.classList.add("is-editing");
    const link = element("a", "app-link");
    link.href = app.url;
    link.target = "_blank";
    link.rel = "noreferrer";
    link.setAttribute("aria-label", `Open ${app.name} in a new tab`);
    const identity = catalogEntryFor(app);
    const health = renderHealth(app);
    link.append(brandMark(app.name, identity?.id, app.accent), element("span", "app-name", app.name), element("span", "app-meta", app.description || app.category || new URL(app.url).host), element("span", "app-open"), health);
    link.querySelector(".app-open").append(icon("external"));
    link.addEventListener("click", () => rememberLaunch(app.id));
    tile.append(link);
    if (state.editing) {
      link.tabIndex = -1;
      link.addEventListener("click", event => event.preventDefault());
      tile.append(renderEditControls(app, index, tile));
    }
    return tile;
  }

  function renderHealth(app) {
    const health = element("span", "health-line");
    const visibleHealth = app.checkEnabled ? app.health : { state: "disabled" };
    health.append(element("span", "health-dot"), document.createTextNode(healthText(visibleHealth)));
    const checked = app.health.checkedAt && new Date(app.health.checkedAt).getUTCFullYear() >= 2020;
    let detailCopy = "Waiting for the first check";
    if (!app.checkEnabled) detailCopy = "Turn on checks in Edit";
    else if (checked && app.health.state === "unavailable") detailCopy = `${app.health.explanation || "Service could not be reached"} · ${formatAge(app.health.checkedAt)}`;
    else if (checked && app.health.state === "unchecked") detailCopy = `${app.health.explanation || "This status is out of date"} · ${formatAge(app.health.checkedAt)}`;
    else if (checked) detailCopy = `${app.health.latencyMs || 0} ms · ${formatAge(app.health.checkedAt)}`;
    const detail = element("span", "health-detail", detailCopy);
    health.title = `${healthText(visibleHealth)}: ${detailCopy}`;
    health.append(detail);
    if (app.favorite) {
      const favorite = element("span", "favorite-mark");
      favorite.title = "Favorite";
      favorite.append(icon("star"));
      health.append(favorite);
    }
    return health;
  }

  function renderEditControls(app, index, tile) {
    const controls = element("div", "edit-controls");
    const handle = button("edit-handle", "Drag", "grip");
    handle.setAttribute("aria-label", `Drag ${app.name} to reorder`);
    handle.dataset.focusRole = "drag";
    const actions = element("div", "tile-actions");
    const moves = element("div", "move-actions");
    const earlier = button("", "", "up");
    earlier.setAttribute("aria-label", `Move ${app.name} earlier`);
    earlier.dataset.focusRole = "move-earlier";
    earlier.disabled = index === 0;
    earlier.addEventListener("click", () => moveWithRetry(app.id, -1));
    const later = button("", "", "down");
    later.setAttribute("aria-label", `Move ${app.name} later`);
    later.dataset.focusRole = "move-later";
    later.disabled = index === state.board.apps.length - 1;
    later.addEventListener("click", () => moveWithRetry(app.id, 1));
    moves.append(earlier, later);
    const edit = button("tile-edit-button", "Edit", "edit");
    edit.dataset.focusRole = "edit";
    edit.addEventListener("click", () => editor.openEdit(app));
    actions.append(moves, edit);
    controls.append(handle, element("strong", "edit-name", app.name), actions);
    enableDrag(handle, tile, () => state.board.version, editor.saveOrder, toast, renderApps, () => refresh());
    return controls;
  }

  function moveWithRetry(id, delta) {
    state.focusAfterRender = { id, role: delta < 0 ? "move-earlier" : "move-later" };
    editor.move(id, delta).catch(error => {
      const action = error.refreshOnly ? ["Refresh", () => refresh()] : ["Retry", () => moveWithRetry(id, delta)];
      toast(error.message, action[0], action[1], true);
    });
  }

  function rememberLaunch(id) {
    state.recentIDs = [id, ...state.recentIDs.filter(item => item !== id)].slice(0, 12);
    state.usage.launches = Math.min(1000000, (state.usage.launches || 0) + 1);
    persist("kinosail-dashboard-recents-v1", state.recentIDs);
    persist("kinosail-dashboard-usage-v1", state.usage);
  }

  return { appCategory, appSearchText, catalogEntryFor, fuzzyMatch, renderApps, renderFilters, renderPulse };
}
