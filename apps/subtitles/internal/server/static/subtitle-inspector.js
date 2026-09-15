(() => {
  "use strict";
  const root = document.querySelector(".subtitle-inspector");
  if (!root) return;
  const form = document.getElementById("subtitle-edit-form");
  const status = document.getElementById("inspector-status");
  const video = document.getElementById("subtitle-preview-video");
  const apply = document.getElementById("apply-subtitle");
  const base = `/api/v1/subtitle-library/${encodeURIComponent(root.dataset.id)}`;
  let draft, draftID = "", draftWordCount = 0, draftPoll;
  let review, prepared, revision = 0, page = 0, busy = false;
  const tracks = { current: video.addTextTrack("subtitles", "Current"), proposed: video.addTextTrack("subtitles", "Proposed") };
  const element = (tag, text, className) => { const node = document.createElement(tag); if (text !== undefined) node.textContent = text; if (className) node.className = className; return node; };
  const time = (seconds) => `${Math.floor(seconds / 60)}:${(seconds % 60).toFixed(3).padStart(6, "0")}`;
  async function request(path, input) {
    const options = input === undefined ? {} : { method: "POST", headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": document.querySelector('meta[name="kinosail-csrf"]')?.content || "" }, body: JSON.stringify(input) };
    const response = await fetch(base + path, { credentials: "same-origin", ...options });
    if (response.status === 204) return null;
    const result = await response.json();
    if (!response.ok) { const error = new Error(typeof result.error === "string" ? result.error : "The request could not be completed. Reload and try again."); error.stepUpRequired = result.stepUpRequired; throw error; }
    return result;
  }
  function invalidate() { revision++; prepared = undefined; apply.disabled = true; }
  function setBusy(value) { busy = value; form.querySelector('button[type="submit"]').disabled = value; apply.disabled = value || !prepared; }
  function showError(error) { status.textContent = error.message || "The subtitle could not be loaded."; if (error.stepUpRequired) { const link = element("a", " Sign in again, then retry here."); link.href = `/login?next=${encodeURIComponent(location.pathname + location.search)}`; link.target = "_blank"; link.rel = "noopener"; status.append(link); } }
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
  function renderCues() {
    const current = review?.current?.cues || [], proposed = review?.proposed?.cues || [];
    let indexes = Array.from({ length: Math.max(current.length, proposed.length) }, (_, index) => index);
    if (document.getElementById("show-flagged").checked) indexes = indexes.filter(i => (proposed[i] || current[i])?.warnings?.length);
    const pages = Math.max(1, Math.ceil(indexes.length / 40)); page = Math.min(page, pages - 1);
    const container = document.getElementById("subtitle-cues"); container.replaceChildren();
    for (const index of indexes.slice(page * 40, (page + 1) * 40)) {
      const row = element("div", undefined, "subtitle-cue-row");
      for (const [name, cue] of [["Current", current[index]], ["Proposed", proposed[index]]]) {
        const column = element("div"); column.append(element("small", `${name} · cue ${index + 1}`));
        if (cue) {
          const seek = element("button", `${time(cue.start)} → ${time(cue.end)}`, "quiet"); seek.type = "button";
          seek.addEventListener("click", () => { video.currentTime = Math.max(0, cue.start - 1); document.querySelector(`input[name="preview-track"][value="${name.toLowerCase()}"]`).checked = true; selectTrack(); video.focus(); });
          column.append(seek, element("p", cue.text), element("small", (cue.warnings || []).join(" · ")));
        } else column.append(element("p", "—"));
        row.append(column);
      }
      container.append(row);
    }
    document.getElementById("cue-page").textContent = `${indexes.length} cues · page ${page + 1} of ${pages}`;
    document.getElementById("previous-cues").disabled = page === 0; document.getElementById("next-cues").disabled = page === pages - 1;
  }
  async function load() {
    const ticket = ++revision; status.classList.add("request-skeleton"); status.setAttribute("aria-busy", "true"); apply.disabled = true; prepared = undefined;
    let result;
    try { result = await request(`/inspect?language=${encodeURIComponent(form.elements.language.value)}`); }
    finally { if (ticket === revision) { status.classList.remove("request-skeleton"); status.removeAttribute("aria-busy"); } }
    if (ticket !== revision) return;
    review = result; form.elements.role.value = review.role === "captions" ? "captions" : "translation"; page = 0; render(); status.textContent = review.current ? "Current subtitle loaded. Preview a change before saving." : "Choose a subtitle file to begin.";
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
  form.elements.file.addEventListener("change", () => { draftID = ""; form.elements.text.value = ""; form.elements.role.value = "translation"; });
  form.elements.language.addEventListener("change", () => { draftID = ""; form.elements.text.value = ""; load().catch(showError); loadDraft().catch(showError); });
  form.addEventListener("submit", async event => {
    event.preventDefault(); if (busy || !review) return;
    invalidate(); const ticket = revision; setBusy(true); status.textContent = "Preparing your preview. Audio synchronization can take several minutes.";
    try { const values = await input(); const result = await request("/preview", values); if (ticket !== revision) return; review = result; prepared = values; page = 0; render(); status.textContent = "Preview ready. Compare the text and timing, then save when satisfied."; }
    catch (error) { showError(error); } finally { setBusy(false); }
  });
  apply.addEventListener("click", async () => {
    if (busy || !prepared) return; setBusy(true); status.textContent = "Saving subtitle and recovery copy…";
    try { review = await request("/apply", prepared); invalidate(); form.elements.file.value = ""; draftID = ""; form.elements.text.value = ""; render(); status.textContent = "Subtitle saved. Your edit is protected from automatic upgrades."; } catch (error) { invalidate(); showError(error); } finally { setBusy(false); }
  });
  document.getElementById("add-anchor").addEventListener("click", () => {
    const container = document.getElementById("subtitle-anchors"); if (container.children.length >= 8) return;
    const row = element("div", undefined, "subtitle-anchor");
    for (const [name, text] of [["anchor-at", "Subtitle time (seconds)"], ["anchor-target", "Correct video time (seconds)"]]) { const label = element("label", text), field = element("input"); field.name = name; field.type = "number"; field.min = "0"; field.max = "129600"; field.step = ".001"; field.required = true; field.value = video.currentTime.toFixed(3); label.append(field); row.append(label); }
    const remove = element("button", "Remove", "quiet"); remove.type = "button"; remove.addEventListener("click", () => { row.remove(); invalidate(); }); row.append(remove); container.append(row); row.querySelector("input").focus(); invalidate();
  });
  document.querySelectorAll('input[name="preview-track"]').forEach(control => control.addEventListener("change", selectTrack));
  document.getElementById("show-flagged").addEventListener("change", () => { page = 0; renderCues(); });
  document.getElementById("previous-cues").addEventListener("click", () => { page--; renderCues(); });
  document.getElementById("next-cues").addEventListener("click", () => { page++; renderCues(); });
  document.getElementById("restore-subtitle").addEventListener("click", async () => { if (busy) return; setBusy(true); try { await request("/restore", { language: review.language }); await load(); status.textContent = "Previous subtitle restored."; } catch (error) { showError(error); } finally { setBusy(false); } });
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
    const language = form.elements.language.value;
    const result = await request(`/draft?language=${encodeURIComponent(language)}`);
    if (language !== form.elements.language.value) return;
    draft = result; draftWordCount = 0;
    document.getElementById("draft-status").textContent = draft.message;
    const running = draft.state === "running" || draft.state === "canceling";
    document.getElementById("start-draft").disabled = running;
    document.getElementById("cancel-draft").hidden = !running;
    document.getElementById("review-draft").hidden = draft.state !== "ready";
    document.getElementById("draft-confidence").hidden = draft.state !== "ready";
    document.getElementById("draft-words").replaceChildren(); renderDraftWords();
    if (running) draftPoll = setTimeout(() => loadDraft().catch(error => { document.getElementById("draft-status").textContent = error.message; }), 5000);
  }
  function renderDraftWords() {
    const words = draft?.words || [], container = document.getElementById("draft-words");
    for (const word of words.slice(draftWordCount, draftWordCount + 100)) {
      const button = element("button", `${word.text.trim()} · ${Math.round(word.confidence * 100)}% · ${time(word.start)}`, word.confidence < .8 ? "quiet subtitle-low-confidence" : "quiet");
      button.type = "button"; button.addEventListener("click", () => { video.currentTime = Math.max(0, word.start - .5); video.focus(); }); container.append(button);
    }
    draftWordCount += 100; document.getElementById("more-draft-words").hidden = draftWordCount >= words.length;
  }
  document.getElementById("more-draft-words").addEventListener("click", renderDraftWords);
  document.getElementById("start-draft").addEventListener("click", async event => {
    const button = event.currentTarget; button.disabled = true;
    try { await request("/draft", { language: form.elements.language.value, method: document.getElementById("draft-method").value, action: "start" }); draftID = ""; invalidate(); await loadDraft(); }
    catch (error) { showError(error); } finally { button.disabled = draft?.state === "running" || draft?.state === "canceling"; }
  });
  document.getElementById("cancel-draft").addEventListener("click", async () => {
    if (!draft?.id) return;
    try { await request("/draft", { language: draft.language, method: draft.method, action: "cancel", draftId: draft.id }); await loadDraft(); } catch (error) { showError(error); }
  });
  document.getElementById("review-draft").addEventListener("click", () => {
    if (draft?.state !== "ready" || busy) return;
    draftID = draft.id; form.elements.role.value = "translation"; form.elements.text.value = ""; form.elements.file.value = ""; form.elements.encoding.value = "auto"; form.elements.offset.value = "0"; form.elements.automaticSync.checked = false; document.getElementById("subtitle-anchors").replaceChildren(); invalidate(); form.requestSubmit();
  });
  window.addEventListener("pagehide", () => clearTimeout(draftPoll));
  load().catch(showError);
  loadDraft().catch(showError);
})();
