import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { configureTestInstance, firstPlayable, login, saveSubtitleChoices } from "./test-instance-helpers";

configureTestInstance();
test.use({ serviceWorkers: "block" });

test("subtitle choices wait for their real save before settings navigation", {tag: "@smoke"}, async ({page}, info) => {
  await login(page);
  await page.goto("/settings#playback");
  const choice = page.getByLabel("Playback subtitle choices");
  const previous = await choice.inputValue(), next = previous === "on" ? "off" : "on";
  let held = false, settled = false, release!: () => void;
  const barrier = new Promise<void>(resolve => release = resolve);
  await page.route("**/settings/subtitles/picker", async route => {
    if (route.request().method() !== "POST") { await route.continue(); return; }
    held = true;
    await barrier;
    await route.continue();
  });
  const attempt = saveSubtitleChoices(page, next).finally(() => {settled = true;});
  const observed = attempt.then(() => ({ok: true}), error => ({ok: false, error: String(error)}));
  try {
    await expect.poll(() => held).toBe(true);
    expect(settled).toBe(false);
    expect((await (await page.request.get("/api/v1/settings")).json()).subtitlePickerLimited).toBe(previous === "on");
    release();
    expect(await observed).toEqual({ok: true});
    await expect(choice).toHaveValue(next);
    expect((await (await page.request.get("/api/v1/settings")).json()).subtitlePickerLimited).toBe(next === "on");
    await info.attach("subtitle-choice-save-barrier", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
      browser: info.project.name, previous, next, held, settled, data: "Real populated Server form and persisted settings; delayed HTTP POST", result: "passed"}), contentType: "application/json"});
  } finally {
    release();
    await observed;
    await page.unrouteAll({behavior: "ignoreErrors"});
    await page.goto("/settings#playback");
    await saveSubtitleChoices(page, previous as "on" | "off");
  }
});

test("volume icon renders balanced sound waves and keeps accessible mute controls", { tag: "@smoke" }, async ({ page }, info) => {
  await login(page);
  await page.goto("/settings#playback");
  const choices = page.locator('form[action="/settings/subtitles/picker"]');
  const previous = await choices.getByLabel("Playback subtitle choices").inputValue();
  expect(["on", "off"]).toContain(previous);
  await saveSubtitleChoices(page, "on");
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
    await saveSubtitleChoices(page, previous as "on" | "off");
  }
});
