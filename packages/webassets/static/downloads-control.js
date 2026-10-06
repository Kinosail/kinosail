const offlineUserPaused = () => offlineMessage("offlineUserPaused", "Download paused. Resume to continue where it stopped.");

function activeOfflineDownloadButton(button, owner) {
  button.disabled = owner.controller.signal.aborted;
  button.textContent = owner.controller.signal.aborted
    ? offlineMessage("offlinePausing", "Pausing download…")
    : offlineMessage("offlinePause", "Pause download");
}

const offlineResumeWaits = new WeakSet();
async function syncOfflineDownloadButton(button, detail) {
  const jobID = button.dataset.jobId;
  const owner = offlineTransfers.get(jobID);
  if (owner) { activeOfflineDownloadButton(button, owner); return; }
  if (detail.state === "ready") {
    button.textContent = button.dataset.downloadLabel || offlineMessage("offlineDownload", "Download to this device");
    button.disabled = false;
    return;
  }
  if (detail.state !== "needs_attention" || offlineResumeWaits.has(button)) return;
  offlineResumeWaits.add(button);
  try {
    await withOfflineJobLock(jobID, () => {
      if (!button.isConnected || offlineTransfers.has(jobID) || offlineStatuses.get(jobID)?.state !== "needs_attention") return;
      button.textContent = offlineMessage("offlineResume", "Resume on this device");
      button.disabled = false;
    });
  } catch (_) { /* Transfer admission still requires a supported lock manager. */ }
  finally { offlineResumeWaits.delete(button); }
}

function controlOfflineDownload(button) {
  const owner = offlineTransfers.get(button.dataset.jobId);
  if (!owner) { void downloadOfflineJob(button); return; }
  if (owner.controller.signal.aborted) return;
  owner.paused = true;
  owner.restoreFocus = document.activeElement === button;
  owner.controller.abort();
  activeOfflineDownloadButton(button, owner);
}

async function downloadOfflineJob(button) {
  const jobID = button.dataset.jobId;
  const expected = {jobID, itemID: button.dataset.itemId, profileID: currentOfflineProfile(), title: button.dataset.title, quality: button.dataset.quality};
  if (offlineTransfers.has(jobID)) return;
  const owner = {controller: new AbortController(), profileID: expected.profileID, startedAt: offlineNow(), transferID: [...crypto.getRandomValues(new Uint8Array(16))].map((byte) => byte.toString(16).padStart(2, "0")).join("")};
  offlineTransfers.set(jobID, owner);
  activeOfflineDownloadButton(button, owner);
  try {
    if (!/^[0-9a-f]{16}$/.test(expected.jobID) || !offlineItemID.test(expected.itemID || "") || !offlineProfileID.test(expected.profileID) || !expected.title || expected.title.length > 512 || !offlineQualities.has(expected.quality)) throw new Error(offlineMessage("offlineVerificationError", "The download could not be verified"));
    showOfflineStatus(jobID, {state: "waiting", error: offlineMessage("offlinePageRequired", "Keep this page open. Interrupted downloads can resume.")});
    const response = await fetch(`/api/v1/downloads/${encodeURIComponent(jobID)}`, {credentials: "same-origin", cache: "no-store", redirect: "error", signal: AbortSignal.any([owner.controller.signal, AbortSignal.timeout(15000)])});
    if (!response.ok) throw new Error(offlineMessage("offlineUnavailable", "The download is no longer available"));
    const job = await offlineProgressJSON(response);
    if (!validOfflineManifest(job, expected)) throw new Error(offlineMessage("offlineVerificationError", "The download could not be verified"));
    await requireOfflineServiceWorker();
    await withOfflineJobLock(jobID, async () => {
      void navigator.storage?.persist?.().catch(() => {});
      return withOfflineTransferSlot(() => transferOfflineJob(job, expected, owner), owner.controller.signal);
    }, true, owner.controller.signal);
  } catch (error) {
    const interrupted = owner.paused ? offlineUserPaused() : offlineMessage("offlinePaused", "Download interrupted. Resume to continue where it stopped.");
    const detail = {state: "needs_attention", error: owner.controller.signal.aborted ? interrupted : error instanceof Error ? error.message : offlineMessage("offlineVerificationError", "The download could not be verified")};
    // A tab waiting for another owner may pause only its own pending admission.
    if (owner.admitted) notifyOffline(jobID, detail);
    else showOfflineStatus(jobID, detail);
  } finally {
    if (offlineTransfers.get(jobID) === owner) {
      offlineTransfers.delete(jobID);
      button.disabled = false;
      button.textContent = offlineStatuses.get(jobID)?.state === "ready"
        ? button.dataset.downloadLabel || offlineMessage("offlineDownload", "Download to this device")
        : offlineMessage("offlineResume", "Resume on this device");
      if (owner.restoreFocus && button.isConnected && (document.activeElement === document.body || document.activeElement === button)) button.focus({preventScroll: true});
    }
  }
}

document.addEventListener("kinosail:offline-download", ({detail}) => {
  if (!/^[0-9a-f]{16}$/.test(detail?.jobID || "")) return;
  const button = document.querySelector(`[data-download-device][data-job-id="${CSS.escape(detail.jobID)}"]`);
  const status = offlineStatuses.get(detail.jobID);
  if (button && status) void syncOfflineDownloadButton(button, status);
});
