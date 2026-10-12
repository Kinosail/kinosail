import {createRequire} from 'node:module';
import {join} from 'node:path';
import {writeFile} from 'node:fs/promises';

const root = process.cwd(), require = createRequire(join(root, `apps/${process.env.QA_RESUME_APP || 'player'}/e2e/package.json`));
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
const directSwitch = process.env.QA_RESUME_DIRECT_SWITCH === 'true';
const channel = process.env.PLAYWRIGHT_CHANNEL || (!process.env.CI && process.platform === 'darwin' ? 'chrome' : undefined);
const browser = await (engine.startsWith('webkit') ? webkit.launch() : chromium.launch({channel}));
const context = await browser.newContext({...(forceConversion ? {userAgent: 'Mozilla/5.0 Chrome/130.0.0.0 Safari/537.36'} : {}), extraHTTPHeaders: {Authorization: `Bearer ${token}`}});
const rows = [], responses = [];
let failureClass, stage = 'setup';

try {
  const planResponse = await context.request.get(`${origin}/api/v1/items/${process.env.QA_RESUME_REFERENCE_ITEM || id}/playback?videoCodecs=h264&audioCodecs=aac`);
  if (planResponse.status() !== 200) throw new Error('playback_plan_status');
  const plan = await planResponse.json(), reference = plan.direct;
  if (!reference?.startsWith('/') || reference.startsWith('//')) throw new Error('reference_source_shape');
  for (const variant of ['reference', 'compatible']) {
    const progress = await context.request.put(`${origin}/api/v1/items/${id}/progress`, {data: {seconds: resume, watched: false}});
    if (progress.status() !== 200) throw new Error('save_resume_status_' + progress.status());
    const page = await context.newPage();
    await page.addInitScript(() => document.addEventListener('playing', event => {
      if (event.target instanceof HTMLVideoElement && window.qaResumeFirstPlaying === undefined)
        window.qaResumeFirstPlaying = event.target.currentTime;
    }, true));
    if (directSwitch && !native) await page.addInitScript(() => {
      // Mac browsers can use native HLS before lazily loading Hls.js. Exercise
      // the same MSE fallback that browsers without native HLS actually use.
      const canPlayType = HTMLMediaElement.prototype.canPlayType;
      HTMLMediaElement.prototype.canPlayType = function(type) {
        return type === 'application/vnd.apple.mpegurl' ? '' : canPlayType.call(this, type);
      };
    });
    if (native) await page.addInitScript(() => {
      // Exercise the real native decoder with the unsupported-MSE policy.
      // This is a capability simulation, not physical Safari acceptance.
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
    });
    if (variant === 'reference') {
      // Only the reference document is synthetic. All original media and the
      // compatible watch page use the same authenticated real Server.
      await page.route(`${origin}/__qa_reference`, route => route.fulfill({contentType: 'text/html',
        body: `<video muted playsinline src="${reference.replaceAll('&', '&amp;').replaceAll('"', '&quot;')}"></video>`}));
      await page.goto(`${origin}/__qa_reference`);
    } else {
      await page.goto(`${origin}/watch/${id}${directSwitch ? '' : '?compatible=1'}`);
      if (directSwitch) {
        await page.waitForFunction(() => document.querySelector('video')?.readyState >= 2);
        const direct = await page.locator('video').evaluate(video => {
          video.pause();
          return {src: video.currentSrc, hls: video.dataset.hls, adaptive: video.dataset.adaptive};
        });
        if (direct.hls || !direct.adaptive?.startsWith('/hls/') || direct.src.startsWith('blob:'))
          throw new Error('direct_switch_requires_direct_first');
        stage = 'direct-switch-open-settings';
        await page.locator('[data-playback-mode-status]').click();
        stage = 'direct-switch-open-behavior';
        await page.locator('.player-behavior > summary').click();
        stage = 'direct-switch-select-compatible';
        await page.locator('video').evaluate((video, resume) => {
          video.pause(); video.currentTime = resume; window.qaResumeFirstPlaying = undefined;
        }, resume);
        await page.locator('[data-playback-mode] input[value="compatible"]').check();
        stage = 'direct-switch-close-settings';
        await page.locator('[data-player-settings-close]').click();
        stage = 'direct-switch-media';
        await page.waitForFunction(() => document.querySelector('video')?.currentSrc.startsWith('blob:'));
      }
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
    const row = await page.evaluate(async ({variant, duration, native, resume, frameRate}) => {
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
          // A seeked event can precede compositor presentation in WebKit.
          // Wait for the requested timestamp, never for an expected pixel hash.
          // Native HLS needs playback to deliver a timestamped frame callback.
          // Pause on the matching clock, independently of the expected pixels.
          const presented = new Promise((resolve, reject) => {
            let callback;
            const timer = setTimeout(() => {
              video.cancelVideoFrameCallback(callback);
              reject(new Error('frame_presentation_timeout'));
            }, 30000);
            const frame = (_, metadata) => {
              const decodedTime = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'currentTime').get.call(video);
              const presentedTime = metadata.mediaTime + video.currentTime - decodedTime;
              if (presentations.length < 64) presentations.push({point, mediaTime: metadata.mediaTime, presentedTime,
                currentTime: video.currentTime, decodedTime, paused: video.paused});
              if (presentedTime >= point - 1 / frameRate - .002 && presentedTime <= point + .002) {
                video.pause(); clearTimeout(timer); resolve({presentedTime, mediaTime: metadata.mediaTime});
              } else callback = video.requestVideoFrameCallback(frame);
            };
            callback = video.requestVideoFrameCallback(frame);
          });
          const [, frame] = await Promise.all([wait('seeked', () => {
            video.currentTime = point;
            if (native && variant === 'compatible') { video.muted = true; video.play().catch(() => {}); }
          }), presented]);
          video.pause();
          const canvas = document.createElement('canvas');
          canvas.width = video.videoWidth; canvas.height = video.videoHeight;
          const drawing = canvas.getContext('2d'); drawing.drawImage(video, 0, 0);
          const bytes = drawing.getImageData(0, 0, canvas.width, canvas.height).data;
          const digest = [...new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))]
            .map(value => value.toString(16).padStart(2, '0')).join('');
          samples.push({point, currentTime: video.currentTime, ...frame, digest});
        }
        video.muted = true;
        // Seeking to the final decoded frame can already set ended, in which
        // case play() correctly restarts at zero. Exercise actual tail playback.
        await wait('seeked', () => video.currentTime = Math.max(0, video.duration - .5));
        eofStart = {currentTime: video.currentTime, ended: video.ended};
        if (video.ended) throw new Error('eof_start_ended');
        await wait('ended', () => video.play());
        ended = video.ended;
      } catch (caught) { error = caught.message; }
      return {variant, error, ended, duration: video.duration, width: video.videoWidth, height: video.videoHeight,
        firstPlaying: window.qaResumeFirstPlaying, eofStart,
        start: Number(video.dataset.start), mode: video.dataset.compatibilityMode,
        adapter: video.currentSrc.startsWith('blob:') ? 'mse' : 'native', currentTime: video.currentTime, samples, presentations};
    }, {variant, duration, native, resume, frameRate});
    rows.push(row);
    await page.screenshot({path: output.replace('.json', `-${variant}.png`)});
    await page.close();
  }
} catch (error) {
  failureClass = error.name === 'TimeoutError' ? 'browser_action_timeout' : error.message;
  await context.pages().at(-1)?.screenshot({path: output.replace('.json', '-failure.png')});
}
finally {
  await context.close();
  await browser.close();
}
const matches = (rows[0]?.samples || []).map((sample, index) => sample.digest === rows[1]?.samples[index]?.digest);
const passed = !failureClass && rows.length === 2 && !rows.some(row => row.error || !row.ended)
  && matches.length === 7 && matches.every(Boolean) && expectedMode.split(',').includes(rows[1].mode)
  && Math.abs(rows[1].duration - rows[0].duration) < .1
  && rows[1].firstPlaying >= resume - .05 && rows[1].firstPlaying < resume + .5
  && rows[1].adapter === (native ? 'native' : 'mse')
  && responses.length > 0 && responses.every(response => response.status < 400);
await writeFile(output, JSON.stringify({engine, browserVersion: browser.version(),
  capabilitySimulation: [native ? 'MSE disabled to exercise native WebKit HLS policy' : '', directSwitch ? 'Native HLS support disabled to exercise lazy MSE fallback' : '', forceConversion ? 'Chrome user agent forces server AAC conversion policy' : ''].filter(Boolean).join('; ') || null,
  proofClass: 'Real authenticated Server process; synthetic source; unmodified watch page and HLS; original-file or independently certified stream-copy MP4 reference',
  directSwitch, stage, passed, failureClass, rows, matches, responses}, null, 2) + '\n');
console.log(JSON.stringify({engine, passed, failureClass, matched: matches.filter(Boolean).length, total: matches.length}));
process.exitCode = passed ? 0 : 1;
