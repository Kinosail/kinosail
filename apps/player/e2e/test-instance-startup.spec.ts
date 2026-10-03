import {expect, test} from '@playwright/test';
import {readFile, writeFile, readdir, stat} from 'node:fs/promises';
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
  const items = (await (await page.request.get('/api/v1/library')).json()).items;
  const id = (name: string) => items.find((item: {title: string}) => item.title === name).id;
  const headers = {...await csrf(), Origin: process.env.KINOSAIL_E2E_URL!};
  const prepare = (name: string, source: string) => page.request.post(`/api/v1/items/${id(name)}/playback-prepare`, {headers, data: {source}});
  const plan = async (name: string) => (await (await page.request.get(`/api/v1/items/${id(name)}/playback?videoCodecs=h264&audioCodecs=aac`)).json());
  const source = (value: {compatible: string}, resume = 0) => value.compatible.replace('/index.m3u8', `${resume ? '-o' + Math.floor(resume * 10) * 100 : ''}/index.m3u8`);
  const receipts: object[] = [];
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
    const response = (value: import('@playwright/test').Response) => { if (value.url().endsWith('/index.m3u8') && value.headers()['x-kinosail-startup-cache']) responses.push(value.headers()['x-kinosail-startup-cache']); };
    page.on('response', response);
    const started = Date.now();
    await page.goto(`/watch/${id(name)}?compatible=1`);
    const video = page.locator('video');
    await expect.poll(() => page.evaluate(() => (window as unknown as {startupFrames: number[]}).startupFrames.length)).toBeGreaterThan(0);
    const firstMovingMs = await page.evaluate(() => (window as unknown as {startupFrames: number[]}).startupFrames[0]);
    expect(await video.evaluate(media => media.error?.code || 0)).toBe(0);
    page.off('response', response);
    return {firstMovingMs, wallMs: Date.now() - started, cache: responses, position: await video.evaluate(media => media.currentTime)};
  }
  for (const name of ['Cold', 'Warm']) {
    expect((await page.request.put(`/api/v1/items/${id(name)}/progress`, {headers, data: {seconds: 12.3}})).ok()).toBe(true);
  }
  const cold = await moving('Cold');
  receipts.push({name: 'cold', ...cold});
  await page.locator('video').evaluate(media => media.pause());
  await page.goto('/?q=Warm');
  await writeFile(info.outputPath('startup-measurements.json'), JSON.stringify(receipts, null, 2));
  if (process.env.KINOSAIL_STARTUP_BASELINE === '1') return;
  // Failure before code: this authenticated route is absent on the baseline.
  const warmSource = source(await plan('Warm'), 12.3);
  expect((await prepare('Warm', warmSource)).status()).toBe(202);
  await expect.poll(async () => (await prepare('Warm', warmSource)).json(), {timeout: 30_000}).toMatchObject({state: 'ready'});
  const warm = await moving('Warm');
  receipts.push({name: 'warm', ...warm});
  expect(warm.cache).toContain('warm');
  expect(warm.position).toBeGreaterThanOrEqual(12.3);
  expect(warm.position).toBeLessThan(16);
  await expect.poll(() => page.locator('video').evaluate(media => media.currentTime), {timeout: 25_000}).toBeGreaterThan(25);
  expect(await page.locator('video').evaluate(media => media.error?.code || 0)).toBe(0);
  await page.locator('video').evaluate(media => { media.pause(); media.currentTime = 42; });
  await page.locator('video').evaluate(media => media.play());
  await expect.poll(() => page.locator('video').evaluate(media => media.currentTime)).toBeGreaterThan(42.25);
  await page.goto('/?q=Direct');
  expect((await prepare('Direct', `/media/${id('Direct')}`)).status()).toBe(202);
  for (const bad of ['https://evil.example/a', `/hls/${id('Cold')}/p/bad/index.m3u8`, warmSource + '?token=secret', warmSource.replace('-o12300', '-o999999999')]) {
    expect((await prepare('Warm', bad)).status()).toBe(400);
  }
  const unauthenticated = await page.context().browser()!.newContext({baseURL: process.env.KINOSAIL_E2E_URL});
  expect((await unauthenticated.request.post(`/api/v1/items/${id('Warm')}/playback-prepare`, {data: {source: warmSource}})).status()).toBe(401);
  await unauthenticated.close();
  const adopt = source(await plan('Adopt'));
  expect((await prepare('Adopt', adopt)).status()).toBe(202);
  const adopted = await moving('Adopt');
  await expect.poll(() => page.locator('video').evaluate(media => media.currentTime), {timeout: 25_000}).toBeGreaterThan(14);
  receipts.push({name: 'adopted-active-preparation', ...adopted});
  expect((await prepare('Compete', source(await plan('Compete')))).status()).toBe(202);
  await page.waitForTimeout(1500);
  const logs = await readFile(join(run, 'server.log'), 'utf8');
  expect(logs).toContain('HLS startup preparation');
  expect(logs).not.toContain('secret');
  async function bytes(path: string): Promise<number> {
    let total = 0;
    for (const name of await readdir(path)) {
      const file = join(path, name); const value = await stat(file);
      total += value.isDirectory() ? await bytes(file) : value.size;
    }
    return total;
  }
  const cacheBytes = await bytes(join(run, 'cache'));
  expect(cacheBytes).toBeLessThan(512 * 1024 * 1024);
  await writeFile(info.outputPath('startup-measurements.json'), JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION, receipts, cacheBytes, result: 'passed'}, null, 2));
});
