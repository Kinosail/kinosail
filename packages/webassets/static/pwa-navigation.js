const commandMenu = document.querySelector("[data-command-menu]");
const commandSearch = commandMenu?.querySelector("[data-command-search]");
const commandOpens = [...document.querySelectorAll("[data-command-open]")];
const commandShortcuts = commandOpens.map((command) => command.querySelector("[data-command-shortcut]"));
let commandMenuReturnFocus;
const compactMenu = document.querySelector(".header-compact-menu");
const navigationMore = document.querySelector(".nav-more");
const desktopMenu = matchMedia("(min-width: 901px)");
const navigationActionSelector = 'nav[aria-label="Main navigation"] a, .header-compact-panel a, .nav-more-menu a, .command-results a, .header-compact-panel form button, .nav-more-menu form button, .command-results form button';
let navigationPending = false;
const sameDocumentTarget = (link) => {
  const target = new URL(link.href, location.href);
  const current = new URL(location.href);
  return target.origin === current.origin && target.pathname === current.pathname && target.search === current.search;
};
// Let target handlers cancel in-page actions before acquiring the navigation lock.
document.addEventListener("click", (event) => {
  const action = event.target.closest?.(navigationActionSelector);
  if (!action || event.defaultPrevented) return;
  if (action instanceof HTMLAnchorElement && (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || action.target && action.target !== "_self" || action.hasAttribute("download") || sameDocumentTarget(action))) return;
  if (navigationPending) {
    event.preventDefault();
    event.stopPropagation();
    return;
  }
  navigationPending = true;
});
window.addEventListener("pageshow", () => { navigationPending = false; });
document.querySelector('nav[aria-label="Main navigation"]')?.addEventListener("click", (event) => {
  const link = event.target.closest?.("a[aria-current='page']");
  if (!link) return;
  const target = new URL(link.href, location.href);
  const current = new URL(location.href);
  if (target.pathname !== current.pathname || target.search !== current.search || target.hash !== current.hash) return;
  event.preventDefault();
  event.stopPropagation();
  navigationMore?.removeAttribute("open");
  compactMenu?.removeAttribute("open");
});
const syncCompactMenu = () => compactMenu?.removeAttribute("open");
syncCompactMenu();
desktopMenu.addEventListener("change", syncCompactMenu);
compactMenu?.addEventListener("toggle", () => {
  if (!compactMenu.open) return;
  navigationMore?.removeAttribute("open");
  if (!desktopMenu.matches) animateMotion(compactMenu.querySelector(".header-compact-panel"), "motion-panel-open");
});
navigationMore?.addEventListener("toggle", () => {
  if (!navigationMore.open) return;
  compactMenu?.removeAttribute("open");
  if (!desktopMenu.matches) animateMotion(navigationMore.querySelector(".nav-more-menu"), "motion-panel-open");
});
document.addEventListener("click", (event) => {
  if (!compactMenu?.contains(event.target)) compactMenu?.removeAttribute("open");
  if (!navigationMore?.contains(event.target)) navigationMore?.removeAttribute("open");
});
window.addEventListener("keydown", (event) => {
  if (event.key !== "Escape") return;
  if (compactMenu?.open) { compactMenu.removeAttribute("open"); compactMenu.querySelector("summary")?.focus(); }
  if (navigationMore?.open) { navigationMore.removeAttribute("open"); navigationMore.querySelector("summary")?.focus(); }
});
const editable = (target) => target.closest?.("input, textarea, select, button, [contenteditable]");
const visibleCommands = () => [...commandMenu.querySelectorAll("[data-command]")].filter((command) => !command.hidden);
function filterCommands() {
  const query = commandSearch.value.trim().toLowerCase();
  for (const command of commandMenu.querySelectorAll("[data-command]")) command.hidden = !command.dataset.commandLabel.toLowerCase().includes(query);
}
function openCommands() {
  if (!commandMenu) return;
  commandMenuReturnFocus = document.activeElement;
  navigationMore?.removeAttribute("open");
  if (!commandMenu?.open) commandMenu.showModal();
  animateMotion(commandMenu, "motion-panel-open");
  commandSearch.value = "";
  filterCommands();
  commandSearch.focus();
}
commandMenu?.addEventListener("close", () => commandMenuReturnFocus?.focus?.({ preventScroll: true }));
if (commandShortcuts.length) {
  const platform = navigator.userAgentData?.platform || navigator.platform || navigator.userAgent || "";
  for (const shortcut of commandShortcuts) if (shortcut) shortcut.textContent = /mac|iphone|ipad|ipod/i.test(platform) ? "⌘K" : "Ctrl K";
}
for (const commandOpen of commandOpens) commandOpen.addEventListener("click", openCommands);
commandSearch?.addEventListener("input", filterCommands);
commandMenu?.addEventListener("click", (event) => { if (event.target === commandMenu) commandMenu.close(); });
commandMenu?.addEventListener("keydown", (event) => {
  if (!["ArrowDown", "ArrowUp", "Enter"].includes(event.key)) return;
  const commands = visibleCommands();
  if (!commands.length) return;
  const current = commands.indexOf(document.activeElement);
  if (event.key === "Enter" && document.activeElement === commandSearch) commands[0].click();
  else if (event.key !== "Enter") commands[(current + (event.key === "ArrowDown" ? 1 : -1) + commands.length) % commands.length].focus();
  event.preventDefault();
});

const browseStateKey = `kinosail:browse:${location.pathname}${location.search}`;
document.addEventListener("click", (event) => {
  const link = event.target.closest?.("#library a.card, .home-shelf .card > a, .destination-card");
  if (!link) return;
  try { sessionStorage.setItem(browseStateKey, JSON.stringify({ href: link.getAttribute("href"), scroll: scrollY })); } catch (_) {}
});
window.addEventListener("pageshow", (event) => {
  if (!event.persisted && performance.getEntriesByType("navigation")[0]?.type !== "back_forward") return;
  try {
    const state = JSON.parse(sessionStorage.getItem(browseStateKey));
    const link = state?.href && document.querySelector(`a[href="${CSS.escape(state.href)}"]`);
    if (link) link.focus({ preventScroll: true });
    if (Number.isFinite(state?.scroll)) scrollTo(0, state.scroll);
  } catch (_) {}
});
