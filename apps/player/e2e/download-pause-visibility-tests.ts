import {expect, test} from "@playwright/test";
import {downloadChunk, downloadHash, downloadPeer, openDownloadPage, inspectDownload} from "./download-pause-fixture";

for (const capability of ["held-snapshot", "missing-query", "rejected-query"]) test(`Paused peer resumes after ${capability}`, async ({page, context}) => {
  const peer = await downloadPeer();
  try {
    const served = await openDownloadPage(page, peer.origin, "indexeddb");
    await expect(page.getByRole("button", {name:"Download to this device", exact:true})).toBeEnabled();
    await page.locator("[data-download-device]").click();
    await expect.poll(async () => (await inspectDownload(page, served.jobID)).job?.bytes).toBe(downloadChunk);
    const retained = await inspectDownload(page, served.jobID);
    const second = await context.newPage();
    await second.addInitScript(({mode,id}) => {
      const observation = {paused:0, requests:0};
      Object.assign(window, {pauseObservation: observation});
      new BroadcastChannel("kinosail-offline").addEventListener("message", ({data}) => {
        if (data?.detail?.state === "needs_attention") observation.paused++;
      });
      const request = navigator.locks.request.bind(navigator.locks);
      navigator.locks.request = ((name: string, ...args: any[]) => {
        if (name === `kinosail-offline:${id}`) observation.requests++;
        return (request as any)(name, ...args);
      }) as typeof navigator.locks.request;
      const query = navigator.locks.query.bind(navigator.locks);
      Object.assign(window, {restorePauseQuery:() => Object.defineProperty(navigator.locks,"query",{value:query,configurable:true})});
      if (mode === "missing-query") Object.defineProperty(navigator.locks, "query", {value:undefined,configurable:true});
      if (mode === "rejected-query") Object.defineProperty(navigator.locks, "query", {value:() => Promise.reject(new Error("Fixture query unavailable")),configurable:true});
    }, {mode:capability,id:served.jobID});
    await openDownloadPage(second, peer.origin, "indexeddb");
    await expect(second.getByRole("button", {name:"Pause download", exact:true})).toHaveCount(0);
    expect((await peer.stats()).closed).toBe(0);
    if (capability === "held-snapshot") await page.evaluate(id => {
      const lease = {acquired:false, release:undefined as undefined | (() => void)};
      Object.assign(window, {pauseLease:lease});
      // A real queued lock outlives both notifications, without changing them.
      void navigator.locks.request(`kinosail-offline:${id}`, async () => {
        lease.acquired = true;
        await new Promise<void>(resolve => { lease.release = resolve; });
      });
    }, served.jobID);
    await page.getByRole("button", {name:"Pause download", exact:true}).click();
    await expect.poll(async () => (await peer.stats()).closed).toBe(1);
    await expect(page.getByRole("button", {name:"Resume on this device", exact:true})).toBeEnabled();
    await expect.poll(() => second.evaluate(() => (window as any).pauseObservation.paused)).toBeGreaterThanOrEqual(2);
    if (capability === "held-snapshot") {
      await expect.poll(() => page.evaluate(() => (window as any).pauseLease.acquired)).toBe(true);
      expect(await second.evaluate(async id => (await navigator.locks.query()).held.some(lock => lock.name === `kinosail-offline:${id}`), served.jobID)).toBe(true);
      await expect(second.getByRole("button", {name:"Resume on this device", exact:true})).toHaveCount(0);
      await page.evaluate(id => {
        const channel = new BroadcastChannel("kinosail-offline");
        for (let n=0;n<20;n++) channel.postMessage({jobID:id, detail:{state:"needs_attention", error:"Download paused."}});
        channel.close();
      }, served.jobID);
      await expect.poll(() => second.evaluate(() => (window as any).pauseObservation.paused)).toBeGreaterThanOrEqual(22);
      expect(await second.evaluate(() => (window as any).pauseObservation.requests)).toBeLessThanOrEqual(1);
      await page.evaluate(() => (window as any).pauseLease.release());
    }
    await expect(second.getByRole("button", {name:"Resume on this device", exact:true})).toBeEnabled({timeout:3000});
    // Capacity reservation and byte inspection still use the real query API.
    await second.evaluate(() => (window as any).restorePauseQuery());
    expect((await inspectDownload(second, served.jobID)).chunks).toEqual(retained.chunks);
    expect((await peer.stats()).ranges).toEqual([0,downloadChunk]);
    expect((await peer.stats()).removals).toBe(0);
    await second.getByRole("button", {name:"Resume on this device", exact:true}).click();
    await expect(second.locator("[data-download-device-status]")).toHaveText("Saved and verified. Play to check compatibility.");
    expect((await inspectDownload(second, served.jobID)).fileHash).toBe(downloadHash);
    expect((await peer.stats()).removals).toBe(0);
  } finally {
    await page.evaluate(() => (window as any).pauseLease?.release?.()).catch(() => {});
    await context.close(); await peer.close();
  }
});

