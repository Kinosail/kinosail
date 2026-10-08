import {expect, type Page, type TestInfo} from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import {login} from "./test-instance-helpers";
import {faultWitness, fetchQueueResponse, holdQueueResponse, fulfillFailure, observeFaultResponse} from "./test-instance-audio-queue-witness.mjs";
type Tracks = (page: Page) => Promise<Array<{id: string}>>;

export async function mobileProgressFault(page: Page, testInfo: TestInfo, albumTracks: Tracks) {
  await login(page);
  const [first] = await albumTracks(page);
  await page.setViewportSize({width: 390, height: 844});
  await page.goto(`/watch/${first.id}`);
  const media = page.locator("audio"), notice = page.locator("[data-progress-notice]");
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.readyState)).toBeGreaterThanOrEqual(2);
  const saved = page.waitForResponse(response => new URL(response.url()).pathname === `/progress/${first.id}` && response.request().method() === "POST");
  await media.evaluate(async (audio: HTMLAudioElement) => {audio.muted = true; audio.currentTime = 1; await audio.play();});
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.currentTime)).toBeGreaterThan(1.1);
  await media.evaluate((audio: HTMLAudioElement) => audio.pause());
  const acknowledgement = await saved;
  expect(acknowledgement.status()).toBe(204);
  await testInfo.attach("mobile-progress-after-real-acknowledgement", {body: JSON.stringify({
    viewport: {width: 390, height: 844}, realAcknowledgementStatus: acknowledgement.status(),
    noticeVisible: await notice.isVisible(), hiddenAttribute: await notice.getAttribute("hidden") !== null,
  }), contentType: "application/json"});
  await expect(notice).toBeHidden({timeout: 1500});
  const failureRoute = `**/progress/${first.id}`;
  const facts = faultWitness();
  const observe = (response: import("@playwright/test").Response) => {
    if (new URL(response.url()).pathname === `/progress/${first.id}` && response.request().method() === "POST" && response.headers()["x-request-id"] === "qa-mobile-progress-failure") observeFaultResponse(facts, response);
  };
  page.on("response", observe);
  try {
    await page.route(failureRoute, route => fulfillFailure(route, facts, {"X-Request-ID": "qa-mobile-progress-failure"}));
    await media.evaluate(async (audio: HTMLAudioElement) => {audio.currentTime = 2; await audio.play();});
    await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.currentTime)).toBeGreaterThan(2.1);
    await media.evaluate((audio: HTMLAudioElement) => audio.pause());
    await expect(notice).toBeVisible();
    await expect(page.locator("[data-progress-status]")).toHaveAttribute("data-progress-failure", "server");
    await expect(page.locator("[data-progress-status]")).toHaveAttribute("data-progress-request-id", "qa-mobile-progress-failure");
    expect((await new AxeBuilder({page}).include("[data-progress-notice]").analyze()).violations).toEqual([]);
    await page.screenshot({path: testInfo.outputPath("mobile-progress-failed.png"), fullPage: true});
    await page.unroute(failureRoute);
    page.off("response", observe);
    const retried = page.waitForResponse(response => new URL(response.url()).pathname === `/progress/${first.id}` && response.request().method() === "POST");
    await page.getByRole("button", {name: "Retry saving position", exact: true}).click();
    expect((await retried).status()).toBe(204);
    await expect(notice).toBeHidden();
    await page.screenshot({path: testInfo.outputPath("mobile-progress-saved.png"), fullPage: true});
    await testInfo.attach("proof-class", {body: "Isolated 503 failure injection in a Go-backed UI; acknowledgements and Retry use the actual Server.", contentType: "text/plain"});
  } finally {
    try {
      await testInfo.attach("mobile-progress-fault-stages", {body: JSON.stringify({...facts}), contentType: "application/json"});
    } finally {
      page.off("response", observe);
      await page.unroute(failureRoute);
    }
  }
}

