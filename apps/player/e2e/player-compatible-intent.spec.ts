import {expect, test} from '@playwright/test';
import {writeFile} from 'node:fs/promises';
import {installPlayerExperienceFixture} from './player-experience-fixture';

let releaseAdapter: () => void;
installPlayerExperienceFixture(false, false, 'iPhone', async page => {
  const ready = new Promise<void>(resolve => { releaseAdapter = resolve; });
  await page.route('**/static/hls.min.js?v=1.7.1', async route => {
    await ready;
    await route.fulfill({contentType: 'text/javascript', body: `
      window.Hls = class {
        static isSupported = () => true;
        static Events = {MANIFEST_PARSED: 'manifest', LEVEL_SWITCHED: 'level', ERROR: 'error'};
        static ErrorTypes = {};
        on() {} loadSource() {} destroy() {}
        attachMedia(value) {
          const media = value.media || value;
          media.dispatchEvent(new Event('loadedmetadata'));
          media.dataset.adapterAttached = 'true';
        }
      };
    `});
  });
  await page.evaluate(() => {
    const base = document.createElement('base');
    base.href = 'https://127.0.0.1:38127/';
    document.head.append(base);
    const media = document.querySelector('video')!;
    media.dataset.adaptive = '/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8';
    media.dataset.playbackPolicy = 'compatible';
    document.body.insertAdjacentHTML('beforeend', '<div data-quality-control><select data-quality></select><span data-quality-state></span></div>');
  });
});

test.afterEach(async ({browserName}, info) => {
  await writeFile(info.outputPath('intent-receipt.json'), JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION || process.env.GITHUB_SHA, result: info.status, command: 'playwright test player-compatible-intent.spec.ts', browser: browserName, data: 'Delayed adapter, simulated decoder metadata, real Pause control', boundary: 'Browser lifecycle fixture; real media is covered by test-instance-startup.spec.ts'}, null, 2));
});

test('compatible attachment respects Pause during resumed autoplay @smoke', async ({page}) => {
  const media = page.locator('video');
  await expect(media).toHaveJSProperty('paused', false);
  // Use keyboard activation while the pending-source status covers the stage.
  await page.getByRole('button', {name: 'Pause', exact: true}).first().press('Enter');
  await expect(media).toHaveJSProperty('paused', true);
  releaseAdapter();
  await expect(media).toHaveAttribute('data-adapter-attached', 'true');
  await expect(media).toHaveJSProperty('paused', true);
});
