import {createRequire} from 'node:module';
import {join} from 'node:path';
import {writeFile} from 'node:fs/promises';

const root = process.cwd(), require = createRequire(join(root, 'apps/player/e2e/package.json'));
const {chromium, webkit} = require('@playwright/test');
const origin = process.env.QA_RESUME_ORIGIN, token = process.env.QA_RESUME_TOKEN;
const id = process.env.QA_RESUME_ITEM, output = process.env.QA_RESUME_OUTPUT;
const engine = process.env.QA_RESUME_ENGINE || 'chromium';
const native = engine === 'webkit-native';
const expectedMode = process.env.QA_RESUME_EXPECTED_MODE || 'remux';
const duration = Number(process.env.QA_RESUME_DURATION || 32);
const channel = process.env.PLAYWRIGHT_CHANNEL || (!process.env.CI && process.platform === 'darwin' ? 'chrome' : undefined);
const browser = await (engine.startsWith('webkit') ? webkit.launch() : chromium.launch({channel}));
const context = await browser.newContext({extraHTTPHeaders: {Authorization: `Bearer ${token}`}});
const rows = [], responses = [];
let failureClass;

try {
  const planResponse = await context.request.get(`${origin}/api/v1/items/${id}/playback?videoCodecs=h264&audioCodecs=aac`);
  if (planResponse.status() !== 200) throw new Error('playback_plan_status');
  const plan = await planResponse.json(), reference = plan.direct;
  if (!reference?.startsWith('/') || reference.startsWith('//')) throw new Error('reference_source_shape');
  for (const variant of ['reference', 'compatible']) {
    const progress = await context.request.put(`${origin}/api/v1/items/${id}/progress`, {data: {seconds: 12.5, watched: false}});
    if (progress.status() !== 200) throw new Error('save_resume_status_' + progress.status());
    const page = await context.newPage();
    await page.addInitScript(() => document.addEventListener('playing', event => {
      if (event.target instanceof HTMLVideoElement && window.qaResumeFirstPlaying === undefined)
        window.qaResumeFirstPlaying = event.target.currentTime;
    }, true));
    if (native) await page.addInitScript(() => {
      // Exercise the real native decoder with the unsupported-MSE policy.
      // This is a capability simulation, not physical Safari acceptance.
      for (const name of ['MediaSource', 'ManagedMediaSource', 'WebKitMediaSource'])
        Object.defineProperty(window, name, {configurable: true, value: undefined});
    });
    page.on('response', response => {
      const path = new URL(response.url()).pathname;
      if (path.startsWith('/hls/')) responses.push({
        kind: path.endsWith('.m3u8') ? 'playlist' : path.endsWith('.m4s') ? 'fragment' : 'initialization',
        status: response.status(), offset: path.match(/-o(\d+)\//)?.[1],
        segment: path.match(/\/(segment-\d{5}\.m4s)$/)?.[1],
        origin: response.headers()['x-kinosail-hls-origin'],
      });
    });
    if (variant === 'reference') {
      // Only the reference document is synthetic. All original media and the
      // compatible watch page use the same authenticated real Server.
      await page.route(`${origin}/__qa_reference`, route => route.fulfill({contentType: 'text/html',
        body: `<video muted playsinline src="${reference.replaceAll('&', '&amp;').replaceAll('"', '&quot;')}"></video>`}));
      await page.goto(`${origin}/__qa_reference`);
    } else await page.goto(`${origin}/watch/${id}?compatible=1`);
    if (variant === 'compatible') {
      await page.waitForFunction(() => document.querySelector('video')?.readyState >= 2);
      const stage = page.locator('.media-stage');
      if (await page.locator('video').evaluate(video => video.paused)) await stage.press('k');
      await page.waitForFunction(() => !document.querySelector('video').paused);
      await stage.press('k');
      await page.waitForFunction(() => document.querySelector('video').paused);
    }
    const row = await page.evaluate(async ({variant, duration, native}) => {
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
      let error;
      try {
        if (video.readyState < 2) await wait('loadeddata');
        video.pause();
        for (const point of [12.55, 13.05, 17.55, 21.55, 25.55, 29.55, Math.min(31.55, duration - .05)]) {
          // A seeked event can precede compositor presentation in WebKit.
          // Wait for the requested timestamp, never for an expected pixel hash.
          // Native WebKit HLS does not emit frame callbacks for paused seeks.
          // Its seeked frame is still compared byte-for-byte with the original.
          const nativePaused = native && variant === 'compatible';
          const presented = nativePaused ? Promise.resolve({observation: 'native-seeked'}) : new Promise((resolve, reject) => {
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
              if (presentedTime >= point - 1 / 24 - .002 && presentedTime <= point + .002) {
                clearTimeout(timer); resolve({presentedTime, mediaTime: metadata.mediaTime});
              } else callback = video.requestVideoFrameCallback(frame);
            };
            callback = video.requestVideoFrameCallback(frame);
          });
          const [, frame] = await Promise.all([wait('seeked', () => video.currentTime = point), presented]);
          video.pause();
          if (nativePaused) await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
          const canvas = document.createElement('canvas');
          canvas.width = video.videoWidth; canvas.height = video.videoHeight;
          const drawing = canvas.getContext('2d'); drawing.drawImage(video, 0, 0);
          const bytes = drawing.getImageData(0, 0, canvas.width, canvas.height).data;
          const digest = [...new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))]
            .map(value => value.toString(16).padStart(2, '0')).join('');
          samples.push({point, currentTime: video.currentTime, ...frame, digest});
        }
      } catch (caught) { error = caught.message; }
      return {variant, error, duration: video.duration, width: video.videoWidth, height: video.videoHeight,
        firstPlaying: window.qaResumeFirstPlaying,
        start: Number(video.dataset.start), mode: video.dataset.compatibilityMode,
        adapter: video.currentSrc.startsWith('blob:') ? 'mse' : 'native', currentTime: video.currentTime, samples, presentations};
    }, {variant, duration, native});
    rows.push(row);
    await page.screenshot({path: output.replace('.json', `-${variant}.png`)});
    await page.close();
  }
} catch (error) { failureClass = error.message; }
finally {
  await context.close();
  await browser.close();
}
const matches = (rows[0]?.samples || []).map((sample, index) => sample.digest === rows[1]?.samples[index]?.digest);
const passed = !failureClass && rows.length === 2 && !rows.some(row => row.error)
  && matches.length === 7 && matches.every(Boolean) && expectedMode.split(',').includes(rows[1].mode)
  && Math.abs(rows[1].duration - rows[0].duration) < .1
  && rows[1].firstPlaying >= 12.45 && rows[1].firstPlaying < 13
  && (!native || rows[1].adapter === 'native')
  && responses.length > 0 && responses.every(response => response.status < 400);
await writeFile(output, JSON.stringify({engine, browserVersion: browser.version(),
  capabilitySimulation: native ? 'MSE disabled to exercise native WebKit HLS policy' : null,
  proofClass: 'Real authenticated Player process; synthetic source; unmodified watch page and HLS; original-file reference',
  passed, failureClass, rows, matches, responses}, null, 2) + '\n');
console.log(JSON.stringify({engine, passed, failureClass, matched: matches.filter(Boolean).length, total: matches.length}));
process.exitCode = passed ? 0 : 1;
