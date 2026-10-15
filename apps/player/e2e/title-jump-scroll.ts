import { expect, type Page, type TestInfo } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

async function record(page: Page, info: TestInfo, name: string) {
  await expect(page.locator('#main[aria-busy="true"]')).toHaveCount(0);
  const receipt = await page.evaluate(async () => {
    await document.fonts.ready;
    const started = performance.now();
    const samples: { at: number; y: number }[] = [];
    await new Promise<void>(resolve => {
      const sample = () => {
        const at = performance.now() - started;
        samples.push({ at, y: window.scrollY });
        if (at >= 800) resolve();
        else requestAnimationFrame(sample);
      };
      requestAnimationFrame(sample);
    });
    const rect = (element: Element) => {
      const { x, y, width, height } = element.getBoundingClientRect();
      return { x, y, width, height };
    };
    const settled = samples.filter(sample => sample.at >= 550);
    return {
      viewport: { width: innerWidth, height: innerHeight },
      documentStarted: performance.timeOrigin,
      scroll: {
        initial: samples[0].y, final: scrollY,
        min: Math.min(...samples.map(sample => sample.y)),
        max: Math.max(...samples.map(sample => sample.y)),
        settledMin: Math.min(...settled.map(sample => sample.y)),
        settledMax: Math.max(...settled.map(sample => sample.y)),
        settledFrames: settled.length, duration: samples.at(-1)!.at,
        extent: document.documentElement.scrollHeight - innerHeight,
      },
      body: rect(document.body), main: rect(document.getElementById("main")!),
      cardCount: document.querySelectorAll("#library a.card").length,
      pending: document.querySelector('#main[aria-busy="true"]') !== null,
      horizontalOverflow: document.documentElement.scrollWidth > innerWidth,
    };
  });
  await info.attach(`${name}-geometry`, { body: JSON.stringify(receipt, null, 2), contentType: "application/json" });
  await page.screenshot({ path: info.outputPath(`${name}.png`), fullPage: true });
  return receipt;
}

async function letter(page: Page, selected: boolean, compact: boolean) {
  const index = page.getByRole("navigation", { name: "Jump to title", exact: true });
  if (compact) {
    await expect(index).toBeHidden();
    await page.getByRole("button", { name: /^Jump to title/ }).click();
    await expect(page.getByRole("dialog", { name: "Jump to title", exact: true })).toBeVisible();
  }
  const navigation = compact ? page.getByRole("navigation", { name: "Choose a title letter" }) : index;
  await navigation.getByRole("link", { name: selected ? "B, 1 title, selected; activate to show all titles" : "B, 1 title", exact: true }).click();
}

export async function verifyTitleJumpScroll(page: Page, info: TestInfo) {
  for (const viewport of [
    { width: 1440, height: 900, compact: false, rapid: true },
    { width: 1440, height: 900, compact: false },
    { width: 390, height: 650, compact: true },
    { width: 390, height: 844, compact: false },
    { width: 1920, height: 1080, compact: false },
  ]) {
    await page.setViewportSize(viewport);
    await page.goto("/?view=movies");
    const prefix = `title-jump-${viewport.rapid ? "rapid-" : ""}${viewport.width}x${viewport.height}`;
    const documentStarted = await page.evaluate(() => performance.timeOrigin);
    if (!viewport.rapid) await record(page, info, `${prefix}-loaded`);
    await letter(page, false, viewport.compact);
    await expect(page).toHaveURL(/letter=B/);
    await expect(page.getByRole("heading", { name: "Beta", exact: true })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Arrival", exact: true })).toHaveCount(0);
    await expect(page.locator("#library a.card").first()).toBeFocused();
    if (viewport.rapid) {
      await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0);
    } else {
      const selected = await record(page, info, `${prefix}-selected`);
      expect.soft(selected.scroll.settledFrames, prefix).toBeGreaterThan(1);
      expect.soft([selected.scroll.settledMin, selected.scroll.settledMax], prefix).toEqual([0, 0]);
    }
    await letter(page, true, viewport.compact);
    await expect(page).not.toHaveURL(/letter=/);
    await expect(page.getByRole("heading", { name: "Arrival", exact: true })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Beta", exact: true })).toBeVisible();
    const cleared = await record(page, info, `${prefix}-cleared`);
    expect.soft(cleared.documentStarted, "Letter filtering must use the same document").toBe(documentStarted);
    expect.soft(cleared.scroll.settledFrames, prefix).toBeGreaterThan(1);
    expect.soft([cleared.scroll.settledMin, cleared.scroll.settledMax], prefix).toEqual([0, 0]);
    expect.soft(cleared.horizontalOverflow, prefix).toBe(false);
    expect.soft((await new AxeBuilder({ page }).include("#main").analyze()).violations, prefix).toEqual([]);
  }
}
