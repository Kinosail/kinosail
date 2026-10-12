import {createRequire} from 'node:module';
import {join} from 'node:path';
import {writeFile} from 'node:fs/promises';

const require = createRequire(join(process.cwd(), 'apps/player/e2e/package.json'));
const {chromium, webkit} = require('@playwright/test');
const origin = process.env.QA_RESUME_ORIGIN, token = process.env.QA_RESUME_TOKEN;
const id = process.env.QA_RESUME_ITEM, output = process.env.QA_RESUME_OUTPUT;
const engine = process.env.QA_RESUME_ENGINE || 'chromium', native = engine === 'webkit-native';
const channel = process.env.PLAYWRIGHT_CHANNEL || (!process.env.CI && process.platform === 'darwin' ? 'chrome' : undefined);
const browser = await (engine.startsWith('webkit') ? webkit.launch() : chromium.launch({channel}));
const cases = [];
const check = (condition, failure) => { if (!condition) throw new Error(failure); };
try {
  for (const [fault, width] of [['invalid-origin', 390], ['future-origin', 1920], ...(native ? [['http-503', 1440]] : [])]) {
    const context = await browser.newContext({extraHTTPHeaders: {Authorization: `Bearer ${token}`}});
    const progress = await context.request.put(`${origin}/api/v1/items/${id}/progress`, {data: {seconds: 12.5, watched: false}});
    check(progress.status() === 200, 'save_resume_status');
    const page = await context.newPage();
    await page.setViewportSize({width, height: 900});
    if (native) await page.addInitScript(() => {
      for (const name of ['MediaSource', 'ManagedMediaSource', 'WebKitMediaSource'])
        Object.defineProperty(window, name, {configurable: true, value: undefined});
    });
    const row = {fault, width, requests: []};
    let phase = 'failure', injected = false;
    page.on('request', request => {
      const path = new URL(request.url()).pathname;
      if (path.startsWith('/hls/')) row.requests.push({phase, method: request.method(),
        kind: path.endsWith('.m3u8') ? 'playlist' : path.endsWith('.m4s') ? 'fragment' : 'initialization',
        offset: Number(path.match(/-o(\d+)\//)?.[1] || 0)});
    });
    await page.route(`${origin}/hls/**`, async route => {
      const request = route.request(), path = new URL(request.url()).pathname;
      if (injected || !path.endsWith('-o12500/index.m3u8')) return route.continue();
      injected = true;
      if (fault === 'http-503') return route.fulfill({status: 503, body: ''});
      const response = await route.fetch();
      await route.fulfill({response, headers: {...response.headers(),
        'x-kinosail-hls-origin': fault === 'invalid-origin' ? 'invalid' : '99.000000'}});
    });
    try {
      await page.goto(`${origin}/watch/${id}?compatible=1`);
      const retry = page.locator('[data-player-fallback]:visible').first();
      await retry.waitFor({state: 'visible', timeout: 30000});
      row.failureUI = await page.locator('[data-player-status]').evaluate(status => ({
        busy: status.getAttribute('aria-busy'), recovery: status.classList.contains('is-recovery'),
        skeletonHidden: !status.querySelector('.buffer-skeleton') || status.querySelector('.buffer-skeleton').hidden,
      }));
      row.failureUI.mode = await page.locator('[data-playback-mode-status]').textContent();
      check(injected && row.failureUI.busy === 'false' && row.failureUI.recovery && row.failureUI.skeletonHidden, 'failure_ui_not_settled');
      check(row.failureUI.mode === 'Playback interrupted', 'failure_misidentified_playback_mode');
      check(row.requests.every(request => request.kind === 'playlist'), 'media_requested_before_origin_accepted');
      await page.screenshot({path: output.replace('.json', `-${fault}-failed.png`)});
      phase = 'retry';
      await retry.click();
      await page.waitForFunction(() => {
        const video = document.querySelector('video');
        return video?.readyState >= 2 && !video.seeking && !video.paused;
      }, null, {timeout: 30000});
      row.recovered = await page.locator('video').evaluate(video => ({time: video.currentTime,
        duration: video.duration, width: video.videoWidth, height: video.videoHeight,
        adapter: video.currentSrc.startsWith('blob:') ? 'mse' : 'native'}));
      check(row.recovered.time >= 12.45 && row.recovered.time < 14, 'retry_lost_resume_position');
      check(row.recovered.width === 640 && row.recovered.height === 360, 'retry_has_no_decoded_picture');
      check(row.requests.some(request => request.phase === 'retry' && request.offset === 12500), 'retry_changed_stream_origin');
      await page.waitForFunction(() => !document.querySelector('[data-player-status]').classList.contains('is-recovery'));
      await page.screenshot({path: output.replace('.json', `-${fault}-loaded.png`)});
      row.passed = true;
    } catch (error) {
      row.passed = false;
      row.failureClass = /^[a-z_]+$/.test(error.message) ? error.message : error.name;
    } finally {
      cases.push(row);
      await page.close();
      await context.close();
    }
  }
  if (native) for (const action of ['seek', 'original']) {
    const context = await browser.newContext({extraHTTPHeaders: {Authorization: `Bearer ${token}`}});
    const progress = await context.request.put(`${origin}/api/v1/items/${id}/progress`, {data: {seconds: 12.5, watched: false}});
    check(progress.status() === 200, 'save_pending_resume_status');
    const page = await context.newPage();
    await page.addInitScript(() => {
      for (const name of ['MediaSource', 'ManagedMediaSource', 'WebKitMediaSource'])
        Object.defineProperty(window, name, {configurable: true, value: undefined});
    });
    const row = {action, requests: []};
    let release, entered, held = false;
    const gate = new Promise(resolve => release = resolve);
    const intercepted = new Promise(resolve => entered = resolve);
    page.on('request', request => {
      const path = new URL(request.url()).pathname;
      if (path.startsWith('/hls/')) row.requests.push({method: request.method(),
        kind: path.endsWith('.m3u8') ? 'playlist' : 'media', offset: Number(path.match(/-o(\d+)\//)?.[1] || 0)});
    });
    await page.route(`${origin}/hls/**`, async route => {
      if (!held && route.request().method() === 'HEAD') {
        held = true; entered(); await gate;
      }
      await route.continue().catch(() => { row.canceledDelivery = true; });
    });
    try {
      await page.goto(`${origin}/watch/${id}?compatible=1`);
      let deadline;
      try {
        await Promise.race([intercepted, new Promise((_, reject) => {
          deadline = setTimeout(() => reject(new Error('metadata_request_not_held')), 30000);
        })]);
      } finally { clearTimeout(deadline); }
      await page.screenshot({path: output.replace('.json', `-${action}-pending.png`)});
      if (action === 'seek') await page.locator('.media-stage').press('ArrowRight');
      else {
        row.step = 'open-playback-options';
        await page.locator('.more-player-actions > summary').click();
        row.step = 'select-original';
        await page.locator('.more-player-actions').getByRole('link', {name: /Direct Play/}).click();
      }
      row.step = 'settle-media';
      release();
      await page.waitForFunction(() => {
        const video = document.querySelector('video');
        return video?.readyState >= 2 && !video.seeking;
      }, null, {timeout: 30000});
      row.recovered = await page.locator('video').evaluate(video => ({time: video.currentTime,
        direct: !new URL(video.currentSrc).pathname.startsWith('/hls/'), width: video.videoWidth}));
      check(row.recovered.width === 640, 'pending_action_has_no_picture');
      check(!row.requests.some(request => request.method === 'GET' && request.offset === 12500), 'superseded_metadata_loaded_old_source');
      if (action === 'seek') {
        check(row.requests.some(request => request.method === 'GET' && request.offset === 10000), 'pending_seek_not_retained');
        check(row.recovered.time >= 9.95 && row.recovered.time < 11, 'pending_seek_wrong_clock');
      } else check(row.recovered.direct, 'original_did_not_cancel_metadata');
      check(await page.locator('[data-player-fallback]:visible').count() === 0, 'canceled_metadata_showed_failure');
      await page.screenshot({path: output.replace('.json', `-${action}-loaded.png`)});
      row.passed = true;
    } catch (error) {
      row.passed = false;
      row.failureClass = /^[a-z_]+$/.test(error.message) ? error.message : error.name;
      await page.screenshot({path: output.replace('.json', `-${action}-failure.png`)});
    } finally {
      release(); cases.push(row);
      await page.close(); await context.close();
    }
  }
} finally {
  await browser.close();
}
const passed = cases.length === (native ? 5 : 2) && cases.every(row => row.passed);
await writeFile(output, JSON.stringify({engine, browserVersion: browser.version(), passed,
  proofClass: 'Real authenticated Player with explicitly injected HTTP metadata failures; synthetic media; actual recovery controls',
  capabilitySimulation: native ? 'MSE disabled to exercise native WebKit HLS policy' : null, cases}, null, 2) + '\n');
console.log(JSON.stringify({engine, passed, cases: cases.map(({fault, passed, failureClass}) => ({fault, passed, failureClass}))}));
process.exitCode = passed ? 0 : 1;
