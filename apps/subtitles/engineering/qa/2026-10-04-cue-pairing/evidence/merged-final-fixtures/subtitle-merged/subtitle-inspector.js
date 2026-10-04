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
(() => {
  "use strict";
  const root = document.querySelector(".subtitle-inspector");
  if (!root) return;
  const form = document.getElementById("subtitle-edit-form");
  const status = document.getElementById("inspector-status");
  const video = document.getElementById("subtitle-preview-video");
  const apply = document.getElementById("apply-subtitle");
  const base = `/api/v1/subtitle-library/${encodeURIComponent(root.dataset.id)}`;
  let draft, draftID = "", draftWordCount = 0, draftPoll, wordObserver, cueObserver;
  let review, prepared, revision = 0, page = 0, busy = false;
  let draftRevision = 0, draftActionRevision = 0, draftFailures = 0, pageActive = true, pendingDraftAction;
  const lockedControls = new Map();
  const tracks = { current: video.addTextTrack("subtitles", "Current"), proposed: video.addTextTrack("subtitles", "Proposed") };
  const element = (tag, text, className) => { const node = document.createElement(tag); if (text !== undefined) node.textContent = text; if (className) node.className = className; return node; };
  const time = (seconds) => `${Math.floor(seconds / 60)}:${(seconds % 60).toFixed(3).padStart(6, "0")}`;
  const sourceCues = window.kinosailSubtitleSourceCues({ element, time, seek: (name, start) => { video.currentTime = Math.max(0, start - 1); document.querySelector(`input[name="preview-track"][value="${name.toLowerCase()}"]`).checked = true; selectTrack(); video.focus(); } });
  async function request(path, input) {
    const options = input === undefined ? {} : { method: "POST", headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": document.querySelector('meta[name="kinosail-csrf"]')?.content || "" }, body: JSON.stringify(input) };
    const response = await fetch(base + path, { credentials: "same-origin", ...options });
    if (response.status === 204) return null;
    const result = await response.json();
    if (!response.ok) { const error = new Error(typeof result.error === "string" ? result.error : "The request could not be completed. Reload and try again."); error.stepUpRequired = result.stepUpRequired; error.status = response.status; throw error; }
    return result;
  }
  function invalidate() { revision++; prepared = undefined; apply.disabled = true; status.removeAttribute("aria-busy"); status.removeAttribute("aria-label"); }
  function setBusy(value, lockInputs = false) {
    if (lockInputs) for (const control of form.querySelectorAll("input, select, textarea, button")) { lockedControls.set(control, control.disabled); control.disabled = true; }
    if (!value) { for (const [control, disabled] of lockedControls) control.disabled = disabled; lockedControls.clear(); }
    busy = value; form.querySelector('button[type="submit"]').disabled = value; apply.disabled = value || !prepared;
    document.getElementById("restore-subtitle").disabled = value;
  }
  function showError(error) { status.removeAttribute("aria-label"); status.textContent = error.message || "The subtitle could not be loaded."; if (error.stepUpRequired) { const link = element("a", " Sign in again, then retry here."); link.href = `/login?next=${encodeURIComponent(location.pathname + location.search)}`; link.target = "_blank"; link.rel = "noopener"; status.append(link); } }
  function renderTrack(name, document) {
    const track = tracks[name];
    for (const cue of Array.from(track.cues || [])) track.removeCue(cue);
    for (const cue of document?.cues || []) track.addCue(new VTTCue(cue.start, cue.end, cue.text));
    track.mode = "hidden";
  }
  function selectTrack() {
    const selected = document.querySelector('input[name="preview-track"]:checked').value;
    for (const [name, track] of Object.entries(tracks)) track.mode = name === selected ? "showing" : "hidden";
  }
  function render() {
    const proposed = review.proposed || review.current;
    const quality = document.getElementById("subtitle-quality"); quality.replaceChildren();
    if (proposed) {
      for (const [name, value] of [["Source", review.source], ["Identity evidence", review.matchEvidence], ["Installed role", review.role === "captions" ? "Declared accessibility captions" : "Dialogue subtitles"], ["Cues", proposed.quality.cueCount], ["Reading above 20 characters/sec", proposed.quality.fastCues], ["Overlapping cues", proposed.quality.overlaps], ["Lines above 42 characters", proposed.quality.longLines], ["Timing evidence", proposed.quality.timing], ["Completeness", proposed.quality.completeness]]) {
        const row = element("p", name); row.append(element("strong", String(value))); quality.append(row);
      }
    } else quality.append(element("p", "No subtitle installed. Import one to inspect its text and timing."));
    const warnings = document.getElementById("subtitle-warnings"); warnings.replaceChildren(...review.warnings.map(text => element("li", text)));
    for (const name of ["current", "proposed"]) renderTrack(name, review[name]);
    const proposedChoice = document.querySelector('input[name="preview-track"][value="proposed"]'); proposedChoice.disabled = !review.proposed;
    if (review.proposed) proposedChoice.checked = true; else document.querySelector('input[name="preview-track"][value="current"]').checked = true;
    selectTrack();
    document.getElementById("export-original").hidden = !review.originalAvailable;
    document.getElementById("restore-subtitle").hidden = !review.restorable;
    for (const link of document.querySelectorAll(".subtitle-inspector-exports a")) { const url = new URL(link.href); url.searchParams.set("language", review.language); link.href = url.href; }
    renderCues();
  }
  function renderCues(append = false) {
    const current = review?.current?.cues || [], proposed = review?.proposed?.cues || [];
    let comparisons = cueComparisons(current, proposed);
    if (document.getElementById("show-flagged").checked) comparisons = comparisons.filter(row => (row.current || []).some(index => current[index]?.warnings?.length) || (row.proposed || []).some(index => proposed[index]?.warnings?.length));
    const pages = Math.max(1, Math.ceil(comparisons.length / 40)); page = Math.min(page, pages - 1);
    const container = document.getElementById("subtitle-cues"); if (!append) { sourceCues.reset(); container.replaceChildren(); }
    for (const comparison of comparisons.slice(page * 40, (page + 1) * 40)) {
      const row = element("div", undefined, "subtitle-cue-row");
      row.append(sourceCues.column("Current", comparison.current || [], current, comparison.kind), sourceCues.column("Proposed", comparison.proposed || [], proposed, comparison.kind));
      container.append(row);
    }
    const progress = document.getElementById("cue-page"), previous = document.getElementById("previous-cues"), next = document.getElementById("next-cues");
    const unit = review.proposed ? "comparisons" : "cues";
    cueObserver?.disconnect();
    if ("IntersectionObserver" in window) {
      previous.hidden = next.hidden = true;
      progress.textContent = `${Math.min((page + 1) * 40, comparisons.length)} of ${comparisons.length} ${unit} shown`;
      if (page < pages - 1) {
        cueObserver = new IntersectionObserver(entries => { if (entries.some(entry => entry.isIntersecting)) { page++; renderCues(true); } }, { rootMargin: "400px 0px" });
        cueObserver.observe(progress);
      }
    } else {
      progress.textContent = `${comparisons.length} ${unit} · page ${page + 1} of ${pages}`;
      previous.hidden = next.hidden = false; previous.disabled = page === 0; next.disabled = page === pages - 1;
    }
  }
  function cueComparisons(current, proposed) {
    if (review.proposed && Array.isArray(review.comparison)) return review.comparison;
    return [...current.map((_, index) => ({ current: [index], kind: review.proposed ? "unpaired-current" : "current" })), ...proposed.map((_, index) => ({ proposed: [index], kind: "unpaired-proposed" }))];
  }
  async function load() {
    const ticket = ++revision; status.setAttribute("aria-label", "Loading subtitle details…"); status.setAttribute("aria-busy", "true"); apply.disabled = true; prepared = undefined; form.querySelector('button[type="submit"]').disabled = true;
    const lockRefresh = !busy;
    const focused = lockRefresh && form.contains?.(document.activeElement) ? document.activeElement : undefined;
    let focusMoved = false;
    const observeFocus = event => { if (event.type === "pointerdown" || event.target !== document.body) focusMoved ||= event.target !== focused; };
    if (focused) { document.addEventListener("focusin", observeFocus, true); document.addEventListener("pointerdown", observeFocus, true); }
    if (lockRefresh) setBusy(true, true);
    let result;
    try { result = await request(`/inspect?language=${encodeURIComponent(form.elements.language.value)}`); }
    catch (error) { if (ticket === revision) throw error; return; }
    finally {
      if (lockRefresh) { setBusy(false); form.querySelector('button[type="submit"]').disabled = true; }
      if (focused) {
        document.removeEventListener("focusin", observeFocus, true); document.removeEventListener("pointerdown", observeFocus, true);
        if (ticket === revision && !focusMoved && document.activeElement === document.body && !focused.disabled) focused.focus({ preventScroll: true });
      }
      if (ticket === revision) { status.removeAttribute("aria-busy"); status.removeAttribute("aria-label"); }
    }
    if (ticket !== revision) return;
    review = result; form.elements.role.value = review.role === "captions" ? "captions" : "translation"; page = 0; render(); form.querySelector('button[type="submit"]').disabled = false; status.textContent = review.current ? "Current subtitle loaded. Preview a change before saving." : "Choose a subtitle file to begin.";
  }
  async function input() {
    const values = { role: form.elements.role.value, language: form.elements.language.value, fingerprint: review.fingerprint, encoding: form.elements.encoding.value, offsetMilliseconds: Math.round(Number(form.elements.offset.value) * 1000), automaticSync: form.elements.automaticSync.checked, removeCredits: form.elements.removeCredits.checked, mergeRepeated: form.elements.mergeRepeated.checked };
    const rows = Array.from(document.querySelectorAll(".subtitle-anchor"));
    if (rows.length) values.anchors = rows.map(row => { const at = Number(row.querySelector('[name="anchor-at"]').value), target = Number(row.querySelector('[name="anchor-target"]').value); return { atMilliseconds: Math.round(at * 1000), offsetMilliseconds: Math.round((target - at) * 1000) }; });
    const file = form.elements.file.files[0];
    if (form.elements.text.value) values.text = form.elements.text.value;
    if (draftID && !file) values.draftId = draftID;
    if (file) {
      if (!file.size || file.size > 4 * 1024 * 1024) throw new Error("Choose a subtitle file no larger than 4 MB.");
      values.data = await new Promise((resolve, reject) => { const reader = new FileReader(); reader.onload = () => resolve(String(reader.result).split(",")[1]); reader.onerror = () => reject(new Error("The selected file could not be read.")); reader.readAsDataURL(file); });
    }
    return values;
  }
  document.getElementById("edit-preview-text").addEventListener("click", () => {
    const source = review?.proposed || review?.current;
    if (!source || busy) return;
    const stamp = seconds => { const milliseconds = Math.round(seconds * 1000); return `${String(Math.floor(milliseconds / 3600000)).padStart(2, "0")}:${String(Math.floor(milliseconds / 60000) % 60).padStart(2, "0")}:${String(Math.floor(milliseconds / 1000) % 60).padStart(2, "0")},${String(milliseconds % 1000).padStart(3, "0")}`; };
    form.elements.text.value = source.cues.map((cue, index) => `${index + 1}\n${stamp(cue.start)} --> ${stamp(cue.end)}\n${cue.text}\n`).join("\n");
    form.elements.offset.value = "0"; form.elements.automaticSync.checked = false; document.getElementById("subtitle-anchors").replaceChildren(); invalidate(); form.elements.text.focus();
  });
  document.getElementById("clear-preview-text").addEventListener("click", () => { form.elements.text.value = ""; invalidate(); });
  form.addEventListener("input", invalidate);
  form.elements.offset.addEventListener("input", () => { if (Number(form.elements.offset.value) !== 0) { form.elements.automaticSync.checked = false; document.getElementById("subtitle-anchors").replaceChildren(); } });
  form.elements.automaticSync.addEventListener("change", () => { if (form.elements.automaticSync.checked) { form.elements.offset.value = "0"; document.getElementById("subtitle-anchors").replaceChildren(); } });
  form.elements.file.addEventListener("change", () => { draftID = ""; form.elements.text.value = ""; form.elements.role.value = "translation"; });
  form.elements.language.addEventListener("change", () => {
    draftActionRevision++; draft = undefined; draftID = ""; draftFailures = 0; form.elements.text.value = ""; wordObserver?.disconnect();
    document.getElementById("draft-status").textContent = "Loading draft details…";
    document.getElementById("start-draft").disabled = true;
    for (const id of ["cancel-draft", "review-draft", "draft-confidence", "more-draft-words"]) document.getElementById(id).hidden = true;
    document.getElementById("draft-words").replaceChildren(); document.getElementById("draft-words-status").textContent = "";
    load().catch(showError); loadDraft().catch(showError);
  });
  form.addEventListener("submit", async event => {
    event.preventDefault(); if (busy || !review || review.language !== form.elements.language.value) return;
    invalidate(); const ticket = revision; setBusy(true); status.textContent = "Preparing your preview. Audio synchronization can take several minutes.";
    try { const values = await input(); const result = await request("/preview", values); if (ticket !== revision) return; review = result; prepared = values; page = 0; render(); status.textContent = "Preview ready. Compare the text and timing, then save when satisfied."; }
    catch (error) { if (ticket === revision) showError(error); } finally { setBusy(false); }
  });
  apply.addEventListener("click", async () => {
    if (busy || !prepared) return; setBusy(true, true); status.textContent = "Saving subtitle and recovery copy…";
    try { review = await request("/apply", prepared); invalidate(); form.elements.file.value = ""; draftID = ""; form.elements.text.value = ""; render(); status.textContent = "Subtitle saved. Your edit is protected from automatic upgrades."; } catch (error) { invalidate(); showError(error); } finally { setBusy(false); }
  });
  document.getElementById("add-anchor").addEventListener("click", () => {
    const container = document.getElementById("subtitle-anchors"); if (container.children.length >= 8) return;
    form.elements.automaticSync.checked = false; form.elements.offset.value = "0";
    const row = element("div", undefined, "subtitle-anchor");
    for (const [name, text] of [["anchor-at", "Subtitle time (seconds)"], ["anchor-target", "Correct video time (seconds)"]]) { const label = element("label", text), field = element("input"); field.name = name; field.type = "number"; field.min = "0"; field.max = "129600"; field.step = ".001"; field.required = true; field.value = video.currentTime.toFixed(3); label.append(field); row.append(label); }
    const remove = element("button", "Remove", "quiet"); remove.type = "button"; remove.addEventListener("click", () => { row.remove(); invalidate(); }); row.append(remove); container.append(row); row.querySelector("input").focus(); invalidate();
  });
  document.querySelectorAll('input[name="preview-track"]').forEach(control => control.addEventListener("change", selectTrack));
  document.getElementById("show-flagged").addEventListener("change", () => { page = 0; renderCues(); });
  document.getElementById("previous-cues").addEventListener("click", () => { page--; renderCues(); });
  document.getElementById("next-cues").addEventListener("click", () => { page++; renderCues(); });
  document.getElementById("restore-subtitle").addEventListener("click", async () => { if (busy || !review || review.language !== form.elements.language.value) return; setBusy(true, true); try { await request("/restore", { language: review.language }); await load(); status.textContent = "Previous subtitle restored."; } catch (error) { showError(error); } finally { setBusy(false); } });
  document.getElementById("analyze-speech").addEventListener("click", async event => {
    const button = event.currentTarget; button.disabled = true; const summary = document.getElementById("speech-summary"); summary.textContent = "Analyzing audio locally. This can take several minutes.";
    try {
      const result = await request("/audio", { language: form.elements.language.value });
      const canvas = document.getElementById("subtitle-waveform"); canvas.hidden = false; canvas.width = Math.max(320, Math.round(canvas.clientWidth * (window.devicePixelRatio || 1))); canvas.height = 100 * (window.devicePixelRatio || 1);
      const context = canvas.getContext("2d"), style = getComputedStyle(root);
      for (const [values, color, middle, height] of [[result.waveform, style.getPropertyValue("--text"), .3, .25], [result.speech, style.getPropertyValue("--signal"), .9, -.3]]) {
        context.strokeStyle = color; context.beginPath(); values.forEach((value, i) => { const x = i / values.length * canvas.width; context.moveTo(x, middle * canvas.height); context.lineTo(x, (middle + value * height) * canvas.height); }); context.stroke();
      }
      summary.textContent = `${time(result.duration)} analyzed. Upper trace: audio amplitude. Lower trace: speech probability. Music and effects can resemble speech.`;
    } catch (error) { summary.textContent = error.message; } finally { button.disabled = false; }
  });
  video.addEventListener("error", () => { document.getElementById("video-help").prepend(document.createTextNode("This video cannot play directly in this browser. ")); }, { once: true });
  async function loadDraft() {
    clearTimeout(draftPoll);
    if (!pageActive) return;
    const ticket = ++draftRevision, language = form.elements.language.value;
    const current = () => pageActive && ticket === draftRevision && language === form.elements.language.value;
    let result;
    try { result = await request(`/draft?language=${encodeURIComponent(language)}`); }
    catch (error) {
      if (!current()) return;
      const retry = !error.status || error.status === 429 || error.status >= 500;
      const delay = Math.min(30000, 5000 * 2 ** Math.min(draftFailures++, 3));
      document.getElementById("draft-status").textContent = retry ? `Draft progress is unavailable. Retrying in ${delay / 1000} seconds…` : error.message;
      if (retry) draftPoll = setTimeout(loadDraft, delay);
      else if (error.stepUpRequired) showError(error);
      return;
    }
    if (!current()) return;
    draftFailures = 0;
    draft = result; draftWordCount = 0; wordObserver?.disconnect();
    document.getElementById("draft-status").textContent = draft.message;
    const running = draft.state === "running" || draft.state === "canceling";
    document.getElementById("start-draft").disabled = running || pendingDraftAction?.language === language;
    document.getElementById("cancel-draft").hidden = !running;
    document.getElementById("review-draft").hidden = draft.state !== "ready";
    document.getElementById("draft-confidence").hidden = draft.state !== "ready";
    document.getElementById("draft-words").replaceChildren(); renderDraftWords();
    if (running) draftPoll = setTimeout(loadDraft, 5000);
  }
  function renderDraftWords() {
    const words = draft?.words || [], container = document.getElementById("draft-words");
    for (const word of words.slice(draftWordCount, draftWordCount + 100)) {
      const button = element("button", `${word.text.trim()} · ${Math.round(word.confidence * 100)}% · ${time(word.start)}`, word.confidence < .8 ? "quiet subtitle-low-confidence" : "quiet");
      button.type = "button"; button.addEventListener("click", () => { video.currentTime = Math.max(0, word.start - .5); video.focus(); }); container.append(button);
    }
    draftWordCount += 100;
    const more = document.getElementById("more-draft-words"), progress = document.getElementById("draft-words-status");
    more.hidden = draftWordCount >= words.length;
    if ("IntersectionObserver" in window) {
      more.style.display = "none"; wordObserver?.disconnect();
      progress.textContent = `${Math.min(draftWordCount, words.length)} of ${words.length} words shown`;
      if (draftWordCount < words.length) {
        wordObserver = new IntersectionObserver(entries => { if (entries.some(entry => entry.isIntersecting)) renderDraftWords(); }, { rootMargin: "400px 0px" });
        wordObserver.observe(progress);
      }
    }
  }
  document.getElementById("more-draft-words").addEventListener("click", renderDraftWords);
  const currentDraftAction = action => pageActive && action.ticket === draftActionRevision;
  async function finishDraftAction(action) {
    if (pendingDraftAction === action) pendingDraftAction = undefined;
    if (pageActive && action.language === form.elements.language.value && !pendingDraftAction) await loadDraft();
  }
  document.getElementById("start-draft").addEventListener("click", async event => {
    const action = { ticket: ++draftActionRevision, language: form.elements.language.value }; pendingDraftAction = action; event.currentTarget.disabled = true;
    try { await request("/draft", { language: action.language, method: document.getElementById("draft-method").value, action: "start" }); if (currentDraftAction(action)) { draftID = ""; invalidate(); } }
    catch (error) { if (currentDraftAction(action)) showError(error); } finally { await finishDraftAction(action); }
  });
  document.getElementById("cancel-draft").addEventListener("click", async () => {
    if (!draft?.id || draft.language !== form.elements.language.value) return;
    const action = { ticket: ++draftActionRevision, language: draft.language }; pendingDraftAction = action;
    try { await request("/draft", { language: action.language, method: draft.method, action: "cancel", draftId: draft.id }); }
    catch (error) { if (currentDraftAction(action)) showError(error); } finally { await finishDraftAction(action); }
  });
  document.getElementById("review-draft").addEventListener("click", () => {
    if (draft?.state !== "ready" || draft.language !== form.elements.language.value || busy) return;
    draftID = draft.id; form.elements.role.value = "translation"; form.elements.text.value = ""; form.elements.file.value = ""; form.elements.encoding.value = "auto"; form.elements.offset.value = "0"; form.elements.automaticSync.checked = false; document.getElementById("subtitle-anchors").replaceChildren(); invalidate(); form.requestSubmit();
  });
  window.addEventListener("pagehide", () => { pageActive = false; draftRevision++; draftActionRevision++; clearTimeout(draftPoll); wordObserver?.disconnect(); });
  window.addEventListener("pageshow", () => { if (!pageActive) { pageActive = true; loadDraft().catch(showError); } });
  load().catch(error => { status.style.minHeight = `${status.getBoundingClientRect().height}px`; showError(error); });
  loadDraft().catch(showError);
})();
