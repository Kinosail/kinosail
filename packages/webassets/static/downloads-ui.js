async function downloadOfflineJob(button) {
  const jobID = button.dataset.jobId;
  const expected = {jobID, itemID: button.dataset.itemId, profileID: currentOfflineProfile(), title: button.dataset.title, quality: button.dataset.quality};
  if (offlineTransfers.has(jobID)) return;
  const owner = {controller: new AbortController(), startedAt: offlineNow(), transferID: [...crypto.getRandomValues(new Uint8Array(16))].map((byte) => byte.toString(16).padStart(2, "0")).join("")};
  offlineTransfers.set(jobID, owner);
  button.disabled = true;
  try {
    if (!/^[0-9a-f]{16}$/.test(expected.jobID) || !offlineItemID.test(expected.itemID || "") || !offlineProfileID.test(expected.profileID) || !expected.title || expected.title.length > 512 || !offlineQualities.has(expected.quality)) throw new Error(offlineMessage("offlineVerificationError", "The download could not be verified"));
    const response = await fetch(`/api/v1/downloads/${encodeURIComponent(jobID)}`, {credentials: "same-origin", cache: "no-store", redirect: "error", signal: AbortSignal.any([owner.controller.signal, AbortSignal.timeout(15000)])});
    if (!response.ok) throw new Error(offlineMessage("offlineUnavailable", "The download is no longer available"));
    const job = await offlineProgressJSON(response);
    if (!validOfflineManifest(job, expected)) throw new Error(offlineMessage("offlineVerificationError", "The download could not be verified"));
    await requireOfflineServiceWorker();
    notifyOffline(jobID, {state: "waiting", error: offlineMessage("offlinePageRequired", "Keep this page open. Interrupted downloads can resume.")});
    await withOfflineJobLock(jobID, async () => {
      void navigator.storage?.persist?.().catch(() => {});
      return withOfflineTransferSlot(() => transferOfflineJob(job, expected, owner), owner.controller.signal);
    }, true, owner.controller.signal);
  } catch (error) {
    notifyOffline(jobID, {state: "needs_attention", error: owner.controller.signal.aborted ? offlineMessage("offlinePaused", "Download interrupted. Resume to continue where it stopped.") : error instanceof Error ? error.message : offlineMessage("offlineVerificationError", "The download could not be verified")});
  } finally {
    if (offlineTransfers.get(jobID) === owner) offlineTransfers.delete(jobID);
    button.disabled = false;
  }
}

function bindOfflineRemovalForms() {
  for (const form of document.querySelectorAll('form[action^="/offline-downloads/"][action$="/remove"]')) {
    if (form.dataset.bound) continue;
    form.dataset.bound = "true";
    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      if (form.dataset.removing) return;
      form.dataset.removing = "true";
      const status = form.querySelector("[data-download-remove-status]");
      const controls = [...form.querySelectorAll('button:not([type]), button[type="submit"], input[type="submit"]')];
      for (const control of controls) control.disabled = true;
      try {
        const id = new URL(form.action).pathname.split("/").at(-2);
        if (!/^[0-9a-f]{16}$/.test(id || "")) throw new Error(offlineRemovalError());
        const removed = await removeOfflineJobSafely(id, true, async () => {
          const body = new URLSearchParams();
          for (const [name, value] of new FormData(form)) if (typeof value === "string") body.append(name, value);
          const response = await fetch(form.action, {method: "POST", body, credentials: "same-origin", headers: {"Content-Type": "application/x-www-form-urlencoded"}, signal: AbortSignal.timeout(15000)});
          if (!response.ok) throw new Error(offlineRemovalError());
          location.assign(response.url || "/offline-downloads");
        });
        if (!removed) throw new Error(offlineRemovalError());
      } catch (_) {
        form.dataset.removing = "";
        for (const control of controls) control.disabled = false;
        if (status) { status.hidden = false; status.textContent = offlineRemovalError(); }
      }
    });
  }
}

