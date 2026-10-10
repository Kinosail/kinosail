import type {JSONValue} from "../../../scripts/testing/json-value";
import {devices, expect, test} from '@playwright/test';
import {createHash} from 'node:crypto';
import {readFile, writeFile} from 'node:fs/promises';
import {configureTestInstance, login} from './test-instance-helpers';

// Gap: positive paused restore was reset by Mark watched before decoded Play.
// This real-process case keeps the positive state through the first Play.
configureTestInstance();
// Match the live MobileSafari initial-Watch policy while decoding on macOS
// WebKit. The public autoplay preference controls the next episode only.
test.use({...devices['iPhone 13']});
test('positive Matroska reentry decodes the saved scene through native HLS', async ({page}, info) => {
  test.skip(process.env.KINOSAIL_POSITIVE_REENTRY_E2E !== '1', 'Owned synthetic loopback runner only');
  test.setTimeout(90_000);
  const base = new URL(process.env.KINOSAIL_E2E_URL!);
  expect(base.protocol).toBe('https:');
  expect(base.hostname).toBe('localhost');
  expect(info.project.name).toBe('webkit');
  const sourceReceipt = JSON.parse(await readFile(process.env.KINOSAIL_POSITIVE_SOURCE_RECEIPT!, 'utf8'));
  expect(sourceReceipt.sourceRevision).toMatch(/^[a-f0-9]{40}$/);
  expect(sourceReceipt.binarySHA256).toMatch(/^[a-f0-9]{64}$/);
  const observations: object[] = [];
  const record = async (value: object) => {
    observations.push({utc: new Date().toISOString(), ...value});
    await writeFile(info.outputPath('positive-reentry.json'), JSON.stringify({sourceReceipt,
      boundaries: 'Real Go/FFV1/native macOS WebKit HLS in iPhone context; next-episode autoplay disabled through public settings; no mocked media responses or decoder clocks; no actual MobileSafari or physical-device proof',
      observations}, null, 2));
  };
  await page.addInitScript(() => localStorage.setItem('kinosail.playback-policy-v2', 'direct-first'));
  await record({phase: 'browser-admission-start'});
  await login(page);
  await record({phase: 'browser-login-complete'});
  const csrf = async () => ({Origin: base.origin,
    'X-Kinosail-CSRF': await page.locator('meta[name="kinosail-csrf"]').getAttribute('content') || ''});
  const settings = await page.request.put('/api/v1/settings/playback', {headers: await csrf(),
    data: {mode: 'automatic', autoplay: false, subtitles: 'off', autoSkip: []}});
  expect(settings.ok()).toBe(true);
  const library = await (await page.request.get('/api/v1/library')).json();
  const item = library.items.find((value: {title: string}) => value.title === 'Positive Reentry');
  expect(item).toBeTruthy();
  const path = `/api/v1/items/${item.id}`;
  const publicPosition = async () => {
    const response = await page.request.get(path + '/watch-progress');
    expect(response.ok()).toBe(true);
    return response.json();
  };
  expect((await page.request.put(path + '/progress', {headers: await csrf(),
    data: {seconds: 6, watched: false}})).ok()).toBe(true);
  await expect.poll(() => publicPosition().then(value => value.seconds)).toBe(6);
  await record({phase: 'positive-seed', public: await publicPosition()});
  const watch = `/watch/${item.id}`;
  const card = () => page.locator(`a.card[href="${watch}"]`).first();
  const snapshot = () => page.evaluate(() => {
    const video = document.querySelector('video')!;
    const offset = /-o(\d+)(?:\/|$)/.exec(new URL(video.currentSrc || video.src, location.href).pathname);
    const rawTime = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'currentTime')!.get!.call(video);
    const decoderDuration = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'duration')!.get!.call(video);
    return {renderedStart: Number(video.dataset.start), fullDuration: Number(video.dataset.duration),
      rawTime, reportedPosition: video.currentTime, decoderDuration: Number.isFinite(decoderDuration) ? decoderDuration : null,
      offsetSeconds: offset ? Number(offset[1]) / 1000 : 0,
      projectedFromSource: rawTime + (offset ? Number(offset[1]) / 1000 : 0),
      readyState: video.readyState, paused: video.paused, ended: video.ended, errorCode: video.error?.code || 0,
      nativeHLS: video.canPlayType('application/vnd.apple.mpegurl'),
      phonePolicy: /iPhone/.test(navigator.userAgent), touchContext: navigator.maxTouchPoints > 0,
      hasInitialAutoplay: video.hasAttribute('autoplay') || video.hasAttribute('data-autoplay'),
      nativeFullscreenCapability: typeof (video as HTMLVideoElement & {webkitEnterFullscreen?: () => void}).webkitEnterFullscreen === 'function',
      sourceIsHLS: new URL(video.currentSrc || video.src, location.href).pathname.startsWith('/hls/'),
      directType: video.dataset.directType, compatibilityMode: video.dataset.compatibilityMode};
  });
  await page.goto('/');
  await card().click();
  await expect.poll(async () => (await snapshot()).readyState, {timeout: 30_000}).toBeGreaterThanOrEqual(2);
  await record({phase: 'unplayed-metadata', ...await snapshot()});
  expect((await snapshot()).phonePolicy).toBe(true);
  expect((await snapshot()).nativeFullscreenCapability).toBe(true);
  expect((await snapshot()).hasInitialAutoplay).toBe(false);
  expect((await snapshot()).paused).toBe(true);
  await page.getByRole('link', {name: 'Library', exact: true}).click();
  await expect(page).toHaveURL(base.origin + '/');
  try {
    await expect.poll(() => publicPosition().then(value => value.seconds)).toBe(6);
  } finally {
    try { await record({phase: 'unplayed-exit-final-read', public: await publicPosition()}); }
    catch { await record({phase: 'unplayed-exit-read-unavailable'}).catch(() => {}); }
  }
  await record({phase: 'unplayed-exit-preserved', public: await publicPosition()});
  await card().click();
  await page.reload();
  await expect.poll(async () => (await snapshot()).readyState, {timeout: 30_000}).toBeGreaterThanOrEqual(2);
  const before = await snapshot();
  await record({phase: 'reentry-before-Play', ...before});
  expect(before.renderedStart).toBe(6);
  expect(before.directType).toBe('video/x-matroska');
  expect(before.compatibilityMode).toBe('transcode');
  expect(before.nativeHLS).not.toBe('');
  expect(before.sourceIsHLS).toBe(true);
  expect(before.paused).toBe(true);
  const assetURL = await page.locator('script[src^="/static/player.js?v="]').getAttribute('src');
  expect(assetURL).toMatch(/\?v=[a-f0-9]{64}$/);
  const asset = await page.request.get(assetURL!);
  const assetSHA256 = createHash('sha256').update(await asset.body()).digest('hex');
  expect(assetSHA256).toBe(new URL(assetURL!, base).searchParams.get('v'));
  await record({phase: 'referenced-served-asset-identity', assetSHA256,
    limit: 'Fresh public asset fetch verifies the referenced version; it does not inspect executed resource bytes.'});
  // Observe existing public telemetry without changing requests or retaining
  // raw bodies, URLs, sessions, headers, or arbitrary diagnostic details.
  const launchTelemetry: object[] = [];
  page.on('request', request => {
    try {
    const endpoint = new URL(request.url());
    if (launchTelemetry.length >= 24 || request.method() !== 'POST' || endpoint.origin !== base.origin || endpoint.pathname !== path + '/playback-events') return;
    const body = request.postData();
    if (!body || body.length > 4096) return;
      const value = JSON.parse(body);
      if (!['error', 'play-request', 'play-rejected'].includes(value.event)) return;
      const namedFailure = /^(?:fullscreen:(?:NotAllowedError|InvalidStateError|NotSupportedError|TypeError|Error):(?:paused-for-retry|playback-retained)|apple-play:(?:NotAllowedError|InvalidStateError|NotSupportedError|AbortError|TypeError|Error))$/;
      const bounded = (number: JSONValue | undefined, maximum: number) => typeof number === 'number' && Number.isFinite(number) && number >= 0 && number <= maximum ? number : null;
      launchTelemetry.push({event: value.event,
        detail: typeof value.detail === 'string' && namedFailure.test(value.detail) ? value.detail : 'unreported',
        elapsedMS: bounded(value.elapsedMs, 31622400000), positionMS: bounded(value.positionMs, 31622400000),
        readyState: bounded(value.readyState, 4), errorCode: bounded(value.errorCode, 4),
        paused: typeof value.paused === 'boolean' ? value.paused : null,
        method: ['direct', 'native-hls', 'transcode', 'remux'].includes(value.method) ? value.method : 'unreported'});
    } catch { /* Optional fixed projection; never retain the rejected body. */ }
  });
  // Read real presented frames. No setter replaces currentTime, play, or metadata.
  await page.evaluate(() => {
    const video = document.querySelector('video')!;
    video.muted = true;
    const state = {frames: [] as {mediaTime: number, rawTime: number, rgb: number[], png: string}[], callbacks: 0,
      events: [] as {event: string, elapsedMS: number, paused: boolean, rawTime: number}[]};
    Object.assign(window, {positiveReentryFrames: state});
    const started = performance.now();
    for (const event of ['play', 'playing', 'pause', 'waiting', 'stalled', 'ended', 'seeking', 'seeked', 'error', 'webkitbeginfullscreen', 'webkitendfullscreen']) {
      video.addEventListener(event, () => {
        if (state.events.length < 30) state.events.push({event, elapsedMS: Math.round(performance.now() - started), paused: video.paused,
          rawTime: Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'currentTime')!.get!.call(video)});
      });
    }
    const observe = (_now: number, metadata: VideoFrameCallbackMetadata) => {
      state.callbacks++;
      if (!video.paused && state.frames.length < 2) {
        const canvas = document.createElement('canvas'); canvas.width = 32; canvas.height = 18;
        const context = canvas.getContext('2d')!; context.drawImage(video, 0, 0, 32, 18);
        const pixels = context.getImageData(0, 0, 32, 18).data;
        const rgb = [0, 0, 0];
        for (let i = 0; i < pixels.length; i += 4) for (let c = 0; c < 3; c++) rgb[c] += pixels[i + c] / (32 * 18);
        const rawTime = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'currentTime')!.get!.call(video);
        state.frames.push({mediaTime: metadata.mediaTime, rawTime, rgb, png: canvas.toDataURL('image/png')});
      }
      if (state.frames.length < 2) video.requestVideoFrameCallback(observe);
    };
    video.requestVideoFrameCallback(observe);
  });
  const playPosted = Date.now();
  await page.getByRole('button', {name: 'Play', exact: true}).first().click();
  await record({phase: 'Play-click-complete', ...await snapshot()});
  try {
    await expect.poll(() => page.evaluate(() => (window as Window & {positiveReentryFrames: {frames: object[]}}).positiveReentryFrames.frames.length), {timeout: 30_000}).toBe(2);
    const frames = await page.evaluate(() => (window as Window & {positiveReentryFrames: {frames: {mediaTime: number, rawTime: number, rgb: number[], png: string}[]}}).positiveReentryFrames.frames);
    await writeFile(info.outputPath('first-decoded-frame.png'), Buffer.from(frames[0].png.split(',')[1], 'base64'));
    await record({phase: 'first-real-frames', elapsedObservationMS: Date.now() - playPosted,
      frames: frames.map(({png: _png, ...value}) => value), ...await snapshot()});
    // Require real clock movement so an unchanged seed6 cannot masquerade as
    // a checkpoint written by this Play. This reads the native getter only.
    await expect.poll(async () => (await snapshot()).rawTime).toBeGreaterThan(frames[1].rawTime + 0.5);
    await page.evaluate(() => document.querySelector('video')!.pause());
    const terminal = await snapshot();
    await record({phase: 'paused-exit-target', ...terminal});
    await page.screenshot({path: info.outputPath('positive-reentry-paused.png')});
    await page.getByRole('link', {name: 'Library', exact: true}).click();
    await expect(page).toHaveURL(base.origin + '/');
    await record({phase: 'initial-exit-read', public: await publicPosition()});
    await expect.poll(() => publicPosition().then(value =>
      Math.abs(value.seconds - terminal.reportedPosition) < 0.2 && Math.abs(value.seconds - 6) > 0.25)).toBe(true);
    const accepted = await publicPosition();
    await record({phase: 'accepted-exit-read', public: accepted});
    expect(accepted.seconds).toBeGreaterThanOrEqual(6.25);
    expect(accepted.seconds).toBeLessThan(8);
    expect(before.offsetSeconds).toBe(6);
    expect(frames[1].mediaTime).toBeGreaterThan(frames[0].mediaTime);
    // Original source is blue before4s and yellow after4s. A projected clock
    // cannot turn an actual opening blue frame into a resumed yellow frame.
    expect(frames[0].rgb[0]).toBeGreaterThan(150);
    expect(frames[0].rgb[1]).toBeGreaterThan(150);
    expect(frames[0].rgb[2]).toBeLessThan(80);
  } finally {
    try {
    const pageIsWatch = new URL(page.url()).pathname.startsWith('/watch/');
    const nativeState = pageIsWatch ? await page.evaluate(() => {
      const video = document.querySelector('video') as HTMLVideoElement & {webkitDisplayingFullscreen?: boolean, webkitPresentationMode?: string, webkitDecodedFrameCount?: number};
      const state = (window as Window & {positiveReentryFrames?: {callbacks: number, frames: object[], events: object[]}}).positiveReentryFrames;
      return {fullscreen: video.webkitDisplayingFullscreen === true,
        presentation: ['inline', 'fullscreen', 'picture-in-picture'].includes(video.webkitPresentationMode || '') ? video.webkitPresentationMode : 'unreported',
        decodedVideoFrames: video.getVideoPlaybackQuality?.().totalVideoFrames ?? null,
        webkitDecodedFrameCount: Number.isFinite(video.webkitDecodedFrameCount) ? video.webkitDecodedFrameCount : null,
        callbackCount: state?.callbacks ?? null, capturedFrames: state?.frames.length ?? null, events: state?.events ?? []};
    }) : null;
    await record({phase: 'final-observation', pageIsWatch, ...(pageIsWatch ? await snapshot() : {}), nativeState, launchTelemetry, public: await publicPosition()});
    } catch {
      // Optional diagnostics must preserve the original playback failure.
      await record({phase: 'final-diagnostics-unavailable'}).catch(() => {});
    }
  }
});
