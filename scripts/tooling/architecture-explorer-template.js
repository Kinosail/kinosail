    (() => {
      const root = document.getElementById("atlas");
      const graph = document.getElementById("graph");
      const table = document.getElementById("table-view");
      const journey = document.getElementById("journey-view");
      const search = document.getElementById("search");
      const list = document.getElementById("package-list");
      const missionList = document.getElementById("mission-list");
      const missionProgress = document.getElementById("mission-progress");
      const missionContinue = document.getElementById("mission-continue");
      const status = document.getElementById("status-line");
      const packages = snapshot.packages;
      const byId = new Map(packages.map((item) => [item.id, item]));
      let selectedId = packages.find((item) => item.name === "internal/server")?.id || packages[0]?.id;
      let activeView = "map";
      let query = "";
      let activeMissionId = "";
      let missionStep = 0;
      let zoomGroup;
      let simulation;

      const missions = [
        { id: "front-door", number: "01", title: "Find the front door", subtitle: "Start at the executable boundary", description: "Trace how the command package enters the server through the shared transport boundary.", steps: [{ target: "internal/server", label: "Select the server", detail: "The HTTP API and web UI live here." }, { target: "cmd/kinosail", label: "Step into the executable", detail: "The command package wires the server lifecycle." }, { target: "packages/servertransport", label: "Inspect the transport", detail: "Shared HTTP limits, health, and TLS protect the front door." }] },
        { id: "private-state", number: "02", title: "Trace private state", subtitle: "Follow backup and persistence", description: "Follow app-owned SQLite state into the shared recovery archive engine.", steps: [{ target: "internal/server", label: "Start at server state", detail: "The server owns the application boundary." }, { target: "internal/database", label: "Open persistence", detail: "SQLite-backed private state is isolated here." }, { target: "internal/backup", label: "Cross the app adapter", detail: "The app supplies its database and settings validation rules." }, { target: "packages/backup", label: "Reach shared recovery", detail: "The shared package validates, encrypts, and restores the archive." }] },
        { id: "direct-connection", number: "03", title: "Follow a direct connection", subtitle: "Inspect the privacy boundary", description: "Explore the packages that keep remote access direct and owner-paired.", steps: [{ target: "internal/server", label: "Begin at the server", detail: "Transport and identity decisions start at the API boundary." }, { target: "packages/owneraccess", label: "Inspect pairing", detail: "Owner-paired private management connections live here." }, { target: "packages/privatefile", label: "Check private files", detail: "The connection manager protects local state through this package." }] },
        { id: "playback-route", number: "04", title: "Locate playback", subtitle: "Use a symbol-led deep dive", description: "Use search and the structure tree to find playback planning inside the large server package.", steps: [{ target: "internal/server", label: "Open the server", detail: "Search the selected module for playback files.", query: "" }, { target: "internal/server", label: "Find playback_plan.go", detail: "Expand the structure tree and follow the source link.", query: "playback" }] }
      ];

      const packageName = (id) => byId.get(id)?.name || id.split("/").slice(-1)[0];
      const matches = (item) => {
        if (!query) return true;
        const haystack = [item.name, item.responsibility, ...item.files.flatMap((file) => [file.name, ...file.types, ...file.functions])].join(" ").toLowerCase();
        return haystack.includes(query);
      };
      const setStatus = (message, tone = "") => {
        status.textContent = message;
        status.dataset.tone = tone;
      };
      const formatNumber = (number) => new Intl.NumberFormat("en-US").format(number);
      const moduleId = (name) => name.startsWith("packages/") ? `${snapshot.sharedModule}/${name.slice(9)}` : `${snapshot.module}/${name}`;
      const activeMission = () => missions.find((mission) => mission.id === activeMissionId);

      document.getElementById("snapshot-commit").textContent = snapshot.commitShort || snapshot.commit.slice(0, 12);
      document.getElementById("graph-meta").textContent = `${snapshot.summary.packageCount} packages · ${snapshot.summary.dependencyCount} internal edges`;

      function renderMissions() {
        missionList.replaceChildren();
        missions.forEach((mission) => {
          const button = document.createElement("button");
          button.type = "button";
          button.className = "mission-item";
          button.setAttribute("aria-current", mission.id === activeMissionId ? "true" : "false");
          button.innerHTML = `<span class="mission-number">${mission.number}</span><span><span class="mission-title">${mission.title}</span><span class="mission-subtitle">${mission.subtitle}</span></span>`;
          button.addEventListener("click", () => startMission(mission.id));
          missionList.append(button);
        });
        const mission = activeMission();
        if (!mission) {
          missionProgress.innerHTML = "Choose a route to begin.";
          missionContinue.textContent = "Choose a mission";
          missionContinue.disabled = true;
          return;
        }
        const complete = missionStep >= mission.steps.length;
        missionProgress.innerHTML = complete ? `<strong>Complete</strong> · ${mission.title}` : `<strong>Step ${missionStep + 1} of ${mission.steps.length}</strong> · ${mission.steps[missionStep].label}`;
        missionContinue.disabled = complete;
        missionContinue.textContent = complete ? "Mission complete" : `Continue: ${mission.steps[missionStep].label}`;
      }

      function startMission(id) {
        const mission = missions.find((item) => item.id === id);
        if (!mission) return;
        activeMissionId = id;
        missionStep = 0;
        query = mission.steps[0].query ?? "";
        search.value = query;
        activeView = "journey";
        selectPackage(moduleId(mission.steps[0].target));
        renderTable();
        updateView();
        renderMissions();
        setStatus(`${mission.title} started. Follow the route below.`);
      }

      function continueMission() {
        const mission = activeMission();
        if (!mission || missionStep >= mission.steps.length) return;
        selectMissionStep(missionStep + 1);
      }

      function selectMissionStep(index) {
        const mission = activeMission();
        if (!mission || index < 0 || index >= mission.steps.length) return;
        missionStep = index;
        query = mission.steps[index].query ?? "";
        search.value = query;
        selectPackage(moduleId(mission.steps[index].target));
        renderList();
        renderTable();
        activeView = "journey";
        updateView();
        renderMissions();
        setStatus(`${mission.title}: ${mission.steps[index].label}.`);
      }

      function renderJourney() {
        const mission = activeMission();
        if (!mission) {
          journey.innerHTML = '<p class="empty-note">Choose a mission from the deck to reveal a guided route.</p>';
          return;
        }
        journey.innerHTML = `<div class="journey-header"><div><p class="eyebrow">Active mission ${mission.number}</p><h3>${mission.title}</h3><p>${mission.description}</p></div><button class="journey-reset" type="button" id="journey-reset">Return to map</button></div><div class="journey-track">${mission.steps.map((step, index) => `<button class="journey-step${index < missionStep ? " is-complete" : ""}" type="button" aria-current="${index === missionStep && missionStep < mission.steps.length ? "step" : "false"}"><span class="journey-step-number">${String(index + 1).padStart(2, "0")}</span><h4>${step.label}</h4><p>${step.detail}</p><code>${moduleId(step.target)}</code></button>`).join("")}</div>`;
        journey.querySelectorAll(".journey-step").forEach((button, index) => button.addEventListener("click", () => selectMissionStep(index)));
        journey.querySelector("#journey-reset").addEventListener("click", () => { activeView = "map"; updateView(); setStatus("Map restored. The mission remains available in the deck."); });
      }

      function renderList() {
        list.replaceChildren();
        packages.filter(matches).forEach((item) => {
          const button = document.createElement("button");
          button.type = "button";
          button.className = "package-item";
          button.dataset.id = item.id;
          button.setAttribute("aria-current", item.id === selectedId ? "true" : "false");
          button.innerHTML = `<span class="package-item-name">${item.name}</span><span class="package-item-count">${item.fileCount}</span>`;
          button.addEventListener("click", () => selectPackage(item.id));
          list.append(button);
        });
        if (!list.children.length) list.innerHTML = '<span class="empty-note">No matching packages.</span>';
      }

      function renderTable() {
        const rows = packages.filter(matches).map((item) => `<tr><td><button type="button" data-id="${item.id}">${item.name}</button></td><td class="responsibility">${item.responsibility}</td><td>${item.fileCount}</td><td>${formatNumber(item.loc)}</td><td>${item.fanOut}</td><td>${item.testFileCount}</td></tr>`).join("");
        table.innerHTML = `<table class="package-table"><thead><tr><th>Package</th><th>Role</th><th>Files</th><th>Lines</th><th>Out</th><th>Tests</th></tr></thead><tbody>${rows || '<tr><td colspan="6">No matching packages.</td></tr>'}</tbody></table>`;
        table.querySelectorAll("button[data-id]").forEach((button) => button.addEventListener("click", () => selectPackage(button.dataset.id)));
      }

      function updateView() {
        const mapActive = activeView === "map";
        const journeyActive = activeView === "journey";
        graph.style.display = mapActive ? "block" : "none";
        if (!mapActive) graph.parentElement.style.height = "";
        table.classList.toggle("is-visible", activeView === "table");
        journey.classList.toggle("is-visible", journeyActive);
        document.querySelector(".graph-help").style.display = mapActive ? "block" : "none";
        document.querySelectorAll("[data-view]").forEach((button) => button.setAttribute("aria-pressed", String(button.dataset.view === activeView)));
        if (activeView === "table") renderTable();
        if (journeyActive) renderJourney();
        if (mapActive) renderGraph();
      }

      function renderGraph() {
        if (!window.d3) {
          graph.style.display = "none";
          table.classList.add("is-visible");
          renderTable();
          setStatus("The map library did not load. Table view is still available.", "error");
          return;
        }
        if (simulation) simulation.stop();
        const width = Math.max(320, graph.clientWidth || graph.parentElement.clientWidth);
        const compact = width < 560;
        const boxWidth = compact ? 132 : 176;
        const boxHeight = compact ? 50 : 56;
        const boxRadius = compact ? 8 : 9;
        const nodeInset = boxWidth / 2 + 8;
        const visible = packages.filter(matches);
        const sharedCount = visible.filter((item) => item.id.includes("/kinosail/packages/")).length;
        const compactRows = Math.ceil(visible.length / 2);
        const sharedRows = Math.ceil(sharedCount / 5);
        const height = compact ? Math.max(520, 76 + Math.max(0, compactRows - 1) * 74) : Math.max(510, 382 + Math.max(0, sharedRows - 1) * 126);
        graph.style.height = `${height}px`;
        graph.parentElement.style.height = `${height}px`;
        const visibleIds = new Set(visible.map((item) => item.id));
        const visibleEdges = snapshot.edges.map((edge) => ({ source: typeof edge.source === "string" ? edge.source : edge.source.id, target: typeof edge.target === "string" ? edge.target : edge.target.id })).filter((edge) => visibleIds.has(edge.source) && visibleIds.has(edge.target));
        graph.replaceChildren();
        const svg = d3.select(graph).attr("viewBox", `0 0 ${width} ${height}`);
        const defs = svg.append("defs");
        defs.append("marker").attr("id", "arrow").attr("viewBox", "0 -4 8 8").attr("refX", 8).attr("refY", 0).attr("markerWidth", 5).attr("markerHeight", 5).attr("orient", "auto").append("path").attr("d", "M0,-4L8,0L0,4").attr("fill", "currentColor");
        zoomGroup = svg.append("g");
        svg.call(d3.zoom().scaleExtent([.55, 2.3]).on("zoom", (event) => zoomGroup.attr("transform", event.transform)));
        const link = zoomGroup.append("g").attr("aria-hidden", "true").selectAll("line").data(visibleEdges).join("line").attr("class", "link");
        const node = zoomGroup.append("g").selectAll("g").data(visible, (item) => item.id).join("g").attr("class", (item) => `node ${item.kind}`).attr("tabindex", 0).attr("role", "button").attr("aria-label", (item) => `Select ${item.name} package`).on("click", (_, item) => selectPackage(item.id)).on("keydown", (event, item) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); selectPackage(item.id); } }).call(d3.drag().on("start", (event, item) => { if (!event.active) simulation.alphaTarget(.22).restart(); item.fx = item.x; item.fy = item.y; }).on("drag", (event, item) => { item.fx = event.x; item.fy = event.y; }).on("end", (event, item) => { if (!event.active) simulation.alphaTarget(0); item.fx = null; item.fy = null; }));
        node.append("rect").attr("x", -boxWidth / 2).attr("y", -boxHeight / 2).attr("width", boxWidth).attr("height", boxHeight).attr("rx", boxRadius).attr("fill", "var(--surface-2)");
        node.append("circle").attr("cx", -boxWidth / 2 + 14).attr("cy", -12).attr("r", compact ? 3.5 : 4);
        node.append("text").attr("class", "node-name").attr("x", -boxWidth / 2 + 25).attr("y", -9).attr("font-size", compact ? 10 : null).text((item) => item.label.length > (compact ? 13 : 18) ? `${item.label.slice(0, compact ? 12 : 17)}…` : item.label);
        node.append("text").attr("class", "node-meta").attr("x", -boxWidth / 2 + 25).attr("y", 11).attr("font-size", compact ? 9 : null).text((item) => compact ? `${item.fileCount}f · ${formatNumber(item.loc)} loc` : `${item.fileCount} files · ${formatNumber(item.loc)} loc`);
        simulation = d3.forceSimulation(visible).force("link", d3.forceLink(visibleEdges).id((item) => item.id).distance(compact ? 108 : 140).strength(compact ? .12 : .7));
        if (compact) {
          const target = (item) => ({ x: nodeInset + (visible.indexOf(item) % 2) * (width - nodeInset * 2), y: 38 + Math.floor(visible.indexOf(item) / 2) * 74 });
          visible.forEach((item) => { const point = target(item); item.x = point.x; item.y = point.y; });
          simulation.force("link").strength(0);
          simulation.force("x", d3.forceX((item) => target(item).x).strength(1)).force("y", d3.forceY((item) => target(item).y).strength(1)).force("charge", null).force("collide", null);
        } else {
          const commands = visible.filter((item) => item.kind === "command");
          const appPackages = visible.filter((item) => item.kind !== "command" && !item.id.includes("/kinosail/packages/"));
          const sharedPackages = visible.filter((item) => item.id.includes("/kinosail/packages/"));
          const target = (item) => {
            const group = item.kind === "command" ? commands : (item.id.includes("/kinosail/packages/") ? sharedPackages : appPackages);
            const columns = Math.min(5, group.length);
            const index = group.indexOf(item);
            const column = index % columns;
            const row = Math.floor(index / columns);
            const x = columns === 1 ? width / 2 : nodeInset + column * (width - nodeInset * 2) / (columns - 1);
            const y = item.kind === "command" ? 52 : (item.id.includes("/kinosail/packages/") ? 330 + row * 126 : 184 + row * 86);
            return { x, y };
          };
          visible.forEach((item) => { const point = target(item); item.x = point.x; item.y = point.y; });
          simulation.force("link").strength(.04);
          simulation.force("x", d3.forceX((item) => target(item).x).strength(.88)).force("y", d3.forceY((item) => target(item).y).strength(.88)).force("charge", d3.forceManyBody().strength(-45)).force("collide", d3.forceCollide(92));
        }
        simulation.on("tick", () => { visible.forEach((item) => { item.x = Math.max(nodeInset, Math.min(width - nodeInset, item.x)); item.y = Math.max(36, Math.min(height - 36, item.y)); }); link.attr("x1", (edge) => edge.source.x).attr("y1", (edge) => edge.source.y).attr("x2", (edge) => edge.target.x).attr("y2", (edge) => edge.target.y); node.attr("transform", (item) => `translate(${item.x},${item.y})`); });
        applyHighlights(link, node);
      }

      function applyHighlights(link, node) {
        const selected = byId.get(selectedId);
        const neighborIds = new Set([selectedId, ...(selected?.imports || [])]);
        packages.forEach((item) => item.imports.includes(selectedId) && neighborIds.add(item.id));
        link.classed("is-highlighted", (edge) => edge.source.id === selectedId || edge.target.id === selectedId || edge.source === selectedId || edge.target === selectedId).classed("is-dimmed", (edge) => !neighborIds.has(edge.source.id || edge.source) && !neighborIds.has(edge.target.id || edge.target));
        node.classed("is-selected", (item) => item.id === selectedId).classed("is-neighbor", (item) => item.id !== selectedId && neighborIds.has(item.id)).classed("is-dimmed", (item) => !matches(item));
      }

      function renderChips(target, ids, emptyMessage) {
        target.replaceChildren();
        if (!ids.length) { target.innerHTML = `<span class="empty-note">${emptyMessage}</span>`; return; }
        ids.forEach((id) => { const button = document.createElement("button"); button.type = "button"; button.className = "chip link-chip"; button.textContent = packageName(id); button.title = id; button.addEventListener("click", () => selectPackage(id)); target.append(button); });
      }

      function sourceLink(path, line = 1) {
        return `https://github.com/MikeO7/kinosail/blob/main/${path}#L${line}`;
      }

      function renderSymbols(items, label, filePath) {
        if (!items.length) return `<div class="symbol-group"><h5>${label}</h5><p class="empty-note">None detected</p></div>`;
        return `<div class="symbol-group"><h5>${label}</h5><ul>${items.map((item) => `<li><a href="${sourceLink(filePath, item.line)}" target="_blank" rel="noreferrer">${item.name}</a><span class="symbol-line">L${item.line}</span></li>`).join("")}</ul></div>`;
      }

      function renderDetail() {
        const item = byId.get(selectedId);
        if (!item) return;
        document.getElementById("detail-name").textContent = item.name;
        document.getElementById("detail-path").textContent = item.id;
        document.getElementById("detail-copy").textContent = item.responsibility;
        const metrics = [["production files", item.fileCount], ["lines", formatNumber(item.loc)], ["symbols", item.types.length + item.functions.length], ["imports out", item.fanOut], ["imports in", item.fanIn], ["test files", item.testFileCount]];
        document.getElementById("metrics").innerHTML = metrics.map(([label, value]) => `<div class="metric"><dt>${label}</dt><dd>${value}</dd></div>`).join("");
        renderChips(document.getElementById("imports"), item.imports, "No first-party imports.");
        renderChips(document.getElementById("dependents"), packages.filter((candidate) => candidate.imports.includes(item.id)).map((candidate) => candidate.id), "No first-party consumers.");
        const visibleFiles = item.files.filter((file) => !query || [file.name, file.path, ...(file.symbols || []).map((symbol) => symbol.name)].join(" ").toLowerCase().includes(query));
        document.getElementById("file-tree").innerHTML = visibleFiles.length ? visibleFiles.map((file, index) => { const symbols = file.symbols || [...file.types.map((name) => ({ name, kind: "type", line: 1 })), ...file.functions.map((name) => ({ name, kind: "function", line: 1 }))]; return `<details class="file-row"${index === 0 ? " open" : ""}><summary><span class="file-name">${file.name}</span><a class="file-source" href="${sourceLink(file.path)}" target="_blank" rel="noreferrer">source</a><span class="file-meta">${file.loc} loc</span></summary><div class="symbol-groups">${renderSymbols(symbols.filter((symbol) => symbol.kind === "type"), "types", file.path)} ${renderSymbols(symbols.filter((symbol) => symbol.kind === "function"), "functions", file.path)}</div></details>`; }).join("") : '<p class="empty-note">No files or symbols match this search.</p>';
      }

      function selectPackage(id) {
        if (!byId.has(id)) return;
        selectedId = id;
        renderList();
        renderDetail();
        if (activeView === "map" && window.d3) {
          const nodes = d3.select(graph).selectAll(".node");
          const links = d3.select(graph).selectAll(".link");
          applyHighlights(links, nodes);
        }
        setStatus(`${packageName(id)} selected. Imports and consumers are highlighted.`);
      }

      search.addEventListener("input", (event) => { query = event.target.value.trim().toLowerCase(); renderList(); renderTable(); renderDetail(); if (activeView === "map") renderGraph(); setStatus(query ? `Showing packages, files, and symbols matching “${query}”.` : "Showing all packages."); });
      document.querySelectorAll("[data-view]").forEach((button) => button.addEventListener("click", () => { activeView = button.dataset.view; updateView(); }));
      missionContinue.addEventListener("click", continueMission);
      window.addEventListener("resize", () => { if (activeView === "map") renderGraph(); });
      renderMissions();
      renderList();
      renderDetail();
      updateView();
      setStatus("Server is selected. Imports and consumers are highlighted.");
    })();
