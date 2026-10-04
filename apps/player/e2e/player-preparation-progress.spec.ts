import {expect, test} from "@playwright/test";
import {installPlayerExperienceFixture} from "./player-experience-fixture";

installPlayerExperienceFixture(false, false, "iPhone", async (page) => {
  await page.route("**/progress/movie", route => route.fulfill({status: 204}));
});

// Isolated exception: queued preparation callbacks cannot be forced reliably on a real decoder.
// The populated checkpoint journey covers viewer seeks; this protects preparation-only history writes.
test("Safari startup preparation restore skips queued seeking progress saves but retains chapter seeks", {tag: "@smoke"}, async ({page}) => {
  const writes: number[] = [];
  page.on("request", request => {
    if (new URL(request.url()).pathname === "/progress/movie") writes.push(Number(new URLSearchParams(request.postData() || "").get("seconds")));
  });
  await page.evaluate(() => {
    const context = window as Window & {setBufferedEnd(value: number): void; setBufferedStart(value: number): void; setReadyState(value: number): void; advanceMediaTime(value: number): void};
    const video = document.querySelector("video")!;
    context.setBufferedEnd(20.1);
    video.dispatchEvent(new Event("loadstart"));
    context.setBufferedStart(21.937);
    context.setBufferedEnd(25);
    context.setReadyState(3);
    context.advanceMediaTime(22.137);
    video.dispatchEvent(new Event("progress"));
  });
  await expect(page.locator("video")).toHaveJSProperty("paused", true);
  await expect(page.locator("video")).toHaveJSProperty("currentTime", 21.937);
  await page.waitForTimeout(100);
  expect(writes).toEqual([]);
  await page.locator("summary").click();
  await page.getByRole("button", {name: "The answer"}).click();
  await expect.poll(() => writes).toEqual([60]);
});

