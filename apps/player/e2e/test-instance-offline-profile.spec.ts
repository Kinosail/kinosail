import {expect} from '@playwright/test';
import {test} from './test-instance-network';
import {configureTestInstance, firstPlayable, login} from './test-instance-helpers';

configureTestInstance();
test.describe.configure({retries: 0});

test('opening the current Viewer Profile in another tab keeps saved video playing, while sign-out stops it', {tag: '@smoke'}, async ({page, context}, testInfo) => {
  test.setTimeout(120_000);
  await login(page);
  const watch = await firstPlayable(page);
  const title = await page.locator(`a.card[href="${watch}"]`).first().getByRole('heading').innerText();
  await page.goto(watch);
  await page.getByText('Playback & downloads', {exact: true}).click();
  await page.getByRole('button', {name: 'Prepare 720p offline', exact: true}).click();
  const job = page.locator('[data-download-job]').filter({has: page.getByRole('heading', {name: title, exact: true})});
  await expect(job.getByText('720p · Ready to download', {exact: true})).toBeVisible({timeout: 30_000});
  await job.getByRole('button', {name: 'Download to this device', exact: true}).click();
  await expect(job.locator('[data-download-device-status]')).toHaveText('Saved and verified. Play to check compatibility.', {timeout: 30_000});
  const id = await job.getAttribute('data-download-job');
  const other = await context.newPage();
  try {
    await other.goto('/');
    await expect(other.locator('.card').first()).toBeVisible();
    await page.goto(`/offline?job=${id}`);
    await page.bringToFront();
    const video = page.locator('video');
    await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.readyState >= 2 && !media.seeking)).toBeTruthy();
    const restored = await video.evaluate((media: HTMLVideoElement) => media.currentTime);
    await video.evaluate((media: HTMLVideoElement) => media.play());
    await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentTime)).toBeGreaterThan(restored + 0.2);
    const position = await video.evaluate((media: HTMLVideoElement) => media.currentTime);
    await other.reload();
    await expect(other.locator('.card').first()).toBeVisible();
    await testInfo.attach('cross-tab-media-state', {body: JSON.stringify(await video.evaluate((media: HTMLVideoElement) => ({paused: media.paused, position: media.currentTime, duration: media.duration, ended: media.ended}))), contentType: 'application/json'});
    await expect.poll(() => video.evaluate((media: HTMLVideoElement, before: number) => !media.paused && media.currentTime > before + 0.2, position)).toBeTruthy();
    await page.screenshot({path: testInfo.outputPath('same-profile-keeps-playing.png')});
    await other.bringToFront();
    await other.locator('.header-compact-menu > summary').click();
    await other.getByRole('button', {name: 'Sign out', exact: true}).filter({visible: true}).click();
    await expect(video).toHaveCount(0);
    await expect(page.getByText('Open Kinosail online and choose a Viewer Profile first.', {exact: true})).toBeVisible();
    await page.screenshot({path: testInfo.outputPath('sign-out-clears-offline-media.png')});
  } finally { await other.close(); }
});
