import { get, post, setCSRF } from "./api.js";
import { brandMark, button, clear, element, field, icon } from "./dom.js";
import { createEditor } from "./editor.js";
import { createBoardView } from "./board-view.js";

const RECENTS_KEY = "kinosail-dashboard-recents-v1";
const USAGE_KEY = "kinosail-dashboard-usage-v1";
const VIEW_KEY = "kinosail-dashboard-view-v1";
const viewModes = new Set(["household", "operations", "tv"]);
const storedRecents = readLocal(RECENTS_KEY, []);
const storedUsage = readLocal(USAGE_KEY, {});
const storedView = readLocal(VIEW_KEY, "household");
const state = { board: null, boardKey: "", catalog: [], owner: null, editing: false, filter: "", category: "all", focusAfterRender: null, recentIDs: Array.isArray(storedRecents) ? storedRecents.filter(item => typeof item === "string").slice(0, 12) : [], usage: { launches: Number.isSafeInteger(storedUsage.launches) && storedUsage.launches >= 0 ? storedUsage.launches : 0 }, viewMode: viewModes.has(storedView) ? storedView : "household", offline: false };
let editor;
let boardView;
let toastTimer;

function readLocal(key, fallback) {
  try {
    const value = JSON.parse(localStorage.getItem(key) || "null");
    return value ?? fallback;
  } catch { return fallback; }
}

function writeLocal(key, value) {
  try { localStorage.setItem(key, JSON.stringify(value)); } catch { /* Private browsing may disable local storage. */ }
}

async function start() {
  try {
    const [me, catalog, board] = await Promise.all([get("/api/v1/me"), get("/api/v1/catalog"), get("/api/v1/board")]);
    state.owner = me.owner;
    state.catalog = catalog.apps.map(entry => ({ ...entry, aliases: catalog.aliases?.[entry.id] || [] }));
    writeLocal("kinosail-dashboard-catalog-v1", state.catalog);
    setCSRF(me.csrf);
    editor = createEditor(state, refresh, showToast);
    boardView = createBoardView(state, editor, showToast, refresh, writeLocal);
    bindEvents();
    updateOnlineState();
    applyViewMode();
    updateBoard(board);
    render();
    if (new URLSearchParams(location.search).get("passkey") === "offer") editor.openSettings({ passkeyOffer: true });
    if ("serviceWorker" in navigator) navigator.serviceWorker.register("/service-worker.js").catch(() => {});
    window.setInterval(() => { if (!state.editing && !document.querySelector("dialog[open]")) refresh(true); }, 15000);
  } catch (error) {
    const cachedBoard = readLocal("kinosail-dashboard-board-v1", null);
    if (cachedBoard?.title && cachedBoard?.apps) {
      updateBoard(cachedBoard);
      state.catalog = readLocal("kinosail-dashboard-catalog-v1", []);
      state.offline = true;
      editor = createEditor(state, refresh, showToast);
      boardView = createBoardView(state, editor, showToast, refresh, writeLocal);
      bindEvents();
      applyViewMode();
      render();
      updateOnlineState();
      showToast("Showing the last saved board while offline", "Retry", () => location.reload());
      return;
    }
    field("loading-state").hidden = true;
    field("board").setAttribute("aria-busy", "false");
    field("pulse-copy").textContent = "Service status unavailable";
    field("directory-summary").textContent = "Your board could not load. Use Retry to reconnect.";
    showToast(error.message, "Retry", () => location.reload(), true);
  }
}

async function refresh(silent = false) {
  try {
    const changed = updateBoard(await get("/api/v1/board"));
    state.offline = false;
    updateOnlineState();
    if (changed || state.focusAfterRender) render();
  } catch (error) {
    state.offline = true;
    updateOnlineState();
    if (!silent) showToast(error.message, "Retry", () => refresh(), true);
    if (!silent) throw error;
  }
}

function updateBoard(board) {
  const key = JSON.stringify(board);
  if (key === state.boardKey) return false;
  state.board = board;
  state.boardKey = key;
  writeLocal("kinosail-dashboard-board-v1", board);
  return true;
}

function render() {
  field("board").setAttribute("aria-busy", "false");
  document.querySelectorAll("[data-needs-board]").forEach(control => { control.disabled = false; });
  field("loading-state").hidden = true;
  field("board-title").textContent = state.board.title;
  document.title = `${state.board.title} · Kinosail Dashboard`;
  boardView.renderPulse();
  boardView.renderFilters();
  boardView.renderApps();
  if (field("command-dialog").open) renderCommands();
}

