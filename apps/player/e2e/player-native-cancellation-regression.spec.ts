import {expect, test, type Page, type TestInfo} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {playerSource} from './static-sources';

// Isolation exception and failure inventory are in the native-progress-intent QA
// directory. Keep production native projection/listeners; control decoder events
// and intercepted public HTTP only. No production intent flags are mutated.
const source = process.env.KINOSAIL_REGRESSION_PLAYER_SOURCE
  ? await readFile(process.env.KINOSAIL_REGRESSION_PLAYER_SOURCE, 'utf8') : playerSource;
const sourceHash = createHash('sha256').update(source).digest('hex');
type Decoder = {
  metadata(raw: number, duration: number): void;
  nativeSeek(raw: number): void;
  retiredMetadata(index: number): void;
  retiredSeeked(): void;
  oldDecoder(raw: number): void;
  snapshot(): {raw: number; projected: number; duration: number; ready: number;
    paused: boolean; seeking: boolean; loads: number; setters: number; playCalls: number;
    nativeMetadataCallbacks: number; events: object[]};
};
type TestWindow = Window & {cancellationDecoder: Decoder};

function decoderFixture() {
  const video = document.querySelector('video')!;
  const state = {raw: 0, duration: 100, ready: 0, paused: true, seeking: false,
    loads: 0, setters: 0, playCalls: 0, epoch: 0, attribute: '/media/movie'};
  const events: object[] = [];
  const metadataCallbacks: EventListener[] = [];
  const originalAttribute = video.getAttribute.bind(video);
  const originalListener = video.addEventListener.bind(video);
  const snapshot = () => ({raw: state.raw, projected: video.currentTime, duration: video.duration,
    ready: state.ready, paused: state.paused, seeking: state.seeking, loads: state.loads,
    setters: state.setters, playCalls: state.playCalls,
    nativeMetadataCallbacks: metadataCallbacks.length, events: [...events]});
  const record = (event: string) => events.push({event, raw: state.raw,
    projected: video.currentTime, duration: video.duration, ready: state.ready,
    paused: state.paused, seeking: state.seeking, loads: state.loads, setters: state.setters});
  const emit = (event: string) => video.dispatchEvent(new Event(event));
  Object.defineProperties(navigator, {
    userAgent: {configurable: true, value: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148'},
    vendor: {configurable: true, value: 'Apple Computer, Inc.'},
  });
  // The production wrapper must capture these prototype clocks itself.
  Object.defineProperties(HTMLMediaElement.prototype, {
    currentTime: {configurable: true, get: () => state.raw, set: (value: number) => {
      state.raw = value; state.seeking = true; state.setters++;
      const epoch = state.epoch;
      queueMicrotask(() => {if (epoch === state.epoch) emit('seeking');});
    }},
    duration: {configurable: true, get: () => state.ready ? state.duration : NaN},
  });
  Object.defineProperties(video, {
    readyState: {get: () => state.ready}, networkState: {get: () => state.ready ? 1 : 2},
    paused: {get: () => state.paused}, seeking: {get: () => state.seeking}, error: {value: null},
    webkitEnterFullscreen: {value: () => {}},
    canPlayType: {value: (type: string) => ['application/vnd.apple.mpegurl', 'video/mp4'].includes(type) ? 'probably' : ''},
    currentSrc: {get: () => new URL(state.attribute, location.href).href},
    src: {get: () => new URL(state.attribute, location.href).href,
      set: (value: string) => {state.attribute = value; record('source-selected');}},
    getAttribute: {value: (name: string) => name === 'src' ? state.attribute : originalAttribute(name)},
    buffered: {get: () => ({length: state.ready ? 1 : 0, start: () => 0, end: () => state.duration})},
    seekable: {get: () => ({length: state.ready ? 1 : 0, start: () => 0, end: () => state.duration})},
    load: {value: () => {
      state.loads++; state.epoch++; state.raw = 0; state.ready = 0; state.seeking = false;
      emit('emptied'); emit('loadstart');
    }},
    play: {value: () => {state.playCalls++; state.paused = false; emit('play'); emit('playing'); return Promise.resolve();}},
    pause: {value: () => {state.paused = true; emit('pause');}},
    addEventListener: {value: (name: string, listener: EventListenerOrEventListenerObject, options?: AddEventListenerOptions | boolean) => {
      if (name === 'loadedmetadata' && typeof listener === 'function' &&
          listener.toString().includes('generation !== adaptiveGeneration')) metadataCallbacks.push(listener);
      originalListener(name, listener, options);
    }},
  });
  for (const event of ['emptied', 'loadstart', 'loadedmetadata', 'seeking', 'seeked', 'play', 'playing', 'pause', 'kinosail:seek-intent']) {
    originalListener(event, () => record(event));
  }
  Object.assign(window, {cancellationDecoder: {
    metadata: (raw: number, duration: number) => {state.raw = raw; state.duration = duration; state.ready = 4; emit('loadedmetadata');},
    nativeSeek: (raw: number) => {
      if (raw < 0 || raw > state.duration || !state.ready) throw new Error('invalid native decoder seek');
      state.raw = raw; state.seeking = true; emit('seeking'); state.seeking = false; emit('seeked');
    },
    retiredMetadata: (index: number) => {
      record('retired-metadata-callback'); metadataCallbacks[index].call(video, new Event('loadedmetadata'));
    },
    retiredSeeked: () => {state.seeking = false; emit('seeked');},
    oldDecoder: (raw: number) => {state.raw = raw; state.ready = 0; record('retired-decoder-clock');},
    snapshot,
  }});
}

async function install(page: Page) {
  const writes: Record<string, string>[] = [], errors: string[] = [], adapterRequests: string[] = [];
  const stored = {seconds: 20, watched: false};
  page.on('pageerror', error => errors.push(error.message));
  await page.route('**/static/hls.min.js*', async route => {
    adapterRequests.push('unexpected-adapter'); await route.abort();
  });
  await page.route('**/progress/movie', async route => {
    expect(route.request().method()).toBe('POST');
    const body = new URLSearchParams(route.request().postData() || '');
    writes.push(Object.fromEntries(body));
    stored.seconds = Number(body.get('seconds')); stored.watched = body.get('watched') === 'true';
    await route.fulfill({status: 204});
  });
  await page.route('**/api/v1/items/movie', route => route.fulfill({json: {
    item: {id: 'movie', kind: 'video', title: 'Cancellation fixture movie', progress: {...stored}},
    listed: false, profileId: 'cancellation-test-profile',
  }}));
  await page.route(`**/static/player.js?v=${sourceHash}`, route => route.fulfill({contentType: 'text/javascript', body: source}));
  await page.route('https://127.0.0.1:38127/', route => route.fulfill({body: '<body>Library</body>', contentType: 'text/html'}));
  await page.route('**/watch/movie', route => route.fulfill({contentType: 'text/html', body: `
    <body data-viewer-profile="cancellation-test-profile"><a href="/">Library</a>
    <video data-direct="/media/movie" data-hls="/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8"
      data-playback-policy="direct" data-playback-override="true" data-direct-type="video/mp4"
      data-start="20" data-duration="100" data-progress="/progress/movie" data-playback-session="cancellation-test-session"></video>
    <div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>
    <fieldset data-playback-mode><legend>Playback</legend>
      <label><input type="radio" name="mode" value="direct-only">Direct Only</label>
      <label><input type="radio" name="mode" value="direct-first">Direct First</label>
      <label><input type="radio" name="mode" value="compatible">Compatibility</label></fieldset>
    <script>(${decoderFixture.toString()})()</script><script src="/static/player.js?v=${sourceHash}"></script></body>`}));
  await page.goto('https://127.0.0.1:38127/watch/movie');
  expect(errors).toEqual([]);
  expect(adapterRequests).toEqual([]);
  expect(await page.evaluate('typeof Hls')).toBe('undefined');
  expect(await page.evaluate('Object.hasOwn(player,"currentTime")')).toBe(true);
  await page.evaluate(() => (window as TestWindow).cancellationDecoder.metadata(0, 100));
  await expect(page.locator('video')).toHaveJSProperty('currentTime', 20);
  await expect(page.locator('video')).toHaveJSProperty('seeking', true);
  expect(await page.evaluate('({managed:managedSeek,target:managedSeekTarget,offset:playbackTimelineOffset,played:Boolean(progressPlayedItem)})'))
    .toEqual({managed: true, target: 20, offset: 0, played: false});
  expect(writes).toEqual([]);
  return {writes, errors, adapterRequests, stored};
}

async function checkpoint(page: Page, info: TestInfo, state: Awaited<ReturnType<typeof install>>, stage: string) {
  const decoder = await page.evaluate(() => (window as TestWindow).cancellationDecoder.snapshot());
  const app = await page.evaluate('({managed:managedSeek,target:Number.isFinite(managedSeekTarget)?managedSeekTarget:null,offset:playbackTimelineOffset,held:Number.isFinite(playbackTimelineSeek)?playbackTimelineSeek:null,generation:adaptiveGeneration,adaptive:adaptiveActive,switching:adaptiveSeekSwitch,resume:pendingResume?.seconds??null,played:Boolean(progressPlayedItem)})');
  const publicProgress = await page.evaluate(async () => (await (await fetch('/api/v1/items/movie', {cache: 'no-store'})).json()).item.progress);
  const evidence = {sourceHash, stage, proofClass: 'controlled decoder lifecycle and intercepted public HTTP; no populated-server/device proof',
    decoder, app, publicProgress, writes: [...state.writes], errors: [...state.errors], adapterRequests: [...state.adapterRequests]};
  await info.attach(`${stage}.json`, {body: JSON.stringify(evidence, null, 2), contentType: 'application/json'});
  expect(state.errors).toEqual([]); expect(state.adapterRequests).toEqual([]);
  expect(decoder.paused).toBe(true); expect(decoder.playCalls).toBe(0);
  return evidence;
}

async function replaceWithNative(page: Page) {
  await page.getByLabel('Compatibility', {exact: true}).check();
  await expect.poll(() => page.evaluate('adaptiveActive && !adaptiveStarting && playbackTimelineOffset')).toBe(20);
  await expect(page.locator('video')).toHaveJSProperty('readyState', 0);
  const before = await page.evaluate(() => (window as TestWindow).cancellationDecoder.snapshot());
  expect(before.raw).toBe(0); expect(before.projected).toBe(20);
  await page.evaluate(() => (window as TestWindow).cancellationDecoder.metadata(0, 80));
  const after = await page.evaluate(() => (window as TestWindow).cancellationDecoder.snapshot());
  expect(after.setters).toBe(before.setters);
  expect(after.projected).toBe(20); expect(after.duration).toBe(100);
}

for (const raw of [0, 15]) {
  test(`paused native decoder seek ${raw} after canceled restore checkpoints movie ${20 + raw}`, async ({page}, info) => {
    const state = await install(page);
    await checkpoint(page, info, state, 'pending-direct-restore');
    await replaceWithNative(page);
    const canceled = await checkpoint(page, info, state, 'active-native-metadata');
    expect(canceled.app.held).toBeNull(); expect(canceled.app.resume).toBeNull();
    expect(canceled.decoder.seeking).toBe(false); expect(state.writes).toEqual([]);
    // Native controls touch the decoder clock, never the projected JS setter.
    // Clip zero is movie20 here. Its synthetic event is not trusted UI proof.
    await page.evaluate(value => (window as TestWindow).cancellationDecoder.nativeSeek(value), raw);
    await checkpoint(page, info, state, 'paused-native-seek');
    await expect.poll(() => state.writes.length, {timeout: 1500,
      message: 'first paused native seek after cancellation must save its projected movie position'}).toBe(1);
    const after = await checkpoint(page, info, state, 'accepted-native-checkpoint');
    expect(after.publicProgress).toEqual({seconds: 20 + raw, watched: false});
    expect(state.writes[0]).toMatchObject({seconds: String(20 + raw), session: 'cancellation-test-session', revision: '1', watched: 'false'});
  });
}

test('direct replacement metadata renews the finite managed target before native zero', async ({page}, info) => {
  const state = await install(page);
  await page.getByLabel('Direct First', {exact: true}).check();
  await expect(page.locator('video')).toHaveJSProperty('readyState', 0);
  await checkpoint(page, info, state, 'direct-cancellation');
  await page.evaluate(() => (window as TestWindow).cancellationDecoder.metadata(0, 100));
  const guarded = await checkpoint(page, info, state, 'direct-metadata-renewed-target');
  expect(guarded.app.target).toBe(20); expect(guarded.decoder.projected).toBe(20);
  expect(state.writes).toEqual([]);
  await page.evaluate(() => (window as TestWindow).cancellationDecoder.nativeSeek(0));
  await expect.poll(() => state.writes.length, {timeout: 1500}).toBe(1);
  const after = await checkpoint(page, info, state, 'accepted-direct-native-zero');
  expect(after.publicProgress).toEqual({seconds: 0, watched: false});
});

test('retired native metadata and seeked retain newer replacement projection without progress intent', async ({page}, info) => {
  const state = await install(page);
  await page.getByLabel('Compatibility', {exact: true}).check();
  await expect.poll(() => page.evaluate('adaptiveGeneration')).toBe(1);
  await expect.poll(() => page.evaluate('adaptiveActive && !adaptiveStarting')).toBe(true);
  await page.getByLabel('Direct Only', {exact: true}).check();
  await page.getByLabel('Compatibility', {exact: true}).check();
  await expect.poll(() => page.evaluate('adaptiveGeneration')).toBe(3);
  await expect.poll(() => page.evaluate('adaptiveActive && !adaptiveStarting')).toBe(true);
  await page.evaluate(() => {
    const decoder = (window as TestWindow).cancellationDecoder;
    decoder.oldDecoder(50); decoder.retiredMetadata(0); decoder.retiredSeeked();
  });
  const held = await checkpoint(page, info, state, 'retired-callback-and-seeked');
  expect(held.decoder.nativeMetadataCallbacks).toBe(2);
  expect(held.decoder.raw).toBe(50); expect(held.decoder.projected).toBe(20);
  expect(held.decoder.duration).toBe(100); expect(held.app.held).toBe(20);
  expect(held.app.switching).toBe(true); expect(held.app.played).toBe(false);
  expect(state.writes).toEqual([]);
  await page.evaluate(() => (window as TestWindow).cancellationDecoder.metadata(0, 80));
  const active = await checkpoint(page, info, state, 'active-new-generation-metadata');
  expect(active.app.held).toBeNull(); expect(active.app.switching).toBe(false);
  expect(active.decoder.projected).toBe(20); expect(active.decoder.duration).toBe(100);
  await page.getByRole('link', {name: 'Library'}).click();
  await expect(page.getByText('Library', {exact: true})).toBeVisible();
  const after = await page.evaluate(async () => (await (await fetch('/api/v1/items/movie', {cache: 'no-store'})).json()).item.progress);
  await info.attach('automatic-exit-readback.json', {body: JSON.stringify({sourceHash, writes: state.writes, after}), contentType: 'application/json'});
  expect(state.writes).toEqual([]); expect(after).toEqual({seconds: 20, watched: false});
});
