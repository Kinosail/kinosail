import { expect, test } from "@playwright/test";
import { installPlayerExperienceFixture } from "./player-experience-fixture";
import { readStaticSource } from "./static-sources";

installPlayerExperienceFixture();

test.beforeEach(async ({ page }) => {
  await page.addStyleTag({ content: await readStaticSource([
    "../../../packages/webassets/static/last-light.css",
    "../internal/server/static/home.css",
  ]) });
  await page.locator("main").evaluate(main => {
    main.insertAdjacentHTML("beforeend", `<section class="cast-section"><header><h2>Cast</h2></header>
      <div class="grid rail">${Array.from({ length: 12 }, (_, index) =>
        `<a class="card" href="#actor-${index}"><span class="art"></span><h3>Actor ${index}</h3><small>Character</small></a>`
      ).join("")}</div></section>`);
  });
});

for (const width of [320, 390, 393, 430, 700, 768, 1440]) {
  test(`cast scrolls inside the playback page at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 844 });
    const rail = page.locator(".cast-section .rail");
    for (const busy of [true, false]) {
      await page.locator(".media-stage").evaluate((stage, busy) => stage.classList.toggle("is-busy", busy), busy);
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
      await rail.evaluate(element => element.scrollTo({ left: element.scrollWidth, behavior: "instant" }));
      expect(await rail.evaluate(element => element.scrollLeft)).toBeGreaterThan(0);
      await expect(rail.getByRole("link").last()).toBeInViewport();
      await page.evaluate(() => window.scrollTo({ left: 100, behavior: "instant" }));
      expect(await page.evaluate(() => window.scrollX)).toBe(0);
      await rail.evaluate(element => element.scrollTo({ left: 0, behavior: "instant" }));
    }
  });

  test(`playback controls with Picture-in-Picture remain inside the picture at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 844 });
    // Exercise the longest time label and optional Picture-in-Picture control.
    await page.locator("[data-player-time]").evaluate(element => element.textContent = "1:23:45 / 2:34:56");
    for (const theater of [false, true]) {
      if (theater) await page.getByRole("button", { name: "Theater", exact: true }).click();
      await page.locator(".media-stage").scrollIntoViewIfNeeded();
      const geometry = await page.locator(".player-control-row").evaluate(row => {
        const stage = row.closest(".media-stage")!.getBoundingClientRect();
        return [...row.querySelectorAll("button")].filter(button => button.getBoundingClientRect().width > 0).map(button => {
          const box = button.getBoundingClientRect();
          const hit = document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2);
          return { name: button.getAttribute("aria-label"), inside: box.left >= stage.left && box.right <= stage.right && box.top >= stage.top && box.bottom <= stage.bottom,
            reachable: hit === button || !!hit && button.contains(hit) };
        });
      });
      expect(geometry.length).toBeGreaterThan(0);
      for (const button of geometry) {
        expect(button.inside, button.name ?? "control").toBe(true);
        expect(button.reachable, button.name ?? "control").toBe(true);
      }
    }
  });
}
