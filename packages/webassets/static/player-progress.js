let progressRevision = 0;
const save = (watched) => player.dataset.castActive === "true" ? Promise.resolve() : player.dataset.offline === "true" ? window.KinosailOfflineMedia?.saveProgress(player, watched) : fetch(player.dataset.progress, {
  method: "POST",
  headers: {"Content-Type": "application/x-www-form-urlencoded", ...(csrf ? {"X-Kinosail-CSRF": csrf} : {})},
  body: new URLSearchParams({seconds: watched ? 0 : player.currentTime, session: playbackSession, revision: ++progressRevision, ...(watched ? {watched: true} : {})}),
  keepalive: true,
}).catch(() => undefined);
let audioQueue = [];
let queuedAudio;
const warmAudio = () => {
  if (!audioQueue.length) return;
  queuedAudio = new Audio(audioQueue[0].stream);
  queuedAudio.preload = "auto";
};
const advanceQueue = async () => {
  if (!audioQueue.length) return false;
  const next = audioQueue.shift();
  window.KinosailOfflineMedia?.unbindProgress(player);
  delete player.dataset.offline;
  player.dataset.progress = `/progress/${next.id}`;
  if (player.dataset.castApi) player.dataset.castApi = `/api/v1/items/${next.id}/cast`;
  player.dataset.title = next.title;
  player.dataset.artwork = next.artwork || "";
  player.dataset.start = next.progress?.seconds || 0;
  player.src = next.stream;
  player.load();
  warmAudio();
  await requestPlay("queue-advance");
  return true;
};
if (player.dataset.queue) fetch(player.dataset.queue).then((response) => response.json()).then(({items}) => {
  audioQueue = items.slice(1);
  warmAudio();
}).catch(() => {});
const resumeFromSavedProgress = () => {
  if (player.dataset.offline === "true") return;
  const start = Number(player.dataset.start);
  if (start > 0 && start < player.duration - 10) {
    if (Math.abs(player.currentTime - start) >= 0.1) setPlayerTime(start);
    if (player.hasAttribute("data-autoplay") && playbackTraceMethod !== "native-hls") requestPlay("resume-progress").catch(() => {});
  }
};
if (player.readyState) resumeFromSavedProgress();
else player.addEventListener("loadedmetadata", resumeFromSavedProgress, {once: true});
player.addEventListener("pause", () => save(false));
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "hidden") {
    if (player.readyState >= HTMLMediaElement.HAVE_METADATA && !player.ended) save(false);
    flushPlaybackTrace();
  }
});
player.addEventListener("ended", async () => {
  if (player.dataset.castActive === "true") return;
	const saved = save(true);
	const advanced = player.dataset.queue && advanceQueue();
	await saved;
	if (advanced && await advanced) return;
  if (player.dataset.next) location.assign(player.dataset.next);
});
setInterval(() => { if (!player.paused) save(); }, 10000);
setInterval(() => { if (!player.paused || player.readyState < HTMLMediaElement.HAVE_FUTURE_DATA) playbackTrace("heartbeat", "periodic"); flushPlaybackTrace(); }, 5000);

if (player.dataset.homeAssistant === "true") {
  const storageKey = `kinosail-home-assistant-player:${document.body.dataset.viewerProfile || document.querySelector("[data-nav-profile]")?.dataset.navProfile || ""}`;
  let playerID = playerStorage.get(storageKey);
  if (!playerID) {
    playerID = crypto.randomUUID();
    playerStorage.set(storageKey, playerID);
  }
  const sendHomeAssistantState = async () => {
    const response = await fetch(`/api/v1/home-assistant/players/${encodeURIComponent(playerID)}`, {
      method: "PUT",
      headers: {"Content-Type": "application/json", ...(csrf ? {"X-Kinosail-CSRF": csrf} : {})},
      body: JSON.stringify({
        name: `Kinosail on ${navigator.userAgentData?.platform || navigator.platform || "web"}`,
        state: player.ended ? "idle" : player.paused ? "paused" : player.readyState < 3 ? "buffering" : "playing",
        title: player.dataset.title || "",
        itemId: player.dataset.progress?.split("/").at(-1)?.split("?")[0] || "",
        position: Number.isFinite(player.currentTime) ? player.currentTime : 0,
        duration: Number.isFinite(player.duration) ? player.duration : 0,
        volume: player.volume,
        muted: player.muted,
      }),
    });
    if (!response.ok) return;
    const command = await response.json();
    if (command.command === "play") await requestPlay("home-assistant");
    if (command.command === "pause") requestPause();
    if (command.command === "stop") { requestPause(); setPlayerTime(0); }
    if (command.command === "seek") setPlayerTime(command.position);
    if (command.command === "volume") player.volume = command.volume;
    if (command.command === "mute") player.muted = command.muted;
    if (command.command === "play_media") location.assign(`/watch/${encodeURIComponent(command.itemId)}`);
  };
  sendHomeAssistantState().catch(() => {});
  if ("EventSource" in window) {
    const events = new EventSource("/api/v1/events");
    events.addEventListener("home-assistant.command", ({data}) => {
      try {
        if (JSON.parse(data).resource === `/api/v1/home-assistant/players/${playerID}`) sendHomeAssistantState().catch(() => {});
      } catch (_) {}
    });
    addEventListener("pagehide", () => events.close(), {once: true});
  }
  setInterval(() => sendHomeAssistantState().catch(() => {}), 5000);
}

const audioChoice = document.querySelector('[data-audio-track]');
if (audioChoice) {
  const current = new URL(location.href).searchParams.get('audio');
  if (current !== null && [...audioChoice.options].some(option => option.value === current)) audioChoice.value = current;
  audioChoice.addEventListener('change', async () => {
    const value = audioChoice.value, status = document.querySelector('[data-audio-status]');
    if (!/^(0|[1-9][0-9]{0,4})$/.test(value)) return;
    audioChoice.disabled = true;
    try {
      const response = await save(false);
      if (!response?.ok) throw new Error();
      const target = new URL(location.href);
      target.searchParams.delete('direct');
      target.searchParams.set('compatible', '1');
      target.searchParams.set('audio', value);
      location.assign(target.pathname + target.search);
    } catch (_) {
      audioChoice.disabled = false;
      if (status) status.textContent = 'Could not save your position. Try changing the audio again.';
    }
  });
}
