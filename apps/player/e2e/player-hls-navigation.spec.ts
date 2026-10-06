import {expect, test} from '@playwright/test';
import {hlsNavigationPeer} from './player-hls-navigation-fixture';

async function movingVideo(page: import('@playwright/test').Page, peer: Awaited<ReturnType<typeof hlsNavigationPeer>>) {
  await page.goto(`${peer.origin}/watch/movie`);
  const video = page.locator('video');
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.readyState)).toBeGreaterThanOrEqual(2);
  await video.evaluate((media: HTMLVideoElement) => media.play());
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentTime)).toBeGreaterThan(0.2);
  await expect.poll(() => peer.snapshot().waitingSegments).toBe(1);
  return video;
}

for (const destination of ['Library', 'Mark watched']) {
  test(`real HLS retry cannot outlive acknowledged ${destination} departure`, {tag: '@smoke'}, async ({page}, info) => {
    const peer = await hlsNavigationPeer();
    const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.name));
    try {
      await movingVideo(page, peer);
      const before = peer.snapshot();
      const click = destination === 'Library' ? page.getByRole('link', {name: destination, exact: true})
        : page.getByRole('button', {name: destination, exact: true});
      await click.click({noWaitAfter: true});
      await expect.poll(() => peer.snapshot().departing).toBe(true);
      peer.releaseRetry();
      // The real library retries a 503 after its own one-second delay. Destination
      // headers remain held so pagehide cannot hide a pre-commit lifecycle leak.
      await page.waitForTimeout(1500);
      const pending = peer.snapshot();
      peer.releaseDestination();
      await expect(page.getByRole('heading', {name: 'Destination'})).toBeVisible();
      await info.attach('real-hls-navigation', {body: JSON.stringify({destination, facts: peer.facts, before, pending,
        errors, proofClass: 'real-H264-Hls.js-HTTP-with-held-destination-no-Go-storage'}), contentType: 'application/json'});
      expect(errors).toEqual([]);
      expect(pending.requests.filter(request => request.afterDestination && request.kind.endsWith('.ts'))).toEqual([]);
    } finally {await peer.close();}
  });
}

test('pending checkpoint keeps the real HLS owner until acknowledgement', {tag: '@smoke'}, async ({page}, info) => {
  const peer = await hlsNavigationPeer();
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.name));
  try {
    await movingVideo(page, peer);
    peer.holdCheckpoint();
    await page.getByRole('link', {name: 'Library', exact: true}).click({noWaitAfter: true});
    await expect.poll(() => peer.snapshot().waitingProgress).toBeGreaterThan(0);
    expect(peer.snapshot().departing).toBe(false);
    const before = peer.snapshot().requests.filter(row => row.kind.endsWith('.ts')).length;
    peer.releaseRetry();
    await expect.poll(() => peer.snapshot().requests.filter(row => row.kind.endsWith('.ts')).length).toBeGreaterThan(before);
    expect(peer.snapshot().departing).toBe(false);
    peer.releaseCheckpoint();
    await expect.poll(() => peer.snapshot().departing).toBe(true);
    peer.releaseDestination();
    await expect(page.getByRole('heading', {name: 'Destination'})).toBeVisible();
    await info.attach('pending-HLS-owner', {body: JSON.stringify({facts: peer.facts, snapshot: peer.snapshot(), errors}), contentType: 'application/json'});
    expect(errors).toEqual([]);
  } finally {await peer.close();}
});

test('cancelled watched submission retains the real HLS owner', {tag: '@smoke'}, async ({page}, info) => {
  const peer = await hlsNavigationPeer();
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.name));
  try {
    await movingVideo(page, peer);
    await page.evaluate(() => {
      // This is a later listener, after shipped listeners, so cancellation must
      // be checked after the complete submit dispatch, including its replay.
      document.addEventListener('submit', event => event.preventDefault());
    });
    await page.getByRole('button', {name: 'Mark watched', exact: true}).click();
    const before = peer.snapshot().requests.filter(row => row.kind.endsWith('.ts')).length;
    peer.releaseRetry();
    await expect.poll(() => peer.snapshot().requests.filter(row => row.kind.endsWith('.ts')).length).toBeGreaterThan(before);
    expect(peer.snapshot().departing).toBe(false);
    await expect(page).toHaveURL(`${peer.origin}/watch/movie`);
    const video = page.locator('video');
    const time = await video.evaluate((media: HTMLVideoElement) => media.currentTime);
    await video.evaluate((media: HTMLVideoElement) => media.play());
    await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentTime)).toBeGreaterThan(time + 0.2);
    await info.attach('cancelled-HLS-owner', {body: JSON.stringify({facts: peer.facts, snapshot: peer.snapshot(), errors}), contentType: 'application/json'});
    expect(errors).toEqual([]);
  } finally {await peer.close();}
});

test('declared PiP ownership retains HLS through an accepted departure', {tag: '@smoke'}, async ({page}, info) => {
  const peer = await hlsNavigationPeer();
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.name));
  try {
    const video = await movingVideo(page, peer);
    await video.evaluate(media => Object.defineProperty(media, 'webkitPresentationMode', {
      configurable: true, value: 'picture-in-picture',
    }));
    await page.getByRole('link', {name: 'Library', exact: true}).click({noWaitAfter: true});
    await expect.poll(() => peer.snapshot().departing).toBe(true);
    // A real 204 completes the accepted request without replacing the document.
    // It exposes pipeline ownership without claiming a physical PiP API session.
    peer.discardDestination();
    const before = peer.snapshot().requests.filter(row => row.kind.endsWith('.ts')).length;
    peer.releaseRetry();
    await expect.poll(() => peer.snapshot().requests.filter(row => row.kind.endsWith('.ts')).length).toBeGreaterThan(before);
    await expect(page).toHaveURL(`${peer.origin}/watch/movie`);
    expect(await video.evaluate((media: HTMLVideoElement) => media.paused)).toBe(false);
    await info.attach('PiP-HLS-owner', {body: JSON.stringify({facts: peer.facts, snapshot: peer.snapshot(), errors,
      proofClass: 'real-HLS-HTTP-declared-PiP-capability-no-physical-API'}), contentType: 'application/json'});
    expect(errors).toEqual([]);
  } finally {await peer.close();}
});
