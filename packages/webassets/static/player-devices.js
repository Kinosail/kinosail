let managedSeek = false;
const setPlayerTime = (seconds) => { managedSeek = true; player.currentTime = seconds; };
const castButtons = [...document.querySelectorAll("[data-cast]")];
const castState = document.querySelector("[data-cast-state]");
const messages = document.body.dataset;
const roomState = document.querySelector("[data-room-state]");
if (player.dataset.room && "WebSocket" in window) {
  const roomID = player.dataset.room;
  const leader = player.dataset.roomLeader === "true";
  const mediaID = player.dataset.roomMedia;
  const roomChoice = document.querySelector("[data-room-choice]");
  let socket;
  let reconnectTimer;
  let reconnectAttempt = 0;
  let lastRevision = 0;
  let leaving = false;
  let roomHeartbeat;
  const roomStatus = (connected) => {
    if (roomState) roomState.textContent = connected
      ? leader ? messages.roomLeading : messages.roomFollowing
      : messages.roomDisconnected;
  };
  const sendRoomEvent = (action, media = mediaID, seconds = player.currentTime) => {
    if (!leader || socket?.readyState !== WebSocket.OPEN) return;
    socket.send(JSON.stringify({action, media, seconds: Number.isFinite(seconds) ? Math.max(0, seconds) : 0}));
  };
  const observeRoomDrift = (drift) => {
    if (socket?.readyState === WebSocket.OPEN && Number.isFinite(drift) && drift >= 0) socket.send(JSON.stringify({action: "observe", drift}));
  };
  const applyRoomEvent = async (event) => {
    if (!event || !["play", "pause", "seek", "media"].includes(event.action) || typeof event.media !== "string" || event.media.length > 128 || !Number.isFinite(event.seconds) || !Number.isSafeInteger(event.revision) || event.revision <= lastRevision) return;
    const initial = lastRevision === 0;
    lastRevision = event.revision;
    if (leader && !initial && event.action !== "media") return;
    if (event.media !== mediaID) {
      const target = new URL(`/watch/${encodeURIComponent(event.media)}`, location.origin);
      target.searchParams.set("room", roomID);
      location.assign(`${target.pathname}${target.search}`);
      return;
    }
    const effectiveAt = Date.parse(event.effectiveAt);
    const seconds = Math.max(0, event.seconds + (event.action === "play" && Number.isFinite(effectiveAt) ? Math.max(0, Date.now() - effectiveAt) / 1000 : 0));
    const drift = Math.abs(player.currentTime - seconds);
    if (drift > 0.75) setPlayerTime(Math.min(seconds, Number.isFinite(player.duration) ? player.duration : seconds));
    observeRoomDrift(drift);
    if (event.action === "play") await requestPlay("watch-room").catch(() => {});
    if (event.action === "pause" || event.action === "seek" || event.action === "media") requestPause();
  };
  const connectRoom = () => {
    clearTimeout(reconnectTimer);
    const protocol = location.protocol === "https:" ? "wss:" : "ws:";
    socket = new WebSocket(`${protocol}//${location.host}/api/v1/watch-rooms/${encodeURIComponent(roomID)}/events`);
    socket.onopen = () => { reconnectAttempt = 0; roomStatus(true); };
    socket.onmessage = ({data}) => {
      try { applyRoomEvent(JSON.parse(data)); } catch (_) {}
    };
    socket.onclose = () => {
      roomStatus(false);
      if (!leaving) {
        const delay = Math.min(30000, 1000 * (2 ** Math.min(reconnectAttempt++, 5))) * (0.8 + Math.random() * 0.4);
        reconnectTimer = setTimeout(connectRoom, delay);
      }
    };
    socket.onerror = () => socket.close();
  };
  if (leader) {
    player.addEventListener("play", () => sendRoomEvent("play"));
    player.addEventListener("pause", () => { if (!player.ended) sendRoomEvent("pause"); });
    player.addEventListener("seeked", () => sendRoomEvent("seek"));
    roomChoice?.addEventListener("change", ({target}) => sendRoomEvent("media", target.value, 0));
    roomHeartbeat = setInterval(() => sendRoomEvent(player.paused ? "pause" : "play"), 15000);
  }
  addEventListener("online", () => { if (!socket || socket.readyState > WebSocket.OPEN) connectRoom(); });
  addEventListener("pagehide", () => { leaving = true; clearTimeout(reconnectTimer); clearInterval(roomHeartbeat); socket?.close(1000, "page hidden"); }, {once: true});
  connectRoom();
} else if (player.dataset.room && roomState) roomState.textContent = messages.roomDisconnected;
const castMessage = (state) => ({
  connecting: messages.castConnecting || "Connecting…",
  connected: messages.castDevice || "Playing on device",
  disconnected: messages.castHere || "Playing here",
  none: messages.castNone || "No device selected",
}[state] || state);
const castStatus = (state) => {
  const message = castMessage(state);
  if (castState) castState.textContent = message;
  castButtons.forEach((button) => button.title = message);
};
const setCastAvailability = (available) => castButtons.forEach((button) => {
  button.hidden = false;
  button.disabled = !available;
  if (!available && castState) castState.textContent = messages.castUnavailable || "No playback device is available. Check the TV connection and use a browser that supports AirPlay or remote playback.";
});
const remote = player.remote;
if (castButtons.length && typeof remote?.prompt === "function") {
  const remoteState = () => remote.state === "connected" ? "connected" : remote.state === "connecting" ? "connecting" : "disconnected";
  if (remote.state === "connected" || remote.state === "connecting") castStatus(remoteState());
  ["connecting", "connect", "disconnect"].forEach((event) => remote.addEventListener(event, () => castStatus(event === "connect" ? "connected" : event === "disconnect" ? "disconnected" : event)));
  castButtons.forEach((button) => button.addEventListener("click", () => remote.prompt().catch(() => castStatus("none"))));
  if (typeof remote.watchAvailability === "function") {
    setCastAvailability(false);
    remote.watchAvailability(setCastAvailability).then((id) => addEventListener("pagehide", () => remote.unwatchAvailability?.(id), {once: true})).catch(() => setCastAvailability(true));
  } else setCastAvailability(true);
} else if (castButtons.length && typeof player.webkitShowPlaybackTargetPicker === "function") {
  setCastAvailability(false);
  player.addEventListener("webkitplaybacktargetavailabilitychanged", (event) => setCastAvailability(event.availability === "available"));
  player.addEventListener("webkitcurrentplaybacktargetiswirelesschanged", () => castStatus(player.webkitCurrentPlaybackTargetIsWireless ? "connected" : "disconnected"));
  castButtons.forEach((button) => button.addEventListener("click", () => player.webkitShowPlaybackTargetPicker()));
} else setCastAvailability(false);
document.querySelectorAll("[data-seek]").forEach((button) => button.addEventListener("click", () => { setPlayerTime(Number(button.dataset.seek)); button.dataset.skipped = "true"; }));
const chapters = [...document.querySelectorAll("[data-chapter]")];
const syncChapter = () => chapters.forEach((button) => {
  const current = player.currentTime >= Number(button.dataset.start) && player.currentTime < Number(button.dataset.end);
  if (current) button.setAttribute("aria-current", "true");
  else button.removeAttribute("aria-current");
});
if (chapters.length) {
  player.addEventListener("timeupdate", syncChapter);
  player.addEventListener("loadedmetadata", syncChapter);
  chapters.forEach((button) => button.addEventListener("click", syncChapter));
  syncChapter();
}
document.querySelector("[data-playback-rate]")?.addEventListener("change", ({target}) => { player.playbackRate = Number(target.value); });
let sleepTimer;
document.querySelector("[data-sleep-timer]")?.addEventListener("change", ({target}) => {
  clearTimeout(sleepTimer);
  const minutes = Number(target.value);
  const state = document.querySelector("[data-sleep-state]");
  state.textContent = minutes ? messages.sleepStops.replace("{minutes}", minutes) : "";
  if (minutes) sleepTimer = setTimeout(() => { requestPause(); state.textContent = messages.sleepEnded; }, minutes * 60000);
});
const autoSkip = new Set((player.dataset.autoSkip || "").split(","));
const markers = document.querySelectorAll("[data-marker]");
player.addEventListener("seeking", () => {
  if (managedSeek) return;
  for (const marker of markers) {
    if (player.currentTime >= Number(marker.dataset.start) && player.currentTime < Number(marker.dataset.seek)) marker.dataset.skipped = "true";
  }
});
player.addEventListener("seeked", () => { managedSeek = false; });
player.addEventListener("timeupdate", () => {
  for (const marker of markers) {
    const active = !marker.dataset.skipped && player.currentTime >= Number(marker.dataset.start) && player.currentTime < Number(marker.dataset.seek) - 1;
    marker.hidden = !active;
    if (active && autoSkip.has(marker.dataset.marker)) {
      marker.dataset.skipped = "true";
      setPlayerTime(Number(marker.dataset.seek));
      break;
    }
  }
});
