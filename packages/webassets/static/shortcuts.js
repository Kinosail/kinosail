(() => {
  const menu = document.querySelector("[data-command-menu]");
  const editable = (target) => target?.closest?.("input, textarea, select, button, [contenteditable]");
  const sameTarget = (link, path) => {
    const target = new URL(link, location.href);
    const expected = new URL(path, location.href);
    return target.pathname === expected.pathname && target.search === expected.search;
  };
  const links = () => [...document.querySelectorAll("a[href]")];
  const findLink = (paths) => paths.map((path) => links().find((link) => sameTarget(link.href, path))).find(Boolean);
  const destinations = [
    ["h", "Home", ["/", "/?view=all"]], ["l", "My List", ["/?view=list"]],
    ["m", "Movies", ["/?view=movies"]], ["s", "Shows", ["/?view=shows"]],
    ["u", "Music", ["/?view=music"]], ["a", "Audiobooks", ["/?view=audiobooks"]],
    ["b", "Books", ["/?view=books"]], ["c", "Collections", ["/?view=collections"]],
    ["f", "Photos", ["/?view=photos"]], ["q", "Playlists", ["/?view=playlists"]],
    ["w", "Unwatched", ["/?view=unwatched"]], ["y", "History", ["/?view=history"]],
    ["d", "Offline downloads", ["/offline-downloads"]], ["t", "Settings", ["/settings"]],
  ].map(([key, label, paths]) => ({ key, label, link: findLink(paths) })).filter(({ link }) => link);
  if (!destinations.some(({ key }) => key === "h") && !document.body.classList.contains("auth")) destinations.unshift({ key: "h", label: "Home", link: { href: "/" } });
  const destinationHref = (destination) => destination.link.href;

  const commandList = menu?.querySelector(".command-results");
  if (commandList) {
    const existing = new Set([...commandList.querySelectorAll("a[href]")].map((link) => new URL(link.href, location.href).pathname + new URL(link.href, location.href).search));
    for (const destination of destinations) {
      const url = new URL(destination.link.href, location.href);
      if (existing.has(url.pathname + url.search)) continue;
      const link = document.createElement("a");
      link.href = destinationHref(destination);
      link.dataset.command = "";
      link.dataset.commandLabel = destination.label;
      link.textContent = destination.label;
      commandList.append(link);
    }
  }

  let shortcutMenu = menu;
  let menuReturnFocus;
  if (!shortcutMenu && !document.body.classList.contains("auth")) {
    shortcutMenu = document.createElement("dialog");
    shortcutMenu.className = "command-menu";
    shortcutMenu.dataset.commandMenu = "";
    shortcutMenu.setAttribute("aria-labelledby", "shortcut-title");
    shortcutMenu.innerHTML = '<header><div><span class="eyebrow">Shortcuts</span><h2 id="shortcut-title">Keyboard shortcuts</h2></div><form method="dialog"><button class="quiet" aria-label="Close">Esc</button></form></header><label class="command-search">Find an action<input data-command-search autocomplete="off" placeholder="Search actions"></label><div class="command-results" aria-label="Actions"></div><p><kbd>/</kbd> Search · <kbd>?</kbd> Shortcuts</p>';
    document.body.append(shortcutMenu);
    shortcutMenu.addEventListener("click", (event) => { if (event.target === shortcutMenu) shortcutMenu.close(); });
  }

  const activeMenu = shortcutMenu || menu;
  const search = activeMenu?.querySelector("[data-command-search]");
  if (activeMenu) activeMenu.style.overflowY = "auto";
  for (const input of document.querySelectorAll("#library-search, [data-settings-search-input], input[type=search]")) input.setAttribute("aria-keyshortcuts", "/ Control+F Meta+F");
  const commands = () => [...(activeMenu?.querySelectorAll("[data-command]") || [])];
  const filter = () => {
    const query = search?.value.trim().toLowerCase() || "";
    for (const command of commands()) command.hidden = !command.dataset.commandLabel.toLowerCase().includes(query);
  };
  if (activeMenu && !menu) {
    for (const destination of destinations) {
      const link = document.createElement("a");
      link.href = destinationHref(destination);
      link.dataset.command = "";
      link.dataset.commandLabel = destination.label;
      link.textContent = destination.label;
      activeMenu.querySelector(".command-results").append(link);
    }
    search?.addEventListener("input", filter);
    activeMenu.addEventListener("keydown", (event) => {
      if (!["ArrowDown", "ArrowUp", "Enter"].includes(event.key)) return;
      const visible = commands().filter((command) => !command.hidden);
      if (!visible.length) return;
      const current = visible.indexOf(document.activeElement);
      if (event.key === "Enter" && document.activeElement === search) visible[0].click();
      else if (event.key !== "Enter") visible[(current + (event.key === "ArrowDown" ? 1 : -1) + visible.length) % visible.length].focus();
      event.preventDefault();
    });
  }

  for (const destination of destinations) {
    const command = [...(activeMenu?.querySelectorAll(`[data-command][href]`) || [])].find((link) => sameTarget(link.href, destinationHref(destination)));
    if (!command || command.querySelector("kbd")) continue;
    const label = document.createElement("span");
    label.textContent = command.textContent;
    const hint = document.createElement("kbd");
    hint.textContent = `G ${destination.key.toUpperCase()}`;
    hint.style.marginLeft = "auto";
    command.replaceChildren(label, hint);
  }

  const shortcutHelp = activeMenu && (() => {
    const help = document.createElement("div");
    help.className = "command-results";
    help.dataset.shortcutHelp = "";
    help.setAttribute("aria-label", "Keyboard shortcuts");
    activeMenu.querySelector(".command-results")?.after(help);
    return help;
  })();
  const helpRows = [
    ["Search this page", "/ · ⌘F / Ctrl F"], ["Open command menu", "⌘K / Ctrl K"], ["Show shortcuts", "?"], ["Move through media", "J / K"],
  ];
  if (shortcutHelp) for (const [label, keys] of helpRows) {
    const row = document.createElement("button");
    row.type = "button";
    row.className = "quiet";
    row.tabIndex = -1;
    row.setAttribute("aria-disabled", "true");
    const text = document.createElement("span");
    text.textContent = label;
    const hint = document.createElement("kbd");
    hint.textContent = keys;
    row.append(text, hint);
    shortcutHelp.append(row);
  }
  for (const destination of destinations) {
    destination.link.setAttribute?.("aria-keyshortcuts", `G ${destination.key.toUpperCase()}`);
  }

  const focusTargets = () => [...document.querySelectorAll("#library a.card, .home-shelf .card > a, .destination-card, .episode, .track-list a, .download-job a")].filter((target) => target.getClientRects().length);
  const moveFocus = (key) => {
    const targets = focusTargets();
    if (!targets.length) return false;
    const current = targets.indexOf(document.activeElement);
    if (current < 0) targets[key === "j" ? 0 : targets.length - 1].focus({ preventScroll: true });
    else targets[(current + (key === "j" ? 1 : -1) + targets.length) % targets.length].focus({ preventScroll: true });
    document.activeElement?.scrollIntoView?.({ block: "nearest", inline: "nearest" });
    return true;
  };

  let pendingGo = false;
  let pendingTimer;
  const clearGo = () => { pendingGo = false; clearTimeout(pendingTimer); };
  const openMenu = () => {
    if (!activeMenu) return;
    menuReturnFocus = document.activeElement;
    if (!activeMenu.open) activeMenu.showModal();
    if (search) { search.value = ""; filter(); search.focus(); }
  };
  activeMenu?.addEventListener("close", () => menuReturnFocus?.focus?.({ preventScroll: true }));
  window.addEventListener("keydown", (event) => {
    if (event.defaultPrevented || activeMenu?.open || (editable(event.target) && !activeMenu?.contains(event.target))) return;
    const key = event.key.toLowerCase();
    if (pendingGo) {
      const destination = destinations.find((item) => item.key === key);
      clearGo();
      if (!destination) return;
      event.preventDefault();
      if (destination.link.click) destination.link.click();
      else location.assign(destinationHref(destination));
      return;
    }
    if (event.metaKey || event.ctrlKey || event.altKey) {
      if (event.key.toLowerCase() === "k" && activeMenu) { event.preventDefault(); openMenu(); }
      else if (event.key.toLowerCase() === "f") {
        const input = document.querySelector("#library-search, [data-settings-search-input], input[type=search]");
        if (!input) return;
        event.preventDefault(); input.focus(); input.select();
      }
      return;
    }
    if (event.key === "?") { event.preventDefault(); openMenu(); return; }
    if (event.key === "/") {
      const input = document.querySelector("#library-search, [data-settings-search-input], input[type=search]");
      if (!input) return;
      event.preventDefault(); input.focus(); input.select();
      return;
    }
    if (key === "g") {
      pendingGo = true;
      pendingTimer = setTimeout(clearGo, 1000);
      event.preventDefault();
      return;
    }
    if (["j", "k"].includes(key) && moveFocus(key)) event.preventDefault();
  });
})();
