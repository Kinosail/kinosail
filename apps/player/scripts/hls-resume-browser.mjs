import {createRequire} from 'node:module';
import {createHash} from 'node:crypto';
import {join} from 'node:path';
import {writeFile} from 'node:fs/promises';

const root = process.cwd(), require = createRequire(join(root, 'apps/player/e2e/package.json'));
const {chromium, webkit} = require('@playwright/test');
const origin = process.env.QA_RESUME_ORIGIN, token = process.env.QA_RESUME_TOKEN;
const id = process.env.QA_RESUME_ITEM, output = process.env.QA_RESUME_OUTPUT;
const engine = process.env.QA_RESUME_ENGINE || 'chromium';
const native = engine === 'webkit-native';
const expectedMode = process.env.QA_RESUME_EXPECTED_MODE || 'remux';
const frameRate = Number(process.env.QA_RESUME_FRAME_RATE || 24);
const duration = Number(process.env.QA_RESUME_DURATION || 32);
const resume = Number(process.env.QA_RESUME_SECONDS || 12.5);
const forceConversion = process.env.QA_RESUME_FORCE_AUDIO_CONVERSION === 'true';
const channel = process.env.PLAYWRIGHT_CHANNEL || (!process.env.CI && process.platform === 'darwin' ? 'chrome' : undefined);
const browser = await (engine.startsWith('webkit') ? webkit.launch() : chromium.launch({channel}));
const context = await browser.newContext({...(forceConversion ? {userAgent: 'Mozilla/5.0 Chrome/130.0.0.0 Safari/537.36'} : {}), extraHTTPHeaders: {Authorization: `Bearer ${token}`}});
const rows = [], diagnostics = [], responses = [], playlists = [], playlistCaptures = [];
const diag = process.env.QA_RESUME_DIAG === '1';
let failureClass, stage = 'setup';

