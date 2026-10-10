import {expect, test, type Page} from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import {login} from "./test-instance-helpers";

export function registerAudioProgressTests(albumTracks: (page: Page) => Promise<{id: string}[]>) {
// Isolated 503 injection in an actual Go-rendered page. Initial and Retry 204
// acknowledgements remain real Server requests; canonical queue cases are unrouted.
test("mobile R03 progress notice stays hidden after real acknowledgement and reopens only on failure", {tag: ["@smoke", "@routed-fault"]}, async ({page}, testInfo) => {
  await login(page);
  const [first] = await albumTracks(page);
  await page.setViewportSize({width: 390, height: 844});
  await page.goto(`/watch/${first.id}`);
  const media = page.locator("audio"), notice = page.locator("[data-progress-notice]");
  await media.evaluate(async (audio: HTMLAudioElement) => {audio.muted = true; await audio.play();});
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.currentTime)).toBeGreaterThan(0.1);
  await media.evaluate((audio: HTMLAudioElement) => {audio.currentTime = 1;});
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => !audio.seeking && audio.readyState >= 3)).toBe(true);
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.currentTime)).toBeGreaterThan(1.1);
  const saved = page.waitForResponse(response => new URL(response.url()).pathname === `/progress/${first.id}` && response.request().method() === "POST");
  const pausedSeconds = await media.evaluate((audio: HTMLAudioElement) => {audio.pause(); return audio.currentTime;});
  const acknowledgement = await saved;
  expect(acknowledgement.status()).toBe(204);
  const acknowledged = new URLSearchParams(acknowledgement.request().postData() ?? "");
  expect(Number(acknowledged.get("seconds"))).toBe(pausedSeconds);
  expect(acknowledged.get("watched")).toBe("false");
  const stored = await page.request.get(`/api/v1/items/${first.id}`);
  expect(stored.status()).toBe(200);
  expect((await stored.json()).item.progress.seconds).toBeCloseTo(pausedSeconds, 3);
  await testInfo.attach("mobile-progress-after-real-acknowledgement", {body: JSON.stringify({
    viewport: {width: 390, height: 844}, realAcknowledgementStatus: acknowledgement.status(), pausedSeconds,
    acknowledgedSeconds: Number(acknowledged.get("seconds")), acknowledgedRevision: acknowledged.get("revision"),
    noticeVisible: await notice.isVisible(), hiddenAttribute: await notice.getAttribute("hidden") !== null,
  }), contentType: "application/json"});
  await expect(notice).toBeHidden({timeout: 1500});
  const failureRoute = `**/progress/${first.id}`;
  await page.route(failureRoute, route => route.fulfill({status: 503, headers: {"X-Request-ID": "qa-mobile-progress-failure"}}));
  await media.evaluate((audio: HTMLAudioElement) => {audio.currentTime = 2;});
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => !audio.seeking && audio.readyState >= 3)).toBe(true);
  await media.evaluate((audio: HTMLAudioElement) => audio.play());
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.currentTime)).toBeGreaterThan(2.1);
  await media.evaluate((audio: HTMLAudioElement) => audio.pause());
  await expect(notice).toBeVisible();
  await expect(page.locator("[data-progress-status]")).toHaveAttribute("data-progress-failure", "server");
  await expect(page.locator("[data-progress-status]")).toHaveAttribute("data-progress-request-id", "qa-mobile-progress-failure");
  expect((await new AxeBuilder({page}).include("[data-progress-notice]").analyze()).violations).toEqual([]);
  await page.screenshot({path: testInfo.outputPath("mobile-progress-failed.png"), fullPage: true});
  await page.unroute(failureRoute);
  const retried = page.waitForResponse(response => new URL(response.url()).pathname === `/progress/${first.id}` && response.request().method() === "POST");
  await page.getByRole("button", {name: "Retry saving position", exact: true}).click();
  expect((await retried).status()).toBe(204);
  await expect(notice).toBeHidden();
  await page.screenshot({path: testInfo.outputPath("mobile-progress-saved.png"), fullPage: true});
  await testInfo.attach("proof-class", {body: "Isolated 503 failure injection in a Go-backed UI; acknowledgements and Retry use the actual Server.", contentType: "text/plain"});
});
}
