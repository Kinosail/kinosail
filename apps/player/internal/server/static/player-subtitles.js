// Fetch captions outside the media loader: a pending default track blocks Safari playback.
const subtitleElements = [...player.querySelectorAll("track[data-subtitle-source]")];
const subtitleStatus = document.querySelector("[data-subtitle-status]");
const subtitleRetry = document.querySelector("[data-subtitle-retry]");
const subtitleSelector = document.querySelector("[data-subtitles]");
const subtitleLoads = new Map();
let subtitleAbort = new AbortController();
let subtitleChoiceChanged = false;
const selectedSubtitle = () => subtitleElements.find((element) => element.track.mode === "showing");
const updateSubtitleStatus = () => {
  const load = subtitleLoads.get(selectedSubtitle());
  const failed = load?.state === "failed";
  if (subtitleStatus) {
    subtitleStatus.textContent = load?.state === "loading" ? "Loading subtitles…" : failed ? "Subtitles unavailable. Retry or choose another track." : "";
    subtitleStatus.hidden = !subtitleStatus.textContent;
    delete subtitleStatus.dataset.failure;
    delete subtitleStatus.dataset.requestId;
    if (failed) {
      subtitleStatus.dataset.failure = load.failure;
      if (load.requestID) subtitleStatus.dataset.requestId = load.requestID;
    }
  }
  if (subtitleRetry) {
    if (!failed && document.activeElement === subtitleRetry) subtitleSelector?.focus();
    subtitleRetry.hidden = !failed;
  }
};
const loadSubtitle = async (element) => {
  if (subtitleLoads.has(element) || subtitleAbort.signal.aborted) return;
  const lifecycleSignal = subtitleAbort.signal;
  const controller = new AbortController();
  const load = {state: "loading", url: "", failure: "", requestID: ""};
  let reader;
  let timer;
  const active = () => subtitleLoads.get(element) === load && load.state === "loading";
  const cleanup = () => {
    clearTimeout(timer);
    element.removeEventListener("load", ready);
    element.removeEventListener("error", decodeFailed);
    lifecycleSignal.removeEventListener("abort", load.cancel);
  };
  const cancel = () => {
    cleanup();
    controller.abort();
    void reader?.cancel().catch(() => {});
    if (load.url) {
      if (element.getAttribute("src") === load.url) element.removeAttribute("src");
      URL.revokeObjectURL(load.url);
      load.url = "";
    }
  };
  const fail = (failure) => {
    if (!active()) return;
    load.state = "failed";
    load.failure = failure;
    cancel();
    updateSubtitleStatus();
  };
  const ready = () => {
    if (!active()) return;
    load.state = "ready";
    cleanup();
    updateSubtitleStatus();
  };
  const decodeFailed = () => fail("decode");
  load.cancel = () => {
    load.state = "cancelled";
    cancel();
  };
  subtitleLoads.set(element, load);
  lifecycleSignal.addEventListener("abort", load.cancel, {once: true});
  timer = setTimeout(() => fail("timeout"), 20_000);
  updateSubtitleStatus();
  try {
    const source = new URL(element.dataset.subtitleSource, location.href);
    if (source.origin !== location.origin) return fail("origin");
    const response = await fetch(source, {signal: controller.signal, redirect: "error"});
    if (!active()) return;
    const requestID = response.headers.get("x-request-id") ?? "";
    if (/^[a-zA-Z0-9_-]{1,64}$/.test(requestID)) load.requestID = requestID;
    if (!response.ok) return fail("http");
    if (response.headers.get("content-type")?.split(";")[0].trim().toLowerCase() !== "text/vtt") return fail("type");
    if (!response.body) return fail("empty");
    reader = response.body.getReader();
    const chunks = [];
    let size = 0;
    while (true) {
      const {done, value} = await reader.read();
      if (!active()) return;
      if (done) break;
      size += value.byteLength;
      if (size > 16 * 1024 * 1024) return fail("oversized");
      chunks.push(value);
    }
    if (!size) return fail("empty");
    load.url = URL.createObjectURL(new Blob(chunks, {type: "text/vtt"}));
    element.addEventListener("load", ready);
    element.addEventListener("error", decodeFailed);
    element.src = load.url;
  } catch (_) {
    fail("network");
  }
};
const loadSelectedSubtitles = () => {
  subtitleElements.filter((element) => element.track.mode === "showing").forEach((element) => { void loadSubtitle(element); });
  updateSubtitleStatus();
};
player.textTracks?.addEventListener?.("change", loadSelectedSubtitles);
subtitleSelector?.addEventListener("change", () => {
  subtitleChoiceChanged = true;
  for (const [element, load] of subtitleLoads) {
    if (load.state === "failed") {
      load.cancel();
      subtitleLoads.delete(element);
    }
  }
  queueMicrotask(loadSelectedSubtitles);
});
subtitleRetry?.addEventListener("click", () => {
  const element = selectedSubtitle();
  const load = subtitleLoads.get(element);
  if (load?.state !== "failed") return;
  load.cancel();
  subtitleLoads.delete(element);
  void loadSubtitle(element);
});
const loadDefaultSubtitles = () => {
  if (subtitleChoiceChanged || subtitleAbort.signal.aborted) return;
  subtitleElements.filter((element) => element.default).forEach((element) => {
    element.track.mode = "showing";
    void loadSubtitle(element);
  });
};
loadDefaultSubtitles();
if (player.readyState === 0) player.addEventListener("loadedmetadata", loadDefaultSubtitles, {once: true});
addEventListener("pagehide", () => {
  if (document.pictureInPictureElement === player || player.webkitPresentationMode === "picture-in-picture") return;
  subtitleAbort.abort();
  for (const load of subtitleLoads.values()) load.cancel();
  updateSubtitleStatus();
});
addEventListener("pageshow", (event) => {
  if (!event.persisted || !subtitleAbort.signal.aborted) return;
  subtitleAbort = new AbortController();
  subtitleLoads.clear();
  loadSelectedSubtitles();
});
