import { expect, test } from "@playwright/test";
import { downloadChunk, downloadHash, downloadIsolated, downloadServer, downloadPeer, openDownloadPage, inspectDownload, attachDownloadEnvironment } from "./download-pause-fixture";
import { observeDownloadOwnership, attachDownloadOwnership } from "./download-ownership-diagnostics";

test.skip(!downloadServer && !downloadIsolated, "requires an explicit disposable native download transport runner");
test.use({serviceWorkers: "allow"});
test.beforeEach(async ({browser}, info) => attachDownloadEnvironment(browser, info));

test("a paused peer offers Resume when the native job lock releases without another status", async ({page, context}, info) => {
  const peer = await downloadPeer();
  try {
    const served = await openDownloadPage(page, peer.origin, "indexeddb", info);
    await page.locator("[data-download-device]").click();
    await expect.poll(async () => (await inspectDownload(page, served.jobID)).job?.bytes).toBe(downloadChunk);
    const second = await context.newPage();
    await openDownloadPage(second, peer.origin, "indexeddb", info);
    const barrier = await context.newPage();
    await barrier.goto(`${peer.origin}/api/v1/downloads/${served.jobID}`);
    await barrier.evaluate(id => {
      const gate = {held: false, release: () => {}};
      (window as any).downloadLockGate = gate;
      void navigator.locks.request(`kinosail-offline:${id}`, {mode: "exclusive"}, () => new Promise<void>(resolve => {
        gate.held = true; gate.release = resolve;
      }));
    }, served.jobID);
    await expect.poll(() => barrier.evaluate(async id => (await navigator.locks.query()).pending.some(lock => lock.name === `kinosail-offline:${id}`), served.jobID)).toBe(true);
    await page.getByRole("button", {name: "Pause download", exact: true}).click();
    await expect.poll(() => barrier.evaluate(() => (window as any).downloadLockGate.held)).toBe(true);
    await expect(second.locator("[data-download-device-status]")).toHaveText("Download paused. Resume to continue where it stopped.");
    await expect(second.getByRole("button", {name: "Resume on this device", exact: true})).toHaveCount(0);
    await barrier.evaluate(() => (window as any).downloadLockGate.release());
    await expect(second.getByRole("button", {name: "Resume on this device", exact: true})).toBeEnabled();
    expect((await peer.stats()).ranges).toEqual([0, downloadChunk]);
    await second.getByRole("button", {name: "Resume on this device", exact: true}).click();
    await expect(second.locator("[data-download-device-status]")).toHaveText("Saved and verified. Play to check compatibility.");
    const stored = await inspectDownload(second, served.jobID);
    expect(stored.fileHash).toBe(downloadHash);
    expect((await peer.stats()).ranges).toEqual([0, downloadChunk, downloadChunk, downloadChunk * 2]);
    expect((await peer.stats()).removals).toBe(0);
    await info.attach("native-lock-release-resume", {body: JSON.stringify({stored, peer: await peer.stats()}), contentType: "application/json"});
  } finally { await context.close(); await peer.close(); }
});

