import {expect, type Page, type TestInfo} from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

export async function inspectDocumentStatus(page: Page, info: TestInfo, state: string) {
  const surface = page.locator("[data-home-assistant-status]");
  await expect(surface).toHaveAttribute("data-home-assistant-status", state);
  for (const viewport of [{width: 390, height: 844}, {width: 1440, height: 900}, {width: 1920, height: 1080}]) {
    await page.setViewportSize(viewport);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect((await new AxeBuilder({page}).include("[data-home-assistant-status]").analyze()).violations).toEqual([]);
    const retry = surface.getByRole("button", {name: "Retry Home Assistant", exact: true});
    if (await retry.isVisible()) {
      const box = await retry.boundingBox();
      expect(box!.width).toBeGreaterThanOrEqual(44);
      expect(box!.height).toBeGreaterThanOrEqual(44);
      await retry.focus();
      await expect(retry).toBeFocused();
    }
    await surface.screenshot({path: info.outputPath(`home-assistant-${state}-${viewport.width}.png`)});
  }
}