async function bindOfflineDownloads() {
  bindOfflineRemovalForms();
  const profile = currentOfflineProfile();
  if (profile) playerStorageValue("kinosail.offline-profile", profile);
  for (const button of document.querySelectorAll("[data-download-device]")) {
    if (button.dataset.bound) continue;
    button.dataset.bound = "true";
    button.addEventListener("click", () => downloadOfflineJob(button));
  }
  if (!hasOfflineStorage || !navigator.locks?.request) {
    for (const status of document.querySelectorAll("[data-download-device-status]")) status.textContent = offlineMessage("offlineStorageUnsupported", "This browser cannot store offline media safely");
    return;
  }
  try {
    const activeWorker = await offlineServiceWorker;
    for (const button of document.querySelectorAll("[data-download-device]")) {
      if (!activeWorker || !navigator.serviceWorker.controller) {
        const status = button.parentElement?.querySelector("[data-download-device-status]");
        if (status) status.textContent = offlineWorkerError();
        continue;
      }
      const live = offlineStatuses.get(button.dataset.jobId);
      if (live) showOfflineStatus(button.dataset.jobId, live);
      const record = await getOfflineJob(button.dataset.jobId);
      if (record?.profileID !== currentOfflineProfile()) continue;
      if (record.state === "ready" && record.readyOffline && record.sha256) notifyOffline(button.dataset.jobId, {state: "ready", playbackVerified: record.playbackAgent === navigator.userAgent.slice(0, 512)});
      else if (!offlineTransfers.has(record.id)) {
        const locks = await navigator.locks.query();
        if (!locks.held.some((lock) => lock.name === offlineJobLockName(record.id))) {
          button.textContent = offlineMessage("offlineResume", "Resume on this device");
          notifyOffline(record.id, {state: "needs_attention", error: record.error || offlineMessage("offlinePaused", "Download interrupted. Resume to continue where it stopped.")});
        }
      }
    }
  } catch (_) {
    for (const status of document.querySelectorAll("[data-download-device-status]")) status.textContent = offlineMessage("offlineStorageUnsupported", "This browser cannot store offline media safely");
  }
}

function bindOfflinePlaybackEvidence(media, record) {
  const source = new URL(`/offline-media/${encodeURIComponent(record.profileID)}/${encodeURIComponent(record.id)}`, location.href).href;
  const loaded = async () => {
    if (media.currentSrc !== source || media.readyState < HTMLMediaElement.HAVE_CURRENT_DATA) return;
    try {
      await updateOfflineProgress(record, (current) => { current.playbackAgent = navigator.userAgent.slice(0, 512); });
      notifyOffline(record.id, {state: "ready", playbackVerified: true});
    } catch (_) {}
  };
  media.addEventListener("loadeddata", loaded, {once: true});
  if (media.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA) void loaded();
  return () => media.removeEventListener("loadeddata", loaded);
}

const offlinePlaybackBindings = new WeakMap();
window.KinosailOfflineMedia = {
  bindProgress: async (media, source, status) => {
    const profileID = activeOfflineProfile();
    const jobID = source.split("/").at(-1);
    const record = await getOfflineJob(jobID);
    if (!validOfflineProgressJob(record) || record.profileID !== profileID || source !== `/offline-media/${encodeURIComponent(profileID)}/${encodeURIComponent(record.id)}` || !record.readyOffline || record.state !== "ready" || offlineSourceTransfers.get(record.id) !== offlineTransferIDOf(record)) throw new Error("Offline download changed. Open Downloads and try again.");
    offlinePlaybackBindings.get(media)?.detach();
    const binding = bindOfflineProgress(media, record, status, () => media.dataset.offline === "true");
    const detachEvidence = bindOfflinePlaybackEvidence(media, record);
    const detachProgress = binding.detach;
    binding.detach = () => { detachEvidence(); detachProgress(); };
    offlinePlaybackBindings.set(media, binding);
    return () => { binding.detach(); offlinePlaybackBindings.delete(media); };
  },
  unbindProgress: (media) => { offlinePlaybackBindings.get(media)?.detach(); offlinePlaybackBindings.delete(media); },
  saveProgress: (media, watched) => offlinePlaybackBindings.get(media)?.save(watched) ?? Promise.resolve({ok: false}),
  source: async (itemID) => {
    if (!hasOfflineStorage || !itemID) return "";
    if (!await offlineServiceWorker || !navigator.serviceWorker.controller) return "";
    const profileID = activeOfflineProfile();
    const record = (await getOfflineJobs()).find((job) => job.profileID === profileID && job.itemID === itemID && job.state === "ready" && job.readyOffline);
    if (!record) return "";
    offlineSourceTransfers.set(record.id, offlineTransferIDOf(record));
    return `/offline-media/${encodeURIComponent(profileID)}/${encodeURIComponent(record.id)}`;
  },
  remove: removeOfflineJobSafely,
};

