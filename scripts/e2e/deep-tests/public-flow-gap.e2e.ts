import { describe, test, surfaceOf } from '@e2e-dev/web';
import { expect } from 'e2e';
import { playerDeepEngine } from '../public-flow-gap-engine.ts';
import { pdfDocumentPixels } from '../pdf-pixels.mjs';
import { requestRecovery, readFixtureReceipt } from '../recovery-control.mjs';
import { api, movie, requireFixtureURL } from '../tests/helpers.ts';
import { fixtureItem, fixtureReader, fixtureWatchProgress } from '../fixture-response.mjs';
import { readGapLaunch } from '../gap-launch.mjs';

describe('owned Player public-flow gaps', { session: 'owner' }, () => {
  test('encrypted recovery CLI restores public state and rejects corrupt input with unchanged data fingerprint', { timeout: 120000, tags: ['recovery'] }, async ({ app, browser }) => {
    requireFixtureURL(app.baseUrl);
    readGapLaunch(process.env.KINOSAIL_E2E_GAP_RUN);
    await app.open('/settings');
    const item = await movie(browser, app.baseUrl), path = '/api/v1/items/' + item.id;
    expect((await api(browser, app.baseUrl, path + '/progress', 'PUT', { seconds: 3 })).status).toBe(200);
    expect((await api(browser, app.baseUrl, path + '/list', 'PUT', { listed: true })).status).toBe(200);
    const progress = async () => {
      const response = await api(browser, app.baseUrl, path + '/watch-progress');
      expect(response.status).toBe(200);
      return fixtureWatchProgress(response.data);
    };
    const saved = await progress();
    const profiles = (await api(browser, app.baseUrl, '/api/v1/profiles')).data;
    const port = new URL(app.baseUrl!).port;
    const receipt = () => readFixtureReceipt(port);
    const before = receipt();
    expect(before.generation).toBe(0);
    requestRecovery(port, { childPID: before.childPID, generation: 0, operation: 'backup' });
    await expect.poll(() => receipt().generation, { timeout: 45000 }).toBe(1);
    expect(receipt().childPID).not.toBe(before.childPID);
    expect(receipt().recovery).toEqual(expect.objectContaining({ operation: 'backup', verified: true }));
    await expect.poll(async () => { try { return (await fetch(new URL('/healthz', app.baseUrl), {redirect: 'error', signal: AbortSignal.timeout(10000)})).status; } catch { return 0; } }).toBe(200);
    await browser.reload();
    expect((await api(browser, app.baseUrl, path + '/progress', 'PUT', { seconds: 9 })).status).toBe(200);
    expect((await api(browser, app.baseUrl, path + '/list', 'PUT', { listed: false })).status).toBe(200);
    expect(await progress()).not.toEqual(saved);
    const changed = receipt();
    requestRecovery(port, { childPID: changed.childPID, generation: 1, operation: 'restore' });
    await expect.poll(() => receipt().generation, { timeout: 45000 }).toBe(2);
    expect(receipt().childPID).not.toBe(changed.childPID);
    expect(receipt().recovery).toEqual(expect.objectContaining({ operation: 'restore', corruptRejected: true, corruptDataUnchanged: true, restored: true }));
    await expect.poll(async () => { try { return (await fetch(new URL('/healthz', app.baseUrl), {redirect: 'error', signal: AbortSignal.timeout(10000)})).status; } catch { return 0; } }).toBe(200);
    await browser.reload();
    expect(await progress()).toEqual(saved);
    expect((await api(browser, app.baseUrl, path)).data.listed).toBe(true);
    expect((await api(browser, app.baseUrl, '/api/v1/profiles')).data).toEqual(profiles);
    await app.screenshot('public-state-after-real-encrypted-cli-restore');
  });

  test('virtual scheduler sleep timer expires, cancels and rearms with a real muted AAC decoder', { timeout: 60000, tags: ['timer'] }, async ({ app, browser, screen }) => {
    requireFixtureURL(app.baseUrl);
    readGapLaunch(process.env.KINOSAIL_E2E_GAP_RUN);
    await app.open('/settings');
    const live = surfaceOf(playerDeepEngine);
    if (!live) throw Error('pinned Player web engine surface unavailable');
    const page = live.page();
    // Public SDK export and official PW1.63 scheduler. Media clock stays real.
    await page.clock.install();
    let failure: unknown;
    try {
    const books = await api(browser, app.baseUrl, '/api/v1/library?view=audiobooks');
    expect(books.status).toBe(200);
    const book = fixtureItem(books.data, 'E2E Audiobook', 'audiobook');
    expect(book).toBeDefined();
    await app.open('/watch/' + book.id);
    const audio = () => browser.evaluate(() => {
      const a = document.querySelector('audio')!;
      return { time: a.currentTime, duration: a.duration, paused: a.paused, ended: a.ended, ready: a.readyState, loop: a.loop };
    });
    await expect.poll(async () => (await audio()).ready).toBeGreaterThanOrEqual(1);
    expect((await audio()).duration).toBeCloseTo(24, 0);
    await browser.evaluate(async () => { const a = document.querySelector('audio')!; a.loop = true; a.muted = true; a.currentTime = 0; await a.play(); return true; });
    await expect.poll(async () => (await audio()).ready).toBeGreaterThanOrEqual(2);
    await expect.poll(async () => (await audio()).time).toBeGreaterThan(0.2);
    await page.clock.pauseAt(new Date(Date.now() + 60000));
    const timer = screen.getByRole('combobox', 'Sleep timer');
    const state = browser.locator('[data-sleep-state]');
    const endedMessage = await browser.evaluate(() => document.body.dataset.sleepEnded ?? '');
    expect(endedMessage).not.toBe('');
    await timer.selectOption('15 minutes');
    await expect(state).toContainText('15');
    await timer.selectOption('Off');
    await page.clock.fastForward(900000);
    expect((await audio()).paused).toBe(false);
    expect((await audio()).ended).toBe(false);
    await expect(state).toHaveText('');
    await timer.selectOption('15 minutes');
    await page.clock.fastForward(899000);
    expect((await audio()).paused).toBe(false);
    await timer.selectOption('15 minutes'); // Replace the pending deadline.
    await page.clock.fastForward(1000);
    expect((await audio()).paused).toBe(false); // Prior deadline was cancelled.
    await page.clock.fastForward(898000);
    expect((await audio()).paused).toBe(false);
    await page.clock.fastForward(1000);
    await expect.poll(async () => (await audio()).paused).toBe(true);
    expect((await audio()).ended).toBe(false);
    await expect(state).toHaveText(endedMessage);
    await page.clock.resume(); // Let production pause persistence/debouncing drain naturally.
    const paused = (await audio()).time;
    await expect.poll(async () => {
      const response = await api(browser, app.baseUrl, '/api/v1/items/' + book.id + '/watch-progress');
      expect(response.status).toBe(200);
      return Math.abs(fixtureWatchProgress(response.data).seconds - paused);
    }).toBeLessThan(0.1);
    await app.screenshot('virtual-timer-expiry-real-aac-pause-and-durable-progress');
    } catch (error) { failure = error; throw error; }
    finally { try { await page.clock.resume(); } catch (error) { if (!failure) throw error; } }
  });

  test('explicitly headed native PDF viewer decodes the fixture colored pattern', { timeout: 60000, tags: ['pdf-headed'] }, async ({ app, browser }) => {
    requireFixtureURL(app.baseUrl);
    readGapLaunch(process.env.KINOSAIL_E2E_GAP_RUN);
    await app.open('/settings');
    const live = surfaceOf(playerDeepEngine);
    if (!live) throw Error('pinned Player web engine surface unavailable');
    const books = await api(browser, app.baseUrl, '/api/v1/library?view=books');
    expect(books.status).toBe(200);
    const book = fixtureItem(books.data, 'E2E PDF', 'book');
    expect(book).toBeDefined();
    const reader = await api(browser, app.baseUrl, '/api/v1/books/' + book.id + '/reader');
    expect(reader.status).toBe(200);
    expect(fixtureReader(reader.data, book).type).toBe('pdf');
    await app.open('/read/' + book.id);
    await expect(browser.locator('iframe.book-reader')).toBeVisible();
    await expect(browser.locator('iframe.book-reader')).toHaveAttribute('src', '/read/' + book.id + '/file');
    // Pixels come only from the actual visible native iframe, never a PDF parser.
    await expect.poll(async () => {
      try { return pdfDocumentPixels(await live.page().locator('iframe.book-reader').screenshot({ timeout: 3000 })).documentPattern; }
      catch { return false; }
    }, { timeout: 20000, message: 'original PDF colored squares must be visibly decoded in headed Chromium' }).toBe(true);
    await app.screenshot('native-pdf-document-decoded-pattern');
  });
});
