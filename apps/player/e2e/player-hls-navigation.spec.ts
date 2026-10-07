import {expect, test} from '@playwright/test';
import {hlsNavigationPeer} from './player-hls-navigation-fixture';

async function captureDepartures(page: import('@playwright/test').Page) {
  await page.evaluate(() => {
    const facts = {capture: 0, bubble: 0, player: 0, bubbling: 0};
    Object.assign(window, {HLSDepartureEvents: facts});
    window.addEventListener('kinosail:navigation', event => {
      facts.capture++;
      if (event.target === document.querySelector('video')) facts.player++;
      if (event.bubbles) facts.bubbling++;
    }, {capture: true});
    window.addEventListener('kinosail:navigation', () => {facts.bubble++;});
  });
}

const departureFacts = (page: import('@playwright/test').Page) => page.evaluate(() =>
  (window as unknown as {HLSDepartureEvents: {capture: number, bubble: number, player: number, bubbling: number}}).HLSDepartureEvents);

async function movingVideo(page: import('@playwright/test').Page, peer: Awaited<ReturnType<typeof hlsNavigationPeer>>) {
  await page.goto(`${peer.origin}/watch/movie`);
  const video = page.locator('video');
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.readyState)).toBeGreaterThanOrEqual(2);
  await video.evaluate((media: HTMLVideoElement) => media.play());
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentTime)).toBeGreaterThan(0.2);
  await expect.poll(() => peer.snapshot().waitingSegments).toBe(1);
  return video;
}

for (const destination of ['Library', 'Back to Movies', 'Mark watched']) {
  test(`real HLS retry cannot outlive acknowledged ${destination} departure`, {tag: '@smoke'}, async ({page}, info) => {
    const peer = await hlsNavigationPeer(destination === 'Back to Movies');
    const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.name));
    try {
      await movingVideo(page, peer);
      await captureDepartures(page);
      const before = peer.snapshot();
      const click = destination === 'Mark watched' ? page.getByRole('button', {name: destination, exact: true})
        : page.getByRole('link', {name: destination, exact: true});
      await click.click({noWaitAfter: true});
      await expect.poll(() => peer.snapshot().departing).toBe(true);
      const departure = await departureFacts(page);
      expect(departure).toEqual({capture: 1, bubble: 0, player: 1, bubbling: 0});
      peer.releaseRetry();
      // The real library retries a 503 after its own one-second delay. Destination
      // headers remain held so pagehide cannot hide a pre-commit lifecycle leak.
      await page.waitForTimeout(1500);
      const pending = peer.snapshot();
      peer.releaseDestination();
      await expect(page.getByRole('heading', {name: 'Destination'})).toBeVisible();
      await info.attach('real-hls-navigation', {body: JSON.stringify({destination, facts: peer.facts, before, pending,
        errors, departure, proofClass: 'real-H264-Hls.js-HTTP-with-held-destination-no-Go-storage'}), contentType: 'application/json'});
      expect(errors).toEqual([]);
      expect(pending.requests.filter(request => request.afterDestination && request.kind.endsWith('.ts'))).toEqual([]);
    } finally {await peer.close();}
  });
}

for (const destination of ['Library', 'Mark watched']) {
  test(`fast ${destination} document commit preserves a clean real fMP4 HLS departure`, {tag: '@smoke'}, async ({page}, info) => {
    const peer = await hlsNavigationPeer(false, true);
    const started = Date.now(), phases: Array<{event: string; elapsedMS: number}> = [];
    const errors: Array<{kind: string; xhrSend: boolean; accessControl: boolean}> = [];
    const observeError = (kind: string, message: string, stack = '') => {
      if (errors.length < 16) errors.push({kind, xhrSend: /openAndSendXhr|loadInternal/.test(stack),
        accessControl: /access[ -]control|CORS/i.test(message)});
    };
    page.on('pageerror', error => observeError('pageerror', error.message, error.stack));
    page.on('console', message => {
      if (message.type() === 'error') observeError('console', message.text());
      if (phases.length < 16 && ['kinosail-fixture:navigation', 'kinosail-fixture:pagehide'].includes(message.text())) {
        phases.push({event: message.text().slice('kinosail-fixture:'.length), elapsedMS: Date.now() - started});
      }
    });
    try {
      await page.addInitScript(() => {
        window.addEventListener('kinosail:navigation', event => {
          if (event.target instanceof HTMLVideoElement) console.debug('kinosail-fixture:navigation');
        }, {capture: true});
        window.addEventListener('pagehide', () => console.debug('kinosail-fixture:pagehide'), {capture: true});
      });
      await movingVideo(page, peer);
      const before = peer.snapshot();
      const click = destination === 'Mark watched' ? page.getByRole('button', {name: destination, exact: true})
        : page.getByRole('link', {name: destination, exact: true});
      await click.click({noWaitAfter: true});
      await expect(page).toHaveURL(`${peer.origin}${destination === 'Mark watched' ? '/watch/after-watched' : '/'}`);
      if (destination === 'Mark watched') {
        await expect(page.locator('video[data-fixture-watch="after-watched"]')).toBeVisible();
      } else await expect(page.getByRole('heading', {name: 'Destination'})).toBeVisible();
      await expect.poll(() => peer.snapshot().closedHeldSegments).toBeGreaterThan(0);
      const after = peer.snapshot();
      await info.attach('fast-fMP4-HLS-departure', {body: JSON.stringify({destination, facts: peer.facts, before, after,
        phases, errors, boundary: 'Real pinned Hls.js/HTTP/fMP4 with fast document replacement; console phases unverified, driver receipt time; no Go storage/TLS/offset recipe/native fullscreen proof'}), contentType: 'application/json'});
      expect(phases.map(value => value.event)).toEqual(['navigation', 'pagehide']);
      expect(errors).toEqual([]);
    } finally {await peer.close();}
  });
}

