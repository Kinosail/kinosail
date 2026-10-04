import {writeFile} from 'node:fs/promises';
import {expect, test} from '@playwright/test';
import {installPlayerExperienceFixture} from './player-experience-fixture';

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
