import { expect, test, type Page } from "@playwright/test";
import { playerSource } from "./static-sources";

async function openAudio(page: Page, policy: string, homeAssistant = false) {
  await page.addInitScript((value) => localStorage.setItem("kinosail.playback-policy-v2", value), policy);
  await page.route("https://audio.test/**", (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/watch/track") return route.fulfill({ contentType: "text/html", body: `
      <!doctype html><meta name="kinosail-csrf" content="session-csrf-token">
      <audio controls src="/stream/track" data-progress="/progress/track" data-queue="/queue/track" data-home-assistant="${homeAssistant}"></audio>` });
    if (path === "/queue/track") return route.fulfill({ json: { items: [
      { id: "track", stream: "/stream/track", title: "First track" },
      { id: "next", stream: "/stream/next", title: "Next track" },
    ] } });
    return route.fulfill({ status: 204 });
  });
  await page.goto("https://audio.test/watch/track");
  await page.evaluate(() => {
    const media = document.querySelector("audio")!;
    media.addEventListener("error", (event) => event.stopImmediatePropagation(), true);
    Object.defineProperties(media, {
      currentTime: { value: 12, writable: true },
      duration: { value: 120 },
      readyState: { value: HTMLMediaElement.HAVE_ENOUGH_DATA },
      play: { value: async () => {} },
      load: { value: () => {} },
    });
  });
}

for (const policy of ["compatible", "direct-first", "direct-only", "", "unknown"]) {
  test(`audio keeps its source and queue with saved policy ${policy || "unset"}`, async ({ page }) => {
    const errors: string[] = [];
    const navigation: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await openAudio(page, policy);
    page.on("request", (request) => { if (request.isNavigationRequest()) navigation.push(request.url()); });
    const queue = page.waitForResponse("https://audio.test/queue/track");
    await page.addScriptTag({ content: playerSource });
    await queue;
    await expect(page.locator("audio")).toHaveAttribute("src", "/stream/track");
    await page.locator("audio").dispatchEvent("ended");
    await expect(page.locator("audio")).toHaveAttribute("src", "/stream/next");
    await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/next");
    expect(await page.evaluate(() => localStorage.getItem("kinosail.playback-policy-v2"))).toBe(policy);
    expect(navigation).toEqual([]);
    expect(errors).toEqual([]);
  });
}

test("Home Assistant state uses the session CSRF token and consumes a command", async ({ page }) => {
  await openAudio(page, "", true);
  const reports: { csrf?: string; body: { itemId: string; position: number; duration: number } }[] = [];
  await page.route("https://audio.test/api/v1/home-assistant/players/*", async (route) => {
    reports.push({ csrf: route.request().headers()["x-kinosail-csrf"], body: route.request().postDataJSON() });
    await route.fulfill({ json: { command: "seek", position: 35 } });
  });
  await page.addScriptTag({ content: playerSource });
  await expect.poll(() => reports.length).toBe(1);
  expect(reports[0].csrf).toBe("session-csrf-token");
  expect(reports[0].body).toMatchObject({ itemId: "track", position: 12, duration: 120 });
  await expect(page.locator("audio")).toHaveJSProperty("currentTime", 35);
});
