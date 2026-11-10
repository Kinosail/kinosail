if (controls && player.tagName === "VIDEO") {
  const stage = player.closest(".media-stage");
  const native = appleNativePlayback || player.hasAttribute("data-native-controls");
  const volumeKey = "kinosail.player.volume", mutedKey = "kinosail.player.muted";
  const storedVolume = playerStorage.get(volumeKey);
  if (storedVolume !== "" && Number(storedVolume) >= 0 && Number(storedVolume) <= 1) player.volume = Number(storedVolume);
  if (playerStorage.get(mutedKey) === "true") player.muted = true;
  player.addEventListener("volumechange", () => {
    playerStorage.set(volumeKey, String(Math.round(player.volume * 100) / 100));
    playerStorage.set(mutedKey, String(player.muted));
  });
  const notice = document.createElement("p");
  notice.className = "player-shortcut-feedback";
  notice.setAttribute("role", "status");
  notice.hidden = true;
  stage.append(notice);
  let noticeTimer;
  const announce = (message) => {
    clearTimeout(noticeTimer);
    notice.textContent = message;
    notice.hidden = false;
    noticeTimer = setTimeout(() => { notice.hidden = true; }, 1200);
  };
  const duration = () => Number.isFinite(player.duration) ? player.duration : Number(player.dataset.duration);
  const seekTo = (seconds) => {
    const total = duration();
    if (!(total > 0)) return false;
    setPlayerTime(Math.max(0, Math.min(total, seconds)), true);
    return true;
  };
  const rates = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3];
  const changeRate = (step) => {
    const current = rates.indexOf(player.playbackRate);
    const next = rates[Math.max(0, Math.min(rates.length - 1, (current < 0 ? rates.indexOf(1) : current) + step))];
    player.playbackRate = next;
    announce(next === 1 ? "Normal speed" : `Speed ${next}×`);
  };
  const changeVolume = (step) => {
    const next = Math.round(Math.max(0, Math.min(1, (player.muted ? 0 : player.volume) + step)) * 100) / 100;
    player.muted = false;
    player.volume = next;
    announce(`Volume ${Math.round(next * 100)}%`);
  };
  const press = (selector) => {
    const button = document.querySelector(selector);
    if (!button || button.hidden || button.disabled) return false;
    button.click();
    return true;
  };
  const actions = {
    f: () => press("[data-player-fullscreen]"),
    i: () => press("[data-player-pip]"),
    c: () => press("[data-player-captions]"),
    m: () => { player.muted = !player.muted; announce(player.muted ? "Muted" : `Volume ${Math.round(player.volume * 100)}%`); return true; },
    j: () => seekTo(player.currentTime - 10),
    l: () => seekTo(player.currentTime + 10),
    home: () => seekTo(0),
    "<": () => { changeRate(-1); return true; },
    ">": () => { changeRate(1); return true; },
  };
  document.addEventListener("keydown", (event) => {
    if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey || document.querySelector("dialog[open]")) return;
    const target = event.target;
    const inStage = target !== document.body && stage.contains(target);
    if ((target !== document.body && !inStage) || target.closest?.("input,select,textarea,a,summary,[contenteditable]:not([contenteditable=false])")) return;
    const key = event.key.toLowerCase();
    if (event.shiftKey && key !== "<" && key !== ">") return;
    let handled = false;
    if (actions[key]) handled = actions[key]();
    else if (/^[0-9]$/.test(key)) handled = seekTo(duration() * Number(key) / 10);
    else if (key === "arrowleft" || key === "arrowright") handled = seekTo(player.currentTime + (key === "arrowright" ? 10 : -10));
    else if ((key === "arrowup" || key === "arrowdown") && (inStage || document.fullscreenElement)) { changeVolume(key === "arrowup" ? 0.05 : -0.05); handled = true; }
    if (handled) event.preventDefault();
  });
  if (!native) player.addEventListener("dblclick", (event) => {
    if (event.pointerType === "touch" || navigator.maxTouchPoints > 0) return;
    press("[data-player-fullscreen]");
  });
  stage.setAttribute("aria-keyshortcuts", "Space K J L ArrowLeft ArrowRight ArrowUp ArrowDown M F C I Home 0 1 2 3 4 5 6 7 8 9");
}