for (const boundary of ["new-owner", "replaced-button", "profile", "pagehide", "same-profile"] as const) {
if (!downloadServer) test(`a pending Resume wait preserves the ${boundary} boundary`, async ({page, context}, info) => {
  const peer = await downloadPeer();
  try {
    const served = await openDownloadPage(page, peer.origin, "indexeddb", info);
    await page.locator("[data-download-device]").click();
    await expect.poll(async () => (await inspectDownload(page, served.jobID)).job?.bytes).toBe(downloadChunk);
    const second = await context.newPage();
    await openDownloadPage(second, peer.origin, "indexeddb", info);
    const barrier = await context.newPage();
    await barrier.goto(`${peer.origin}/api/v1/downloads/${served.jobID}`);
    await barrier.evaluate(id => {
      const gate = {held: false, release: () => {}};
      (window as any).downloadLockGate = gate;
      void navigator.locks.request(`kinosail-offline:${id}`, {mode: "exclusive"}, () => new Promise<void>(resolve => {
        gate.held = true; gate.release = resolve;
      }));
    }, served.jobID);
    await expect.poll(() => barrier.evaluate(async id => (await navigator.locks.query()).pending.some(lock => lock.name === `kinosail-offline:${id}`), served.jobID)).toBe(true);
    await page.getByRole("button", {name: "Pause download", exact: true}).click();
    await expect.poll(() => barrier.evaluate(() => (window as any).downloadLockGate.held)).toBe(true);
    await expect(second.locator("[data-download-device-status]")).toHaveText("Download paused. Resume to continue where it stopped.");
    await expect.poll(() => barrier.evaluate(async id => (await navigator.locks.query()).pending.some(lock => lock.name === `kinosail-offline:${id}` && lock.mode === "shared"), served.jobID)).toBe(true);
    const previous = await second.locator("[data-download-device]").elementHandle();
    if (boundary === "new-owner") await second.locator("[data-download-device]").click();
    else if (boundary === "replaced-button") await previous!.evaluate(node => node.replaceWith(node.cloneNode(true)));
    else if (boundary === "pagehide") await second.evaluate(() => dispatchEvent(new PageTransitionEvent("pagehide")));
    else {
      await second.evaluate(() => {
        (window as any).profileRevisions = 0;
        addEventListener("kinosail:offline-profile", () => { (window as any).profileRevisions++; });
      });
      const other = await context.newPage();
      await other.goto(`${peer.origin}/offline-downloads?profile=${boundary === "profile" ? "other" : "profile"}`);
      await expect.poll(() => second.evaluate(() => (window as any).profileRevisions)).toBeGreaterThan(0);
      if (boundary === "same-profile") {
        // Replay the public identity notification after tab bootstrap settles.
        // It changes the identity revision while preserving the Viewer Profile.
        await second.evaluate(() => dispatchEvent(new Event("kinosail:offline-profile")));
      }
    }
    await barrier.evaluate(() => (window as any).downloadLockGate.release());
    await expect.poll(() => barrier.evaluate(async id => (await navigator.locks.query()).pending.some(lock => lock.name === `kinosail-offline:${id}`), served.jobID)).toBe(false);
    if (boundary === "same-profile") await expect(second.getByRole("button", {name: "Resume on this device", exact: true})).toBeEnabled();
    else await expect(second.getByRole("button", {name: "Resume on this device", exact: true})).toHaveCount(0);
    if (boundary === "new-owner") {
      await expect(second.locator("[data-download-device-status]")).toHaveText("Saved and verified. Play to check compatibility.");
      expect((await inspectDownload(second, served.jobID)).fileHash).toBe(downloadHash);
    } else {
      expect(await previous!.textContent()).toBe(boundary === "same-profile" ? "Resume on this device" : "Download to this device");
      expect((await peer.stats()).ranges).toEqual([0, downloadChunk]);
    }
    expect((await peer.stats()).removals).toBe(0);
    await info.attach("pending-resume-boundary", {body: JSON.stringify({boundary, peer: await peer.stats()}), contentType: "application/json"});
  } finally { await context.close(); await peer.close(); }
});
}

