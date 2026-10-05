import { describe, test } from '@e2e-dev/web';
import { expect } from 'e2e';
import { api, movie } from './helpers';

// Real process/public routes: no synthetic download manager or media response.
describe('prepared downloads and captions', { session: 'owner' }, () => {
  test('original preparation seals source bytes and invalid requests preserve the owned job', async ({ app, browser, screen }) => {
    await app.open('/settings');
    const item = await movie(browser);
    const playback = await api(browser, `/api/v1/items/${item.id}/playback`);
    expect(playback.status).toBe(200);
    const source = () => browser.evaluate(async path => {
      const response = await fetch(path);
      const bytes = await response.arrayBuffer();
      const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), byte => byte.toString(16).padStart(2, '0')).join('');
      return { status: response.status, size: bytes.byteLength, sha256 };
    }, playback.data.direct);
    const original = await source();
    expect(original.status).toBe(200);
    expect(original.size).toBeGreaterThan(32);
    expect(original.size).toBeLessThan(1024 * 1024);
    const previous = (await api(browser, '/api/v1/downloads')).data;
    const started = await api(browser, `/api/v1/items/${item.id}/downloads`, 'POST', { quality: 'original' });
    expect(started.status).toBe(202);
    expect(previous.downloads.some((job: { id: string }) => job.id === started.data.id)).toBe(false);
    const path = `/api/v1/downloads/${started.data.id}`;
    try {
      await expect.poll(async () => (await api(browser, path)).data.state).toBe('ready');
      const ready = await api(browser, path);
      expect(ready.data).toEqual(expect.objectContaining({ itemId: item.id, readyOffline: true, quality: 'original', size: original.size, sha256: original.sha256 }));
      const manifest = await api(browser, `${path}/manifest`);
      expect(manifest.status).toBe(200);
      expect(manifest.data).toEqual({ version: 1, id: started.data.id, size: original.size, sha256: original.sha256, chunkSize: 8 * 1024 * 1024, chunks: [original.sha256] });
      await app.open('/offline-downloads');
      await expect(screen.getByRole('heading', 'Example Movie', { exact: true })).toBeVisible();
      await expect(screen.getByRole('button', 'Download to this device', { exact: true })).toBeEnabled();
      const bytes = await browser.evaluate(async ({ path, size }) => {
        const digest = async (buffer: ArrayBuffer) => Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', buffer)), byte => byte.toString(16).padStart(2, '0')).join('');
        const full = await fetch(path + '/file'), fullBytes = await full.arrayBuffer();
        const chunk = await fetch(path + '/file', { cache: 'no-store', headers: { Range: `bytes=0-${size - 1}` } }), chunkBytes = await chunk.arrayBuffer();
        const small = await fetch(path + '/file', { headers: { Range: 'bytes=0-31' } }), smallBytes = await small.arrayBuffer();
        const unchanged = await fetch(path + '/file', { headers: { 'If-None-Match': full.headers.get('ETag')! } });
        return { full: { status: full.status, size: fullBytes.byteLength, sha256: await digest(fullBytes), etag: full.headers.get('ETag') },
          chunk: { status: chunk.status, size: chunkBytes.byteLength, sha256: await digest(chunkBytes), range: chunk.headers.get('Content-Range'), digest: chunk.headers.get('Content-Digest') },
          small: { status: small.status, size: smallBytes.byteLength, range: small.headers.get('Content-Range'), equal: new Uint8Array(smallBytes).every((byte, index) => byte === new Uint8Array(fullBytes)[index]) },
          unchanged: { status: unchanged.status, size: (await unchanged.arrayBuffer()).byteLength } };
      }, { path, size: original.size });
      expect(bytes.full).toEqual({ status: 200, size: original.size, sha256: original.sha256, etag: `"${original.sha256}"` });
      expect(bytes.chunk).toEqual({ status: 206, size: original.size, sha256: original.sha256, range: `bytes 0-${original.size - 1}/${original.size}`, digest: `sha-256=:${Buffer.from(original.sha256, 'hex').toString('base64')}:` });
      expect(bytes.small).toEqual({ status: 206, size: 32, range: `bytes 0-31/${original.size}`, equal: true });
      expect(bytes.unchanged).toEqual({ status: 304, size: 0 });
      const jobs = (await api(browser, '/api/v1/downloads')).data;
      const tracks = await api(browser, `/api/v1/items/${item.id}/download-tracks`);
      expect(tracks.status).toBe(200);
      const bodies = [
        ['missing', '{}'], ['null', 'null'], ['type', '{"quality":1}'], ['unknown quality', '{"quality":"unknown"}'],
        ['unknown field', '{"quality":"original","unknown":true}'], ['conflict', '{"quality":"original","quality":"720p"}'],
        ['case conflict', '{"quality":"original","Quality":"720p"}'], ['original tracks', '{"quality":"original","tracks":{"audio":0,"subtitles":[]}}'],
        ['malformed', '{'], ['trailing', '{"quality":"original"}{}'], ['oversized', ' '.repeat(1024 * 1024) + '{"quality":"original"}'],
      ];
      for (const [name, body] of bodies) {
        const status = await browser.evaluate(async ({ id, body }) => (await fetch(`/api/v1/items/${id}/downloads`, { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Kinosail-CSRF': document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')!.content }, body })).status, { id: item.id, body });
        expect(status, name).toBe(name === 'oversized' ? 413 : 400);
        expect((await api(browser, '/api/v1/downloads')).data, name).toEqual(jobs);
        expect(await source(), name).toEqual(original);
        expect((await api(browser, `/api/v1/items/${item.id}/download-tracks`)).data, name).toEqual(tracks.data);
      }
      for (const range of ['bytes=abc', 'bytes=10-1', `bytes=${original.size}-`, 'bytes=' + '9'.repeat(129) + '-']) {
        expect(await browser.evaluate(async ({ path, range }) => (await fetch(path + '/file', { headers: { Range: range } })).status, { path, range })).toBe(416);
        expect((await api(browser, path)).data).toEqual(ready.data);
        expect((await api(browser, '/api/v1/downloads')).data).toEqual(jobs);
        expect(await source()).toEqual(original);
      }
      for (const id of ['bad', '0'.repeat(16), 'x'.repeat(257), '%00']) {
        expect((await api(browser, `/api/v1/downloads/${id}`, 'DELETE')).status).toBe(404);
        expect((await api(browser, '/api/v1/downloads')).data).toEqual(jobs);
      }
      expect((await fetch(new URL(path + '/file', app.baseUrl))).status).toBe(401);
      expect((await api(browser, path)).data).toEqual(ready.data);
      expect((await api(browser, path + '/manifest')).data).toEqual(manifest.data);
      expect(await source()).toEqual(original);
      await app.screenshot('real-prepared-download');
    } finally {
      expect((await api(browser, path, 'DELETE')).status).toBe(204);
    }
    expect((await api(browser, '/api/v1/downloads')).data).toEqual(previous);
    expect((await api(browser, path)).status).toBe(404);
    expect((await api(browser, path + '/file')).status).toBe(404);
    expect(await source()).toEqual(original);
  });

  test('real sidecar captions expose timed cues and Off preserves decoded media', async ({ app, browser, screen }) => {
    await app.open('/settings');
    const item = await movie(browser);
    const playback = await api(browser, `/api/v1/items/${item.id}/playback`);
    expect(playback.status).toBe(200);
    const track = playback.data.subtitles.find((value: { language: string }) => value.language === 'en');
    expect(track.source).toMatch(/^\/subtitle\//);
    const before = await browser.evaluate(async path => {
      const response = await fetch(path), text = await response.text();
      return { status: response.status, type: response.headers.get('Content-Type'), text };
    }, track.source);
    expect(before.status).toBe(200);
    expect(before.type).toMatch(/^text\/vtt/);
    expect(before.text).toContain('WEBVTT');
    expect(before.text).toContain('Example dialogue.');
    await app.open(`/watch/${item.id}?direct=1`);
    await browser.evaluate(async () => { const video = document.querySelector('video')!; video.muted = true; await video.play(); });
    await expect.poll(() => browser.evaluate(() => document.querySelector('video')!.getVideoPlaybackQuality().totalVideoFrames)).toBeGreaterThan(2);
    await browser.evaluate(() => document.querySelector('video')!.pause());
    await screen.getByRole('button', 'Settings', { exact: true }).tap();
    expect(await browser.evaluate(() => document.querySelector('video track')!.getAttribute('data-subtitle-source') ?? document.querySelector('video track')!.getAttribute('src'))).toBe(track.source);
    await screen.getByRole('combobox', 'Subtitles', { exact: true }).selectOption({ value: '0' });
    await expect.poll(() => browser.evaluate(() => (document.querySelector('video track') as HTMLTrackElement).track.cues?.length)).toBe(2);
    await browser.evaluate(() => { document.querySelector('video')!.currentTime = 1; });
    await expect.poll(() => browser.evaluate(() => ((document.querySelector('video track') as HTMLTrackElement).track.activeCues?.[0] as VTTCue | undefined)?.text)).toBe('Example dialogue.');
    await app.screenshot('real-caption-cue');
    const source = await browser.evaluate(() => document.querySelector('video')!.currentSrc);
    await screen.getByRole('combobox', 'Subtitles', { exact: true }).selectOption({ value: 'off' });
    expect(await browser.evaluate(() => [...document.querySelector('video')!.textTracks].every(track => track.mode === 'disabled'))).toBe(true);
    expect(await browser.evaluate(() => document.querySelector('video')!.currentSrc)).toBe(source);
    expect(await browser.evaluate(() => document.querySelector('video')!.error)).toBe(null);
    await screen.getByRole('combobox', 'Subtitles', { exact: true }).selectOption({ value: '0' });
    await expect.poll(() => browser.evaluate(() => ((document.querySelector('video track') as HTMLTrackElement).track.activeCues?.[0] as VTTCue | undefined)?.text)).toBe('Example dialogue.');
    expect(await browser.evaluate(() => (document.querySelector('video track') as HTMLTrackElement).track.mode)).toBe('showing');
    expect(await browser.evaluate(() => document.querySelector('video')!.currentSrc)).toBe(source);
    for (const index of ['-1', '99', 'bad', '1'.repeat(129)]) {
      expect(await browser.evaluate(async path => (await fetch(path)).status, `/subtitle/${item.id}/${index}`)).toBe(404);
    }
    const after = await browser.evaluate(async path => { const response = await fetch(path); return { status: response.status, type: response.headers.get('Content-Type'), text: await response.text() }; }, track.source);
    expect(after).toEqual(before);
  });
});