export async function queueReadFault(page: Page, testInfo: TestInfo, albumTracks: Tracks) {
  await login(page);
  const [first] = await albumTracks(page);
  const path = `**/api/v1/audio/${first.id}/queue`;
  const facts = faultWitness(), failure = faultWitness();
  const observe = (response: import("@playwright/test").Response) => {
    if (new URL(response.url()).pathname === `/api/v1/audio/${first.id}/queue` && response.request().method() === "GET" && response.headers()["x-request-id"] === "qa-queue-read-failure") observeFaultResponse(failure, response);
  };
  let release: (() => void) | undefined;
  try {
    await page.route(path, async route => {
      const response = await fetchQueueResponse(route, facts);
      expect(response.status()).toBe(200);
      await holdQueueResponse(route, response, facts, (value: () => void) => release = value);
    });
    await page.goto(`/watch/${first.id}`);
    await expect.poll(() => !!release).toBe(true);
    const controls = page.locator("[data-audio-queue-controls]");
    const next = page.getByRole("button", {name: "Next track", exact: true});
    const previous = page.getByRole("button", {name: "Previous track", exact: true});
    const inspect = async (state: string) => {
      for (const viewport of [{width: 390, height: 844}, {width: 1440, height: 900}, {width: 1920, height: 1080}]) {
        await page.setViewportSize(viewport);
        await expect(controls).toBeVisible();
        for (const button of [previous, next]) {
          const bounds = (await button.boundingBox())!;
          expect(bounds.height).toBeGreaterThanOrEqual(44);
          expect(bounds.width).toBeGreaterThanOrEqual(44);
        }
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
        expect((await new AxeBuilder({page}).include("[data-audio-queue-controls]").analyze()).violations).toEqual([]);
        await testInfo.attach(`${state}-${viewport.width}-queue`, {body: JSON.stringify({viewport,
          bounds: await controls.boundingBox(), busy: await controls.getAttribute("aria-busy"),
          previousDisabled: await previous.isDisabled(), nextDisabled: await next.isDisabled(),
          status: await page.locator("[data-audio-queue-status]").textContent()}), contentType: "application/json"});
        await page.screenshot({path: testInfo.outputPath(`${state}-${viewport.width}.png`), fullPage: true});
      }
    };
    await expect(controls).toHaveAttribute("aria-busy", "true");
    await expect(next).toBeDisabled();
    await expect(previous).toBeDisabled();
    await inspect("pending");
    release!();
    await expect(next).toBeEnabled();
    await expect(controls).not.toHaveAttribute("aria-busy");
    await inspect("loaded");
    await page.unroute(path);
    await page.route(path, async route => {
      const response = await route.fetch();
      expect(response.status()).toBe(200);
      const body = await response.json();
      await route.fulfill({response, json: {...body, items: body.items.slice(0, 1)}});
    });
    await page.reload();
    await expect(page.locator("[data-audio-queue-status]")).toHaveText("Track 1 of 1");
    await expect(next).toBeDisabled();
    await inspect("empty");
    await page.unroute(path);
    page.on("response", observe);
    await page.route(path, route => fulfillFailure(route, failure, {"X-Request-ID": "qa-queue-read-failure"}));
    await page.reload();
    await expect(page.locator("[data-audio-queue-status]")).toHaveAttribute("data-queue-failure", "server");
    await expect(page.locator("[data-audio-queue-status]")).toHaveAttribute("data-queue-request-id", "qa-queue-read-failure");
    await expect(next).toBeDisabled();
    await inspect("failed");
    await page.unroute(path);
    page.off("response", observe);
    await page.getByRole("button", {name: "Retry loading queue", exact: true}).click();
    await expect(next).toBeEnabled();
    await expect(page.locator("[data-audio-queue-status]")).not.toHaveAttribute("data-queue-failure");
  } finally {
    try {
      await testInfo.attach("queue-read-fault-stages", {body: JSON.stringify({pending: {...facts}, failure: {...failure}}), contentType: "application/json"});
    } finally {
      release?.();
      page.off("response", observe);
      await page.unroute(path);
    }
  }
}
