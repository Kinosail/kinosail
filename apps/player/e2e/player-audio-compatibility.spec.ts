import type {JSONObject} from "../../../scripts/testing/json-value";
import {expect, test} from '@playwright/test';
import {readFile, writeFile} from 'node:fs/promises';
import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {startDirectPlayer} from './player-direct-fallback-fixture';

test.afterEach(async ({browserName}, info) => {
  await writeFile(info.outputPath('audio-receipt.json'), JSON.stringify({
    revision: execFileSync('git', ['rev-parse', 'HEAD'], {encoding: 'utf8'}).trim(),
    pendingDiffSHA256: createHash('sha256').update(execFileSync('git', ['diff', 'HEAD'])).digest('hex'),
    command: 'pnpm exec playwright test player-audio-compatibility.spec.ts',
    data: 'Unsupported selected audio, automatic skip video plan, no media error; simulated decoder lifecycle',
    environment: {browser: browserName, platform: process.platform}, result: info.status,
  }, null, 2));
});

for (const scenario of [
  {name: 'confirmed unsupported audio', required: 'true', policy: 'automatic', compatible: true},
  {name: 'pre-planned compatible stream', required: 'true', policy: 'automatic', compatible: true, planned: true},
  {name: 'explicit Direct Play only', required: 'true', policy: 'direct', compatible: false},
  {name: 'compatible audio', required: 'false', policy: 'automatic', compatible: false},
  {name: 'missing audio evidence', required: undefined, policy: 'automatic', compatible: false},
  {name: 'malformed audio evidence', required: 'TRUE', policy: 'automatic', compatible: false},
]) test(`automatic skip video plan respects ${scenario.name}${scenario.compatible ? ' @smoke' : ''}`, async ({page}) => {
  await startDirectPlayer(page, {compatibleMode: 'transcode', compatibleLabel: 'Transcoding video',
    compatibleReason: 'Exact automatic skipping requires video conversion.',
    audioCompatibilityRequired: scenario.required, playbackPolicy: scenario.policy, start: 42, initialHls: scenario.planned});
  if (scenario.compatible) {
    await expect.poll(() => page.evaluate(() => (window as Window & {loadedSource?: string}).loadedSource)).toMatch(/^\/movie\.m3u8/);
    await expect(page.locator('[data-playback-mode-status]')).toHaveText('Transcoding video');
    await expect(page.locator('video')).toHaveJSProperty('currentTime', 42);
    await page.locator('video').dispatchEvent('canplay');
    await expect(page.locator('[data-playback-recovery]')).toBeHidden();
  } else {
    expect(await page.evaluate(() => (window as Window & {loadedSource?: string}).loadedSource)).toBeUndefined();
    await expect(page.locator('[data-playback-mode-status]')).toHaveText('Direct Play');
  }
});

test('browsing prepares the audible rendition when automatic skipping masks its audio mode @smoke', async ({page}) => {
  const id = '0123456789abcdef';
  const source = `/hls/${id}/p/t-a0-s0-none-t0-b0/index.m3u8`;
  const preparation: JSONObject[] = [];
  await page.route('https://audio.test/', route => route.fulfill({contentType: 'text/html', body: `<a href="/watch/${id}">Episode</a>`}));
  await page.route(`https://audio.test/api/v1/items/${id}/playback`, route => route.fulfill({json: {
    policy: 'automatic', plan: {allowed: true, mode: 'direct', audioCompatibilityRequired: true},
    compatiblePlan: {mode: 'transcode'}, compatible: source, direct: `/media/${id}`, directType: 'video/x-matroska',
  }}));
  await page.route(`https://audio.test/api/v1/items/${id}/playback-prepare`, route => {
    if (route.request().method() === 'POST') preparation.push(route.request().postDataJSON());
    return route.fulfill({json: {state: 'ready'}});
  });
  await page.route('**/static/hls.min.js*', route => route.fulfill({body: ''}));
  await page.goto('https://audio.test/');
  await page.addScriptTag({content: await readFile('../internal/server/static/playback-capabilities.js', 'utf8')});
  await page.evaluate(() => {
    (window as Window & {kinosailPlaybackCapabilities: {supports: () => Promise<boolean>}}).kinosailPlaybackCapabilities.supports = async () => false;
  });
  await page.addScriptTag({content: await readFile('../internal/server/static/startup-preparation.js', 'utf8')});
  await page.getByRole('link', {name: 'Episode'}).focus();
  await expect.poll(() => preparation).toEqual([{source}]);
});
