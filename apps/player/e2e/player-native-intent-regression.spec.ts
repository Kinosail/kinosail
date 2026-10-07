import {expect, test, type Page, type TestInfo} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {playerSource} from './static-sources';

// Isolation exception: live e30 reproduced saved-progress loss, but real-media
// E2E cannot deterministically queue WebKit playing or interrupt a managed seek.
// These controlled clocks retain production listeners and public HTTP writes.
// KINOSAIL_REGRESSION_PLAYER_SOURCE selects an immutable source for RED replay.
const source = process.env.KINOSAIL_REGRESSION_PLAYER_SOURCE
  ? await readFile(process.env.KINOSAIL_REGRESSION_PLAYER_SOURCE, 'utf8') : playerSource;
const sourceHash = createHash('sha256').update(source).digest('hex');
type Control = {
  configure(time: number, ready: number, end: number): void;
  quantizeTime(quantum: number): void;
  replaceSource(time: number): void;
  beginNativeSeek(time: number): void;
  endNativeSeek(): void;
  snapshot(): object;
};
type TestWindow = Window & {intentDecoder: Control};

function decoderFixture(preparation: boolean) {
  const video = document.querySelector('video')!;
  let time = preparation ? 20 : 0, ready = preparation ? 0 : 4;
  let end = preparation ? 20.1 : 60, paused = true, seeking = false;
  let loaded = !preparation, playCalls = 0, quantum = 0;
  let source = 'https://127.0.0.1:38127/media/movie';
  const events: object[] = [];
  Object.defineProperty(navigator, 'userAgent', {configurable: true,
    value: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148'});
  Object.defineProperty(video, 'webkitEnterFullscreen', {configurable: true,
    value: preparation ? undefined : () => {}});
  Object.defineProperties(video, {
    currentTime: {configurable: true, get: () => time, set: (value: number) => {
      time = quantum ? Math.round(value / quantum) * quantum : value; seeking = true;
      queueMicrotask(() => video.dispatchEvent(new Event('seeking')));
    }},
    duration: {configurable: true, value: 100},
    readyState: {get: () => ready}, networkState: {value: 1},
    paused: {get: () => paused}, seeking: {get: () => seeking}, error: {value: null},
    currentSrc: {get: () => loaded ? source : ''},
    seekable: {get: () => ({length: loaded ? 1 : 0, start: () => 0, end: () => 100})},
    buffered: {get: () => ({length: 1, start: () => 0, end: () => end})},
    load: {value: () => {}},
    play: {value: () => {
      playCalls++; paused = false; video.dispatchEvent(new Event('play'));
      // Preparation remains pending until the test delivers its decoder events.
      return new Promise<void>(() => {});
    }},
    pause: {value: () => {paused = true; video.dispatchEvent(new Event('pause'));}},
  });
  video.addEventListener('loadstart', () => {loaded = true;});
  for (const event of ['loadstart', 'loadedmetadata', 'play', 'playing', 'canplay', 'pause', 'seeking', 'seeked', 'kinosail:seek-intent']) {
    video.addEventListener(event, () => events.push({event, time, ready, paused, seeking}));
  }
  Object.assign(window, {intentDecoder: {
    configure: (value: number, state: number, buffered: number) => {time = value; ready = state; end = buffered;},
    quantizeTime: (value: number) => {quantum = value;},
    replaceSource: (value: number) => {
      source = 'https://127.0.0.1:38127/media/replacement';
      video.src = source; video.dataset.start = '0';
      loaded = false; ready = 0; time = 0; seeking = false;
      video.dispatchEvent(new Event('emptied'));
      loaded = true; ready = 4; time = value;
      video.dispatchEvent(new Event('loadedmetadata'));
    },
    // Native controls mutate the decoder and publish events; they do not call
    // the application's explicit custom-control or MediaSession callbacks.
    beginNativeSeek: (value: number) => {seeking = true; time = value; video.dispatchEvent(new Event('seeking'));},
    endNativeSeek: () => {seeking = false; video.dispatchEvent(new Event('seeked'));},
    snapshot: () => ({time, ready, paused, seeking, playCalls, events: [...events]}),
  }});
}

async function install(page: Page, preparation = false) {
  const writes: object[] = [], errors: string[] = [];
  const stored = {seconds: 20, watched: false};
  page.on('pageerror', error => errors.push(error.message));
  await page.route('**/progress/movie', async route => {
    expect(route.request().method()).toBe('POST');
    const body = new URLSearchParams(route.request().postData() || '');
    writes.push(Object.fromEntries(body));
    stored.seconds = Number(body.get('seconds')); stored.watched = body.get('watched') === 'true';
    await route.fulfill({status: 204});
  });
  await page.route('**/api/v1/items/movie', route => route.fulfill({json: {
    item: {id: 'movie', kind: 'video', title: 'Intent fixture movie', progress: {...stored}},
    listed: false, profileId: 'intent-test-profile',
  }}));
  await page.route(`**/static/player.js?v=${sourceHash}`, route => route.fulfill({contentType: 'text/javascript', body: source}));
  await page.route('https://127.0.0.1:38127/', route => route.fulfill({body: '<body>Library</body>', contentType: 'text/html'}));
  await page.route('**/watch/movie', route => route.fulfill({contentType: 'text/html', body: `
    <body data-viewer-profile="intent-test-profile"><a href="/">Library</a>
    <div class="media-stage"><video data-progress="/progress/movie" data-start="${preparation ? 20 : 0}"
      data-duration="100" data-playback-session="intent-test-session"></video>
    <div data-player-status><span data-player-message>Loading video…</span><progress data-buffered></progress></div></div>
    <script>(${decoderFixture.toString()})(${JSON.stringify(preparation)})</script>
    <script src="/static/player.js?v=${sourceHash}"></script></body>`}));
  await page.goto('https://127.0.0.1:38127/watch/movie');
  expect(errors, 'production script must initialize without a page error').toEqual([]);
  await expect(page.locator('video')).toHaveJSProperty('paused', true);
  return {writes, errors, stored};
}

async function publicProgress(page: Page) {
  return page.evaluate(async () => (await (await fetch('/api/v1/items/movie', {cache: 'no-store'})).json()).item.progress);
}
async function proof(page: Page, info: TestInfo, state: Awaited<ReturnType<typeof install>>, extra: object = {}) {
  const decoder = await page.evaluate(() => (window as TestWindow).intentDecoder?.snapshot());
  const evidence = {sourceHash, proofClass: 'controlled browser lifecycle, not populated-server/device proof',
    decoder, writes: state.writes, errors: state.errors, ...extra};
  await info.attach('intent-sequence.json', {body: JSON.stringify(evidence, null, 2), contentType: 'application/json'});
  expect(state.errors, 'RED must be a semantic assertion, not a script error').toEqual([]);
  return decoder;
}

test('unplayed Watch metadata and Library exit preserve positive public progress', async ({page}, info) => {
  const state = await install(page);
  const before = await publicProgress(page);
  expect(before.seconds).toBe(20);
  await expect(page.locator('video')).toHaveJSProperty('readyState', 4);
  await expect(page.locator('video')).toHaveJSProperty('currentTime', 0);
  const decoder = await proof(page, info, state, {before});
  await page.getByRole('link', {name: 'Library'}).click();
  await expect(page.getByText('Library', {exact: true})).toBeVisible();
  const after = await publicProgress(page);
  await info.attach('public-progress-after-exit.json', {body: JSON.stringify({before, after, decoder, writes: state.writes}), contentType: 'application/json'});
  expect(state.writes, 'no unplayed checkpoint may overwrite the saved position').toEqual([]);
  expect(after).toEqual(before);
});

for (const completion of ['same playing dispatch', 'queued playing after canplay']) {
  test(`automatic preparation ${completion} stays unplayed on Library exit`, async ({page}, info) => {
    const state = await install(page, true);
    await page.locator('video').dispatchEvent('loadstart');
    await expect(page.locator('video')).toHaveJSProperty('paused', false);
    expect(await page.evaluate('Boolean(playbackPreparation)')).toBe(true);
    await page.evaluate(() => (window as TestWindow).intentDecoder.configure(20.2, 3, 23));
    await page.locator('video').dispatchEvent(completion.startsWith('same') ? 'playing' : 'canplay');
    await expect(page.locator('video')).toHaveJSProperty('paused', true);
    expect(await page.evaluate('Boolean(playbackPreparation)')).toBe(false);
    if (completion.startsWith('queued')) await page.locator('video').dispatchEvent('playing');
    await page.evaluate(() => (window as TestWindow).intentDecoder.endNativeSeek());
    await expect(page.locator('video')).toHaveJSProperty('paused', true);
    const decoder = await proof(page, info, state, {explicitViewerPlayOrSeek: false, completion});
    await page.getByRole('link', {name: 'Library'}).click();
    await info.attach('exit-writes.json', {body: JSON.stringify({decoder, writes: state.writes}), contentType: 'application/json'});
    expect(state.writes, 'automatic preparation must not authorize a progress checkpoint').toEqual([]);
  });
}

for (const {target, restore} of [{target: 0, restore: 20}, {target: 35, restore: 20}, {target: 0, restore: 0.05}]) {
  test(`first paused native seek to ${target} supersedes pending managed restoration${restore === 20 ? "" : " at 0.05"} and saves`, async ({page}, info) => {
    const state = await install(page);
    // Exercise the real automatic source-change restore, withholding only the
    // decoder's seeked completion until the user's native seek supersedes it.
    await page.evaluate(`resumeAfterSourceChange(false, true, ${restore})`);
    await page.evaluate(value => (window as TestWindow).intentDecoder.configure(value, 4, 60), restore === 20 ? 0 : 20);
    await page.locator('video').dispatchEvent('loadedmetadata');
    await expect(page.locator('video')).toHaveJSProperty('currentTime', restore);
    await expect(page.locator('video')).toHaveJSProperty('seeking', true);
    expect(await page.evaluate('managedSeek')).toBe(true);
    const automatic = await proof(page, info, state, {target});
    expect(state.writes, 'the automatic restore must remain unplayed').toEqual([]);
    await page.evaluate(value => {
      const control = (window as TestWindow).intentDecoder;
      control.beginNativeSeek(value); control.endNativeSeek();
    }, target);
    await expect(page.locator('video')).toHaveJSProperty('paused', true);
    await expect(page.locator('video')).toHaveJSProperty('seeking', false);
    await proof(page, info, state, {automatic, target, explicitNativeSeekingAndSeeked: true});
    await expect.poll(() => state.writes.length, {timeout: 1500,
      message: 'the explicit native seek must save after managed restoration is superseded'}).toBe(1);
    expect(state.stored.seconds).toBe(target);
  });
}

for (const {restore, quantum, expected} of [
  {restore: 0.05, quantum: 0, expected: 0.05},
  {restore: 0.056, quantum: 0.01, expected: 0.06},
]) test(`automatic fractional restoration ${restore} with decoder quantum ${quantum} stays unplayed`, async ({page}, info) => {
  const state = await install(page);
  await page.evaluate(value => (window as TestWindow).intentDecoder.quantizeTime(value), quantum);
  await page.evaluate(() => (window as TestWindow).intentDecoder.configure(20, 4, 60));
  await page.evaluate(`resumeAfterSourceChange(false, true, ${restore})`);
  await page.locator('video').dispatchEvent('loadedmetadata');
  await expect(page.locator('video')).toHaveJSProperty('currentTime', expected);
  await page.evaluate(() => (window as TestWindow).intentDecoder.endNativeSeek());
  await proof(page, info, state, {restore, quantum, explicitViewerPlayOrSeek: false});
  await page.getByRole('link', {name: 'Library'}).click();
  expect(state.writes, 'automatic fractional or rounded restoration must not save').toEqual([]);
});

for (const target of [undefined, 0, 35]) {
  test(`replacement after cancelled pending restoration ${target === undefined ? 'stays unplayed' : `saves native seek to ${target}`}`, async ({page}, info) => {
    const state = await install(page);
    await page.evaluate('resumeAfterSourceChange(false, true, 20)');
    await page.locator('video').dispatchEvent('loadedmetadata');
    await expect(page.locator('video')).toHaveJSProperty('seeking', true);
    expect(state.writes).toEqual([]);
    // The decoder cancels the old source's unfinished seek without seeked. The
    // replacement has metadata but no application restore or user play intent.
    await page.evaluate(() => (window as TestWindow).intentDecoder.replaceSource(10));
    await expect(page.locator('video')).toHaveJSProperty('currentTime', 10);
    await expect(page.locator('video')).toHaveJSProperty('seeking', false);
    if (target !== undefined) {
      await page.evaluate(value => {
        const decoder = (window as TestWindow).intentDecoder;
        decoder.beginNativeSeek(value); decoder.endNativeSeek();
      }, target);
      await expect.poll(() => state.writes.length, {timeout: 1500}).toBe(1);
      expect(state.stored.seconds).toBe(target);
    }
    await proof(page, info, state, {target: target ?? 'no explicit input', replacementReadyTime: 10,
      oldSeekCompletionCancelled: true, publicProgress: await publicProgress(page)});
    await page.getByRole('link', {name: 'Library'}).click();
    if (target === undefined) expect(state.writes).toEqual([]);
    else expect(state.stored.seconds).toBe(target);
  });
}
