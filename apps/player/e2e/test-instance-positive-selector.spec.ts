import {devices, expect, test} from '@playwright/test';
import {createHash} from 'node:crypto';
import {readFile, writeFile} from 'node:fs/promises';
import {configureTestInstance, login} from './test-instance-helpers';

// Gap: native fullscreen admission can fail before the selected stream moves.
// Observe the untouched phone selector, then separately decode its exact URL
// in an ordinary inline rendered fixture. This is not native launch acceptance.
configureTestInstance();
test.use({...devices['iPhone 13']});
test('positive saved selector requests and separately labeled inline decoder control', async ({page, browser}, info) => {
  test.skip(process.env.KINOSAIL_POSITIVE_REENTRY_E2E !== '1', 'Owned synthetic loopback runner only');
  test.setTimeout(90_000);
  const base = new URL(process.env.KINOSAIL_E2E_URL!);
  expect(base.protocol).toBe('https:'); expect(base.hostname).toBe('localhost');
  expect(info.project.name).toBe('webkit');
  const sourceReceipt = JSON.parse(await readFile(process.env.KINOSAIL_POSITIVE_SOURCE_RECEIPT!, 'utf8'));
  const observations: object[] = [], sourceRequests: object[] = [], scriptResponses: object[] = [], servedWatch: object[] = [], progressWrites: object[] = [];
  const pendingResponses: Promise<void>[] = [];
  let phase = 'login';
  const record = async (value: object) => {
    observations.push({utc: new Date().toISOString(), ...value});
    await writeFile(info.outputPath('positive-selector.json'), JSON.stringify({sourceReceipt,
      boundaries: 'Untouched product phone-policy selector and real public HLS requests; separately labeled desktop macOS WebKit inline rendered-fixture decoder control. No native fullscreen, audible, physical-device, production fix or checkpoint acceptance.',
      observations, sourceRequests, servedWatch, scriptResponses, progressWrites}, null, 2));
  };
  const bounded = (value: string | null, max = 604800) => {
    const number = value === null ? NaN : Number(value);
    return Number.isFinite(number) && number >= 0 && number <= max ? number : null;
  };
  const hlsProjection = (value: string) => {
    const endpoint = new URL(value);
    if (endpoint.origin !== base.origin || !endpoint.pathname.startsWith('/hls/')) return null;
    const offset = /-o(\d+)(?:\/|$)/.exec(endpoint.pathname);
    const milliseconds = offset ? bounded(offset[1], 604800000) : 0;
    if (milliseconds === null) return null;
    return {offsetSeconds: milliseconds / 1000,
      role: endpoint.pathname.endsWith('/index.m3u8') ? 'master' : endpoint.pathname.endsWith('.m3u8') ? 'playlist' : 'media'};
  };
  page.on('request', request => {
    try {
      const projection = hlsProjection(request.url());
      if (projection && sourceRequests.length < 64 && ['GET', 'HEAD'].includes(request.method()))
        sourceRequests.push({utc: new Date().toISOString(), phase, operation: 'request', method: request.method(), ...projection});
    } catch { /* Fixed optional projection; no raw requests retained. */ }
  });
  await page.addInitScript(() => {
    localStorage.setItem('kinosail.playback-policy-v2', 'direct-first');
    const events: object[] = []; Object.assign(window, {selectorMediaEvents: events});
    const bound = new WeakSet<HTMLVideoElement>();
    const observe = () => {
      const video = document.querySelector('video'); if (!video || bound.has(video)) return;
      bound.add(video);
      for (const name of ['loadedmetadata', 'seeking', 'seeked', 'play', 'playing', 'pause', 'kinosail:seek-intent'])
        video.addEventListener(name, event => {
          if (events.length >= 30) return;
          const rawTime = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'currentTime')!.get!.call(video);
          const start = Number(video.dataset.start);
          events.push({event: name, trusted: event.isTrusted, elapsedMS: Math.round(performance.now()), paused: video.paused,
            rawTime: Number.isFinite(rawTime) && rawTime >= 0 && rawTime <= 31622400 ? rawTime : null,
            renderedStart: Number.isFinite(start) && start >= 0 && start <= 31622400 ? start : null});
        });
    };
    new MutationObserver(observe).observe(document, {childList: true, subtree: true}); observe();
  });
  await login(page);
  const csrf = async () => ({Origin: base.origin,
    'X-Kinosail-CSRF': await page.locator('meta[name="kinosail-csrf"]').getAttribute('content') || ''});
  expect((await page.request.put('/api/v1/settings/playback', {headers: await csrf(),
    data: {mode: 'automatic', autoplay: false, subtitles: 'off', autoSkip: []}})).ok()).toBe(true);
  const library = await (await page.request.get('/api/v1/library')).json();
  const item = library.items.find((value: {title: string}) => value.title === 'Positive Selector');
  const nativeItem = library.items.find((value: {title: string}) => value.title === 'Positive Reentry');
  expect(item).toBeTruthy();
  expect(nativeItem).toBeTruthy(); expect(item.id).not.toBe(nativeItem.id);
  const path = `/api/v1/items/${item.id}`, watch = `/watch/${item.id}`;
  const progressEndpoint = `/progress/${item.id}`;
  page.on('request', request => {
    try {
      const endpoint = new URL(request.url());
      if (progressWrites.length >= 24 || endpoint.origin !== base.origin || ![progressEndpoint, path + '/progress'].includes(endpoint.pathname) || !['POST', 'PUT'].includes(request.method())) return;
      const body = request.postData(); if (!body || body.length > 4096) return;
      const value = endpoint.pathname === progressEndpoint ? Object.fromEntries(new URLSearchParams(body)) : JSON.parse(body);
      progressWrites.push({utc: new Date().toISOString(), phase, operation: 'request',
        route: endpoint.pathname === progressEndpoint ? 'page-progress' : 'api-progress',
        seconds: bounded(typeof value.seconds === 'number' ? String(value.seconds) : value.seconds, 31622400),
        revision: bounded(typeof value.revision === 'number' ? String(value.revision) : value.revision, 1000000)});
    } catch { /* Only known numeric fields; never retain body/session. */ }
  });
  const publicPosition = async () => {
    const response = await page.request.get(path + '/watch-progress');
    expect(response.ok()).toBe(true); return response.json();
  };
  page.on('response', response => {
    const responsePhase = phase;
    const operation = (async () => {
      try {
        const endpoint = new URL(response.url());
        if (endpoint.origin !== base.origin) return;
        if ([progressEndpoint, path + '/progress'].includes(endpoint.pathname) && ['POST', 'PUT'].includes(response.request().method()) && progressWrites.length < 24)
          progressWrites.push({utc: new Date().toISOString(), phase: responsePhase, operation: 'response', status: response.status()});
        const projection = hlsProjection(response.url());
        if (projection && sourceRequests.length < 64)
          sourceRequests.push({utc: new Date().toISOString(), phase: responsePhase, operation: 'response', status: response.status(), ...projection});
        if (endpoint.pathname === watch && servedWatch.length < 4) {
          const body = await response.body();
          if (body.length > 1048576) return;
          const tag = /<video\b[^>]{0,16000}>/.exec(body.toString('utf8'))?.[0] || '';
          servedWatch.push({phase: responsePhase, status: response.status(),
            start: bounded(/\bdata-start="([0-9.eE+-]{1,64})"/.exec(tag)?.[1] || null),
            fullDuration: bounded(/\bdata-duration="([0-9.eE+-]{1,64})"/.exec(tag)?.[1] || null, 31622400)});
        }
        if (endpoint.pathname === '/static/player.js' && /^[a-f0-9]{64}$/.test(endpoint.searchParams.get('v') || '') && scriptResponses.length < 4) {
          const bytes = await response.body(); if (bytes.length > 1048576) return;
          const sha256 = createHash('sha256').update(bytes).digest('hex');
          scriptResponses.push({phase: responsePhase, status: response.status(), sha256,
            matchesReferencedVersion: sha256 === endpoint.searchParams.get('v'),
            resourceType: response.request().resourceType() === 'script' ? 'script' : 'other'});
        }
      } catch { /* Response projections must not change selector behavior. */ }
    })();
    if (pendingResponses.length < 128) pendingResponses.push(operation);
  });
  expect((await page.request.put(path + '/progress', {headers: await csrf(), data: {seconds: 6, watched: false}})).ok()).toBe(true);
  await expect.poll(() => publicPosition().then(value => value.seconds)).toBe(6);
  await record({phase: 'seed6', public: await publicPosition()});
  phase = 'initial-Watch-before-any-Play';
  await page.goto('/');
  await page.locator(`a.card[href="${watch}"]`).first().click();
  await expect.poll(() => page.locator('video').evaluate(video => (video as HTMLVideoElement).readyState), {timeout: 30_000}).toBeGreaterThanOrEqual(2);
  await record({phase: 'isolated-item-initial-metadata', public: await publicPosition(),
    events: await page.evaluate(() => (window as unknown as {selectorMediaEvents: object[]}).selectorMediaEvents),
    separateProgressKey: true});
  phase = 'reload-before-any-Play';
  await page.reload();
  await expect.poll(() => page.locator('video').evaluate(video => (video as HTMLVideoElement).readyState), {timeout: 30_000}).toBeGreaterThanOrEqual(2);
  await Promise.all(pendingResponses);
  const selectedURL = await page.locator('video').evaluate(video => (video as HTMLVideoElement).currentSrc || (video as HTMLVideoElement).src);
  const selected = hlsProjection(selectedURL);
  expect(selected).not.toBeNull();
  const state = await page.locator('video').evaluate(element => {
    const video = element as HTMLVideoElement;
    return {renderedStart: Number(video.dataset.start), fullDuration: Number(video.dataset.duration),
      rawTime: Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'currentTime')!.get!.call(video),
      reportedPosition: video.currentTime, paused: video.paused, readyState: video.readyState,
      directType: video.dataset.directType, compatibilityMode: video.dataset.compatibilityMode,
      policy: ['direct-first', 'direct-only', 'compatible'].includes(localStorage.getItem('kinosail.playback-policy-v2') || '') ? localStorage.getItem('kinosail.playback-policy-v2') : 'unreported',
      nativeHLS: video.canPlayType('application/vnd.apple.mpegurl')};
  });
  await record({phase: 'selected-source-before-any-Play', state, selected, public: await publicPosition(),
    events: await page.evaluate(() => (window as unknown as {selectorMediaEvents: object[]}).selectorMediaEvents)});
  expect(state.paused).toBe(true);
  expect(state.directType).toBe('video/x-matroska'); expect(state.compatibilityMode).toBe('transcode');
  expect(state.policy).toBe('direct-first'); expect(state.nativeHLS).not.toBe('');
  const control = await browser.newContext({...devices['Desktop Safari'], storageState: await page.context().storageState(), ignoreHTTPSErrors: false});
  let controlWrites = 0;
  control.on('request', request => {
    try {
      const endpoint = new URL(request.url());
      if (endpoint.origin === base.origin && [path + '/progress', progressEndpoint].includes(endpoint.pathname) && !['GET', 'HEAD'].includes(request.method())) controlWrites++;
    } catch { /* Counter only; no body or URL retained. */ }
  });
  let controlFailed = false;
  let inline: Awaited<ReturnType<typeof control.newPage>> | undefined;
  try {
    const decoderPage = await control.newPage(); inline = decoderPage;
    await decoderPage.goto(base.origin + '/healthz');
    await decoderPage.setContent('<!doctype html><title>Separate public decoder control</title><video controls playsinline muted width="320" height="180"></video><button>Decode observed source inline</button>');
    await decoderPage.evaluate(source => {
      const video = document.querySelector('video')!;
      const state = {frames: [] as {mediaTime: number, rawTime: number, rgb: number[], png: string}[], callbacks: 0, playFailure: ''};
      Object.assign(window, {inlineDecoderControl: state});
      video.muted = true;
      video.src = source;
      const observe = (_now: number, metadata: VideoFrameCallbackMetadata) => {
        state.callbacks++;
        if (!video.paused && state.frames.length < 2) {
          const canvas = document.createElement('canvas'); canvas.width = 32; canvas.height = 18;
          const context = canvas.getContext('2d')!; context.drawImage(video, 0, 0, 32, 18);
          const pixels = context.getImageData(0, 0, 32, 18).data, rgb = [0, 0, 0];
          for (let i = 0; i < pixels.length; i += 4) for (let c = 0; c < 3; c++) rgb[c] += pixels[i + c] / (32 * 18);
          state.frames.push({mediaTime: metadata.mediaTime,
            rawTime: Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'currentTime')!.get!.call(video), rgb, png: canvas.toDataURL('image/png')});
        }
        if (state.frames.length < 2) video.requestVideoFrameCallback(observe);
      };
      video.requestVideoFrameCallback(observe);
      document.querySelector('button')!.onclick = () => { video.play().catch(error => {
        state.playFailure = ['NotAllowedError', 'InvalidStateError', 'NotSupportedError', 'AbortError'].includes(error.name) ? error.name : 'unreported';
      }); };
    }, selectedURL);
    const started = Date.now();
    await decoderPage.getByRole('button', {name: 'Decode observed source inline', exact: true}).click();
    await expect.poll(() => decoderPage.evaluate(() => (window as unknown as {inlineDecoderControl: {frames: object[]}}).inlineDecoderControl.frames.length), {timeout: 30_000}).toBe(2);
    const frames = await decoderPage.evaluate(() => (window as unknown as {inlineDecoderControl: {frames: {mediaTime: number, rawTime: number, rgb: number[], png: string}[]}}).inlineDecoderControl.frames);
    await writeFile(info.outputPath('inline-control-first-frame.png'), Buffer.from(frames[0].png.split(',')[1], 'base64'));
    await record({phase: 'separate-inline-control-moving-frames', elapsedObservationMS: Date.now() - started,
      selected, frames: frames.map(({png: _png, ...value}) => value), controlWrites, public: await publicPosition()});
    expect(frames[1].mediaTime).toBeGreaterThan(frames[0].mediaTime);
    expect(state.renderedStart).toBe(6);
    expect(controlWrites).toBe(0); expect((await publicPosition()).seconds).toBe(6);
    await record({phase: 'source-regression-verdict', selectedOffsetMatchesSaved6: selected!.offsetSeconds === 6,
      firstFrameMatchesSavedYellow: frames[0].rgb[0] > 150 && frames[0].rgb[1] > 150 && frames[0].rgb[2] < 80,
      boundary: 'This diagnoses selected source bytes through an inline control; native fullscreen and changed accepted checkpoint remain separate failed or unadmitted gates.'});
    expect(sourceRequests.some(value => (value as {operation: string}).operation === 'request')).toBe(true);
    expect(servedWatch.some(value => (value as {start: number}).start === 6)).toBe(true);
    expect(selected!.offsetSeconds).toBe(6);
    expect(frames[0].rgb[0]).toBeGreaterThan(150); expect(frames[0].rgb[1]).toBeGreaterThan(150); expect(frames[0].rgb[2]).toBeLessThan(80);
  } catch (error) {
    controlFailed = true;
    throw error;
  } finally {
    try {
      const decoder = inline && await inline.evaluate(() => {
        const video = document.querySelector('video') as HTMLVideoElement & {webkitDisplayingFullscreen?: boolean};
        const value = (window as unknown as {inlineDecoderControl?: {callbacks: number, frames: object[], playFailure: string}}).inlineDecoderControl;
        return {paused: video.paused, readyState: video.readyState, errorCode: video.error?.code || 0,
          rawTime: Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'currentTime')!.get!.call(video),
          fullscreen: video.webkitDisplayingFullscreen === true, callbacks: value?.callbacks ?? null,
          capturedFrames: value?.frames.length ?? null, playFailure: value?.playFailure || ''};
      });
      await record({phase: 'separate-inline-control-final-decoder-state', decoder});
    } catch { await record({phase: 'separate-inline-control-final-state-unavailable'}).catch(() => {}); }
    try { await record({phase: 'separate-inline-control-ended', controlWrites, public: await publicPosition()}); }
    catch { await record({phase: 'separate-inline-control-read-unavailable', controlWrites}).catch(() => {}); }
    try { await control.close(); }
    catch {
      await record({phase: 'separate-inline-control-close-failed'}).catch(() => {});
      if (!controlFailed) throw new Error('Inline control context cleanup failed');
    }
  }
});
