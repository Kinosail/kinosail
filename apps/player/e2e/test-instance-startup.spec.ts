import {expect, test} from '@playwright/test';
import {readFile, writeFile, readdir, stat, utimes} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {join} from 'node:path';
import {totp} from './happy-path-helpers';

test('bounded startup preparation preserves the exact stream and playback priority', async ({page}, info) => {
  test.skip(process.env.KINOSAIL_STARTUP_E2E !== '1', 'Requires disposable synthetic local runner');
  test.setTimeout(240_000);
  const run = process.env.KINOSAIL_STARTUP_RUN!;
  const csrf = async () => ({'X-Kinosail-CSRF': await page.locator('meta[name="kinosail-csrf"]').getAttribute('content') || ''});
  await expect.poll(async () => { try { return (await page.request.get('/healthz')).ok(); } catch { return false; } }, {timeout: 30_000}).toBe(true);
  await page.goto('/setup');
  await page.getByLabel('Name').fill('Owner');
  await page.locator('#new-password').fill('test-startup-password');
  await page.getByLabel(/Add extra sign-in protection now/).uncheck();
  await page.getByRole('button', {name: 'Create Owner & continue'}).click();
  await page.getByRole('button', {name: 'Use an authenticator app instead'}).click();
  const secret = (await page.locator('code').first().textContent())!;
  await page.getByLabel('Authentication code').fill(totp(secret));
  await page.getByRole('button', {name: 'Turn on extra sign-in protection'}).click();
  await page.getByRole('link', {name: 'Continue to household setup'}).click();
  await page.getByRole('link', {name: 'Continue to optional viewing history'}).click();
  await page.getByRole('link', {name: 'Finish and open Library'}).click();
  // Retain the old immutable URL in this browser's cache across the upgrade.
  await page.route('**/static/theme.js?v=electric-2', route => route.fulfill({contentType: 'text/javascript', headers: {'Cache-Control': 'public, max-age=31536000, immutable'}, body: 'window.legacyStartupTheme = true;'}));
  await page.addScriptTag({url: '/static/theme.js?v=electric-2'});
  await page.goto('/');
  expect(await page.locator('script[src^="/static/theme.js?v="]').getAttribute('src')).toMatch(/\?v=[a-f0-9]{64}$/);
  const items = (await (await page.request.get('/api/v1/library')).json()).items;
  const id = (name: string) => items.find((item: {title: string}) => item.title === name).id;
  const headers = {...await csrf(), Origin: process.env.KINOSAIL_E2E_URL!};
  await page.route('**/playback-prepare', route => route.abort());
  const prepare = (name: string, source: string) => page.request.post(`/api/v1/items/${id(name)}/playback-prepare`, {headers, data: {source}});
  let negotiatedQuery = '';
  const plan = async (name: string) => (await (await page.request.get(`/api/v1/items/${id(name)}/playback${negotiatedQuery}`)).json());
  const source = (value: {compatible: string}, resume = 0) => value.compatible.replace('/index.m3u8', `${resume ? '-o' + Math.floor(resume * 10) * 100 : ''}/index.m3u8`);
  const receipts: object[] = [];
  async function record(value: object) {
    receipts.push(value);
    await writeFile(info.outputPath('startup-measurements.json'), JSON.stringify(receipts, null, 2));
  }
  async function bytes(path: string): Promise<number> {
    let total = 0;
    for (const name of await readdir(path)) {
      const file = join(path, name); const value = await stat(file);
      total += value.isDirectory() ? await bytes(file) : value.size;
    }
    return total;
  }
  const cacheDirectory = async (name: string, token = '') => join(run, 'cache', (await readdir(join(run, 'cache'))).find(value => value.startsWith(id(name)) && value.includes(token))!);
  async function initHashes(name: string, token = '') {
    const directory = await cacheDirectory(name, token);
    const hashes: Record<string, string> = {};
    for (const rendition of await readdir(directory)) {
      if (rendition.startsWith('.')) continue;
      const path = join(directory, rendition, 'init.mp4');
      try { hashes[rendition] = createHash('sha256').update(await readFile(path)).digest('hex'); } catch {}
    }
    return hashes;
  }

  async function moving(name: string) {
    await page.addInitScript(() => {
      (window as unknown as {startupFrames: number[]}).startupFrames = [];
      document.addEventListener('DOMContentLoaded', () => {
        const media = document.querySelector('video');
        if (!media) return;
        let last = -1;
        const frame = (now: number, metadata: VideoFrameCallbackMetadata) => {
          if (last >= 0 && metadata.mediaTime > last) (window as unknown as {startupFrames: number[]}).startupFrames.push(now);
          last = metadata.mediaTime;
          media.requestVideoFrameCallback(frame);
        };
        media.requestVideoFrameCallback(frame);
      });
    });
    const responses: string[] = [];
    const response = (value: import('@playwright/test').Response) => {
      const url = new URL(value.url());
      if (url.pathname.endsWith('/playback')) negotiatedQuery = url.search;
      if (url.pathname.endsWith('/index.m3u8') && value.headers()['x-kinosail-startup-cache']) responses.push(value.headers()['x-kinosail-startup-cache']);
    };
    page.on('response', response);
    const started = Date.now();
    const link = page.locator(`a[href="/watch/${id(name)}"]:visible`).first();
    await expect(link).toBeVisible();
    await link.click();
    const video = page.locator('video');
    await expect.poll(() => page.evaluate(() => (window as unknown as {startupFrames: number[]}).startupFrames.length)).toBeGreaterThan(0);
    const frame = await page.evaluate(() => {
      const navigationMS = (window as unknown as {startupFrames: number[]}).startupFrames[0];
      return {navigationMS, epochMS: performance.timeOrigin + navigationMS};
    });
    const firstMovingMs = frame.epochMS - started;
    expect(await video.evaluate(media => media.error?.code || 0)).toBe(0);
    page.off('response', response);
    return {firstMovingMs, navigationFirstMovingMs: frame.navigationMS, wallMs: Date.now() - started, cache: responses, position: await video.evaluate(media => media.currentTime)};
  }
  for (const name of ['Cold', 'Warm']) {
    expect((await page.request.put(`/api/v1/items/${id(name)}/progress`, {headers, data: {seconds: 12.3}})).ok()).toBe(true);
  }
  expect((await page.request.put('/api/v1/settings/playback', {headers, data: {mode: 'automatic', autoplay: true, subtitles: 'on', autoSkip: []}})).ok()).toBe(true);
  await page.evaluate(() => localStorage.setItem('kinosail.playback-policy-v2', 'compatible'));
  await page.goto('/?q=Cold');
  const cold = await moving('Cold');
  await record({name: 'cold', ...cold});
  await page.locator('video').evaluate(media => media.pause());
  await page.goto('/?q=Warm');
  await page.unroute('**/playback-prepare');
  await writeFile(info.outputPath('startup-measurements.json'), JSON.stringify(receipts, null, 2));
  if (process.env.KINOSAIL_STARTUP_BASELINE === '1') return;
  // Failure before code: this authenticated route is absent on the baseline.
  const warmSource = source(await plan('Warm'), 12.3);
  const triggered = page.waitForRequest(request => request.method() === 'POST' && request.url().endsWith(`/items/${id('Warm')}/playback-prepare`));
  await page.locator(`a[href="/watch/${id('Warm')}"]`).first().dispatchEvent('focusin');
  expect((await triggered).postDataJSON()).toEqual({source: warmSource});
  await expect.poll(async () => (await prepare('Warm', warmSource)).json(), {timeout: 30_000}).toMatchObject({state: 'ready'});
  const originalInit = await initHashes('Warm');
  expect(Object.keys(originalInit).length).toBeGreaterThan(0);
  const warm = await moving('Warm');
  await record({name: 'warm', ...warm});
  expect(warm.cache).toContain('warm');
  expect(warm.position).toBeGreaterThanOrEqual(12.3);
  expect(warm.position).toBeLessThan(16);
  await expect.poll(() => page.locator('video').evaluate(media => media.currentTime), {timeout: 25_000}).toBeGreaterThan(25);
  expect(await page.locator('video').evaluate(media => media.error?.code || 0)).toBe(0);
  expect(await initHashes('Warm')).toEqual(originalInit);
  await page.locator('video').evaluate(media => {
    if (media.textTracks[0]) media.textTracks[0].mode = 'showing';
  });
  await expect.poll(() => page.locator('video').evaluate(media => media.textTracks[0]?.cues?.length || 0)).toBeGreaterThan(0);
  await page.locator('video').evaluate(media => { media.pause(); media.currentTime = 42; });
  await page.locator('video').evaluate(media => media.play());
  await expect.poll(() => page.locator('video').evaluate(media => media.currentTime)).toBeGreaterThan(42.25);
  await page.reload();
  await expect.poll(() => page.locator('video').evaluate(media => media.currentTime)).toBeGreaterThan(40);
  await page.locator('video').evaluate(media => media.pause());
  await page.goto('/?q=Direct');
  expect((await prepare('Direct', `/media/${id('Direct')}`)).status()).toBe(202);
  const beforeRejected = (await readdir(join(run, 'cache'))).sort();
  for (const bad of ['https://evil.example/a', `/hls/${id('Cold')}/p/bad/index.m3u8`, warmSource + '?token=secret', warmSource.replace('-o12300', '-o999999999')]) {
    expect((await prepare('Warm', bad)).status()).toBe(400);
  }
  const unauthenticated = await page.context().browser()!.newContext({baseURL: process.env.KINOSAIL_E2E_URL});
  expect((await unauthenticated.request.post(`/api/v1/items/${id('Warm')}/playback-prepare`, {data: {source: warmSource}})).status()).toBe(401);
  await unauthenticated.close();
  expect((await readdir(join(run, 'cache'))).sort()).toEqual(beforeRejected);
  await page.evaluate(() => localStorage.setItem('kinosail.playback-policy-v2', 'direct-first'));
  const directPlayback = await moving('Direct');
  await record({name: 'direct-first', ...directPlayback});
  expect(await page.locator('video').evaluate(media => media.currentSrc)).toContain(`/media/${id('Direct')}`);
  expect((await readdir(join(run, 'cache'))).some(value => value.startsWith(id('Direct')))).toBe(false);
  await page.locator('video').evaluate(media => media.pause());
  await page.evaluate(() => localStorage.setItem('kinosail.playback-policy-v2', 'compatible'));
  await page.goto('/?q=Adopt');
  const adopt = source(await plan('Adopt'));
  expect((await prepare('Adopt', adopt)).status()).toBe(202);
  await expect.poll(async () => {
    try { return Object.keys(await initHashes('Adopt')).length; } catch { return 0; }
  }, {timeout: 20_000, intervals: [50]}).toBeGreaterThan(0);
  // A real native HLS master request adopts the running preparation atomically.
  expect((await page.request.get(adopt)).ok()).toBe(true);
  const adoptedInit = await initHashes('Adopt');
  const adopted = await moving('Adopt');
  await expect.poll(() => page.locator('video').evaluate(media => media.currentTime), {timeout: 25_000}).toBeGreaterThan(14);
  expect(await initHashes('Adopt')).toEqual(adoptedInit);
  await record({name: 'adopted-active-preparation', ...adopted});
  expect((await prepare('Compete', source(await plan('Compete')))).status()).toBe(202);
  await page.waitForTimeout(1500);
  expect((await readdir(join(run, 'cache'))).some(value => value.startsWith(id('Compete')))).toBe(false);
  await page.locator('video').evaluate(media => media.pause());
  await page.route('**/playback-prepare', route => route.abort());
  await page.goto('/?q=Compete');
  await page.waitForLoadState('networkidle');
  // Keep admission closed without new media GETs clearing the queue under test.
  const queueTrace = (sequence: number, event: string, paused: boolean) => page.request.post(`/api/v1/items/${id('Compete')}/playback-events`, {headers, data: {session: 'startup-queue-bound', sequence, event, paused}});
  expect((await queueTrace(1, 'play-request', false)).status()).toBe(204);
  const competePlan = await plan('Compete');
  expect((await prepare('Compete', source(competePlan))).status()).toBe(202);
  expect((await prepare('Compete', source(competePlan, 20))).status()).toBe(202);
  expect((await prepare('Compete', source(competePlan, 30))).status()).toBe(202);
  expect((await prepare('Compete', source(competePlan, 40))).status()).toBe(429);
  expect((await page.request.delete(`/api/v1/items/${id('Compete')}/playback-prepare`, {headers})).status()).toBe(204);
  expect((await queueTrace(2, 'pause', true)).status()).toBe(204);
  await page.goto('/?q=Invalidation');
  await page.route('**/playback-prepare', route => route.abort());
  const staleSource = source(await plan('Invalidation'));
  expect((await prepare('Invalidation', staleSource)).status()).toBe(202);
  await expect.poll(async () => (await prepare('Invalidation', staleSource)).json(), {timeout: 30_000}).toMatchObject({state: 'ready'});
  const staleDirectory = await cacheDirectory('Invalidation');
  const beforeVersion = await readFile(join(staleDirectory, 'index.m3u8'), 'utf8');
  const changed = new Date(Date.now() + 2000);
  await utimes(join(run, 'media', 'Invalidation.mkv'), changed, changed);
  expect(await (await prepare('Invalidation', staleSource)).json()).toMatchObject({state: 'queued'});
  await expect.poll(async () => (await prepare('Invalidation', staleSource)).json(), {timeout: 30_000}).toMatchObject({state: 'ready'});
  expect(await readFile(join(staleDirectory, 'index.m3u8'), 'utf8')).not.toBe(beforeVersion);
  // Force the full video conversion path with bounded software encoders.
  const settings = {quality: 'speed', codec: 'auto', accelerator: 'none', toneMap: true};
  expect((await page.request.put('/api/v1/settings/transcoder', {headers, data: settings})).ok()).toBe(true);
  const transcode = await (await page.request.get(`/api/v1/items/${id('Invalidation')}/playback?videoCodecs=h264`)).json();
  const burnSupported = process.env.KINOSAIL_STARTUP_BURN_SUPPORTED === '1';
  const burnSource = burnSupported ? source(transcode).replace('-s0-none-', '-s0-external-') : source(transcode);
  const transcodeToken = burnSource.split('/')[4];
  expect((await prepare('Invalidation', burnSource)).status()).toBe(202);
  await expect.poll(async () => {
    try { return Object.keys(await initHashes('Invalidation', transcodeToken)).length; } catch { return 0; }
  }, {timeout: 20_000, intervals: [100]}).toBeGreaterThan(0);
  await expect.poll(async () => {
    try { return (await readFile(join(await cacheDirectory('Invalidation', transcodeToken), 'index.m3u8'))).length; } catch { return 0; }
  }, {timeout: 20_000, intervals: [50]}).toBeGreaterThan(0);
  const partialInit = await initHashes('Invalidation', transcodeToken);
  expect((await page.request.delete(`/api/v1/items/${id('Invalidation')}/playback-prepare`, {headers})).status()).toBe(204);
  await page.waitForTimeout(600);
  expect((await prepare('Invalidation', burnSource)).status()).toBe(202);
  await expect.poll(async () => (await prepare('Invalidation', burnSource)).json(), {timeout: 30_000}).toMatchObject({state: 'ready'});
  const burnDirectories = (await readdir(join(run, 'cache'))).filter(value => value.startsWith(id('Invalidation')) && value.includes(transcodeToken));
  expect(burnDirectories).toHaveLength(1);
  const burnDirectory = join(run, 'cache', burnDirectories[0]);
  const readBurnVersion = () => readFile(join(burnDirectory, 'index.m3u8'), 'utf8');
  expect(await initHashes('Invalidation', transcodeToken)).toEqual(partialInit);
  if (burnSupported) {
  const subtitleVersion = await readBurnVersion();
  await utimes(join(run, 'media', 'Invalidation.en.srt'), changed, changed);
  expect(await (await prepare('Invalidation', burnSource)).json()).toMatchObject({state: 'queued'});
  await expect.poll(async () => (await prepare('Invalidation', burnSource)).json(), {timeout: 30_000}).toMatchObject({state: 'ready'});
  expect(await readBurnVersion()).not.toBe(subtitleVersion);
  } else await record({name: 'burn-in-subtitle-invalidation', result: 'not run: host FFmpeg lacks subtitles filter'});
  const settingsVersion = await readBurnVersion();
  expect((await page.request.put('/api/v1/settings/transcoder', {headers, data: {...settings, quality: 'quality'}})).ok()).toBe(true);
  expect(await (await prepare('Invalidation', burnSource)).json()).toMatchObject({state: 'queued'});
  await expect.poll(async () => (await prepare('Invalidation', burnSource)).json(), {timeout: 30_000}).toMatchObject({state: 'ready'});
  expect(await readBurnVersion()).not.toBe(settingsVersion);
  // Decode across the prepared eight seconds using the original init and lazy segments.
  const continuationInit = await initHashes('Invalidation', transcodeToken);
  const master = await page.request.get(burnSource);
  expect(master.ok()).toBe(true);
  const rendition = (await master.text()).split('\n').find(line => line.endsWith('/index.m3u8'))!;
  const base = burnSource.replace('index.m3u8', rendition.replace('index.m3u8', ''));
  const fragments = [];
  for (const file of ['init.mp4', ...Array.from({length: 6}, (_, index) => `segment-${String(index).padStart(5, '0')}.m4s`)]) {
    const response = await page.request.get(base + file);
    expect(response.ok()).toBe(true);
    fragments.push(await response.body());
  }
  const spanning = info.outputPath('synthetic-original-init-spanning-window.mp4');
  await writeFile(spanning, Buffer.concat(fragments));
  const decode = execFileSync('ffmpeg', ['-nostdin', '-v', 'error', '-xerror', '-threads', '2', '-i', spanning, '-progress', 'pipe:1', '-f', 'null', '-'], {encoding: 'utf8'});
  expect(Number([...decode.matchAll(/frame=(\d+)/g)].at(-1)?.[1] || 0)).toBeGreaterThanOrEqual(240);
  expect(await initHashes('Invalidation', transcodeToken)).toEqual(continuationInit);
  const resources = JSON.parse(await readFile(join(run, 'resources.json'), 'utf8'));
  expect(resources.peakSpeculativeFFmpeg).toBeLessThanOrEqual(1);
  expect(resources.peakFFmpeg).toBeLessThanOrEqual(2);
  await record({name: 'resource-bounds', ...resources});
  const logs = await readFile(join(run, 'server.log'), 'utf8');
  expect(logs).toContain('HLS startup preparation');
  expect(logs).not.toContain('secret');
  const cacheBytes = await bytes(join(run, 'cache'));
  expect(cacheBytes).toBeLessThan(512 * 1024 * 1024);
  await writeFile(info.outputPath('startup-measurements.json'), JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION, receipts, cacheBytes, result: 'passed'}, null, 2));
});
