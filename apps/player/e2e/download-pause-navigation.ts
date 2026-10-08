import type {Page, Request, Response, TestInfo} from "@playwright/test";
import {createHash} from "node:crypto";
import {downloadsSource} from "./static-sources";

// A usable Firefox document can be missing its automation commit notification.
// Preserve direct native bootstrap proof before permitting one replacement GET.
export async function gotoDownloadPage(page: Page, origin: string, info: TestInfo, timeout = 10_000) {
  const url = `${origin}/offline-downloads`, workerURL = `${origin}/service-worker.js?v=55`;
  const previousDocument = await page.evaluate(() => performance.timeOrigin);
  const started = Date.now();
  let observed: Response | undefined, bundle: Response | undefined, responseAfter = Infinity;
  let requests = 0, responses = 0, pageErrors = 0, sideEffects = 0;
  const observeRequest = (request: Request) => {
    if (request.isNavigationRequest() && request.frame() === page.mainFrame()) requests++;
    const path = new URL(request.url()).pathname;
    if (/^\/api\/v1\/downloads\/[^/]+\/file$/.test(path) || !["GET", "HEAD"].includes(request.method())) sideEffects++;
  };
  const observeResponse = (response: Response) => {
    const request = response.request();
    if (request.isNavigationRequest() && request.frame() === page.mainFrame() && request.method() === "GET" && response.url() === url) {
      observed = response; responseAfter = Date.now() - started; responses++;
    }
    if (request.frame() === page.mainFrame() && new URL(response.url()).origin === origin && new URL(response.url()).pathname === "/static/downloads.js") bundle = response;
  };
  const observeError = () => {pageErrors++;};
  const navigate = () => page.goto(url, {waitUntil: "commit", timeout});
  const validate = async (response: Response | null) => {
    if (!response || response.status() !== 200 || response.url() !== url || response.request().redirectedFrom() ||
        await page.evaluate(() => location.href) !== url) throw new Error("Downloads navigation did not return the exact document with HTTP200");
    return response;
  };
  page.on("request", observeRequest); page.on("response", observeResponse); page.on("pageerror", observeError);
  try {
    return await validate(await navigate());
  } catch (error) {
    if (page.context().browser()?.browserType().name() !== "firefox" || !(error instanceof Error) || error.name !== "TimeoutError" ||
        requests !== 1 || responses !== 1 || !observed || observed.status() !== 200 || observed.request().redirectedFrom() ||
        responseAfter > 2000 || pageErrors || sideEffects || !bundle || bundle.status() !== 200 || bundle.request().redirectedFrom()) throw error;
    const checksum = createHash("sha256").update(await bundle.body()).digest("hex");
    if (checksum !== createHash("sha256").update(downloadsSource).digest("hex")) throw error;
    const inspectDocument = () => page.evaluate(async workerURL => {
      const buttons = document.querySelectorAll<HTMLButtonElement>("[data-download-device]");
      const button = buttons[0], status = button?.parentElement?.querySelector<HTMLElement>("[data-download-device-status]");
      const visible = (element?: HTMLElement | null) => Boolean(element && !element.closest("[inert]") &&
        element.getBoundingClientRect().width && element.getBoundingClientRect().height && getComputedStyle(element).visibility === "visible");
      const profile = document.body.dataset.viewerProfile || document.querySelector<HTMLElement>("#downloads")?.dataset.viewerProfile;
      const api = (window as Window & {KinosailOfflineMedia?: {source?: unknown; remove?: unknown}}).KinosailOfflineMedia;
      const registration = await navigator.serviceWorker.getRegistration(), controller = navigator.serviceWorker.controller;
      const articleJob = button?.closest<HTMLElement>("[data-download-job]")?.dataset.downloadJob;
      const title = button?.dataset.title, quality = button?.dataset.quality;
      return {url: location.href, timeOrigin: performance.timeOrigin, state: document.readyState,
        profile, jobID: button?.dataset.jobId, itemID: button?.dataset.itemId, articleJob, title, quality,
        controlsReady: Boolean(buttons.length === 1 && visible(button) && !button.matches(":disabled") && button.dataset.bound === "true" &&
          /^[a-f0-9]{16}$/.test(button.dataset.jobId || "") && /^[a-f0-9]{16}$/.test(button.dataset.itemId || "") &&
          /^[A-Za-z0-9_-]{1,256}$/.test(profile || "") && articleJob === button.dataset.jobId &&
          title && title.length <= 512 && ["original", "compatible", "1080p", "720p", "480p", "audio"].includes(quality || "") &&
          visible(status) && status?.textContent?.trim() === "Not stored on this device" &&
          typeof api?.source === "function" && typeof api?.remove === "function"),
        worker: {controller: controller?.scriptURL, controllerState: controller?.state, active: registration?.active?.scriptURL,
          activeState: registration?.active?.state, scope: registration?.scope,
          ready: controller?.scriptURL === workerURL && controller.state === "activated" && registration?.active?.scriptURL === workerURL &&
            registration.active.state === "activated" && registration.scope === `${location.origin}/`}};
    }, workerURL);
    const originalDocument = await inspectDocument();
    if (!originalDocument.controlsReady || !originalDocument.worker.ready || originalDocument.state !== "complete" ||
        originalDocument.timeOrigin === previousDocument || originalDocument.url !== url || pageErrors || sideEffects) throw error;
    const trace = info.outputPath("download-original-navigation-trace.zip");
    await page.context().tracing.stopChunk({path: trace});
    await page.context().tracing.startChunk({title: "Download journey after verified original Firefox document"});
    await info.attach("download-original-navigation-trace", {path: trace, contentType: "application/zip"});
    await info.attach("firefox-download-navigation-recovery", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
      url, requests, responses, status: observed.status(), responseAfter, elapsed: Date.now() - started, checksum, pageErrors, sideEffects,
      originalDocument, browser: page.context().browser()?.version(), action: "one superseding GET after verified original native bootstrap"}), contentType: "application/json"});
    const preservedDocument = await inspectDocument();
    if (pageErrors || sideEffects || requests !== 1 || JSON.stringify(preservedDocument) !== JSON.stringify(originalDocument)) throw error;
    page.off("request", observeRequest); page.off("response", observeResponse); page.off("pageerror", observeError);
    return await validate(await navigate());
  } finally {
    page.off("request", observeRequest); page.off("response", observeResponse); page.off("pageerror", observeError);
  }
}