try {
  const planResponse = await context.request.get(`${origin}/api/v1/items/${process.env.QA_RESUME_REFERENCE_ITEM || id}/playback?videoCodecs=h264&audioCodecs=aac`);
  if (planResponse.status() !== 200) throw new Error('playback_plan_status');
  const plan = await planResponse.json(), reference = plan.direct;
  if (!reference?.startsWith('/') || reference.startsWith('//')) throw new Error('reference_source_shape');
  for (const variant of ['reference', 'compatible', ...(diag ? [...Array(12).fill('reference-natural'), 'reference-race', 'reference-race', 'reference-control'] : [])]) {
    const progress = await context.request.put(`${origin}/api/v1/items/${id}/progress`, {data: {seconds: resume, watched: false}});
    if (progress.status() !== 200) throw new Error('save_resume_status_' + progress.status());
    const page = await context.newPage();
    await page.addInitScript(() => document.addEventListener('playing', event => {
      if (event.target instanceof HTMLVideoElement && window.qaResumeFirstPlaying === undefined)
        window.qaResumeFirstPlaying = event.target.currentTime;
    }, true));
    if (variant === 'reference-race' || variant === 'reference-control') await page.addInitScript(() => {
      const descriptor = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'currentTime');
      const nativeRequest = HTMLVideoElement.prototype.requestVideoFrameCallback;
      let pending, raced = 0;
      window.qaRaces = 0;
      Object.defineProperty(HTMLMediaElement.prototype, 'currentTime', {configurable: true, get: descriptor.get, set(value) {
        if (this instanceof HTMLVideoElement && raced < 6 && this.paused && value < this.duration - 1) {
          raced++; pending = value; descriptor.set.call(this, value - 1); return;
        }
        descriptor.set.call(this, value);
      }});
      HTMLVideoElement.prototype.requestVideoFrameCallback = function (callback) {
        const video = this;
        return nativeRequest.call(video, (now, metadata) => {
          if (pending !== undefined && Math.abs(metadata.mediaTime - (pending - 1)) < .1) {
            const target = pending; pending = undefined; window.qaRaces++;
            descriptor.set.call(video, target);
            const until = performance.now() + 1500; while (performance.now() < until);
          }
          callback(now, metadata);
        });
      };
    });
    if (native) await page.addInitScript(() => {
      for (const name of ['MediaSource', 'ManagedMediaSource', 'WebKitMediaSource'])
        Object.defineProperty(window, name, {configurable: true, value: undefined});
    });
    let encoderReleased = false;
    if (variant === 'compatible' && process.env.QA_RESUME_RELEASE_ENCODER) page.on('request', async request => {
      const url = new URL(request.url());
      const segment = url.pathname.match(/^\/hls\/.*\/segment-(\d{5})\.m4s$/)?.[1];
      if (!encoderReleased && url.origin === origin && segment && Number(segment) > 0) {
        encoderReleased = true;
        await writeFile(process.env.QA_RESUME_RELEASE_ENCODER, segment);
      }
    });
    page.on('response', response => {
      const path = new URL(response.url()).pathname;
      if (path.startsWith('/hls/')) responses.push({
        kind: path.endsWith('.m3u8') ? 'playlist' : path.endsWith('.m4s') ? 'fragment' : 'initialization',
        status: response.status(), offset: path.match(/-o(\d+)\//)?.[1],
        segment: path.match(/\/(segment-\d{5}\.m4s)$/)?.[1],
        origin: response.headers()['x-kinosail-hls-origin'], clock: response.headers()['x-kinosail-hls-clock'],
      });
      if (!path.startsWith('/hls/') || !path.endsWith('.m3u8') || playlists.length >= 16) return;
      const capture = {variant, offset: path.match(/-o(\d+)\//)?.[1], state: 'pending'};
      playlists.push(capture);
      playlistCaptures.push((async () => {
        let timer;
        try {
          const bytes = await Promise.race([response.body(), new Promise((_, reject) => {
            timer = setTimeout(() => reject(new Error('capture_timeout')), 2000);
          })]);
          if (bytes.length > 65536) {capture.state = 'oversized'; return;}
          const lines = bytes.toString('utf8').split('\n').map(line => line.trim());
          const durations = lines.filter(line => /^#EXTINF:\d+(?:\.\d+)?,/.test(line)).map(line => Number(line.match(/^#EXTINF:([\d.]+)/)[1]));
          const segments = lines.map(line => line.match(/^segment-(\d{5})\.m4s(?:\?|$)/)?.[1]).filter(Boolean);
          if (durations.length > 64 || segments.length > 64) {capture.state = 'too_many_segments'; return;}
          if (durations.some(value => !Number.isFinite(value) || value <= 0 || value > 60)) {capture.state = 'invalid_durations'; return;}
          const targetDuration = Number(lines.find(line => /^#EXT-X-TARGETDURATION:\d+$/.test(line))?.split(':')[1]);
          Object.assign(capture, {state: 'captured', sha256: createHash('sha256').update(bytes).digest('hex'),
            master: lines.some(line => line.startsWith('#EXT-X-STREAM-INF:')),
            ended: lines.includes('#EXT-X-ENDLIST'), event: lines.includes('#EXT-X-PLAYLIST-TYPE:EVENT'),
            vod: lines.includes('#EXT-X-PLAYLIST-TYPE:VOD'), durations, segments,
            targetDuration: Number.isInteger(targetDuration) && targetDuration > 0 && targetDuration <= 60 ? targetDuration : null});
        } catch (error) {capture.state = error instanceof Error && error.message === 'capture_timeout' ? 'timeout' : 'unavailable';}
        finally {clearTimeout(timer);}
      })());
    });
    if (variant.startsWith('reference')) {
      await page.route(`${origin}/__qa_reference`, route => route.fulfill({contentType: 'text/html',
        body: `<video muted playsinline src="${reference.replaceAll('&', '&amp;').replaceAll('"', '&quot;')}"></video>`}));
      await page.goto(`${origin}/__qa_reference`);
    } else {
      await page.goto(`${origin}/watch/${id}?compatible=1`);
    }
    if (variant === 'compatible') {
      await page.waitForFunction(() => document.querySelector('video')?.readyState >= 2);
      const stage = page.locator('.media-stage');
      if (await page.locator('video').evaluate(video => video.paused)) await stage.press('k');
      await page.waitForFunction(() => !document.querySelector('video').paused);
      await page.waitForFunction(() => Number.isFinite(window.qaResumeFirstPlaying));
      await stage.press('k');
      await page.waitForFunction(() => document.querySelector('video').paused);
    }
    const row = await page.evaluate(async ({variant, duration, native, resume, frameRate, currentFrames}) => {
      const video = document.querySelector('video'), samples = [], presentations = [];
      if (!video) return {variant, error: 'video_missing', samples};
      const wait = (event, action) => new Promise((resolve, reject) => {
        const timer = setTimeout(() => {
          video.removeEventListener(event, done);
          reject(new Error(event + '_timeout'));
        }, 30000);
        const done = () => { clearTimeout(timer); resolve(); };
        video.addEventListener(event, done, {once: true});
        action?.();
      });
      let error, ended = false, eofStart;
      try {
        if (video.readyState < 2) await wait('loadeddata');
        video.pause();
        const points = resume >= 60 ? [resume + .05, resume + .55, resume + 1.55, resume + 2.55, resume + 3.55, resume + 4.55, duration - .05]
          : [resume + .05, resume + .55, resume + 5.05, resume + 9.05, resume + 13.05, resume + 17.05, duration > 40 ? duration - .05 : Math.min(31.55, duration - .05)];
        for (const point of points) {
          let confirmCurrent;
          const presented = new Promise((resolve, reject) => {
            let callback, settled = false, lastCurrent;
            const timer = setTimeout(() => {
              settled = true;
              video.cancelVideoFrameCallback(callback);
              reject(new Error('frame_presentation_timeout'));
            }, 30000);
            const observe = (mediaTime, source) => {
              const decodedTime = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'currentTime').get.call(video);
              const presentedTime = mediaTime + video.currentTime - decodedTime;
              if (presentations.length < 64) presentations.push({point, source, mediaTime, presentedTime,
                currentTime: video.currentTime, decodedTime, paused: video.paused, seeking: video.seeking});
              if (presentedTime < point - 1 / frameRate - .002 || presentedTime > point + .002) return;
              settled = true;
              video.pause(); clearTimeout(timer); video.cancelVideoFrameCallback(callback);
              resolve({presentedTime, mediaTime, source});
            };
            const frame = (_, metadata) => {
              if (settled) return;
              observe(metadata.mediaTime, 'callback');
              if (!settled) callback = video.requestVideoFrameCallback(frame);
            };
            const current = () => {
              if (settled) return;
              let image;
              try { image = new VideoFrame(video); } catch {}
              const mediaTime = image ? image.timestamp / 1e6 : undefined;
              image?.close();
              if (mediaTime !== undefined && mediaTime !== lastCurrent) { lastCurrent = mediaTime; observe(mediaTime, 'current'); }
              if (!settled) requestAnimationFrame(current);
            };
            confirmCurrent = () => { if (currentFrames) current(); };
            callback = video.requestVideoFrameCallback(frame);
          });
          const [, frame] = await Promise.all([wait('seeked', () => {
            video.currentTime = point;
            if (native && variant === 'compatible') { video.muted = true; video.play().catch(() => {}); }
          }).then(() => confirmCurrent()), presented]);
          video.pause();
          const canvas = document.createElement('canvas');
          canvas.width = video.videoWidth; canvas.height = video.videoHeight;
          const drawing = canvas.getContext('2d'); drawing.drawImage(video, 0, 0);
          const bytes = drawing.getImageData(0, 0, canvas.width, canvas.height).data;
          const digest = [...new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))]
            .map(value => value.toString(16).padStart(2, '0')).join('');
          samples.push({point, currentTime: video.currentTime, duration: video.duration, ...frame, digest});
        }
        video.muted = true;
        await wait('seeked', () => video.currentTime = Math.max(0, video.duration - .5));
        eofStart = {currentTime: video.currentTime, ended: video.ended};
        if (video.ended) throw new Error('eof_start_ended');
        await wait('ended', () => video.play());
        ended = video.ended;
      } catch (caught) { error = caught.message; }
      return {variant, error, ended, duration: video.duration, width: video.videoWidth, height: video.videoHeight,
        firstPlaying: window.qaResumeFirstPlaying, eofStart, seeking: video.seeking, readyState: video.readyState,
        start: Number(video.dataset.start), mode: video.dataset.compatibilityMode,
        adapter: video.currentSrc.startsWith('blob:') ? 'mse' : 'native', currentTime: video.currentTime, samples, presentations};
    }, {variant, duration, native, resume, frameRate, currentFrames: engine === 'chromium' && variant !== 'reference-control'});
    if (variant.startsWith('reference-')) row.races = await page.evaluate(() => window.qaRaces);
    (variant.startsWith('reference-') ? diagnostics : rows).push(row);
    await page.screenshot({path: output.replace('.json', `-${variant}.png`)});
    await Promise.all(playlistCaptures);
    await page.close();
  }
} catch (error) {
  failureClass = error.name === 'TimeoutError' ? 'browser_action_timeout' : error.message;
  await context.pages().at(-1)?.screenshot({path: output.replace('.json', '-failure.png')});
}
finally {
  await Promise.all(playlistCaptures);
  await context.close();
  await browser.close();
}
const matches = (rows[0]?.samples || []).map((sample, index) => sample.digest === rows[1]?.samples[index]?.digest);
const expected = (rows[0]?.samples || []).map(sample => [sample.mediaTime, sample.digest].join());
const diagnosticRow = row => row.variant === 'reference-control' ? row.error === 'frame_presentation_timeout' && row.races >= 1
  : !row.error && row.ended && row.samples.length === 7 && row.samples.every((sample, index) => [sample.mediaTime, sample.digest].join() === expected[index]);
const diagnosticSummary = diag ? {rows: diagnostics.length, passed: diagnostics.filter(diagnosticRow).length,
  byVariant: Object.fromEntries(['reference-natural', 'reference-race', 'reference-control'].map(name => [name, {
    rows: diagnostics.filter(row => row.variant === name).length, passed: diagnostics.filter(row => row.variant === name && diagnosticRow(row)).length,
    errors: diagnostics.filter(row => row.variant === name && row.error).map(row => row.error),
    races: diagnostics.filter(row => row.variant === name).reduce((sum, row) => sum + (row.races || 0), 0),
    currentSamples: diagnostics.filter(row => row.variant === name).flatMap(row => row.samples).filter(sample => sample.source === 'current').length,
    staleCallbacks: diagnostics.filter(row => row.variant === name).flatMap(row => row.presentations).filter(entry => entry.source === 'callback' && Math.abs(entry.mediaTime - entry.point) > .1).length,
    staleInitial: diagnostics.filter(row => row.variant === name && row.presentations[0]?.mediaTime === 0).length}]))} : null;
const passed = (!diag || diagnosticSummary.rows === 15 && diagnosticSummary.passed === 15) && !failureClass && rows.length === 2 && !rows.some(row => row.error || !row.ended)
  && matches.length === 7 && matches.every(Boolean) && expectedMode.split(',').includes(rows[1].mode)
  && Math.abs(rows[1].duration - rows[0].duration) < .1
  && rows[1].firstPlaying >= resume - .05 && rows[1].firstPlaying < resume + .5
  && rows[1].adapter === (native ? 'native' : 'mse')
  && responses.length > 0 && responses.every(response => response.status < 400);
await writeFile(output, JSON.stringify({engine, browserVersion: browser.version(),
  capabilitySimulation: [native ? 'MSE disabled to exercise native WebKit HLS policy' : '', forceConversion ? 'Chrome user agent forces server AAC conversion policy' : ''].filter(Boolean).join('; ') || null,
  proofClass: 'Real authenticated Server process; synthetic source; unmodified watch page and HLS; original-file or independently certified stream-copy MP4 reference',
  stage, passed, failureClass, rows, diagnosticSummary, diagnostics, matches, responses, playlists}, null, 2) + '\n');
console.log(JSON.stringify({engine, passed, failureClass, matched: matches.filter(Boolean).length, total: matches.length, diagnosticSummary}));
process.exitCode = passed ? 0 : 1;
