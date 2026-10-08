import {test, expect, type Request, type Response} from '@playwright/test';
import {writeFile, chmod} from 'node:fs/promises';
import {join, resolve} from 'node:path';
import {installPresentedFrames, firstPresentedFrame} from './hls-presented-frame.mjs';
import {readPresentationState, readPresentationMap} from './hls-presentation-state.mjs';
import {offlineBrowserAPI} from './offline-browser-api.mjs';
import {fixtureBrowserFetch, decodeFixtureJSON, fixtureWatchProgress} from '../../../scripts/e2e/fixture-response.mjs';

// This case uses the same owned Go Server and Owner setup as the startup case.
// It observes real decoded pixels; no currentTime assignment or fake seek event.
export function registerPresentedSeek() {
 if (process.env.KINOSAIL_HLS_PRESENTATION_PROOF !== '1') return;
 test('copied HLS backward seek presents the exact requested source frame first', async ({browser, baseURL}, info) => {
  test.setTimeout(120_000);
  if (typeof baseURL !== 'string' || baseURL.length > 2048) throw new Error('owned startup origin required');
  const origin = new URL(baseURL);
  if (origin.protocol !== 'http:' || origin.hostname !== 'localhost' || !origin.port || origin.port === '0'
   || baseURL !== origin.origin || origin.username || origin.password) throw new Error('owned startup origin required');
  const run = process.env.KINOSAIL_STARTUP_RUN!;
  const source = readPresentationMap(run, resolve('../../../.verification/startup'));
  const fields = ['schema', 'frameRate', 'frameCount', 'seekSeconds', 'targetFrame', 'prerollFrame', 'sourcePTS',
   'command', 'probe', 'sourceSHA256', 'probeSHA256', 'pixelFormat', 'marker'];
  if (!source || typeof source !== 'object' || Array.isArray(source) || Object.keys(source).sort().join(',') !== fields.sort().join(',')
   || source.schema !== 1 || source.frameRate !== 24 || source.frameCount !== 768 || source.targetFrame !== 300
   || source.prerollFrame !== 288 || source.seekSeconds !== 12.5 || source.pixelFormat !== 'yuv420p'
   || !/^[a-f0-9]{64}$/.test(source.sourceSHA256) || !/^[a-f0-9]{64}$/.test(source.probeSHA256)
   || !['command', 'probe'].every(key => Array.isArray(source[key]) && source[key].length <= 80
    && source[key].every((value: unknown) => typeof value === 'string' && Buffer.byteLength(value) <= 2048))
   || source.marker !== 'ten binary luminance bits + complement + black/white guards'
   || !Array.isArray(source.sourcePTS) || source.sourcePTS.length !== 768
   || source.sourcePTS.some((value: number, i: number) => !Number.isFinite(value) || value < 0 || value >= 33
    || Math.abs(value - source.sourcePTS[0] - i / 24) > .0011)) throw new Error('source frame map rejected');
  const storageState = readPresentationState(run, resolve('../../../.verification/startup'), origin.origin);
  const context = await browser.newContext({baseURL, storageState, ignoreHTTPSErrors: false});
  let page: import('@playwright/test').Page;
  try {page = await context.newPage();} catch (error) {await context.close().catch(() => {}); throw error;}
  const rows: {kind: string, offset: number, status: number | null, range: boolean, failed: boolean}[] = [];
  let itemID = '', overflow = false, phase = 'owned-library', primary: unknown;
  let observation: unknown = null, initial: unknown = null, persisted: unknown = null;
  const selected = (request: Request) => {
   if (request.url().length > 4096) return null;
   const url = new URL(request.url());
   if (url.origin !== origin.origin || request.method() !== 'GET') return null;
   const match = url.pathname.match(new RegExp(`^/hls/${itemID}/p/([ra])-a0-s0-none-t0-b0-o(30000|12500)/(?:index\\.m3u8|[0-9]+p/(?:index\\.m3u8|init\\.mp4|segment-[0-9]{5}\\.m4s))$`));
   if (!match) return null;
   return {kind: url.pathname.endsWith('index.m3u8') ? 'playlist' : 'fragment', offset: Number(match[2]) / 1000};
  };
  const record = (request: Request, status: number | null, failed: boolean) => {
   const row = selected(request); if (!row) return;
   if (rows.length === 64) {overflow = true; return;}
   rows.push({...row, status, failed, range: /^bytes=[0-9]+-[0-9]*$/.test(request.headers().range || '')});
  };
  const response = (value: Response) => record(value.request(), value.status(), false);
  const failed = (value: Request) => record(value, null, true);
  async function api(path: string, method = 'GET', body: unknown = null) {
   if (page.url().length > 4096 || new URL(page.url()).origin !== origin.origin) throw new Error('startup page left owned origin');
   const result = await page.evaluate(fixtureBrowserFetch, {origin: origin.origin, path, method,
    body: body === null ? null : JSON.stringify(body)});
   expect([200, 202, 204]).toContain(result.status);
   return result.status === 204 ? null : decodeFixtureJSON(result.raw);
  }
  try {
   await page.goto('/');
   const library = await offlineBrowserAPI(page, 'library', baseURL);
   const matches = library.filter((item: {kind: string, title: string}) => item.kind === 'video' && item.title === 'HLS Presented');
   expect(matches).toHaveLength(1); itemID = matches[0].id;
   expect(itemID).toMatch(/^[a-f0-9]{16}$/);
   await api('/api/v1/settings/transcoder', 'PUT', {quality: 'automatic', codec: 'auto', accelerator: 'none', toneMap: true});
   await api(`/api/v1/items/${itemID}/progress`, 'PUT', {seconds: 30});
   await page.evaluate(() => localStorage.setItem('kinosail.playback-policy-v2', 'compatible'));
   await page.addInitScript(installPresentedFrames);
   page.on('response', response); page.on('requestfailed', failed);
   phase = 'resume-old-window';
   await page.goto(`/watch/${itemID}`);
   const video = page.locator('video');
   await expect.poll(() => video.evaluate(v => v.readyState), {timeout: 30_000}).toBeGreaterThanOrEqual(2);
   await expect.poll(() => rows.some(row => row.kind === 'playlist' && row.offset === 30 && row.status === 200)).toBe(true);
   await page.evaluate(() => (window as any).hlsPresented.prepare());
   await page.locator('.media-stage').focus();
   await expect(page.locator('.media-stage')).toBeFocused();
   if (await video.evaluate(v => v.paused)) await page.keyboard.press('Space');
   await expect.poll(() => page.evaluate(() => (window as any).hlsPresented.snapshot().frames.length)).toBeGreaterThan(0);
   await page.locator('.media-stage').focus();
   await expect(page.locator('.media-stage')).toBeFocused();
   await page.keyboard.press('Space');
   await expect.poll(() => video.evaluate(v => v.paused)).toBe(true);
   initial = await page.evaluate(() => (window as any).hlsPresented.snapshot());
   expect((initial as any).frames.every((frame: any) => frame.frameIndex >= 720)).toBe(true);
   await page.getByRole('button', {name: 'Settings', exact: true}).click();
   const slider = page.getByRole('slider', {name: 'Movie position', exact: true});
   await expect(slider).toBeVisible();
   const box = await slider.boundingBox(); if (!box || box.width < 100 || box.height < 1) throw new Error('public seek control unavailable');
   const max = Number(await slider.getAttribute('max')); expect(max).toBeGreaterThan(31); expect(max).toBeLessThan(33);
   phase = 'trusted-backward-seek';
   await page.mouse.move(box.x + box.width - 8, box.y + box.height / 2);
   await page.mouse.down();
   let x = box.x + 8 + (box.width - 16) * 12.5 / max;
   for (let attempt = 0; attempt < 12; attempt++) {
    await page.mouse.move(x, box.y + box.height / 2);
    const value = Number(await slider.inputValue());
    if (value === 12.5) break;
    x += (12.5 - value) * (box.width - 16) / max;
    if (x < box.x || x > box.x + box.width) throw new Error('trusted seek adjustment escaped control');
   }
   expect(await slider.inputValue()).toBe('12.5');
   await page.mouse.up();
   await expect.poll(() => page.evaluate(() => (window as any).hlsPresented.snapshot().committed)).toBe(true);
   await expect.poll(() => rows.some(row => row.kind === 'playlist' && row.offset === 12.5 && row.status === 200), {timeout: 30_000}).toBe(true);
   phase = 'first-settled-presented-frame';
   await expect.poll(() => page.evaluate(() => (window as any).hlsPresented.snapshot().frames.some((f: any) => f.afterCommit && f.visible && !f.pending && f.frameIndex < 720)), {timeout: 20_000}).toBe(true);
   observation = await page.evaluate(() => (window as any).hlsPresented.snapshot());
   await page.evaluate(() => (window as any).hlsPresented.stop());
   const first = firstPresentedFrame(observation);
   expect(source.sourcePTS[first.frameIndex] - source.sourcePTS[0]).toBeCloseTo(12.5, 3);
   expect(overflow).toBe(false); expect(rows.some(row => row.failed || row.status !== 200 && row.status !== 206)).toBe(false);
   phase = 'persisted-public-position';
   await expect.poll(async () => {
    persisted = fixtureWatchProgress(await api(`/api/v1/items/${itemID}/watch-progress`));
    return (persisted as {seconds: number}).seconds;
   }).toBeCloseTo(12.5, 1);
   await page.getByRole('button', {name: 'Close playback settings'}).click();
   await page.locator('.media-stage').focus();
   await expect(page.locator('.media-stage')).toBeFocused();
   if (await video.evaluate(v => v.paused)) await page.keyboard.press('Space');
   await expect.poll(() => video.evaluate(v => v.currentTime), {timeout: 20_000}).toBeGreaterThan(12.75);
   expect(await video.evaluate(v => v.error?.code || 0)).toBe(0);
   phase = 'passed';
  } catch (error) {primary = error; throw error;}
  finally {
   await page.mouse.up().catch(() => {});
   if (observation === null) observation = await page.evaluate(() => (window as any).hlsPresented?.snapshot() ?? null).catch(() => null);
   await page.evaluate(() => (window as any).hlsPresented?.stop()).catch(() => {});
   page.off('response', response); page.off('requestfailed', failed);
   const receipt = {schema: 1, phase, result: primary ? 'failed' : 'passed', revision: process.env.KINOSAIL_TEST_REVISION,
    command: 'existing startup runner; opt-in real HLS presentation case', sourceSHA256: source.sourceSHA256,
    sourcePTS: {origin: source.sourcePTS[0], target: source.sourcePTS[300], preroll: source.sourcePTS[288]},
    requestedSeconds: 12.5, initial, observation, persisted, network: rows, overflow,
    boundary: 'Chromium real decoded coded pixels; no physical Safari, audible output, or old raw-preroll oracle change'};
   try {
    const raw = JSON.stringify(receipt); if (Buffer.byteLength(raw) > 24576) throw new Error('presentation receipt exceeded bound');
    const path = info.outputPath('hls-presented-frame.json'); await writeFile(path, raw); await chmod(path, 0o600);
    await info.attach('hls-presented-frame', {path, contentType: 'application/json'});
   } catch (error) {if (!primary) throw error;}
   finally {await context.close().catch(error => {if (!primary) throw error;});}
  }
 });
}
