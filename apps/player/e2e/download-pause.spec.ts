import { expect, test } from "@playwright/test";
import { downloadChunk, downloadBytes, downloadHash, firstBlockHash, downloadIsolated, downloadServer, downloadPeer, openDownloadPage, inspectDownload, attachDownloadEnvironment } from "./download-pause-fixture";

test.skip(!downloadServer && !downloadIsolated, "requires an explicit disposable download transport runner");
test.use({serviceWorkers: "allow"});
test.beforeEach(async ({browser}, info) => attachDownloadEnvironment(browser, info));

for (const storage of ["opfs", "indexeddb"] as const) {
for (const width of downloadServer ? [390, 1440, 1920] : [storage === "opfs" ? 390 : 1440]) {
  test(`${downloadServer ? "real Server" : "isolated native browser"}: Pause retains verified ${storage} blocks and Resume continues missing ranges at ${width}px`, async ({page: fixturePage, context: fixtureContext, browser}, info) => {
    test.setTimeout(60_000);
    // WebKit's ephemeral contexts expose OPFS but cannot open its directory.
    const context = storage === "opfs" ? await browser.browserType().launchPersistentContext("", {
      serviceWorkers: "allow", channel: info.project.use.channel,
    }) : fixtureContext;
    const page = storage === "opfs" ? context.pages()[0] : fixturePage;
    let peer: Awaited<ReturnType<typeof downloadPeer>> | undefined;
    let failed = false;
    try {
      // Playwright Test records every context, including this persistent one.
      await page.setViewportSize({width, height: width === 1920 ? 1080 : 844});
      const transport = await downloadPeer();
      peer = transport;
      const served = await openDownloadPage(page, transport.origin, storage);
      await info.attach("served-download-bundle", {body: JSON.stringify(served), contentType: "application/json"});
      const button = page.locator("[data-download-device]"), status = page.locator("[data-download-device-status]");
      await expect(status).toHaveText("Not stored on this device");
      await button.click();
      await expect.poll(async () => (await transport.stats()).ranges).toEqual([0, downloadChunk]);
      await expect.poll(async () => (await inspectDownload(page, served.jobID)).job?.bytes).toBe(downloadChunk);
      await expect(status).toContainText("Keep this page open");
      const before = await inspectDownload(page, served.jobID);
      expect(before.job?.storage).toBe(storage);
      expect(before.chunks).toEqual([{offset: 0, length: downloadChunk, sha256: firstBlockHash, actual: firstBlockHash}]);
      await info.attach("verified-first-block-before-pause", {body: JSON.stringify(before), contentType: "application/json"});
      await page.screenshot({path: info.outputPath(`${storage}-${width}-pending.png`), fullPage: true});
      const pause = page.getByRole("button", {name: "Pause download", exact: true});
      await expect(pause).toBeEnabled({timeout: 2_000});
      await pause.focus(); await page.keyboard.press("Enter");
      const resume = page.getByRole("button", {name: "Resume on this device", exact: true});
      await expect(resume).toBeEnabled();
      await expect(resume).toBeFocused();
      await expect(status).toHaveText("Download paused. Resume to continue where it stopped.");
      await expect.poll(async () => (await transport.stats()).closed).toBe(1);
      const paused = await inspectDownload(page, served.jobID);
      expect(paused.job?.readyOffline).toBe(false);
      expect(paused.job?.bytes).toBe(downloadChunk);
      expect(paused.chunks).toEqual(before.chunks);
      expect(paused.fileSize).toBe(downloadChunk);
      expect(paused.fileHash).toBe(firstBlockHash);
      expect(paused.locks.filter((name) => name.startsWith("kinosail-offline"))).toEqual([]);
      await info.attach("paused-verified-storage-and-peer", {body: JSON.stringify({stored: paused, peer: await transport.stats()}), contentType: "application/json"});
      await page.screenshot({path: info.outputPath(`${storage}-${width}-paused.png`), fullPage: true});
      await context.setOffline(true); await resume.click();
      await expect(resume).toBeEnabled();
      await expect(status).not.toHaveText("Download paused. Resume to continue where it stopped.");
      await page.screenshot({path: info.outputPath(`${storage}-${width}-offline-failure.png`), fullPage: true});
      // WebKit's network emulation also blocks native File reads. Inspect after
      // restoring transport, before the user explicitly resumes the transfer.
      await context.setOffline(false);
      expect((await inspectDownload(page, served.jobID)).chunks).toEqual(before.chunks);
      expect((await transport.stats()).ranges).toEqual([0, downloadChunk]);
      await resume.focus(); await page.keyboard.press("Enter");
      await expect(status).toHaveText("Saved and verified. Play to check compatibility.", {timeout: 25_000});
      const ready = await inspectDownload(page, served.jobID);
      expect(ready.job?.readyOffline).toBe(true);
      expect(ready.job?.state).toBe("ready");
      expect(ready.job?.sha256).toBe(downloadHash);
      expect(ready.fileSize).toBe(downloadBytes.length);
      expect(ready.fileHash).toBe(downloadHash);
      expect(ready.job?.transferID).not.toBe(paused.job?.transferID);
      expect(ready.job?.profileID).toBe(paused.job?.profileID);
      expect(ready.job?.itemID).toBe(paused.job?.itemID);
      expect((await transport.stats()).ranges).toEqual([0, downloadChunk, downloadChunk, downloadChunk * 2]);
      expect((await transport.stats()).removals).toBe(0);
      await expect(page.locator("[data-download-play]")).toBeVisible();
      await expect(page.getByRole("button", {name: "Pause download", exact: true})).toHaveCount(0);
      const serverJob = await page.request.get(`${transport.origin}/api/v1/downloads/${served.jobID}`);
      expect(serverJob.ok()).toBe(true);
      expect(await serverJob.json()).toMatchObject({state: "ready", readyOffline: true, sha256: downloadHash});
      await info.attach("resumed-integrity-and-ranges", {body: JSON.stringify({stored: ready, peer: await transport.stats()}), contentType: "application/json"});
      await page.screenshot({path: info.outputPath(`${storage}-${width}-ready.png`), fullPage: true});
    } catch (error) {
      failed = true;
      await page.screenshot({path: info.outputPath("persistent-storage-failure.png")}).catch(() => {});
      throw error;
    } finally {
      const cleanupFailures: string[] = [];
      let firstCleanupError: unknown;
      const cleanup = async (stage: string, action: () => Promise<unknown>) => {
        try { await action(); }
        catch (error) { if (!cleanupFailures.length) firstCleanupError = error; cleanupFailures.push(stage); }
      };
      await cleanup("restore-transport", () => context.setOffline(false));
      await cleanup("close-context", () => context.close());
      if (peer) await cleanup("close-peer", () => peer.close());
      if (cleanupFailures.length) {
        await info.attach("download-cleanup-failures", {body: JSON.stringify(cleanupFailures), contentType: "application/json"});
        if (!failed) throw firstCleanupError;
      }
    }
  });
}
}