for (const contextual of [false, true]) test(`pending checkpoint keeps the real HLS owner until ${contextual ? 'contextual Back ' : ''}acknowledgement`, {tag: '@smoke'}, async ({page}, info) => {
  const peer = await hlsNavigationPeer(contextual);
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.name));
  try {
    await movingVideo(page, peer);
    await captureDepartures(page);
    peer.holdCheckpoint();
    await page.getByRole('link', {name: contextual ? 'Back to Movies' : 'Library', exact: true}).click({noWaitAfter: true});
    await expect.poll(() => peer.snapshot().waitingProgress).toBeGreaterThan(0);
    expect(peer.snapshot().departing).toBe(false);
    expect(await departureFacts(page)).toEqual({capture: 0, bubble: 0, player: 0, bubbling: 0});
    const before = peer.snapshot().requests.filter(row => row.kind.endsWith('.ts')).length;
    peer.releaseRetry();
    await expect.poll(() => peer.snapshot().requests.filter(row => row.kind.endsWith('.ts')).length).toBeGreaterThan(before);
    expect(peer.snapshot().departing).toBe(false);
    peer.releaseCheckpoint();
    await expect.poll(() => peer.snapshot().departing).toBe(true);
    expect(await departureFacts(page)).toEqual({capture: 1, bubble: 0, player: 1, bubbling: 0});
    peer.releaseDestination();
    await expect(page.getByRole('heading', {name: 'Destination'})).toBeVisible();
    await expect(page).toHaveURL(`${peer.origin}${peer.facts.returnPath}`);
    await info.attach('pending-HLS-owner', {body: JSON.stringify({facts: peer.facts, snapshot: peer.snapshot(), errors}), contentType: 'application/json'});
    expect(errors).toEqual([]);
  } finally {await peer.close();}
});

for (const destination of ['ordinary query', 'fragment', 'other origin']) test(`browse checkpoint wait excludes ${destination} destinations`, {tag: '@smoke'}, async ({page}, info) => {
  const contextual = destination !== 'ordinary query';
  const peer = await hlsNavigationPeer(contextual);
  try {
    await movingVideo(page, peer);
    peer.holdCheckpoint();
    const link = page.getByRole('link', {name: contextual ? 'Back to Movies' : 'Library', exact: true});
    await link.evaluate((anchor: HTMLAnchorElement, destination) => {
      if (destination === 'ordinary query') anchor.search = '?view=movies';
      else if (destination === 'fragment') anchor.hash = 'details';
      else anchor.hostname = 'localhost';
    }, destination);
    await link.click({noWaitAfter: true});
    // Held checkpoints cannot permit departure if this link is wrongly classified as Back.
    await expect.poll(() => peer.snapshot().departing).toBe(true);
    await info.attach('excluded-browse-checkpoint', {body: JSON.stringify({destination, facts: peer.facts,
      snapshot: peer.snapshot(), boundary: 'real HLS and HTTP; no Go storage'}), contentType: 'application/json'});
  } finally {await peer.close();}
});

test('unplayed contextual Back retires HLS before destination commits', {tag: '@smoke'}, async ({page}, info) => {
  const peer = await hlsNavigationPeer(true);
  try {
    await page.goto(`${peer.origin}/watch/movie`);
    await expect.poll(() => page.locator('video').evaluate((media: HTMLVideoElement) => media.readyState)).toBeGreaterThanOrEqual(2);
    await expect.poll(() => peer.snapshot().waitingSegments).toBe(1);
    const before = peer.snapshot();
    await page.getByRole('link', {name: 'Back to Movies', exact: true}).click({noWaitAfter: true});
    await expect.poll(() => peer.snapshot().departing).toBe(true);
    peer.releaseRetry();
    await page.waitForTimeout(1500);
    const pending = peer.snapshot();
    peer.releaseDestination();
    await expect(page.getByRole('heading', {name: 'Destination'})).toBeVisible();
    await expect(page).toHaveURL(`${peer.origin}${peer.facts.returnPath}`);
    expect(pending.requests.filter(request => request.afterDestination && request.kind.endsWith('.ts'))).toEqual([]);
    expect(pending.requests.filter(request => request.kind === 'progress').length).toBe(before.requests.filter(request => request.kind === 'progress').length);
    await info.attach('unplayed-contextual-HLS-exit', {body: JSON.stringify({facts: peer.facts, before, pending,
      boundary: 'real HLS without playback intent; no Go storage'}), contentType: 'application/json'});
  } finally {await peer.close();}
});

test('cancelled watched submission retains the real HLS owner', {tag: '@smoke'}, async ({page}, info) => {
  const peer = await hlsNavigationPeer();
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.name));
  try {
    await movingVideo(page, peer);
    await captureDepartures(page);
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
    expect(await departureFacts(page)).toEqual({capture: 0, bubble: 0, player: 0, bubbling: 0});
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
