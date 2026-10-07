import {expect, test} from "@playwright/test";
import {openAudio, queueItem, startQueue} from "./player-audio-queue-fixture";
import {playerSource} from "./static-sources";

// These held responses exercise closed-page and ownership races that the real
// Server cannot produce deterministically. They are isolated browser controls.
for (const boundary of ["pagehide", "cast", "room"]) {
  test(`late queue item authorization cannot cross ${boundary}`, async ({page}) => {
    await openAudio(page, "");
    let lookup: import("@playwright/test").Route | undefined;
    const writes: string[] = [];
    page.on("request", request => {
      if (new URL(request.url()).pathname.startsWith("/progress/")) writes.push(request.url());
    });
    await page.route("https://audio.test/api/v1/items/next", route => {lookup = route;});
    await startQueue(page);
    await page.getByRole("button", {name: "Next track", exact: true}).click();
    await expect.poll(() => !!lookup).toBe(true);
    await expect(page.locator("[data-audio-queue-controls]")).toHaveAttribute("aria-busy", "true");
    if (boundary === "pagehide") await page.evaluate(() => {
      // Avoid a real page-exit checkpoint here; this case isolates the late read.
      document.querySelector("audio")!.dataset.offline = "true";
      dispatchEvent(new PageTransitionEvent("pagehide"));
    });
    else await page.locator("audio").evaluate((audio, owner) => {
      if (owner === "cast") audio.dataset.castActive = "true";
      else audio.dataset.room = "existing-room";
    }, boundary);
    await lookup!.fulfill({json: {item: queueItem("next"), profileId: "qa-viewer"}});
    if (boundary !== "pagehide") await expect(page.locator("[data-audio-queue-status]")).toHaveAttribute("data-queue-failure", "ownership");
    // Busy clears only when the held queue operation completes, so the old
    // identity assertions cannot pass before its response is consumed.
    await expect(page.locator("[data-audio-queue-controls]")).not.toHaveAttribute("aria-busy");
    await expect(page.locator("[data-audio-next]")).toBeDisabled();
    await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/track");
    await expect(page.locator(".title-block h1")).toHaveText("First track");
    expect(writes).toEqual([]);
    if (boundary === "pagehide") await expect(page.locator("audio")).not.toHaveAttribute("src", /media\/next/);
  });
}

test("queued short track resumes its saved position without claiming unplayed progress", async ({page}) => {
  await openAudio(page, "");
  await page.locator("audio").evaluate(audio => Object.defineProperty(audio, "duration", {value: 8}));
  await page.route("https://audio.test/api/v1/items/next", route => route.fulfill({json: {
    item: {...queueItem("next"), progress: {seconds: 3, watched: false}}, profileId: "qa-viewer",
  }}));
  const nextWrites: string[] = [];
  page.on("request", request => {
    if (new URL(request.url()).pathname === "/progress/next") nextWrites.push(request.postData() || "");
  });
  await startQueue(page);
  await page.getByRole("button", {name: "Next track", exact: true}).click();
  await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/next");
  await expect(page.locator("audio")).toHaveJSProperty("currentTime", 3);
  await page.locator("audio").dispatchEvent("pause");
  expect(nextWrites).toEqual([]);
});

test("late initial queue response cannot warm media or publish controls after pagehide", async ({page}) => {
  await openAudio(page, "");
  let lookup: import("@playwright/test").Route | undefined;
  const nextReads: string[] = [];
  page.on("request", request => {
    if (new URL(request.url()).pathname === "/media/next") nextReads.push(request.url());
  });
  await page.route("https://audio.test/api/v1/audio/track/queue", route => {lookup = route;});
  await page.addScriptTag({content: playerSource});
  await expect.poll(() => !!lookup).toBe(true);
  await expect(page.locator("[data-audio-queue-controls]")).toHaveAttribute("aria-busy", "true");
  await page.evaluate(() => dispatchEvent(new PageTransitionEvent("pagehide")));
  await lookup!.fulfill({json: {items: [queueItem("track"), queueItem("next")]}});
  await expect(page.locator("[data-audio-queue-controls]")).not.toHaveAttribute("aria-busy");
  await expect(page.locator("[data-audio-next]")).toBeDisabled();
  await expect(page.locator("audio")).not.toHaveAttribute("data-queue-total");
  expect(nextReads).toEqual([]);
});