let downloadsEvents;
let downloadsPoll;
let refreshingDownloads = false;
let downloadsRefreshQueued = false;
const stopDownloadsPoll = () => { clearInterval(downloadsPoll); downloadsPoll = undefined; };
const refreshDownloads = async () => {
  const current = document.querySelector("#downloads");
  if (!current) return;
  if (refreshingDownloads) { downloadsRefreshQueued = true; return; }
  refreshingDownloads = true;
  try {
    const response = await fetch("/offline-downloads", {credentials: "same-origin", cache: "no-store"});
    if (!response.ok) return;
    const replacement = new DOMParser().parseFromString(await response.text(), "text/html").querySelector("#downloads");
    if (replacement && current.isConnected) {
      const active = document.activeElement;
      const focused = current.contains(active);
      const jobID = active.closest?.('[data-download-job]')?.dataset.downloadJob;
      const control = active.dataset?.downloadFocus;
      const scroll = [window.scrollX, window.scrollY];
      for (const oldJob of current.querySelectorAll('[data-download-job]')) {
        const nextJob = [...replacement.querySelectorAll('[data-download-job]')].find((job) => job.dataset.downloadJob === oldJob.dataset.downloadJob);
        if (!nextJob) continue;
        // Keep in-flight event handlers and disabled buttons attached to their job.
        if (oldJob.querySelector('[data-download-device]:disabled,form[data-removing="true"]')) nextJob.replaceWith(oldJob);
        else if (oldJob.querySelector('details[open]')) nextJob.querySelector('details')?.setAttribute('open', '');
      }
      current.replaceWith(replacement);
      if (focused) {
        const target = [...replacement.querySelectorAll('[data-download-focus]')].find((node) =>
          node.dataset.downloadFocus === control && node.closest('[data-download-job]')?.dataset.downloadJob === jobID && !node.disabled);
        if (target) target.focus({ preventScroll: true });
        else replacement.querySelector('h1')?.focus();
        if (target) window.scrollTo(...scroll);
      } else window.scrollTo(...scroll);
      if (replacement.dataset.downloadsPending !== "true") stopDownloadsPoll();
      await bindOfflineDownloads();
      bindLiveDownloads();
    }
  } catch (_) {}
  finally {
    refreshingDownloads = false;
    if (downloadsRefreshQueued) { downloadsRefreshQueued = false; void refreshDownloads(); }
  }
};
const startDownloadsPoll = () => {
  if (document.querySelector("#downloads")?.dataset.downloadsPending !== "true") { stopDownloadsPoll(); return; }
  if (!downloadsPoll) downloadsPoll = setInterval(refreshDownloads, 3000);
};
function bindLiveDownloads() {
  if (!document.querySelector("#downloads") || downloadsEvents) return;
  if (!("EventSource" in window)) { startDownloadsPoll(); return; }
  downloadsEvents = new EventSource("/api/v1/events");
  downloadsEvents.addEventListener("download.updated", refreshDownloads);
  downloadsEvents.onopen = stopDownloadsPoll;
  downloadsEvents.onerror = startDownloadsPoll;
}

