import { expect, test, type Page } from '@playwright/test';
import { configureLayoutAudit, login } from './layout-audit-helpers';

configureLayoutAudit();
test.beforeEach(async ({ page }) => {
  test.skip(process.env.KINOSAIL_TEST_INSTANCE !== '1', 'requires the populated test instance');
  await login(page);
});

async function camera(page: Page, raw: string, options = { denied: false, pending: false }) {
  await page.addInitScript(({ raw, options }) => {
    const state = { stopped: 0, requests: 0, release: () => {} };
    Object.assign(window, { qrTest: state });
    Object.defineProperty(navigator.mediaDevices, 'getUserMedia', { value: async () => {
      state.requests++;
      if (options.denied) throw new DOMException('denied', 'NotAllowedError');
      if (options.pending) await new Promise<void>(resolve => { state.release = resolve; });
      return { getTracks: () => [{ stop: () => { state.stopped++; } }] };
    } });
    Object.defineProperty(HTMLMediaElement.prototype, 'srcObject', { set() {}, get() { return null; } });
    Object.defineProperty(HTMLMediaElement.prototype, 'readyState', { get: () => 2 });
    Object.defineProperty(HTMLVideoElement.prototype, 'videoWidth', { get: () => 640 });
    Object.defineProperty(HTMLVideoElement.prototype, 'videoHeight', { get: () => 480 });
    HTMLMediaElement.prototype.play = async () => {};
    CanvasRenderingContext2D.prototype.drawImage = () => {};
    Object.assign(window, { qrRaw: raw });
  }, { raw, options });
  await page.route('**/static/qr-decoder.js*', route => route.fulfill({
    contentType: 'text/javascript', body: 'window.jsQR = () => ({data: window.qrRaw});',
  }));
  await page.goto('/quick-connect');
}

for (const raw of ['', '12345', '1234567', '12a456', '１２３４５６', 'x'.repeat(2049),
  'https://other.invalid/connect?code=123456', 'javascript:123456',
  '/connect?code=123456', 'SAME/connect?code=123456&code=234567',
  'SAME/connect?code=123456&other=x', 'SAME/connect?code=123456#other',
  'SAME/other?code=123456', 'SAME/connect?code=%31%32%33%34%35%36']) {
  test(`rejects QR content without approval: ${raw.slice(0, 65)}`, async ({ page }) => {
    const origin = new URL(page.url()).origin;
    await camera(page, raw.replace('SAME', origin));
    const posts: string[] = [];
    page.on('request', request => { if (request.method() === 'POST') posts.push(request.url()); });
    await page.getByRole('button', { name: 'Scan QR code', exact: true }).click();
    await expect(page.locator('[data-qr-status]')).toContainText('connected to this Server');
    await expect(page).toHaveURL(/\/quick-connect$/);
    expect(posts).toEqual([]);
    await page.getByRole('button', { name: 'Stop scanning' }).click();
    expect(await page.evaluate(() => (window as any).qrTest.stopped)).toBe(1);
  });
}

for (const raw of ['123456', 'SAME/connect?code=123456', 'SAME/quick-connect?code=123456']) {
  test(`scan opens confirmation without approving: ${raw}`, async ({ page }) => {
    const origin = new URL(page.url()).origin;
    await camera(page, raw.replace('SAME', origin));
    let posted = false;
    page.on('request', request => { if (request.method() === 'POST') posted = true; });
    await page.route('**/quick-connect?code=123456', async route => {
      expect(await page.evaluate(() => (window as any).qrTest.stopped)).toBe(1);
      await route.fulfill({ body: 'Confirmation requested' });
    });
    await page.getByRole('button', { name: 'Scan QR code', exact: true }).click();
    await expect(page).toHaveURL(/\/quick-connect\?code=123456$/);
    expect(posted).toBe(false);
  });
}

test('permission denial leaves manual code entry available', async ({ page }) => {
  await camera(page, '', { denied: true, pending: false });
  await page.getByRole('button', { name: 'Scan QR code', exact: true }).click();
  await expect(page.locator('[data-qr-status]')).toContainText('Camera access was denied');
  await expect(page.locator('[data-quick-connect-digit]').first()).toBeEditable();
  await expect(page.locator('[data-qr-camera]')).toBeHidden();
  await expect(page.locator('[data-qr-start]')).toBeFocused();
});

test('stopping while permission is pending closes a late camera stream', async ({ page }) => {
  await camera(page, '', { denied: false, pending: true });
  await page.getByRole('button', { name: 'Scan QR code', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).qrTest.requests)).toBe(1);
  await page.getByRole('button', { name: 'Stop scanning' }).click();
  await page.evaluate(() => (window as any).qrTest.release());
  await expect.poll(() => page.evaluate(() => (window as any).qrTest.stopped)).toBe(1);
  await expect(page.locator('[data-qr-camera]')).toBeHidden();
});
