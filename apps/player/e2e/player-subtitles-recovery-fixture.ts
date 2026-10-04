import { expect, type Page } from "@playwright/test";
import { createServer, type ServerResponse } from "node:http";
import { createHash } from "node:crypto";
import { readStaticSource } from "./static-sources";

const source = await readStaticSource(["../internal/server/static/player-subtitles.js"]);
export const caption = "WEBVTT\n\n00:00:01.000 --> 00:01:00.000\nRecovered captions\n";
export const serverOrigin = process.env.KINOSAIL_CAPTION_BROWSER_URL;
export const isolated = process.env.KINOSAIL_CAPTION_ISOLATED === "1";

export async function captionPeer(page: Page, mode: "headers" | "body") {
  if (serverOrigin) {
    expect((await page.request.post(`${serverOrigin}/__caption-fixture?mode=${mode}`)).ok()).toBe(true);
    return {
      origin: serverOrigin,
      async stats() { return (await page.request.get(`${serverOrigin}/__caption-fixture`)).json() as Promise<{calls: number; closed: number}>; },
      async close() {},
    };
  }
  const pending = new Set<ServerResponse>();
  let calls = 0;
  let closed = 0;
  // Isolated real HTTP transport. The shell does not establish Go rendering or media decode.
  const web = createServer((request, response) => {
    if (request.url === "/captions.vtt") {
      calls += 1;
      if (calls > 1) {
        response.writeHead(200, { "Content-Type": "text/vtt" });
        response.end(caption);
        return;
      }
      pending.add(response);
      response.on("close", () => { pending.delete(response); closed += 1; });
      if (mode === "body") {
        response.writeHead(200, { "Content-Type": "text/vtt", "X-Request-ID": "caption-fixture-1" });
        response.write("WEBVTT\n\n");
      }
      return;
    }
    response.writeHead(200, { "Content-Type": "text/html" });
    response.end(`<video aria-label="Fixture video"><track default kind="subtitles" label="English" srclang="en" data-subtitle-source="/captions.vtt"></video><label>Subtitles<select data-subtitles><option value="0">English</option><option value="off">Off</option></select></label><small role="status" data-subtitle-status hidden></small><button type="button" data-subtitle-retry hidden>Retry subtitles</button>`);
  });
  await new Promise<void>((resolve) => web.listen(0, "127.0.0.1", resolve));
  const address = web.address();
  if (!address || typeof address === "string") throw new Error("Missing fixture port");
  return {
    origin: `http://127.0.0.1:${address.port}`,
    async stats() { return {calls, closed}; },
    async close() {
      for (const response of pending) response.destroy();
      web.closeAllConnections();
      await new Promise<void>((resolve, reject) => web.close((error) => error ? reject(error) : resolve()));
    },
  };
}

export async function openCaptionPlayer(page: Page, origin: string) {
  await page.clock.install();
  if (serverOrigin) {
    const response = await page.request.get(`${origin}/api/v1/library`);
    expect(response.ok()).toBe(true);
    const library = await response.json();
    expect(library.items).toHaveLength(1);
    expect(library.items[0].title).toBe("Caption Recovery");
    await page.goto(`${origin}/watch/${library.items[0].id}?playback=direct`);
    const bundle = await page.locator('script[src^="/static/player.js"]').getAttribute("src");
    const asset = new URL(bundle!, origin);
    expect(asset.origin).toBe(origin);
    const version = asset.searchParams.get("v");
    expect(version).toMatch(/^[a-f0-9]{64}$/);
    const responseAsset = await page.request.get(asset.toString());
    expect(responseAsset.ok()).toBe(true);
    const checksum = createHash("sha256").update(await responseAsset.body()).digest("hex");
    expect(checksum).toBe(version);
    await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.readyState)).toBeGreaterThan(0);
    await page.locator("video").evaluate(async (video: HTMLVideoElement) => { video.muted = true; video.currentTime = 0; await video.play(); });
    await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(0.1);
    await page.getByRole("button", {name: "Settings", exact: true}).first().click();
    return { version, checksum };
  } else {
    await page.goto(origin);
    await page.addScriptTag({ content: `const player = document.querySelector("video");\n${source}` });
    return null;
  }
}
