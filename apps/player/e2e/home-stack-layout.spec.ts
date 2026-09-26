import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";

const styles = await Promise.all([
  "../../../packages/webassets/static/player-app.css",
  "../../../packages/webassets/static/last-light.css",
  "../internal/server/static/home.css",
].map(path => readFile(new URL(path, import.meta.url), "utf8")));

test("Home media shelves wrap down the page without horizontal scrolling", { tag: "@smoke" }, async ({ page }, testInfo) => {
  await page.setContent(`<html data-theme="dark"><body class="library-page"><main class="library-shell last-light">
    <section class="home-shelf" data-home-shelf="recent-movies"><header><h2>Recently added movies</h2></header>
      <div class="grid rail recent-grid">${Array.from({ length: 10 }, (_, index) => `<a class="card recent-card" href="/item/${index}"><div class="poster"></div><h2>Movie ${index + 1}</h2></a>`).join("")}</div>
    </section></main></body></html>`);
  for (const content of styles) await page.addStyleTag({ content });
  for (const width of [390, 768, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    const layout = await page.locator('[data-home-shelf="recent-movies"] .recent-grid').evaluate(grid => {
      const cards = [...grid.querySelectorAll<HTMLElement>(".card")];
      return {
        pageWidth: document.documentElement.scrollWidth,
        gridWidth: grid.clientWidth,
        scrollWidth: grid.scrollWidth,
        firstTop: cards[0].getBoundingClientRect().top,
        lastTop: cards.at(-1)!.getBoundingClientRect().top,
      };
    });
    expect(layout.pageWidth, `${width}px page overflow`).toBeLessThanOrEqual(width);
    expect(layout.scrollWidth, `${width}px shelf overflow`).toBeLessThanOrEqual(layout.gridWidth + 1);
    expect(layout.lastTop, `${width}px cards do not wrap`).toBeGreaterThan(layout.firstTop);
    await page.screenshot({ path: testInfo.outputPath(`home-shelves-${width}.png`), fullPage: true });
  }
});
