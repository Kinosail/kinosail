import {writeFile} from 'node:fs/promises';
import {expect, test} from '@playwright/test';
import {installPlayerExperienceFixture} from './player-experience-fixture';

declare const playbackRequest: number;

test.use({hasTouch: true, viewport: {width: 390, height: 844}, ignoreHTTPSErrors: false});
installPlayerExperienceFixture(false, true);

test.afterEach(async ({browserName}, info) => {
  await writeFile(info.outputPath('apple-intent-receipt.json'), JSON.stringify({
    revision: process.env.KINOSAIL_TEST_REVISION || process.env.GITHUB_SHA,
    command: 'playwright test player-apple-intent.spec.ts --workers=1', result: info.status,
    browser: browserName, data: 'Real Play controls; simulated Apple API and deferred media Play promise',
    boundary: 'Public browser lifecycle reproduction; no production transport or physical iPhone proof',
  }, null, 2));
});

async function pendingPlay(page: import('@playwright/test').Page) {
  await page.evaluate(() => (window as Window & {setPlayPending: (value: boolean) => void}).setPlayPending(true));
  await page.getByRole('button', {name: 'Play', exact: true}).tap();
  await expect(page.locator('video')).toHaveJSProperty('paused', false);
}

test('an obsolete Apple Play rejection preserves a newer successful Play @smoke', async ({page}) => {
  await pendingPlay(page);
  await page.evaluate(() => {
    const state = window as Window & {setPaused: (value: boolean) => void; setPlayPending: (value: boolean) => void};
    state.setPaused(true);
    state.setPlayPending(false);
    document.querySelector('video')!.dispatchEvent(new Event('pause'));
  });
  await page.getByRole('button', {name: 'Play', exact: true}).tap();
  await expect(page.locator('video')).toHaveJSProperty('paused', false);
  await page.evaluate(() => (window as Window & {rejectPendingPlay: (name: string) => void}).rejectPendingPlay('AbortError'));
  await expect(page.locator('video')).toHaveJSProperty('paused', false);
  await expect(page.locator('video')).toHaveJSProperty('webkitDisplayingFullscreen', true);
  await expect(page.locator('.player-control-feedback')).toBeHidden();
});

test('an obsolete Apple Play rejection respects later fullscreen dismissal @smoke', async ({page}) => {
  await pendingPlay(page);
  await page.locator('video').evaluate(media => (media as HTMLVideoElement & {webkitExitFullscreen: () => void}).webkitExitFullscreen());
  await page.evaluate(() => (window as Window & {rejectPendingPlay: (name: string) => void}).rejectPendingPlay('AbortError'));
  await expect(page.locator('video')).toHaveJSProperty('paused', true);
  await expect(page.getByRole('button', {name: 'Play', exact: true})).toBeVisible();
  expect(await page.locator('.player-control-feedback').isVisible()).toBe(false);
});


// Native Pause is an Apple platform action unavailable in populated Chromium.
// This existing simulated-Apple fixture isolates the event seam; physical/Toy Story attribution stays separate.
test('simulated native Apple Pause cancels pending Play without a startup failure @smoke', async ({page}, info) => {
  const observations: Array<Record<string, unknown>> = [];
  const observe = async (stage: string) => observations.push(await page.evaluate(label => {
    const video = document.querySelector('video') as HTMLVideoElement & {webkitDisplayingFullscreen: boolean};
    const feedback = document.querySelector('.player-control-feedback') as HTMLElement | null;
    return {stage: label, elapsedMs: Math.round(performance.now()), paused: video.paused,
      fullscreen: video.webkitDisplayingFullscreen, requestGeneration: typeof playbackRequest === 'number' ? playbackRequest : null,
      feedbackVisible: Boolean(feedback && !feedback.hidden && feedback.getClientRects().length),
      feedbackText: feedback?.textContent?.trim() || ''};
  }, stage));
  await pendingPlay(page);
  await observe('play-promise-pending');
  await page.locator('video').evaluate((media: HTMLVideoElement) => media.pause());
  await observe('native-pause');
  await page.evaluate(() => (window as Window & {rejectPendingPlay: (name: string) => void}).rejectPendingPlay('AbortError'));
  await page.waitForTimeout(0);
  await observe('interrupted-play-rejected');
  await page.screenshot({path: info.outputPath('native-pause-after-interruption.png'), fullPage: true});
  await info.attach('native-pause-intent-observation', {body: JSON.stringify({observations,
    hypothesis: 'Pause makes the earlier pending Play obsolete', media: 'simulated Apple API/deferred Play, no decoded frames'}),
    contentType: 'application/json'});
  await expect(page.locator('video')).toHaveJSProperty('paused', true);
  await expect(page.locator('video')).toHaveJSProperty('webkitDisplayingFullscreen', true);
  await expect(page.locator('.player-control-feedback')).toBeHidden();
  await page.evaluate(() => (window as Window & {setPlayPending: (value: boolean) => void}).setPlayPending(false));
  await page.getByRole('button', {name: 'Play', exact: true}).tap();
  await expect(page.locator('video')).toHaveJSProperty('paused', false);
  await expect(page.locator('.player-control-feedback')).toBeHidden();
});
