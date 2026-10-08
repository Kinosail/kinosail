import { expect, test } from "@playwright/test";
import { compactViewports, initiallyOccludedTargets, occludedTargets } from "./subtitle-dashboard-helpers";

export function registerSubtitleLandscapeTests() {
test("Landscape keeps the current task reachable during pending and failed navigation", async ({ page }) => {
  for (const viewport of compactViewports.filter(({height}) => height <= 600)) {
    await page.setViewportSize(viewport);
    await page.goto("/");
    let release!: () => void;
    const pending = new Promise<void>(resolve => { release = resolve; });
    const destination = "**/?view=library";
    await page.route(destination, async route => {
      await pending;
      await route.fulfill({status: 503, body: "Temporary fixture outage"});
    });
    try {
      const targets = [".subtitle-overview-copy h2", ".subtitle-coverage-stat > p", ".subtitle-overview-actions :is(button, a.button)"];
      const primary = await page.locator(".subtitle-overview-actions :is(button, a.button)").boundingBox();
      await page.getByRole("navigation", {name: "Main navigation"}).getByRole("link", {name: "Library", exact: true}).click();
      await expect(page.locator("#subtitle-feedback")).toContainText("Loading library");
      expect(await initiallyOccludedTargets(page, targets, [".app-header nav"])).toEqual([]);
      expect(await page.locator(".subtitle-overview-actions :is(button, a.button)").boundingBox()).toEqual(primary);
      release();
      await expect(page.locator("#subtitle-feedback")).toContainText("Could not load this view");
      expect(await initiallyOccludedTargets(page, targets, [".app-header nav"])).toEqual([]);
      expect(await page.locator(".subtitle-overview-actions :is(button, a.button)").boundingBox()).toEqual(primary);
      await page.unroute(destination);
      await page.getByRole("link", {name: "Reload view", exact: true}).click();
      await expect(page.locator("#subtitle-feedback")).toBeHidden();
      await expect(page.locator("#subtitle-content")).toBeVisible();
    } finally { release(); await page.unroute(destination); }
  }
});

test("Enlarged settings keep native timeout choices inside the phone and landscape page", async ({ page }, info) => {
  for (const viewport of [{width: 320, height: 800}, {width: 390, height: 844}, {width: 844, height: 390}]) {
    await page.setViewportSize(viewport);
    await page.goto("/settings#session-timeouts");
    await page.evaluate(() => { document.documentElement.style.fontSize = "200%"; });
    await page.evaluate(() => document.fonts.ready);
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
    const choice = page.locator('#session-timeouts select[name="inactiveHours"]').first();
    await expect(choice).toBeEnabled();
    await choice.selectOption("1");
    await expect(choice).toHaveValue("1");
    const geometry = await page.locator("#session-timeouts select").evaluateAll(nodes => nodes.map(node => {
      const rect = node.getBoundingClientRect(), parent = node.parentElement!.getBoundingClientRect();
      return {left: rect.left, right: rect.right, parentLeft: parent.left, parentRight: parent.right};
    }));
    for (const box of geometry) {
      expect(box.left).toBeGreaterThanOrEqual(box.parentLeft - 1);
      expect(box.right).toBeLessThanOrEqual(box.parentRight + 1);
    }
    await info.attach("enlarged-native-timeout-choices", {body: JSON.stringify({viewport, geometry}), contentType: "application/json"});
    await page.screenshot({path: info.outputPath(`timeout-choices-${viewport.width}.png`)});
  }
});

}
