import { expect, test } from "@playwright/test";
import { readStaticSource } from "./static-sources";

const source = await readStaticSource(["../internal/server/static/player-subtitles.js"]);
const caption = "WEBVTT\n\n00:00:01.000 --> 00:01:00.000\nHello\n";

async function install(page: import("@playwright/test").Page, path = "/captions.vtt") {
  await page.route("http://subtitle.test/", (route) => route.fulfill({contentType: "text/html", body: `<video><track default kind="subtitles" label="English" data-subtitle-source="${path}"></video><select data-subtitles><option value="0">English</option><option value="off">Off</option></select><small data-subtitle-status hidden></small>`}));
  await page.goto("http://subtitle.test/");
  await page.addScriptTag({content: `const player = document.querySelector("video");\n${source}`});
}

test("pending default captions never enter the media loading path", async ({page}) => {
  let release!: () => void;
  const held = new Promise<void>((resolve) => { release = resolve; });
  await page.route("**/captions.vtt", async (route) => { await held; await route.fulfill({contentType: "text/vtt", body: caption}); });
  await install(page);
  await expect(page.locator("[data-subtitle-status]")).toHaveText("Loading subtitles…");
  await expect(page.locator("track")).not.toHaveAttribute("src", /.+/);
  release();
  await expect(page.locator("track")).toHaveAttribute("src", /^blob:/);
  await expect(page.locator("[data-subtitle-status]")).toBeHidden();
});

test("turning captions off while loading preserves the choice", async ({page}) => {
  let release!: () => void;
  const held = new Promise<void>((resolve) => { release = resolve; });
  await page.route("**/captions.vtt", async (route) => { await held; await route.fulfill({contentType: "text/vtt", body: caption}); });
  await install(page);
  await page.locator("track").evaluate((element: HTMLTrackElement) => { element.track.mode = "disabled"; });
  release();
  await expect(page.locator("track")).toHaveAttribute("src", /^blob:/);
  expect(await page.locator("track").evaluate((element: HTMLTrackElement) => element.track.mode)).toBe("disabled");
  await expect(page.locator("[data-subtitle-status]")).toBeHidden();
});

for (const failure of ["http", "type", "empty", "oversized"]) {
  test(`rejects ${failure} captions without assigning a media source`, async ({page}) => {
    await page.route("**/captions.vtt", (route) => route.fulfill({status: failure === "http" ? 503 : 200, contentType: failure === "type" ? "text/vtt-invalid" : "text/vtt", body: failure === "empty" ? "" : failure === "oversized" ? "x".repeat(16 * 1024 * 1024 + 1) : caption}));
    await install(page);
    await expect(page.locator("[data-subtitle-status]")).toContainText("Subtitles unavailable");
    await expect(page.locator("track")).not.toHaveAttribute("src", /.+/);
  });
}

test("rejects a foreign caption origin before any request", async ({page}) => {
  const requests: string[] = [];
  page.on("request", (request) => requests.push(request.url()));
  await install(page, "https://foreign.invalid/captions.vtt");
  await expect(page.locator("[data-subtitle-status]")).toContainText("Subtitles unavailable");
  expect(requests.some((url) => url.includes("foreign.invalid"))).toBe(false);
  await expect(page.locator("track")).not.toHaveAttribute("src", /.+/);
});

test("failed captions can be retried through the existing selector", async ({page}) => {
  let calls = 0;
  await page.route("**/captions.vtt", (route) => route.fulfill({status: ++calls === 1 ? 503 : 200, contentType: "text/vtt", body: caption}));
  await install(page);
  await expect(page.locator("[data-subtitle-status]")).toContainText("Subtitles unavailable");
  await page.locator("select").dispatchEvent("change");
  await expect(page.locator("track")).toHaveAttribute("src", /^blob:/);
  await expect(page.locator("[data-subtitle-status]")).toBeHidden();
});
