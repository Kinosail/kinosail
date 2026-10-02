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

test('show skip link moves keyboard focus to the main content @smoke', async ({ page }, testInfo) => {
  await page.goto('/?view=shows');
  await page.locator('a.show-details[href^="/show/"]').first().click();
  const skip = page.getByRole('link', { name: 'Skip to content', exact: true });
  const main = page.getByRole('main');
  await expect(main).toHaveAttribute('id', 'main');
  await skip.focus();
  await skip.press('Enter');
  await expect(page).toHaveURL(/#main$/);
  expect(await main.evaluate(element => element === document.activeElement)).toBeTruthy();
  await page.screenshot({ path: testInfo.outputPath('show-skip-target.png') });
});
