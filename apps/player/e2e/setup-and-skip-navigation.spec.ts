import { expect, test } from '@playwright/test';
import { configureLayoutAudit, login, viewports } from './layout-audit-helpers';

configureLayoutAudit();
test.beforeEach(async ({ page }) => {
  test.skip(process.env.KINOSAIL_TEST_INSTANCE !== '1', 'requires the populated test instance');
  await login(page);
});

for (const viewport of viewports) {
  test(`TMDB instructions stay readable at ${viewport.width}px${viewport.width === 390 ? " @smoke" : ""}`, async ({ page }, testInfo) => {
    await page.setViewportSize(viewport);
    for (const path of ['/settings/configuration', '/onboarding/connection']) {
      await page.goto(path);
      const section = page.locator('section').filter({ has: page.getByRole('heading', { name: 'Movie artwork and details', exact: true }) });
      const instructions = section.locator('ol');
      await expect(instructions.locator('li')).toHaveCount(3);
      const bounds = await instructions.locator('li').evaluateAll(elements => elements.map(element => {
        const box = element.getBoundingClientRect();
        return { left: box.left, right: box.right, viewport: document.documentElement.clientWidth };
      }));
      for (const box of bounds) {
        expect(box.left).toBeGreaterThanOrEqual(0);
        expect(box.right).toBeLessThanOrEqual(box.viewport);
      }
      expect(await instructions.evaluate(element => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
      await section.screenshot({ path: testInfo.outputPath(`${path.replaceAll('/', '-')}.png`) });
    }
  });
}

for (const width of [1440, 390]) {
  test(`show skip link moves keyboard focus to the main content at ${width}px @smoke`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 });
    await page.goto('/?view=shows');
    await page.locator('a.show-details[href^="/show/"]').first().click();
    const show = page.url();
    for (const path of [show, '/account', '/settings', '/offline-downloads']) {
      if (path !== show) await page.goto(path);
      const skip = page.getByRole('link', { name: 'Skip to content', exact: true });
      const main = page.getByRole('main');
      const target = `#${await main.getAttribute('id')}`;
      await skip.focus();
      await skip.press('Enter');
      await expect.poll(() => new URL(page.url()).hash).toBe(target);
      await expect(main).toBeFocused();
      await page.screenshot({ path: testInfo.outputPath(`skip-${new URL(page.url()).pathname.replaceAll('/', '-')}-${width}.png`) });
    }
  });
}
