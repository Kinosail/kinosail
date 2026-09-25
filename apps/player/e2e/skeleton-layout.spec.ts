import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";

const styles = await Promise.all([
  "../../../packages/webassets/static/player-app.css",
  "../../../packages/webassets/static/last-light.css",
  "../internal/server/static/home.css",
].map(path => readFile(new URL(path, import.meta.url), "utf8")));

for (const width of [390, 1440]) {
  for (const artwork of ["poster", "backdrop"]) {
    test(`Player loading keeps ${artwork} Home geometry at ${width}px`, { tag: "@smoke" }, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 900 });
      const image = `data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='400' height='600'/%3E`;
      await page.setContent(`<html data-theme="dark"><body class="library-page"><main id="main" class="library-shell last-light">
        <nav class="home-sections"><a>For you</a><a>My List</a></nav>
        <section class="home-feature" data-artwork="${artwork}">
          <img src="${image}" width="${artwork === "poster" ? 400 : 1600}" height="${artwork === "poster" ? 600 : 900}" alt="">
          <div class="home-feature-copy"><h2>Example movie title</h2><div class="home-feature-status"><p>1 hour left</p></div>
          <div class="home-feature-actions"><a class="button">Resume</a><a class="button quiet">Details</a></div></div>
        </section>
        <section class="home-shelf continue-shelf"><header><h2>Continue watching</h2></header><div class="grid rail resume-grid">
          <article class="card resume-card"><a class="resume-link"><img class="poster" src="${image}" width="400" height="600" alt=""><h3>Example movie</h3><small>1 hour left</small><span class="resume-action">Play</span></a></article>
        </div></section>
        <section class="home-shelf"><header><h2>Recently added</h2></header><div class="grid rail"><a class="card"><img class="poster" src="${image}" width="400" height="600" alt=""><h2>Another movie</h2></a></div></section>
      </main></body></html>`);
      for (const content of styles) await page.addStyleTag({ content });
      const selectors = [".home-feature", ".home-feature > img", ".resume-card", ".resume-link .poster", ".home-shelf .card:not(.resume-card)"];
      const boxes = await Promise.all(selectors.map(selector => page.locator(selector).boundingBox()));
      await page.locator("#main").evaluate(main => main.classList.add("request-skeleton"));
      if (artwork === "poster") await page.screenshot({ path: testInfo.outputPath(`${width}-pending.png`), fullPage: true });
      const pending = await Promise.all(selectors.map(selector => page.locator(selector).boundingBox()));
      for (let index = 0; index < boxes.length; index++) {
        expect(pending[index]?.width).toBeCloseTo(boxes[index]!.width, 0);
        expect(pending[index]?.height).toBeCloseTo(boxes[index]!.height, 0);
      }
      await expect(page.locator(".home-feature > img")).toHaveCSS("visibility", "hidden");
      await expect(page.locator(".resume-link .poster")).toHaveCSS("visibility", "hidden");
      await expect(page.locator(".home-feature .button").first()).toHaveCSS("color", "rgba(0, 0, 0, 0)");
      await expect(page.locator(".home-feature h2")).toHaveCSS("animation-name", "skeleton-shimmer");
      await expect(page.locator(".home-feature .button").first()).toHaveCSS("animation-name", "skeleton-shimmer");
      expect(await page.locator(".home-feature").evaluate(feature => getComputedStyle(feature, "::before").animationName)).toBe("skeleton-shimmer");
      await page.emulateMedia({ reducedMotion: "reduce" });
      await expect(page.locator(".home-feature h2")).toHaveCSS("animation-name", "none");
      expect(await page.locator(".home-feature").evaluate(feature => getComputedStyle(feature, "::before").animationName)).toBe("none");
      await page.locator("#main").evaluate(main => main.classList.remove("request-skeleton"));
      await expect(page.locator(".home-feature > img")).toHaveCSS("visibility", "visible");
      if (artwork === "poster") await page.screenshot({ path: testInfo.outputPath(`${width}-loaded.png`), fullPage: true });
    });
  }
}
