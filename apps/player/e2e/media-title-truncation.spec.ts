import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";

const styles = await Promise.all([
  "../../../packages/webassets/static/player-app.css",
  "../../../packages/webassets/static/last-light.css",
  "../internal/server/static/home.css",
].map(path => readFile(new URL(path, import.meta.url), "utf8")));

test("media cards keep long titles to two visible lines", { tag: "@smoke" }, async ({ page }, testInfo) => {
  const longTitle = "Borat: Cultural Learnings of America for Make Benefit Glorious Nation of Kazakhstan";
  for (const width of [390, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    await page.setContent(`<html data-theme="dark"><body class="library-page"><main class="library-shell last-light">
      <div class="grid rail">
        <a class="card" href="/watch/long"><div class="poster art"></div><h2>${longTitle}</h2></a>
        <a class="card" href="/watch/short"><div class="poster art"></div><h2>Blade Runner 2049</h2></a>
        <a class="card" href="/watch/cast"><div class="poster art"></div><h3>${longTitle}</h3></a>
      </div>
    </main></body></html>`);
    for (const content of styles) await page.addStyleTag({ content });
    for (const heading of await page.locator(".card h2, .card h3").all()) {
      const size = await heading.evaluate(element => {
        const lineHeight = parseFloat(getComputedStyle(element).lineHeight);
        return { height: element.getBoundingClientRect().height, lineHeight, clipped: element.scrollHeight > element.clientHeight + 1 };
      });
      expect(size.height, `title height at ${width}px`).toBeLessThanOrEqual(size.lineHeight * 2 + 1);
      if (await heading.textContent() === longTitle) expect(size.clipped, `long title at ${width}px`).toBe(true);
    }
    await expect(page.getByRole("link", { name: longTitle }).first()).toHaveAttribute("href", "/watch/long");
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
    await page.screenshot({ path: testInfo.outputPath(`titles-${width}.png`) });
  }
});
