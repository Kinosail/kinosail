// Fetch captions outside the media loader: a pending default track blocks Safari playback.
const subtitleElements = [...player.querySelectorAll("track[data-subtitle-source]")];
const subtitleStatus = document.querySelector("[data-subtitle-status]");
const subtitleLoads = new Map();
const subtitleAbort = new AbortController();
const updateSubtitleStatus = () => {
  if (!subtitleStatus) return;
  const selected = subtitleElements.find((element) => element.track.mode === "showing");
  const state = subtitleLoads.get(selected)?.state;
  subtitleStatus.textContent = state === "loading" ? "Loading subtitles…" : state === "failed" ? "Subtitles unavailable. Select the track again to retry." : "";
  subtitleStatus.hidden = !subtitleStatus.textContent;
};
const loadSubtitle = async (element) => {
  if (subtitleLoads.has(element) || subtitleAbort.signal.aborted) return;
  const load = {state: "loading", url: ""};
  subtitleLoads.set(element, load);
  updateSubtitleStatus();
  let reader;
  try {
    const source = new URL(element.dataset.subtitleSource, location.href);
    if (source.origin !== location.origin) throw new Error("Invalid subtitle origin");
    const response = await fetch(source, {signal: subtitleAbort.signal, redirect: "error"});
    if (!response.ok || response.headers.get("content-type")?.split(";")[0].trim().toLowerCase() !== "text/vtt") throw new Error("Subtitles unavailable");
    reader = response.body.getReader();
    const chunks = [];
    let size = 0;
    while (true) {
      const {done, value} = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > 16 * 1024 * 1024) throw new Error("Subtitle too large");
      chunks.push(value);
    }
    if (!size || subtitleAbort.signal.aborted) throw new Error("Subtitle unavailable");
    load.url = URL.createObjectURL(new Blob(chunks, {type: "text/vtt"}));
    element.addEventListener("load", () => { load.state = "ready"; updateSubtitleStatus(); }, {once: true});
    element.addEventListener("error", () => { load.state = "failed"; updateSubtitleStatus(); }, {once: true});
    element.src = load.url;
  } catch (_) {
    load.state = "failed";
    await reader?.cancel().catch(() => {});
  }
  updateSubtitleStatus();
};
const loadSelectedSubtitles = () => {
  subtitleElements.filter((element) => element.track.mode === "showing").forEach((element) => { void loadSubtitle(element); });
  updateSubtitleStatus();
};
player.textTracks?.addEventListener?.("change", loadSelectedSubtitles);
document.querySelector("[data-subtitles]")?.addEventListener("change", () => {
  for (const [element, load] of subtitleLoads) {
    if (load.state === "failed") {
      if (load.url) URL.revokeObjectURL(load.url);
      subtitleLoads.delete(element);
    }
  }
  queueMicrotask(loadSelectedSubtitles);
});
subtitleElements.filter((element) => element.default).forEach((element) => {
  element.track.mode = "showing";
  void loadSubtitle(element);
});
addEventListener("pagehide", () => {
  if (document.pictureInPictureElement === player || player.webkitPresentationMode === "picture-in-picture") return;
  subtitleAbort.abort();
  for (const load of subtitleLoads.values()) if (load.url) URL.revokeObjectURL(load.url);
});