for (const state of ["ready", "transferring", "unknown"]) test(`A released paused lock does not overwrite the later ${state} state`, async ({page, context}) => {
  const peer = await downloadPeer();
  try {
    const served = await openDownloadPage(page, peer.origin, "indexeddb");
    await page.locator("[data-download-device]").click();
    await expect.poll(async () => (await inspectDownload(page, served.jobID)).job?.bytes).toBe(downloadChunk);
    const second = await context.newPage();
    await openDownloadPage(second, peer.origin, "indexeddb");
    await page.evaluate(id => {
      Object.assign(window, {releaseResumeGuard:undefined});
      void navigator.locks.request(`kinosail-offline:${id}`, async () => {
        await new Promise<void>(resolve => Object.assign(window, {releaseResumeGuard:resolve}));
      });
    }, served.jobID);
    await page.getByRole("button", {name:"Pause download", exact:true}).click();
    await expect(page.getByRole("button", {name:"Resume on this device", exact:true})).toBeEnabled();
    await expect(second.locator("[data-download-device-status]")).toContainText("Download paused.");
    await expect.poll(() => page.evaluate(() => typeof (window as any).releaseResumeGuard)).toBe("function");
    const retained = await inspectDownload(second, served.jobID);
    await second.evaluate(({id,state}) => {
      // Public status notifications cannot authorize a transfer or change bytes.
      document.dispatchEvent(new CustomEvent("kinosail:offline-download", {detail:{jobID:id, state}}));
      const channel = new BroadcastChannel("kinosail-offline");
      channel.postMessage({jobID:id,detail:{state}}); channel.close();
    }, {id:served.jobID,state});
    await page.evaluate(({id,state}) => {
      const channel = new BroadcastChannel("kinosail-offline");
      channel.postMessage({jobID:id,detail:{state,error:"Synthetic later status"}}); channel.close();
    }, {id:served.jobID,state});
    await expect(second.locator("[data-download-device-status]")).toHaveText(state === "ready" ? "Saved and verified. Play to check compatibility." : state === "transferring" ? "Synthetic later status · Keep this page open" : "Synthetic later status");
    await page.evaluate(() => (window as any).releaseResumeGuard());
    await second.evaluate(async id => { await navigator.locks.request(`kinosail-offline:${id}`, () => {}); }, served.jobID);
    await expect(second.getByRole("button", {name:"Resume on this device", exact:true})).toHaveCount(0);
    const after = await inspectDownload(second, served.jobID);
    expect(after.job).toEqual(retained.job);
    expect(after.chunks).toEqual(retained.chunks);
    expect(after.fileHash).toBe(retained.fileHash);
    expect(after.locks).toEqual([]);
    expect((await peer.stats()).ranges).toEqual([0,downloadChunk]);
    expect((await peer.stats()).removals).toBe(0);
  } finally {
    await page.evaluate(() => (window as any).releaseResumeGuard?.()).catch(() => {});
    await context.close(); await peer.close();
  }
});
