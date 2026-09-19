import { expect, test } from "@playwright/test";
import { readStaticSource } from "./static-sources";

const css = await readStaticSource(["../../../packages/webassets/static/player-app.css", "../../../packages/webassets/static/player-stage.css"]);

for (const width of [320, 390, 430, 700, 1024]) {
  for (const recovery of [false, true]) {
    test(`player toolbar stays clear of ${recovery ? "recovery" : "buffering"} at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await page.setContent(`<body class="player-page"><main class="player-shell">
        <div class="media-stage is-busy">
          <video style="aspect-ratio:16/9" aria-label="1917"></video>
          <div class="player-stage-toolbar"><strong>1917 — a long movie title</strong>
            <button class="player-mode-status">Starting Transcoding audio</button>
            <div class="player-stage-actions"><button class="quiet play-on-tv" data-tv-open>Play on TV</button></div>
          </div>
          <div class="player-buffer ${recovery ? "is-recovery" : ""}" role="status">
            <span class="buffer-skeleton"></span><span>${recovery ? "Playback stopped. Try again to continue watching." : "Buffering…"}</span>
            ${recovery ? '<button>Try again</button>' : '<progress></progress>'}
          </div>
        </div></main></body>`);
      await page.addStyleTag({ content: css });
      const toolbar = await page.locator(".player-stage-toolbar").boundingBox();
      const status = await page.locator(".player-buffer").boundingBox();
      expect(toolbar).not.toBeNull();
      expect(status).not.toBeNull();
      expect(toolbar!.y + toolbar!.height).toBeLessThanOrEqual(status!.y);
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
      await expect(page.getByRole("button", { name: "Play on TV" })).toBeVisible();
      await page.locator(".player-buffer").evaluate(el => el.setAttribute("hidden", ""));
      await expect(page.locator(".player-buffer")).toBeHidden();
      await page.locator("body").evaluate(el => el.classList.add("player-theater"));
      await expect(page.locator(".player-stage-toolbar")).toHaveCSS("position", "absolute");
    });
  }
}
