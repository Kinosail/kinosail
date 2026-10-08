import type { BrowserContext, TestInfo } from "@playwright/test";

export async function observeDownloadOwnership(context: BrowserContext) {
  await context.addInitScript(() => {
    const events: unknown[] = [];
    const identities = new WeakMap<Element, number>();
    let nextIdentity = 0;
    const labels = new Set(["Download to this device", "Resume on this device", "Pause download", "Pausing download…"]);
    const record = (kind: string, data: object = {}) => {
      const buttons = [...document.querySelectorAll<HTMLButtonElement>("[data-download-device]")];
      events.push({kind, time: performance.timeOrigin + performance.now(), ...data,
        buttons: buttons.map(button => {
          if (!identities.has(button)) identities.set(button, ++nextIdentity);
          const status = button.parentElement?.querySelector("[data-download-device-status]")?.textContent || "";
          return {identity: identities.get(button), label: labels.has(button.textContent || "") ? button.textContent : "other",
            disabled: button.disabled, connected: button.isConnected, canonicalJob: /^[a-f0-9]{16}$/.test(button.dataset.jobId || ""),
            paused: status.startsWith("Download paused."), ready: status.startsWith("Saved and verified."), transferring: status.includes("Keep this page open")};
        }), profilePresent: Boolean(document.querySelector<HTMLElement>("[data-viewer-profile]")?.dataset.viewerProfile)});
      if (events.length > 150) events.shift();
    };
    Object.assign(window, {downloadOwnershipEvents: events, snapshotDownloadOwnership: () => record("final")});
    if (navigator.locks?.query) {
      const query = navigator.locks.query.bind(navigator.locks);
      navigator.locks.query = async () => {
        try {
          const result = await query();
          const names = (locks: LockInfo[] = []) => locks.map(lock => lock.name || "").filter(name => /^kinosail-offline[a-z0-9:-]{0,160}$/.test(name));
          record("locks", {held: names(result.held), pending: names(result.pending)});
          return result;
        } catch (error) { record("locks-rejected"); throw error; }
      };
    }
    document.addEventListener("kinosail:offline-download", event => {
      const detail = (event as CustomEvent).detail;
      const state = ["waiting", "transferring", "needs_attention", "ready"].includes(detail?.state) ? detail.state : "other";
      record("status", {state, canonicalJob: /^[a-f0-9]{16}$/.test(detail?.jobID || "")});
    });
    document.addEventListener("DOMContentLoaded", () => {
      record("loaded");
      new MutationObserver(changes => {
        if (changes.some(change => [...change.addedNodes, ...change.removedNodes].some(node => node instanceof Element &&
          (node.matches("#downloads,[data-download-device]") || node.querySelector("[data-download-device]"))))) record("controls-replaced");
      }).observe(document.body, {childList: true, subtree: true});
    });
  });
}

export async function attachDownloadOwnership(context: BrowserContext, info: TestInfo) {
  for (const [index, page] of context.pages().entries()) {
    const events = await page.evaluate(() => {
      const observed = window as Window & {downloadOwnershipEvents?: unknown[]; snapshotDownloadOwnership?: () => void};
      observed.snapshotDownloadOwnership?.();
      return observed.downloadOwnershipEvents || [];
    }).catch(() => []);
    await info.attach(`download-ownership-tab-${index}`, {body: JSON.stringify(events), contentType: "application/json"});
  }
}
