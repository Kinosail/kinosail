import { execFileSync } from "node:child_process";
import { writeFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";
import { configureTestInstance, login } from "./test-instance-helpers";

configureTestInstance();
test.use({ serviceWorkers: "block" });

for (const viewport of [{ width: 390, height: 650 }, { width: 1440, height: 900 }]) {
  test(`compressed assets execute and reuse the warm cache at ${viewport.width}px`, async ({ page, request }, testInfo) => {
    await page.setViewportSize(viewport);
    await login(page);
    const asset = await page.locator('link[rel="stylesheet"][href^="/static/app.css"]').getAttribute("href");
    expect(asset).toBeTruthy();
    const plain = await request.get(asset!, { headers: { "Accept-Encoding": "identity" } });
    const compressed = await request.get(asset!, { headers: { "Accept-Encoding": "gzip" } });
    expect(plain.ok()).toBeTruthy();
    expect(compressed.ok()).toBeTruthy();
    expect(compressed.headers()["content-encoding"]).toBe("gzip");
    expect(compressed.headers()["vary"]).toContain("Accept-Encoding");
    expect(compressed.headers()["cache-control"]).toBe("public, max-age=31536000, immutable");
    expect(await compressed.body()).toEqual(await plain.body());
    const plainBytes = Number(plain.headers()["content-length"]);
    const compressedBytes = Number(compressed.headers()["content-length"]);
    expect(compressedBytes).toBeGreaterThan(0);
    expect(compressedBytes).toBeLessThan(plainBytes / 2);

    await page.goto("/?view=movies");
    await expect(page.locator("a.card").first()).toBeVisible();
    await expect(page.locator("html")).toHaveClass(/\bjs\b/);
    const cards = await page.locator("a.card").count();
    expect(cards).toBeGreaterThan(0);
    expect(cards).toBeLessThanOrEqual(100);
    await page.goto("/?view=movies&sort=title");
    await expect(page.locator("a.card").first()).toBeVisible();
    const resources = await page.evaluate(() => performance.getEntriesByType("resource").map((entry) => {
      const resource = entry as PerformanceResourceTiming;
      return { path: new URL(resource.name).pathname, duration: resource.duration, transferSize: resource.transferSize, encodedBodySize: resource.encodedBodySize, decodedBodySize: resource.decodedBodySize };
    }));
    const stylesheet = resources.find((entry) => entry.path === "/static/app.css");
    expect(stylesheet?.transferSize).toBe(0);
    if (viewport.width < 600) {
      await page.getByRole("button", { name: "Jump to title" }).click();
      await expect(page.getByRole("dialog", { name: "Jump to title" })).toBeVisible();
    }
    await page.screenshot({ path: testInfo.outputPath("loaded-library.png") });
    const evidence = {
      revision: process.env.KINOSAIL_E2E_REVISION ?? execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim(),
      command: "pnpm --dir e2e exec playwright test static-compression.spec.ts --project=chromium --workers=1",
      fixture: { populatedMovies: cards, serviceWorkers: "blocked" },
      environment: { viewport, browser: testInfo.project.name, version: page.context().browser()?.version() },
      result: "passed", plainBytes, compressedBytes, resources,
    };
    const artifact = testInfo.outputPath("compression-evidence.json");
    await writeFile(artifact, JSON.stringify(evidence, null, 2));
    await testInfo.attach("compression-evidence", { path: artifact, contentType: "application/json" });
  });
}
