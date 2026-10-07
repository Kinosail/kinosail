import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { configureTestInstance, firstPlayable, login } from "./test-instance-helpers";

configureTestInstance();
test.use({ serviceWorkers: "block" });

test("volume icon renders balanced sound waves and keeps accessible mute controls", { tag: "@smoke" }, async ({ page }, info) => {
  await login(page);
  await page.goto("/settings#playback");
  const choices = page.locator('form[action="/settings/subtitles/picker"]');
  const previous = await choices.getByLabel("Playback subtitle choices").inputValue();
  await choices.getByLabel("Playback subtitle choices").selectOption("on");
  await choices.getByRole("button", { name: "Save subtitle choices" }).click();
  try {
    const watch = await firstPlayable(page);
    for (const viewport of [{ width: 844, height: 390 }, { width: 1440, height: 900 }, { width: 1920, height: 1080 }]) {
      const { width } = viewport;
      await page.setViewportSize(viewport);
      await page.goto(watch);
      const video = page.locator("video");
      await video.evaluate((media: HTMLVideoElement) => media.pause());
      const mute = page.locator("[data-player-mute]");
      await expect(mute).toBeVisible();
      await expect(mute).toHaveAccessibleName("Mute");
      await expect(mute).toHaveAttribute("aria-pressed", "false");
      const icon = mute.locator("svg");
      await expect(icon).toHaveAttribute("aria-hidden", "true");
      await page.locator(".media-stage").screenshot({ path: info.outputPath(`${width}-volume-controls.png`) });
      await mute.screenshot({ path: info.outputPath(`${width}-volume-icon.png`) });
      const pixels = await icon.evaluate(async (svg: SVGSVGElement) => {
        const source = svg.cloneNode(true) as SVGSVGElement;
        source.setAttribute("width", "96");
        source.setAttribute("height", "96");
        source.style.fill = getComputedStyle(svg).fill;
        const image = new Image();
        image.src = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(new XMLSerializer().serializeToString(source))}`;
        await image.decode();
        const canvas = document.createElement("canvas");
        canvas.width = canvas.height = 96;
        const context = canvas.getContext("2d")!;
        context.drawImage(image, 0, 0);
        const data = context.getImageData(0, 0, 96, 96).data;
        let ink = 0, difference = 0;
        for (let y = 0; y < 96; y++) for (let x = 0; x < 96; x++) {
          const alpha = data[(y * 96 + x) * 4 + 3];
          ink += alpha;
          difference += Math.abs(alpha - data[((95 - y) * 96 + x) * 4 + 3]);
        }
        return { ink, asymmetry: difference / ink };
      });
      expect(pixels.ink).toBeGreaterThan(100_000);
      // Both speaker and sound waves mirror across the icon's horizontal center.
      expect(pixels.asymmetry).toBeLessThan(0.03);
      expect((await new AxeBuilder({ page }).include(".player-control-row").analyze()).violations).toEqual([]);
      await mute.focus();
      await mute.press("Enter");
      await expect(mute).toHaveAccessibleName("Unmute");
      await expect(mute).toHaveAttribute("aria-pressed", "true");
      expect(await video.evaluate((media: HTMLVideoElement) => media.muted)).toBe(true);
      await mute.press("Enter");
      await expect(mute).toHaveAccessibleName("Mute");
      expect(await video.evaluate((media: HTMLVideoElement) => media.muted)).toBe(false);
    }
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(page.locator("[data-player-mute]")).toBeHidden();
    await page.locator(".media-stage").screenshot({ path: info.outputPath("390-compact-controls.png") });
  } finally {
    await page.goto("/settings#playback");
    await choices.getByLabel("Playback subtitle choices").selectOption(previous);
    await choices.getByRole("button", { name: "Save subtitle choices" }).click();
  }
});
