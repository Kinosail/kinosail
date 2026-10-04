import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { expect, test } from "@playwright/test";
import { reviewSubtitlePairing } from "./subtitle-pairing-journey";

const manifestPath = process.env.KINOSAIL_SUBTITLE_PAIRING_NATIVE_MANIFEST;
test.skip(!manifestPath, "requires the disposable native-fixture ready.json");

for (const width of [390, 1440]) {
  test(`native Server pairs dialogue with playable media at ${width}px`, async ({ page }, testInfo) => {
    const manifest = JSON.parse(await readFile(manifestPath!, "utf8"));
    const root = dirname(manifestPath!);
    const sidecar = join(root, "media", "R07 Example.en.srt");
    const original = await readFile(sidecar);
    expect(createHash("sha256").update(original).digest("hex")).toBe(manifest.sidecarSHA256);
    const rejectedMutations = [];
    for (const action of ["apply", "restore"]) {
      // Invalid JSON remains harmless if this fixture's outer mutation guard ever regresses.
      const response = await page.request.post(`${manifest.url}/api/v1/subtitle-library/${manifest.id}/${action}`, { data: "invalid-json", headers: { "Content-Type": "application/json" } });
      expect(response.status()).toBe(405);
      expect(await response.text()).toContain("disposable fixture permits reads and previews only");
      rejectedMutations.push({ action, status: response.status() });
    }
    const requests: { method: string; path: string }[] = [];
    const media: { status: number; contentRange: string | undefined }[] = [];
    page.on("request", request => requests.push({ method: request.method(), path: new URL(request.url()).pathname }));
    page.on("response", response => {
      if (new URL(response.url()).pathname.startsWith("/media/")) media.push({ status: response.status(), contentRange: response.headers()["content-range"] });
    });
    await testInfo.attach("native-verification-context", { contentType: "application/json", body: JSON.stringify({ ...manifest, width, browser: testInfo.project.name, browserVersion: page.context().browser()?.version(), command: "playwright test subtitle-pairing-native.spec.ts --workers=1", routing: "no Playwright routes; actual loopback HTML, assets, inspect, preview, and media responses" }) });
    await page.setViewportSize({ width, height: 900 });
    await page.goto(`${manifest.url}/subtitles/inspect/${manifest.id}?language=en`);
    const video = page.locator("video");
    await expect.poll(() => video.evaluate(video => video.readyState)).toBeGreaterThanOrEqual(2);
    expect(await video.evaluate(video => video.duration)).toBe(12);
    await video.evaluate(video => { video.muted = true; return video.play(); });
    await expect.poll(() => video.evaluate(video => video.currentTime)).toBeGreaterThan(0.2);
    await expect.poll(() => video.evaluate(video => video.getVideoPlaybackQuality().totalVideoFrames)).toBeGreaterThan(0);
    await video.evaluate(video => { video.pause(); video.currentTime = 0; });
    await reviewSubtitlePairing(page, testInfo);
    expect(media.some(response => response.status === 206 && response.contentRange?.endsWith("/396548"))).toBe(true);
    expect(requests.filter(request => request.method !== "GET").map(request => request.path)).toEqual([`/api/v1/subtitle-library/${manifest.id}/preview`, `/api/v1/subtitle-library/${manifest.id}/preview`]);
    expect(await readFile(sidecar)).toEqual(original);
    await expect(readFile(sidecar + ".kinosail.bak")).rejects.toMatchObject({ code: "ENOENT" });
    await testInfo.attach("native-transport-and-integrity", { contentType: "application/json", body: JSON.stringify({ requests, media, rejectedMutations, installedSubtitleUnchanged: true, recoverySidecarAbsent: true, directPlaybackDuration: 12 }) });
  });
}
