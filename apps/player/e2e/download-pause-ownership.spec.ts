import { expect, test } from "@playwright/test";
import { downloadChunk, downloadHash, downloadIsolated, downloadServer, downloadPeer, openDownloadPage, inspectDownload, attachDownloadEnvironment } from "./download-pause-fixture";
import { observeDownloadOwnership, attachDownloadOwnership } from "./download-ownership-diagnostics";

test.skip(!downloadServer && !downloadIsolated, "requires an explicit disposable native download transport runner");
test.use({serviceWorkers: "allow"});
test.beforeEach(async ({browser}, info) => attachDownloadEnvironment(browser, info));

test("same Viewer Profile in another tab preserves the transfer owner until explicit Pause", async ({page, context}, info) => {
  await observeDownloadOwnership(context);
  const peer = await downloadPeer();
  try {
    const served = await openDownloadPage(page, peer.origin, "indexeddb");
    await page.locator("[data-download-device]").click();
    await expect.poll(async () => (await inspectDownload(page, served.jobID)).job?.bytes).toBe(downloadChunk);
    const retained = await inspectDownload(page, served.jobID);
    const second = await context.newPage();
    await openDownloadPage(second, peer.origin, "indexeddb");
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
    await attachDownloadOwnership(context, info);
    await info.attach("download-ownership-peer-at-failure", {body: JSON.stringify(await peer.stats()), contentType: "application/json"});
    throw error;
  } finally { await context.close(); await peer.close(); }
});

test("navigation interrupts the transfer and reload offers explicit Resume with verified data", async ({page, context}, info) => {
  const peer = await downloadPeer();
  try {
    const served = await openDownloadPage(page, peer.origin, "indexeddb");
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
    const served = await openDownloadPage(page, peer.origin, "indexeddb");
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
