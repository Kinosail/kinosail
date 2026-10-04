(() => {
  "use strict";
  window.kinosailSubtitleSourceCues = ({ element, time, seek: seekCue }) => {
    let expandedSource;
    function cueColumn(name, indexes, cues, kind) {
      const source = { column: element("div"), name, indexes, cues, kind, page: 0, expanded: false };
      renderSourceColumn(source);
      return source.column;
    }
    function renderSourceColumn(source) {
      const { column, name, indexes, cues, kind, expanded } = source;
      column.replaceChildren();
      const identity = indexes.length > 2 ? `${indexes.length} source cues` : indexes.length ? `${indexes.length === 1 ? "cue" : "cues"} ${indexes.map(index => index + 1).join(", ")}` : "no cue";
      column.append(element("small", `${name} · ${identity}`));
      const effect = { removed: "Removed during cleanup", merged: "Merged", "unpaired-current": "Correspondence not verified", "unpaired-proposed": "Correspondence not verified" }[kind];
      if (effect) column.append(element("small", ` · ${effect}`));
      const start = expanded ? source.page * 20 : 0, end = expanded ? start + 20 : 2;
      for (const index of indexes.slice(start, end)) {
        const cue = cues[index]; if (cue) appendSourceCue(column, name, index, cue);
      }
      if (!indexes.length) column.append(element("p", "—"));
      if (indexes.length <= 2) return;
      const toggle = sourceButton(expanded ? `Close ${name} source cues` : `Review ${indexes.length} ${name} source cues`, () => {
        if (!expanded && expandedSource && expandedSource !== source) {
          expandedSource.expanded = false;
          if (expandedSource.column.isConnected) renderSourceColumn(expandedSource);
        }
        source.expanded = !expanded; expandedSource = source.expanded ? source : undefined;
        renderSourceColumn(source);
        column.querySelector(source.expanded ? "input" : "button[data-source-toggle]").focus({ preventScroll: true });
      });
      toggle.dataset.sourceToggle = ""; toggle.setAttribute("aria-expanded", String(expanded)); column.append(toggle);
      if (expanded) appendSourcePages(source);
    }
    function appendSourceCue(column, name, index, cue) {
      const seek = sourceButton(`${time(cue.start)} → ${time(cue.end)}`, () => seekCue(name, cue.start));
      seek.setAttribute("aria-label", `Seek ${name} cue ${index + 1}: ${time(cue.start)} to ${time(cue.end)}`);
      column.append(seek, element("p", cue.text), element("small", (cue.warnings || []).join(" · ")));
    }
    function sourceButton(text, action) {
      const button = element("button", text, "quiet"); button.type = "button"; button.addEventListener("click", action); return button;
    }
    function appendSourcePages(source) {
      const pages = Math.ceil(source.indexes.length / 20), controls = element("div", undefined, "subtitle-cue-pagination");
      const label = element("label", `${source.name} source page`), input = element("input");
      input.type = "number"; input.min = "1"; input.max = String(pages); input.value = String(source.page + 1);
      input.setAttribute("aria-label", `${source.name} source page`); label.append(input, element("span", `of ${pages}`));
      const changePage = value => {
        if (Number.isInteger(value)) source.page = Math.max(0, Math.min(pages - 1, value - 1));
        renderSourceColumn(source); source.column.querySelector("input").focus({ preventScroll: true });
      };
      input.addEventListener("change", () => changePage(Number(input.value)));
      input.addEventListener("keydown", event => { if (event.key === "Enter") { event.preventDefault(); changePage(Number(input.value)); } });
      const previous = sourceButton(`Previous ${source.name} source page`, () => changePage(source.page));
      const next = sourceButton(`Next ${source.name} source page`, () => changePage(source.page + 2));
      previous.disabled = source.page === 0; next.disabled = source.page === pages - 1;
      controls.append(label, previous, next); source.column.append(controls);
    }
    return { column: cueColumn, reset() { expandedSource = undefined; } };
  };
})();
