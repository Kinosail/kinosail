import {expect, type Page} from '@playwright/test';
import {createHash} from 'node:crypto';
import {playerSource} from './static-sources';

// Decoder isolation reaches the public playback-radio replacement while an
// automatic seek is unfinished. Real decoder/native-pointer ownership is
// independently verified; this fixture keeps source choice deterministic.
export const radioSourceHash = createHash('sha256').update(playerSource).digest('hex');
export type RadioWindow = Window & {radioDecoder: {
  metadata(raw: number, duration: number): void;
  nativeSeek(raw: number): void;
  snapshot(): {raw: number; projected: number; duration: number; ready: number;
    paused: boolean; seeking: boolean; setters: number; playCalls: number; events: object[]};
}};

function decoderFixture() {
  const video = document.querySelector('video')!;
  const state = {raw: 0, duration: 100, ready: 0, paused: true, seeking: false,
    loads: 0, setters: 0, playCalls: 0, epoch: 0, attribute: '/media/movie'};
  const events: object[] = [];
  const originalAttribute = video.getAttribute.bind(video);
  const originalListener = video.addEventListener.bind(video);
  const snapshot = () => ({raw: state.raw, projected: video.currentTime, duration: video.duration,
    ready: state.ready, paused: state.paused, seeking: state.seeking, loads: state.loads,
    setters: state.setters, playCalls: state.playCalls,
    events: [...events]});
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
  });
  for (const event of ['emptied', 'loadstart', 'loadedmetadata', 'seeking', 'seeked', 'play', 'playing', 'pause', 'kinosail:seek-intent']) {
    originalListener(event, () => record(event));
  }
  Object.assign(window, {radioDecoder: {
    metadata: (raw: number, duration: number) => {state.raw = raw; state.duration = duration; state.ready = 4; emit('loadedmetadata');},
    nativeSeek: (raw: number) => {
      if (raw < 0 || raw > state.duration || !state.ready) throw new Error('invalid native decoder seek');
      state.raw = raw; state.seeking = true; emit('seeking'); state.seeking = false; emit('seeked');
    },
    snapshot,
  }});
}

export async function installRadioCancellation(page: Page) {
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
    item: {id: 'movie', kind: 'video', title: 'Radio fixture movie', progress: {...stored}},
    listed: false, profileId: 'radio-test-profile',
  }}));
  await page.route(`**/static/player.js?v=${radioSourceHash}`, route => route.fulfill({contentType: 'text/javascript', body: playerSource}));
  await page.route('https://127.0.0.1:38127/', route => route.fulfill({body: '<body>Library</body>', contentType: 'text/html'}));
  await page.route('**/watch/movie', route => route.fulfill({contentType: 'text/html', body: `
    <body data-viewer-profile="radio-test-profile"><a href="/">Library</a>
    <video data-direct="/media/movie" data-hls="/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8"
      data-playback-policy="direct" data-playback-override="true" data-direct-type="video/mp4"
      data-start="20" data-duration="100" data-progress="/progress/movie" data-playback-session="radio-test-session"></video>
    <div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>
    <fieldset data-playback-mode><legend>Playback</legend>
      <label><input type="radio" name="mode" value="direct-only">Direct Only</label>
      <label><input type="radio" name="mode" value="direct-first">Direct First</label>
      <label><input type="radio" name="mode" value="compatible">Compatibility</label></fieldset>
    <script>(${decoderFixture.toString()})()</script><script src="/static/player.js?v=${radioSourceHash}"></script></body>`}));
  await page.goto('https://127.0.0.1:38127/watch/movie');
  expect(errors).toEqual([]);
  expect(adapterRequests).toEqual([]);
  expect(await page.evaluate('typeof Hls')).toBe('undefined');
  expect(await page.evaluate('Object.hasOwn(player,"currentTime")')).toBe(true);
  await page.evaluate(() => (window as RadioWindow).radioDecoder.metadata(0, 100));
  await expect(page.locator('video')).toHaveJSProperty('currentTime', 20);
  await expect(page.locator('video')).toHaveJSProperty('seeking', true);
  expect(writes).toEqual([]);
  return {writes, errors, adapterRequests, stored};
}

