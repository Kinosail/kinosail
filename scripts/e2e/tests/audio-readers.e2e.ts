import { describe, test } from '@e2e-dev/web';
import { expect } from 'e2e';
import { api, requireFixtureURL } from './helpers';
import { fixtureItem, fixtureAlbum, fixtureAlbumTracks, fixtureReader } from '../fixture-response.mjs';

describe('populated audio and readers', { session: 'owner' }, () => {
  test('album queue plays two real tracks and rejects invalid progress without mutation', async ({ app, browser, screen }) => {
    requireFixtureURL(app.baseUrl);
    await app.open('/settings');
    test.skip((await browser.title()).includes('Subtitles'), 'Music belongs to Kinosail Player');
    const albums = await api(browser, app.baseUrl, '/api/v1/albums');
    expect(albums.status).toBe(200);
    const album = fixtureAlbum(albums.data);
    expect(album).toBeDefined();
    const detail = await api(browser, app.baseUrl, '/api/v1/albums/' + album.id);
    expect(detail.status).toBe(200);
    const tracks = fixtureAlbumTracks(detail.data, album);
    expect(tracks.map((t: { title: string }) => t.title)).toEqual(['E2E Track One', 'E2E Track Two']);
    const [first, second] = tracks;
    const queuePath = '/api/v1/audio/' + first.id + '/queue';
    const queue = await api(browser, app.baseUrl, queuePath);
    expect(queue.data.items.map((t: { id: string }) => t.id)).toEqual([first.id, second.id]);
    await app.open('/album/' + album.id);
    await expect(browser.locator('h1').filter({ hasText: 'E2E Album' })).toBeVisible();
    await screen.getByRole('link', /1 E2E Track One/).click();
    await expect(browser.locator('audio')).toHaveCount(1);
    await expect.poll(() => browser.evaluate(() => document.querySelector('audio')?.getAttribute('data-progress') ?? null)).toBe('/progress/' + first.id);
    await browser.evaluate(async () => { const audio = document.querySelector('audio')!; audio.muted = true; await audio.play(); });
    await expect.poll(() => browser.evaluate(() => document.querySelector('audio')?.currentTime ?? 0)).toBeGreaterThan(0.2);
    await browser.evaluate(() => document.querySelector('audio')!.pause());
    const progressPath = '/api/v1/items/' + first.id + '/watch-progress';
    await expect.poll(async () => (await api(browser, app.baseUrl, progressPath)).data.seconds).toBeGreaterThan(0);
    const before = await api(browser, app.baseUrl, progressPath);
    for (const body of [{}, { seconds: null }, { seconds: -1 }, { seconds: '1' }, { seconds: 1000000001 }, { seconds: 1, unknown: true }]) {
      expect((await api(browser, app.baseUrl, '/api/v1/items/' + first.id + '/progress', 'PUT', body)).status).toBe(400);
      expect((await api(browser, app.baseUrl, progressPath)).data).toEqual(before.data);
      expect((await api(browser, app.baseUrl, queuePath)).data.items.map((t: { id: string }) => t.id)).toEqual([first.id, second.id]);
    }
    expect((await api(browser, app.baseUrl, '/api/v1/audio/missing/queue')).status).toBe(404);
    await browser.evaluate(async () => { const audio = document.querySelector('audio')!; audio.currentTime = audio.duration - 0.3; await audio.play(); });
    await expect.poll(() => browser.evaluate(() => document.querySelector('audio')?.getAttribute('data-progress') ?? null)).toBe('/progress/' + second.id);
    await expect.poll(() => browser.evaluate(() => document.querySelector('audio')?.currentTime ?? 0)).toBeGreaterThan(0.2);
    expect(await browser.evaluate(() => document.querySelector('audio')!.readyState)).toBeGreaterThanOrEqual(2);
    await browser.evaluate(() => document.querySelector('audio')!.pause());
    await app.screenshot('decoded-album-queue');
  });

  test('EPUB chapters render real content and invalid chapter saves preserve progress', async ({ app, browser, screen }) => {
    requireFixtureURL(app.baseUrl);
    await app.open('/settings');
    test.skip((await browser.title()).includes('Subtitles'), 'Readers belong to Kinosail Player');
    const books = await api(browser, app.baseUrl, '/api/v1/library?view=books');
    expect(books.status).toBe(200);
    const book = fixtureItem(books.data, 'E2E EPUB', 'book');
    expect(book).toBeDefined();
    const reader = await api(browser, app.baseUrl, '/api/v1/books/' + book.id + '/reader');
    expect(reader.status).toBe(200);
    fixtureReader(reader.data, book);
    expect(reader.data.type).toBe('epub');
    expect(reader.data.pages).toHaveLength(2);
    await app.open('/read/' + book.id);
    await expect(browser.locator('iframe.book-reader')).toBeVisible();
    await expect(browser.frameLocator('iframe.book-reader').getByRole('heading', 'E2E chapter one')).toBeVisible();
    await screen.getByRole('button', 'Chapter 2').click();
    await expect(browser.frameLocator('iframe.book-reader').getByRole('heading', 'E2E chapter two')).toBeVisible();
    const path = '/api/v1/books/' + book.id + '/reader/progress';
    const saved = await api(browser, app.baseUrl, path);
    expect(saved.data.page).toBe(2);
    for (const body of [{}, { page: 0 }, { page: 3 }, { page: '2' }, { page: 1, unknown: true }]) {
      expect((await api(browser, app.baseUrl, path, 'PUT', body)).status).toBe(400);
      expect((await api(browser, app.baseUrl, path)).data).toEqual(saved.data);
    }
    await browser.reload();
    await expect(browser.frameLocator('iframe.book-reader').getByRole('heading', 'E2E chapter two')).toBeVisible();
    const asset = reader.data.pages[0].url;
    const headers = await browser.evaluate(async path => { const r = await fetch(path); return { status: r.status, csp: r.headers.get('Content-Security-Policy'), frame: r.headers.get('X-Frame-Options') }; }, asset);
    expect(headers.status).toBe(200);
    expect(headers.csp).toContain("frame-ancestors 'self'");
    expect(headers.frame).toBe('SAMEORIGIN');
    await app.screenshot('resumed-epub-chapter');
  });

  test('comic pages and photos decode actual images and reject missing assets', async ({ app, browser }) => {
    requireFixtureURL(app.baseUrl);
    await app.open('/settings');
    test.skip((await browser.title()).includes('Subtitles'), 'Books and Photos belong to Kinosail Player');
    const comics = await api(browser, app.baseUrl, '/api/v1/library?view=books');
    expect(comics.status).toBe(200);
    const comic = fixtureItem(comics.data, 'E2E Comic', 'book');
    expect(comic).toBeDefined();
    await app.open('/read/' + comic.id);
    await expect(browser.locator('.reader-pages img')).toHaveCount(2);
    await expect.poll(() => browser.evaluate(() => [...document.querySelectorAll<HTMLImageElement>('.reader-pages img')].filter(i => i.complete && i.naturalWidth === 64 && i.naturalHeight === 48).length)).toBe(2);
    const photos = await api(browser, app.baseUrl, '/api/v1/library?view=photos');
    expect(photos.status).toBe(200);
    const photo = fixtureItem(photos.data, 'E2E Photo', 'photo');
    expect(photo).toBeDefined();
    await app.open('/watch/' + photo.id);
    await expect.poll(() => browser.evaluate(() => [...document.querySelectorAll<HTMLImageElement>('main img')].some(i => i.complete && i.naturalWidth === 64 && i.naturalHeight === 48))).toBe(true);
    const original = await api(browser, app.baseUrl, '/api/v1/library?view=books');
    for (const path of ['/api/v1/books/missing/reader', '/read/' + comic.id + '/asset/unknown.png', '/media/missing']) {
      expect(await browser.evaluate(async path => (await fetch(path)).status, path)).toBe(404);
    }
    expect((await api(browser, app.baseUrl, '/api/v1/library?view=books')).data).toEqual(original.data);
    await app.screenshot('decoded-photo');
  });
});
