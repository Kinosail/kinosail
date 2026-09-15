import { expect, test } from "@playwright/test";
import { createHmac } from "node:crypto";

test.skip(!process.env.KINOSAIL_TEST_INSTANCE, "requires the populated test instance");
test.beforeEach(async ({ page }) => page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false })));

function totp(): string {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  const bits = [...(process.env.KINOSAIL_TEST_TOTP_SECRET ?? "")].map((character) => alphabet.indexOf(character).toString(2).padStart(5, "0")).join("");
  const secret = Buffer.from(bits.match(/.{8}/g)?.map((byte) => Number.parseInt(byte, 2)) ?? []);
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30_000)));
  const digest = createHmac("sha1", secret).update(counter).digest();
  const offset = digest[19] & 15;
  return ((digest.readUInt32BE(offset) & 0x7fffffff) % 1_000_000).toString().padStart(6, "0");
}

for (const compatible of [false, true]) test(`one play request does not immediately pause (${compatible ? "compatible" : "direct"})`, async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Name").fill("Owner");
  await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
  await page.getByLabel("6-digit code").fill(totp());
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
  await page.getByRole("link", { name: "Movies", exact: true }).click();
  const watch = await page.locator('a.card[href^="/watch/"]').first().getAttribute("href");
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
