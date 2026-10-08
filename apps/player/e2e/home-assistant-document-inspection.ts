import {recordHomeAssistantEvidence} from "./home-assistant-document-evidence";
import {expect, type Page, type TestInfo} from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

export async function inspectDocumentStatus(page: Page, info: TestInfo, state: string) {
  const surface = page.locator("[data-home-assistant-status]");
  await expect(surface).toHaveAttribute("data-home-assistant-status", state);
  const measurements: Array<{viewportWidth: number; retryVisible: boolean; width?: number; height?: number; keyboardFocused: boolean}> = [];
  for (const viewport of [{width: 390, height: 844}, {width: 1440, height: 900}, {width: 1920, height: 1080}]) {
    await page.setViewportSize(viewport);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect((await new AxeBuilder({page}).include("[data-home-assistant-status]").analyze()).violations).toEqual([]);
    const retry = surface.getByRole("button", {name: "Retry Home Assistant", exact: true});
    if (state === "unavailable") await expect(retry).toBeVisible();
    const retryVisible = await retry.isVisible();
    const box = retryVisible ? await retry.boundingBox() : null;
    if (retryVisible) {
      expect(box!.width).toBeGreaterThanOrEqual(44);
      expect(box!.height).toBeGreaterThanOrEqual(44);
      await retry.focus();
      await expect(retry).toBeFocused();
    }
    measurements.push({viewportWidth: viewport.width, retryVisible, width: box?.width, height: box?.height,
      keyboardFocused: retryVisible});
    await surface.screenshot({path: info.outputPath(`home-assistant-${state}-${viewport.width}.png`)});
  }
  await recordHomeAssistantEvidence(info, "actual-rendered-document-status", {state, measurements,
    axeViolations: 0, horizontalOverflow: false, screenshotsCaptured: true, screenshotPixelsInspected: false});
}