async function renderOfflineLibrary() {
  const target = document.querySelector("[data-offline-library]");
  if (!target) return;
  if (!hasOfflineStorage) {
    const message = document.createElement("p");
    message.textContent = offlineMessage("offlineStorageUnsupported", "This browser cannot store offline media safely");
    target.replaceChildren(message);
    return;
  }
  const profile = activeOfflineProfile();
  let jobs;
  try { jobs = (await getOfflineJobs()).filter((job) => job.profileID === profile && job.state === "ready" && job.readyOffline); }
  catch (_) {
    const message = document.createElement("p");
    message.textContent = offlineMessage("offlineStorageUnsupported", "This browser cannot store offline media safely");
    target.replaceChildren(message);
    return;
  }
  const requested = new URL(location.href).searchParams.get("job");
  const selected = jobs.find((job) => job.id === requested);
  target.replaceChildren();
  if (selected) {
    const back = document.createElement("a");
    back.href = "/offline";
    back.className = "button quiet";
    back.textContent = offlineMessage("offlineBack", "Back to downloads");
    target.append(back);
    const status = document.createElement("p");
    status.setAttribute("role", "status");
    const heading = document.createElement("h2");
    heading.textContent = selected.title;
    const media = document.createElement(selected.quality === "audio" ? "audio" : "video");
    media.controls = true;
    media.playsInline = true;
    media.setAttribute("aria-label", selected.title);
    bindOfflineProgress(media, selected, status);
    bindOfflinePlaybackEvidence(media, selected);
    media.src = `/offline-media/${encodeURIComponent(profile)}/${encodeURIComponent(selected.id)}`;
    if (media instanceof HTMLVideoElement) {
      const stage = document.createElement("div");
      stage.className = "media-stage";
      stage.append(media);
      target.append(heading, stage);
    } else target.append(heading, media);
    target.append(status);
    return;
  }
  if (!jobs.length) {
    const message = document.createElement("p");
    message.textContent = profile
      ? offlineMessage("offlineNoReady", "No downloads are ready on this device for this Viewer Profile.")
      : offlineMessage("offlineSelectProfile", "Open Kinosail online and choose a Viewer Profile first.");
    target.append(message);
    return;
  }
  const list = document.createElement("ul");
  for (const job of jobs) {
    const item = document.createElement("li");
    const link = document.createElement("a");
    link.href = `/offline?job=${encodeURIComponent(job.id)}`;
    link.textContent = `${job.title} · ${job.quality} · ${job.playbackAgent === navigator.userAgent.slice(0, 512) ? offlineMessage("offlineReady", "Ready offline on this device") : offlineMessage("offlineVerified", "Saved and verified. Play to check compatibility.")}`;
    item.append(link);
    list.append(item);
  }
  target.append(list);
}

const bindOfflineChannel = () => {
  if (offlineChannel || !("BroadcastChannel" in window)) return;
  offlineChannel = new BroadcastChannel("kinosail-offline");
  offlineChannel.addEventListener("message", ({data}) => {
    if (data?.type === "cancel") cancelOfflineTransfer(data);
    else showOfflineStatus(data?.jobID, data?.detail || {});
  });
};
bindOfflineChannel();
bindOfflineDownloads();
bindLiveDownloads();
void syncOfflineProgress();
renderOfflineLibrary().catch(() => {});
addEventListener("pagehide", () => {
  for (const owner of offlineTransfers.values()) owner.controller.abort();
  closeOfflineDatabase();
  downloadsEvents?.close();
  downloadsEvents = undefined;
  offlineChannel?.close();
  offlineChannel = undefined;
  stopDownloadsPoll();
});
addEventListener("pageshow", ({persisted}) => {
  if (!persisted) return;
  bindOfflineChannel();
  bindOfflineDownloads();
  bindLiveDownloads();
  renderOfflineLibrary().catch(() => {});
});
document.body.addEventListener("htmx:afterSwap", bindOfflineDownloads);