test("same Viewer Profile in another tab preserves the transfer owner until explicit Pause", async ({page, context}, info) => {
  await observeDownloadOwnership(context);
  const peer = await downloadPeer();
  let failed = false;
  try {
    const served = await openDownloadPage(page, peer.origin, "indexeddb", info);
    await page.locator("[data-download-device]").click();
    await expect.poll(async () => (await inspectDownload(page, served.jobID)).job?.bytes).toBe(downloadChunk);
    const retained = await inspectDownload(page, served.jobID);
    const second = await context.newPage();
    await openDownloadPage(second, peer.origin, "indexeddb", info);
    await info.attach("same-profile-tab-peer", {body: JSON.stringify(await peer.stats()), contentType: "application/json"});
    await expect(page.locator("[data-download-device-status]")).toContainText("Keep this page open", {timeout: 2_000});
    expect((await peer.stats()).closed).toBe(0);
    expect((await inspectDownload(page, served.jobID)).job?.transferID).toBe(retained.job?.transferID);
    // Only the owning tab exposes Pause. A status broadcast is not cancellation authority.
    await expect(second.getByRole("button", {name: "Pause download", exact: true})).toHaveCount(0);
    await page.getByRole("button", {name: "Pause download", exact: true}).click();
    await expect.poll(async () => (await peer.stats()).closed).toBe(1);
    await expect(second.getByRole("button", {name: "Resume on this device", exact: true})).toBeEnabled();
    await second.getByRole("button", {name: "Resume on this device", exact: true}).click();
    await expect(second.locator("[data-download-device-status]")).toHaveText("Saved and verified. Play to check compatibility.");
    const ready = await inspectDownload(second, served.jobID);
    expect(ready.fileHash).toBe(downloadHash);
    expect(ready.job?.transferID).not.toBe(retained.job?.transferID);
    expect(ready.job?.profileID).toBe(retained.job?.profileID);
    expect((await peer.stats()).ranges).toEqual([0, downloadChunk, downloadChunk, downloadChunk * 2]);
    expect((await peer.stats()).removals).toBe(0);
    await info.attach("cross-tab-safe-resume", {body: JSON.stringify({stored: ready, peer: await peer.stats()}), contentType: "application/json"});
  } catch (error) {
    failed = true;
    await attachDownloadOwnership(context, info).catch(() => {});
    await peer.stats().then(stats => info.attach("download-ownership-peer-at-failure", {body: JSON.stringify(stats), contentType: "application/json"})).catch(() => {});
    throw error;
  } finally {
    let cleanupError: unknown;
    try { await context.close(); } catch (error) { cleanupError = error; }
    try { await peer.close(); } catch (error) { cleanupError ??= error; }
    if (!failed && cleanupError) throw cleanupError;
  }
});

test("navigation interrupts the transfer and reload offers explicit Resume with verified data", async ({page, context}, info) => {
  const peer = await downloadPeer();
  try {
    const served = await openDownloadPage(page, peer.origin, "indexeddb", info);
    await page.locator("[data-download-device]").click();
    await expect.poll(async () => (await inspectDownload(page, served.jobID)).job?.bytes).toBe(downloadChunk);
    const retained = await inspectDownload(page, served.jobID);
    await page.goto(`${peer.origin}/offline-downloads?returned=1`);
    await expect.poll(async () => (await peer.stats()).closed).toBe(1);
    const resume = page.getByRole("button", {name: "Resume on this device", exact: true});
    await expect(resume).toBeEnabled();
    expect((await inspectDownload(page, served.jobID)).chunks).toEqual(retained.chunks);
    expect((await peer.stats()).ranges).toEqual([0, downloadChunk]);
    await resume.click();
    await expect(page.locator("[data-download-device-status]")).toHaveText("Saved and verified. Play to check compatibility.");
    expect((await inspectDownload(page, served.jobID)).fileHash).toBe(downloadHash);
    expect((await peer.stats()).ranges).toEqual([0, downloadChunk, downloadChunk, downloadChunk * 2]);
    expect((await peer.stats()).removals).toBe(0);
    await info.attach("navigation-verified-resume", {body: JSON.stringify({stored: await inspectDownload(page, served.jobID), peer: await peer.stats()}), contentType: "application/json"});
  } finally { await context.close(); await peer.close(); }
});

if (!downloadServer) test("isolated native profile boundary: changing Viewer Profile still cancels the old owner", async ({page, context}, info) => {
  const peer = await downloadPeer();
  try {
    const served = await openDownloadPage(page, peer.origin, "indexeddb", info);
    await page.locator("[data-download-device]").click();
    await expect.poll(async () => (await inspectDownload(page, served.jobID)).job?.bytes).toBe(downloadChunk);
    const retained = await inspectDownload(page, served.jobID);
    const second = await context.newPage();
    await second.goto(`${peer.origin}/offline-downloads?profile=other`);
    await expect.poll(async () => (await peer.stats()).closed).toBe(1);
    expect((await inspectDownload(page, served.jobID)).chunks).toEqual(retained.chunks);
    expect((await inspectDownload(page, served.jobID)).job?.profileID).toBe(retained.job?.profileID);
    expect((await peer.stats()).ranges).toEqual([0, downloadChunk]);
    expect((await peer.stats()).removals).toBe(0);
    await info.attach("changed-profile-cancellation-retention", {body: JSON.stringify({stored: await inspectDownload(page, served.jobID), peer: await peer.stats()}), contentType: "application/json"});
  } finally { await context.close(); await peer.close(); }
});
