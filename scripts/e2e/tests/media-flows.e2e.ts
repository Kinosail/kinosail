import { describe, test } from '@e2e-dev/web';
import { expect } from 'e2e';
import { api, movie } from './helpers';

describe('media workflows', { session: 'owner' }, () => {
  test('Direct First playback decodes real moving frames and serves bounded ranges', async ({ app, browser }) => {
    await app.open('/settings');
    const item = await movie(browser);
    const playback = await api(browser, `/api/v1/items/${item.id}/playback`);
    expect(playback.status).toBe(200);
    expect(playback.data.direct).toMatch(/^\/media\//);
    const range = await browser.evaluate(async path => {
      const response = await fetch(path, { headers: { Range: 'bytes=0-31' } });
      return { status: response.status, range: response.headers.get('Content-Range'), size: (await response.arrayBuffer()).byteLength };
    }, playback.data.direct);
    expect(range.status).toBe(206);
    expect(range.range).toMatch(/^bytes 0-31\/\d+$/);
    expect(range.size).toBe(32);
    await app.open(`/watch/${item.id}`);
    await browser.evaluate(async () => {
      const video = document.querySelector('video')!;
      video.muted = true;
      await video.play();
    });
    await expect.poll(() => browser.evaluate(() => document.querySelector('video')!.getVideoPlaybackQuality().totalVideoFrames)).toBeGreaterThan(2);
    const start = await browser.evaluate(() => document.querySelector('video')!.currentTime);
    await expect.poll(() => browser.evaluate(() => document.querySelector('video')!.currentTime)).toBeGreaterThan(start + 0.25);
    await browser.evaluate(() => document.querySelector('video')!.pause());
    expect(await browser.evaluate(() => document.querySelector('video')!.paused)).toBe(true);
    await app.screenshot('decoded-playback');
  });

  test('playlist import, reorder and delete preserve item membership', async ({ app, browser }) => {
    await app.open('/settings');
    const item = await movie(browser);
    const name = `E2E playlist ${Date.now()}`;
    const path = `/api/v1/playlists/${encodeURIComponent(name)}`;
    expect((await api(browser, '/api/v1/playlists', 'POST', { name, ids: [item.id] })).status).toBe(201);
    try {
      await browser.reload();
      const original = await api(browser, `${path}?format=kinosail`);
      expect(original.status).toBe(200);
      expect(original.data.ids).toEqual([item.id]);
      for (const ids of [[item.id, item.id], ['missing'], []]) {
        expect((await api(browser, `${path}/order`, 'PUT', { ids })).status).toBe(400);
        expect((await api(browser, `${path}?format=kinosail`)).data.ids).toEqual([item.id]);
      }
      expect((await api(browser, `${path}/order`, 'PUT', { ids: [item.id] })).status).toBe(200);
    } finally {
      expect((await api(browser, path, 'DELETE')).status).toBe(204);
    }
    expect((await api(browser, path)).status).toBe(404);
  });

  test('subtitle preview is read-only and rejected or stale saves preserve the sidecar', async ({ app, browser }) => {
    await app.open('/settings');
    test.skip(!(await browser.title()).includes('Subtitles'), 'Subtitle editing belongs to Kinosail Subtitles');
    const item = await movie(browser);
    const base = `/api/v1/subtitle-library/${item.id}`;
    const original = await api(browser, `${base}/inspect?language=en`);
    expect(original.status).toBe(200);
    expect(original.data.fingerprint).toMatch(/^[a-f0-9]{64}$/);
    const preview = await api(browser, `${base}/preview`, 'POST', { language: 'en', offsetMilliseconds: 500 });
    expect(preview.status).toBe(200);
    expect((await api(browser, `${base}/inspect?language=en`)).data.fingerprint).toBe(original.data.fingerprint);
    for (const body of [{}, { language: 'en', offsetMilliseconds: 120001 }, { language: 'en', offsetMilliseconds: 1, automaticSync: true }, { language: 'en', unknown: true }, { language: 'en', fingerprint: 'bad' }]) {
      expect((await api(browser, `${base}/apply`, 'POST', body)).status).toBe(400);
      expect((await api(browser, `${base}/inspect?language=en`)).data.fingerprint).toBe(original.data.fingerprint);
    }
    const saved = await api(browser, `${base}/apply`, 'POST', { language: 'en', fingerprint: original.data.fingerprint, offsetMilliseconds: 500 });
    expect(saved.status).toBe(200);
    expect(saved.data.fingerprint).not.toBe(original.data.fingerprint);
    expect((await api(browser, `${base}/apply`, 'POST', { language: 'en', fingerprint: original.data.fingerprint, offsetMilliseconds: 1000 })).status).toBe(409);
    expect((await api(browser, `${base}/inspect?language=en`)).data.fingerprint).toBe(saved.data.fingerprint);
    expect((await api(browser, `${base}/restore`, 'POST', { language: 'en' })).status).toBe(204);
    expect((await api(browser, `${base}/inspect?language=en`)).data.fingerprint).toBe(original.data.fingerprint);
  });
});
