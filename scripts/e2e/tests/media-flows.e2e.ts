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

  test('an unplayed watch page preserves saved progress and manual watched status', async ({ app, browser, screen }) => {
    await app.open('/settings');
    // Model a browser that requires a fresh user gesture for Play. The original
    // decoder remains in use for explicit Play and real moving-frame assertions.
    await browser.addInitScript(() => {
      const play = HTMLMediaElement.prototype.play;
      Object.assign(window,{restoreAutoplayPolicy:()=>{HTMLMediaElement.prototype.play=play;}});
      HTMLMediaElement.prototype.play = function () {
        return Promise.reject(new DOMException('', 'NotAllowedError'));
      };
    });
    const item = await movie(browser), path = `/api/v1/items/${item.id}`;
    expect((await api(browser, path + '/progress', 'PUT', { seconds: 3, watched: false })).status).toBe(200);
    const saved = (await api(browser, path)).data.item.progress;
    await app.open(`/watch/${item.id}`);
    await expect.poll(() => browser.evaluate(() => document.querySelector('video')!.readyState)).toBeGreaterThanOrEqual(2);
    await expect.poll(() => browser.evaluate(() => document.querySelector('video')!.currentTime)).toBe(3);
    expect(await browser.evaluate(() => document.querySelector('video')!.paused)).toBe(true);
    await screen.getByRole('link', 'Library', { exact: true }).click();
    await expect(browser).toHaveURL('/');
    await expect.poll(() => api(browser, path).then(value => value.data.item.progress)).toEqual(saved);
    await app.open(`/watch/${item.id}`);
    await screen.getByRole('button', 'Mark watched', { exact: true }).click();
    await expect(screen.getByRole('button', 'Mark unwatched', { exact: true })).toBeVisible();
    const watched = (await api(browser, path)).data.item.progress;
    expect(watched.watched).toBe(true);
    await expect.poll(() => browser.evaluate(() => document.querySelector('video')!.readyState)).toBeGreaterThanOrEqual(3);
    for(const viewport of [{width:390,height:844},{width:1440,height:900},{width:1920,height:1080}]) {
      await browser.setViewport(viewport);
      expect(await browser.evaluate(() => (document.querySelector('[data-player-status]') as HTMLElement).hidden)).toBe(true);
      await expect(screen.getByRole('button','Play',{exact:true}).first()).toBeVisible();
      await app.screenshot(`ready-paused-watched-${viewport.width}`);
    }
    await browser.setViewport({width:1440,height:900});
    await screen.getByRole('link', 'Library', { exact: true }).click();
    await expect(browser).toHaveURL('/');
    await expect.poll(() => api(browser, path).then(value => value.data.item.progress)).toEqual(watched);
    await app.screenshot('unplayed-progress-preserved');
    await app.open(`/watch/${item.id}`);
    expect(await browser.evaluate(() => document.querySelector('video')!.paused)).toBe(true);
    await browser.evaluate(() => { (window as Window & {restoreAutoplayPolicy():void}).restoreAutoplayPolicy();document.querySelector('video')!.muted = true; });
    await screen.getByRole('button', 'Play', { exact: true }).first().click();
    await expect.poll(() => browser.evaluate(() => document.querySelector('video')!.getVideoPlaybackQuality().totalVideoFrames)).toBeGreaterThan(2);
    await browser.evaluate(async () => {
      const video = document.querySelector('video')!; video.pause();
      await new Promise<void>(resolve => { video.addEventListener('seeked', () => resolve(), {once: true}); video.currentTime = 0; });
    });
    await screen.getByRole('link', 'Library', { exact: true }).click();
    await expect(browser).toHaveURL('/');
    await expect.poll(() => api(browser, path + '/watch-progress').then(value => value.data.seconds)).toBe(0);
    expect((await api(browser, path)).data.item.progress.watched ?? false).toBe(false);
    await app.open(`/watch/${item.id}`);
    await browser.evaluate(() => { (window as Window & {restoreAutoplayPolicy():void}).restoreAutoplayPolicy();document.querySelector('video')!.muted = true; });
    await screen.getByRole('button', 'Play', { exact: true }).first().click();
    await expect.poll(() => browser.evaluate(() => document.querySelector('video')!.getVideoPlaybackQuality().totalVideoFrames)).toBeGreaterThan(2);
    await screen.getByRole('button', 'Mark watched', { exact: true }).click();
    await expect(screen.getByRole('button', 'Mark unwatched', { exact: true })).toBeVisible();
    expect(await browser.evaluate(() => document.querySelector('video')!.paused)).toBe(true);
    const completed = (await api(browser, path)).data.item.progress;
    expect(completed.watched).toBe(true);
    await screen.getByRole('link', 'Library', { exact: true }).click();
    await expect(browser).toHaveURL('/');
    await expect.poll(() => api(browser, path).then(value => value.data.item.progress)).toEqual(completed);
    expect((await api(browser, path + '/progress', 'PUT', { seconds: 0, watched: false })).status).toBe(200);
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