function bindEvents() {
  field("add-button").addEventListener("click", editor.openAdd);
  field("empty-add-button").addEventListener("click", editor.openAdd);
  field("settings-button").addEventListener("click", editor.openSettings);
  field("view-button").addEventListener("click", cycleViewMode);
  field("edit-button").addEventListener("click", toggleEditing);
  field("done-editing-button").addEventListener("click", toggleEditing);
  field("board-search").addEventListener("input", event => { state.filter = event.target.value; boardView.renderApps(); });
  field("clear-search-button").addEventListener("click", () => { field("board-search").value = ""; state.filter = ""; boardView.renderApps(); field("board-search").focus(); });
  field("check-all-button").addEventListener("click", checkAll);
  field("command-button").addEventListener("click", openCommands);
  field("command-search").addEventListener("input", renderCommands);
  field("command-search").addEventListener("keydown", event => {
    const commands = [...field("command-results").querySelectorAll("button")];
    if (event.key === "ArrowDown" && commands.length) { event.preventDefault(); commands[0].focus(); }
    if (event.key === "Enter" && commands.length) { event.preventDefault(); commands[0].click(); }
  });
  field("command-results").addEventListener("keydown", event => {
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
    const commands = [...field("command-results").querySelectorAll("button")];
    const current = commands.indexOf(document.activeElement);
    if (current < 0) return;
    event.preventDefault();
    commands[(current + (event.key === "ArrowDown" ? 1 : -1) + commands.length) % commands.length].focus();
  });
  field("profile-select").addEventListener("change", event => setViewMode(event.target.value));
  field("clear-local-data").addEventListener("click", () => {
    state.recentIDs = [];
    state.usage = { launches: 0 };
    writeLocal(RECENTS_KEY, state.recentIDs);
    writeLocal(USAGE_KEY, state.usage);
    boardView.renderFilters();
    boardView.renderApps();
    showToast("Local recents cleared");
  });
  document.addEventListener("keydown", event => {
    if ((event.metaKey || event.ctrlKey) && event.key.toLocaleLowerCase() === "k") { event.preventDefault(); openCommands(); }
  });
  window.addEventListener("online", updateOnlineState);
  window.addEventListener("offline", updateOnlineState);
}

function updateOnlineState() {
  field("offline-banner").hidden = navigator.onLine && !state.offline;
  field("offline-banner").textContent = navigator.onLine ? "Dashboard connection lost. Showing the last loaded board." : "You are offline. Showing the last loaded board.";
}

function applyViewMode() {
  document.body.dataset.viewMode = state.viewMode;
  const label = state.viewMode === "tv" ? "TV mode" : state.viewMode === "operations" ? "Operations" : "Household";
  field("view-button").querySelector("span").textContent = label;
  field("view-button").setAttribute("aria-pressed", String(state.viewMode === "tv"));
  field("view-button").setAttribute("aria-label", `Switch dashboard view, current ${label}`);
  const viewChoice = [...field("profile-select").querySelectorAll("input")].find(option => option.value === state.viewMode);
  if (viewChoice) viewChoice.checked = true;
}

function setViewMode(value) {
  if (!viewModes.has(value)) return;
  state.viewMode = value;
  writeLocal(VIEW_KEY, value);
  applyViewMode();
  boardView.renderApps();
}

function cycleViewMode() {
  const modes = ["household", "operations", "tv"];
  setViewMode(modes[(modes.indexOf(state.viewMode) + 1) % modes.length]);
}

function toggleEditing() {
  state.editing = !state.editing;
  if (state.editing) { state.filter = ""; state.category = "all"; field("board-search").value = ""; }
  boardView.renderFilters();
  boardView.renderApps();
}

async function checkAll() {
  const control = field("check-all-button");
  control.disabled = true;
  field("pulse-copy").dataset.summaryKey = "";
  field("pulse-copy").textContent = "Checking your services…";
  try {
    const result = await post("/api/v1/board/check");
    if (updateBoard(result.board)) render();
    else boardView.renderPulse();
    showToast(result.check.canceled ? `Checked ${result.check.completed} of ${result.check.requested} services` : "Service checks finished");
  }
  catch (error) { showToast(error.message, "Retry", checkAll, true); }
  finally { control.disabled = false; }
}

function openCommands() {
  field("command-search").value = "";
  renderCommands();
  field("command-dialog").showModal();
  field("command-search").focus();
}

function renderCommands() {
  const results = field("command-results");
  clear(results);
  const query = field("command-search").value.toLocaleLowerCase();
  const commands = [
    { name: "Add application", detail: "Board action", icon: "plus", run: editor.openAdd },
    { name: "Edit board", detail: "Board action", icon: "edit", run: toggleEditing },
    { name: "Check all services", detail: "Network action", icon: "refresh", run: checkAll },
    ...state.board.apps.map(app => ({ name: app.name, detail: boardView.appCategory(app), search: boardView.appSearchText(app), identity: boardView.catalogEntryFor(app), run: () => window.open(app.url, "_blank", "noopener,noreferrer") }))
  ];
  commands.filter(item => !query || boardView.fuzzyMatch(`${item.name} ${item.detail} ${item.search || ""}`.toLocaleLowerCase(), query)).slice(0, 14).forEach(item => {
    const control = button("command-item", "");
    const mark = item.identity ? brandMark(item.name, item.identity.id, item.identity.accent) : element("span", "command-mark");
    if (!item.identity) mark.append(icon(item.icon));
    const copy = element("span", "", item.name);
    control.append(mark, copy, element("small", "", item.detail));
    control.addEventListener("click", () => { field("command-dialog").close(); item.run(); });
    results.append(control);
  });
  if (!results.childElementCount) results.append(element("p", "field-help", "No matching applications or commands. Try another search."));
}

export function showToast(message, actionLabel = "", action = null, isError = false) {
  window.clearTimeout(toastTimer);
  const toast = field("toast");
  toast.dataset.error = String(isError);
  const passive = !actionLabel && !isError;
  toast.dataset.passive = String(passive);
  field("toast-message").textContent = message;
  field("toast-action").hidden = !actionLabel;
  field("toast-action").textContent = actionLabel;
  field("toast-action").onclick = action || null;
  toast.hidden = false;
  toastTimer = window.setTimeout(() => { toast.hidden = true; }, actionLabel ? 9000 : passive ? 1600 : 4200);
}

start();
