import { fixtureTotp } from "../../../scripts/testing/fixture-totp";
import { submitFixtureLogin } from "./fixture-credential-login";
import { expect, test } from "@playwright/test";
import { firstPlayable } from "./test-instance-helpers";

test.skip(!process.env.KINOSAIL_TEST_INSTANCE, "requires the populated test instance");
test.beforeEach(async ({ page }) => page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false })));

function totp(): string {
  return fixtureTotp();
}

for (const compatible of [false, true]) test(`one play request does not immediately pause (${compatible ? "compatible" : "direct"})`, async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Name").fill("Owner");
  await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
  await submitFixtureLogin(page, totp);
  if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
  await page.getByRole("link", { name: "Movies", exact: true }).click();
  const watch = await firstPlayable(page);
  await page.evaluate(async (id) => {
    const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')!.content;
    const response = await fetch(`/api/v1/items/${id}/progress`, { method: "PUT",
      headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf }, body: JSON.stringify({ seconds: 0, watched: false }) });
    if (!response.ok) throw new Error(`reset playback progress: ${response.status}`);
  }, watch.split("/").at(-1)!);
  await page.goto(`${watch}${compatible ? "?compatible=1" : ""}`);
  const video = page.locator("video");
  const result = await video.evaluate(async (element: HTMLVideoElement) => {
    const events: Array<{ event: string; time: number }> = [];
    for (const event of ["play", "playing", "pause", "waiting", "stalled", "error"]) {
      element.addEventListener(event, () => events.push({ event, time: performance.now() }));
    }
    element.muted = true;
    await element.play();
    await new Promise((resolve) => setTimeout(resolve, 1500));
    return { events, paused: element.paused, currentTime: element.currentTime, error: element.error?.message };
  });
  console.log(JSON.stringify(result));
  expect(result.events.filter(({ event }) => event === "pause")).toEqual([]);
  expect(result.paused).toBe(false);
  expect(result.currentTime).toBeGreaterThan(0.25);
});
