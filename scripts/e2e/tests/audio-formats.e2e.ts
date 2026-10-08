import { describe, test } from '@e2e-dev/web';
import { expect } from 'e2e';
import { api, requireFixtureURL } from './helpers';

describe('additional real media formats', { session: 'owner' }, () => {
  test('M4B chapters seek decoded audio and retain speed, timer and progress controls', async ({ app, browser, screen }) => {
    requireFixtureURL(app.baseUrl);
    await app.open('/settings');
    test.skip((await browser.title()).includes('Subtitles'), 'Audiobooks belong to Kinosail Player');
    const catalog = await api(browser, app.baseUrl, '/api/v1/library?view=audiobooks');
    const book = catalog.data.items.find((i: { title: string }) => i.title === 'E2E Audiobook');
    expect(book).toBeDefined();
    expect(book.kind).toBe('audiobook');
    await app.open('/watch/' + book.id);
    await browser.evaluate(async () => { const audio = document.querySelector('audio')!; audio.muted = true; await audio.play(); });
    await expect.poll(() => browser.evaluate(() => document.querySelector('audio')!.currentTime)).toBeGreaterThan(0.2);
    await browser.evaluate(() => document.querySelector('audio')!.pause());
    await screen.getByText('Chapters', { exact: true }).click();
    await screen.getByRole('button', /E2E second chapter/).click();
    await expect.poll(() => browser.evaluate(() => document.querySelector('audio')!.currentTime)).toBeGreaterThan(3.9);
    await screen.getByRole('combobox', 'Playback speed').selectOption('1.5×');
    expect(await browser.evaluate(() => document.querySelector('audio')!.playbackRate)).toBe(1.5);
    await screen.getByRole('combobox', 'Sleep timer').selectOption('15 minutes');
    await expect(screen.getByRole('status')).toContainText('15');
    await screen.getByRole('combobox', 'Sleep timer').selectOption('Off');
    const path = '/api/v1/items/' + book.id + '/progress';
    expect((await api(browser, app.baseUrl, path, 'PUT', { seconds: 4 })).status).toBe(200);
    const saved = (await api(browser, app.baseUrl, '/api/v1/items/' + book.id)).data.item.progress;
    expect((await api(browser, app.baseUrl, path, 'PUT', { seconds: -1 })).status).toBe(400);
    expect((await api(browser, app.baseUrl, '/api/v1/items/' + book.id)).data.item.progress).toEqual(saved);
    await browser.reload();
    await expect.poll(() => browser.evaluate(() => document.querySelector('audio')!.currentTime)).toBeGreaterThan(3.9);
    // The real Go audio template has no #t fragment or adaptive resume owner.
    // This tail therefore independently exercises the shipped JS resume guard.
    await browser.evaluate(() => document.querySelector('audio')!.pause());
    await screen.getByRole('link', 'Library', { exact: true }).click();
    await expect(browser).toHaveURL('/');
    expect((await api(browser, app.baseUrl, path, 'PUT', { seconds: 16, watched: false })).status).toBe(200);
    const tail = (await api(browser, app.baseUrl, '/api/v1/items/' + book.id)).data.item.progress;
    expect(tail.seconds).toBe(16);
    expect(Boolean(tail.watched)).toBe(false);
    await app.open('/watch/' + book.id);
    await expect(browser.locator('audio')).toHaveAttribute('data-start', '16');
    const source = await browser.evaluate(() => {
      const audio = document.querySelector('audio')!;
      return { fragment: new URL(audio.getAttribute('src')!, location.href).hash,
        hls: audio.hasAttribute('data-hls'), adaptive: audio.hasAttribute('data-adaptive') };
    });
    expect(source).toEqual({ fragment: '', hls: false, adaptive: false });
    await expect.poll(() => browser.evaluate(() => document.querySelector('audio')!.readyState)).toBeGreaterThanOrEqual(1);
    expect(await browser.evaluate(() => document.querySelector('audio')!.duration)).toBeCloseTo(24, 0);
    await expect.poll(() => browser.evaluate(() => Math.abs(document.querySelector('audio')!.currentTime - 16))).toBeLessThan(0.1);
    await browser.evaluate(async () => { const audio = document.querySelector('audio')!; audio.muted = true; await audio.play(); });
    await expect.poll(() => browser.evaluate(() => document.querySelector('audio')!.readyState)).toBeGreaterThanOrEqual(2);
    await expect.poll(() => browser.evaluate(() => document.querySelector('audio')!.currentTime)).toBeGreaterThan(16.2);
    const paused = await browser.evaluate(() => { const audio = document.querySelector('audio')!; audio.pause(); return audio.currentTime; });
    await expect.poll(async () => Math.abs((await api(browser, app.baseUrl, '/api/v1/items/' + book.id)).data.item.progress.seconds - paused)).toBeLessThan(0.1);
    expect(Boolean((await api(browser, app.baseUrl, '/api/v1/items/' + book.id)).data.item.progress.watched)).toBe(false);
    await app.screenshot('decoded-audiobook-chapters');
  });

  test('PDF reader exposes its same-origin document and bounded ranges (rendering remains browser-specific)', async ({ app, browser }) => {
    requireFixtureURL(app.baseUrl);
    await app.open('/settings');
    test.skip((await browser.title()).includes('Subtitles'), 'PDF readers belong to Kinosail Player');
    const catalog = await api(browser, app.baseUrl, '/api/v1/library?view=books');
    const book = catalog.data.items.find((i: { title: string }) => i.title === 'E2E PDF');
    expect(book).toBeDefined();
    const reader = await api(browser, app.baseUrl, '/api/v1/books/' + book.id + '/reader');
    expect(reader.data.type).toBe('pdf');
    await app.open('/read/' + book.id);
    await expect(browser.locator('iframe.book-reader')).toBeVisible();
    expect(await browser.locator('iframe.book-reader').getAttribute('src')).toBe('/read/' + book.id + '/file');
    const file = await browser.evaluate(async path => {
      const response = await fetch(path, { headers: { Range: 'bytes=0-7' } });
      return { status: response.status, type: response.headers.get('content-type'), frame: response.headers.get('x-frame-options'), range: response.headers.get('content-range'), bytes: await response.text() };
    }, '/read/' + book.id + '/file');
    expect(file.status).toBe(206);
    expect(file.type).toBe('application/pdf');
    expect(file.frame).toBe('SAMEORIGIN');
    expect(file.range).toMatch(/^bytes 0-7\/\d+$/);
    expect(file.bytes).toBe('%PDF-1.4');
    expect(await browser.evaluate(async path => (await fetch(path)).status, '/read/missing/file')).toBe(404);
    expect(await browser.evaluate(async path => (await fetch(path)).status, '/read/' + book.id + '/asset/unknown')).toBe(404);
    expect((await api(browser, app.baseUrl, '/api/v1/library?view=books')).data).toEqual(catalog.data);
    await app.screenshot('pdf-reader-route-partial-rendering-proof');
  });
});
