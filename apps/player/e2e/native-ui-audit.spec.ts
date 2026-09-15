import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { uiElementAttachment, uiElementInventory } from '../../../scripts/testing/ui-element-inventory';

const origin = process.env.KINOSAIL_NATIVE_AUDIT_URL;
for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
  test(`native web component families at ${viewport.width}px`, async ({ page }, testInfo) => {
    test.skip(!origin, 'Requires the local native export and its populated QA server. Not hardware proof.');
    await page.setViewportSize(viewport);
    await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
    const record = async (state: string) => {
      expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth), state).toBeLessThanOrEqual(1);
      const elements = await page.evaluate(uiElementInventory);
      expect(elements.filter(item => item.visible && item.role === 'button' && (item.width < 44 || item.height < 44)), state).toEqual([]);
      expect((await new AxeBuilder({ page }).analyze()).violations, state).toEqual([]);
      await testInfo.attach(`${state}-elements.json`, await uiElementAttachment(testInfo.outputPath(`${state}-elements.json`), elements));
      await page.screenshot({ path: testInfo.outputPath(`${state}.png`), fullPage: true });
    };
    await page.goto(origin!);
    const input = page.getByRole('textbox', { name: 'Kinosail Server URL' });
    await expect(input).toBeVisible();
    await record('setup');
    await input.fill(origin!);
    let writes = 0;
    let release: () => void = () => {};
    const pending = new Promise<void>(resolve => { release = resolve; });
    await page.route('**/api/v1/quick-connect', async route => {
      writes++;
      await pending;
      await route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: 'Server unavailable. Try again.' }) });
    });
    await input.press('Enter');
    try {
      await expect(page.getByRole('button', { name: 'Connecting…' })).toBeDisabled();
      await expect(page.getByRole('button', { name: 'Connecting…' })).toHaveAttribute('aria-busy', 'true');
      await input.press('Enter');
      await record('connecting');
      expect(writes).toBe(1);
    } finally { release(); }
    await expect(page.getByText('Kinosail Server could not complete the request.')).toBeVisible();
    await record('connection-error');
    await page.unroute('**/api/v1/quick-connect');
    await page.route('**/api/v1/quick-connect/token', route => route.fulfill({ status: 503, contentType: 'application/json', body: '{}' }));
    await page.getByRole('button', { name: 'Connect', exact: true }).click();
    await expect(page.getByText('Approval check stopped.')).toBeVisible();
    await expect(page.getByText('Waiting for approval…')).not.toBeVisible();
    await record('approval-error');
    await page.unroute('**/api/v1/quick-connect/token');
    await page.getByRole('button', { name: 'Retry approval', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Continue watching' })).toBeVisible();
    await record('home');
    await page.getByRole('button', { name: 'Resume', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Arrival', exact: true })).toBeVisible();
    await record('details');
    await page.route('**/media/**', route => route.fulfill({ status: 415, body: 'Unsupported fixture media' }));
    await page.getByRole('button', { name: 'Resume', exact: true }).click();
    await expect(page.getByText('Direct playback stopped', { exact: true })).toBeVisible();
    await record('playback-error');
    await page.getByRole('button', { name: 'Return to details', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Arrival', exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Back', exact: true }).click();
    await page.route('**/api/v1/library*', route => route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: 'Library unavailable.' }) }));
    await page.reload();
    await expect(page.getByRole('button', { name: 'Try again', exact: true })).toBeVisible();
    await record('library-error');
    await page.unroute('**/api/v1/library*');
    await page.getByRole('button', { name: 'Try again', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Continue watching' })).toBeVisible();
    await page.route('**/api/v1/library*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [], total: 0 }) }));
    await page.reload();
    await expect(page.getByRole('button', { name: 'Refresh library', exact: true })).toBeVisible();
    await record('library-empty');
    await page.unroute('**/api/v1/library*');
    await page.getByRole('button', { name: 'Refresh library', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Continue watching' })).toBeVisible();
    await page.getByRole('button', { name: 'Sign out', exact: true }).click();
    await expect(input).toBeVisible();
  });
}
