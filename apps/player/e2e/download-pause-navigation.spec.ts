import {expect, test, type Page} from "@playwright/test";
import {createHash} from "node:crypto";
import {downloadPeer, openDownloadPage} from "./download-pause-fixture";
import {downloadsSource, serviceWorkerSource} from "./static-sources";

const scenarios = ["ready", "http-error", "normal-http-error", "redirect", "normal-redirect", "slow-response", "missing-script", "changed-script", "missing-api", "missing-worker", "wrong-worker", "wrong-scope", "missing-control", "disabled-control", "unbound-control", "invalid-job", "invalid-item", "mismatched-article", "missing-profile", "invalid-profile", "missing-title", "oversized-title", "missing-quality", "unknown-quality", "bootstrap-error", "unexpected-transfer", "unexpected-removal", "late-error", "late-transfer", "late-document", "late-control", "late-title", "late-quality", "other-error", "repeated-timeout", "replacement-http-error", "replacement-redirect"] as const;

for (const scenario of scenarios) test(`download navigation preserves original ${scenario} evidence`, {tag: "@smoke"}, async ({browser}, info) => {
  let calls = 0, documents = 0;
  const peer = await downloadPeer(true, () => {
    if (calls === 1 && (scenario === "http-error" || scenario === "normal-http-error") || calls === 2 && scenario === "replacement-http-error") return {status: 500};
    if (calls === 1 && (scenario === "redirect" || scenario === "normal-redirect") || calls === 2 && scenario === "replacement-redirect") return {status: 302, location: "/different"};
    return calls === 1 && scenario === "slow-response" ? {status: 200, delayMs: 2100} : undefined;
  });
  const context = await browser.newContext({serviceWorkers: scenario === "missing-worker" ? "block" : "allow"});
  const page = await context.newPage();
  page.on("request", request => {
    if (request.isNavigationRequest() && request.frame() === page.mainFrame() && request.url() === `${peer.origin}/offline-downloads`) documents++;
  });
  if (scenario === "missing-script") await page.route(`${peer.origin}/static/downloads.js*`, route => route.fulfill({status: 404, body: ""}));
  if (scenario === "changed-script") await page.route(`${peer.origin}/static/downloads.js*`, route => route.fulfill({status: 200, contentType: "text/javascript", body: downloadsSource + "\n// Controlled source drift"}));
  const original = page.goto.bind(page);
  page.goto = async (...args: Parameters<Page["goto"]>) => {
    calls++;
    const response = await original(...args);
    await page.waitForFunction(() => document.readyState === "complete");
    if (scenario !== "missing-script" && scenario !== "missing-worker") {
      await page.waitForFunction(() => navigator.serviceWorker.controller?.state === "activated");
      await expect(page.locator("[data-download-device]")).toHaveAttribute("data-bound", "true");
    }
    if (calls === 1) {
      if (scenario === "wrong-worker" || scenario === "wrong-scope") {
        await page.evaluate(async scenario => {
          await navigator.serviceWorker.register(scenario === "wrong-worker" ? "/service-worker.js?v=wrong" : "/service-worker.js?v=55", scenario === "wrong-scope" ? {scope: "/offline-downloads"} : {});
        }, scenario);
        await page.waitForFunction(async scenario => scenario === "wrong-worker"
          ? navigator.serviceWorker.controller?.scriptURL.endsWith("?v=wrong")
          : (await navigator.serviceWorker.getRegistration())?.scope.endsWith("/offline-downloads"), scenario);
      }
      await page.evaluate(async scenario => {
        const button = document.querySelector<HTMLButtonElement>("[data-download-device]")!;
        if (scenario === "missing-control") button.remove();
        if (scenario === "disabled-control") button.disabled = true;
        if (scenario === "unbound-control") delete button.dataset.bound;
        if (scenario === "invalid-job") button.dataset.jobId = "not-canonical";
        if (scenario === "invalid-item") button.dataset.itemId = "not-canonical";
        if (scenario === "mismatched-article") button.closest<HTMLElement>("[data-download-job]")!.dataset.downloadJob = "cccccccccccccccc";
        if (scenario === "missing-profile") delete document.body.dataset.viewerProfile;
        if (scenario === "invalid-profile") document.body.dataset.viewerProfile = "malformed/profile";
        if (scenario === "missing-title") delete button.dataset.title;
        if (scenario === "oversized-title") button.dataset.title = "x".repeat(513);
        if (scenario === "missing-quality") delete button.dataset.quality;
        if (scenario === "unknown-quality") button.dataset.quality = "unknown";
        if (scenario === "missing-api") delete (window as Window & {KinosailOfflineMedia?: object}).KinosailOfflineMedia;
        if (scenario === "bootstrap-error") setTimeout(() => {throw new Error("Controlled bootstrap failure");}, 0);
        if (scenario === "unexpected-transfer") await fetch("/api/v1/downloads/aaaaaaaaaaaaaaaa/file", {headers: {range: "bytes=0-0"}});
        if (scenario === "unexpected-removal") await fetch("/offline-downloads/aaaaaaaaaaaaaaaa/remove", {method: "POST"});
      }, scenario);
      if (scenario === "bootstrap-error") await expect.poll(() => pageErrors).toBe(1);
    }
    if (calls === 1 && scenario !== "normal-http-error" && scenario !== "normal-redirect" || scenario === "repeated-timeout") {
      const error = new Error("Controlled navigation bookkeeping timeout after real native bootstrap");
      error.name = scenario === "other-error" ? "Error" : "TimeoutError";
      throw error;
    }
    return response;
  };
  let pageErrors = 0;
  page.on("pageerror", () => pageErrors++);
  const attach = info.attach.bind(info);
  info.attach = async (name, options) => {
    await attach(name, options);
    if (name !== "firefox-download-navigation-recovery") return;
    await page.evaluate(async scenario => {
      if (scenario === "late-error") setTimeout(() => {throw new Error("Controlled preservation-window bootstrap failure");}, 0);
      if (scenario === "late-transfer") await fetch("/api/v1/downloads/aaaaaaaaaaaaaaaa/file", {headers: {range: "bytes=0-0"}});
      if (scenario === "late-document") history.replaceState(null, "", "/different");
      if (scenario === "late-control") document.querySelector<HTMLButtonElement>("[data-download-device]")!.disabled = true;
      if (scenario === "late-title") document.querySelector<HTMLButtonElement>("[data-download-device]")!.dataset.title = "changed title";
      if (scenario === "late-quality") document.querySelector<HTMLButtonElement>("[data-download-device]")!.dataset.quality = "compatible";
    }, scenario);
    if (scenario === "late-error") await expect.poll(() => pageErrors).toBe(1);
  };
  try {
    const result = await openDownloadPage(page, peer.origin, "indexeddb", info).then(served => ({ok: true, served}), error => ({ok: false, error: String(error)}));
    const eligible = browser.browserType().name() === "firefox" && scenario === "ready";
    expect(result.ok).toBe(eligible);
    const replacement = browser.browserType().name() === "firefox" && ["ready", "repeated-timeout", "replacement-http-error", "replacement-redirect"].includes(scenario);
    expect(calls).toBe(replacement ? 2 : 1);
    // A redirect followed inside the native worker exposes its final URL to Firefox.
    expect(documents).toBe(eligible ? 3 : replacement && scenario !== "replacement-redirect" ? 2 : 1);
    const stats = await peer.stats();
    expect(stats.ranges).toEqual(scenario === "unexpected-transfer" || scenario === "late-transfer" && browser.browserType().name() === "firefox" ? [0] : []);
    expect(stats.removals).toBe(scenario === "unexpected-removal" ? 1 : 0);
    if (eligible) {
      await expect(page.locator("[data-download-device]")).toBeEnabled();
      await expect(page.locator("[data-download-device-status]")).toHaveText("Not stored on this device");
    }
    await info.attach("download-navigation-control", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
      scenario, browser: browser.browserType().name(), version: browser.version(), calls, documents, pageErrors, stats, result,
      command: "playwright test download-pause-navigation.spec.ts --workers=1 --retries=0", environment: process.platform,
      downloadsSHA256: createHash("sha256").update(downloadsSource).digest("hex"), workerSHA256: createHash("sha256").update(serviceWorkerSource).digest("hex"),
      data: "Fictional native HTTP download shell and committed bundle/worker; controlled automation timeout and negative boundaries", outcome: "passed"}), contentType: "application/json"});
  } finally {
    info.attach = attach;
    await context.close();
    await peer.close();
  }
});
