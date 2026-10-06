// Complete actual Server/watch-page observation. No mocked media or decoder APIs.
import {chromium} from '@playwright/test';
import {createHash} from 'node:crypto';
import {readFileSync, writeFileSync, renameSync, existsSync} from 'node:fs';

const target = process.argv[2];
const bytes = readFileSync(0);
if (bytes.length > 4096 || process.argv.length !== 3) throw new Error('renderer_input_bound');
const input = JSON.parse(bytes);
if (!/^http:\/\/localhost:[1-9][0-9]{0,4}$/.test(input.url) ||
    !/^[a-zA-Z0-9_.-]{16,2048}$/.test(input.token) ||
    ![input.itemID, input.referenceID].every(id => /^[a-f0-9]{16}$/.test(id)) ||
    !/^\/hls\/[a-f0-9]{16}\/p\/r-[a-zA-Z0-9-]+\/index\.m3u8$/.test(input.hls)) {
  throw new Error('renderer_input_shape');
}
const deadline = Date.now() + 240_000;
const result = {playbackRate: 0.25, reference: {}, public: {}};
const save = () => writeFileSync(target, `${JSON.stringify(result)}\n`, {mode: 0o600});
save();
let browserServer;
const stop = async failureClass => {
  result.failureClass = failureClass;
  save();
  if (browserServer) await browserServer.kill();
  process.exit(1);
};
const watchdog = setTimeout(() => {void stop('renderer_deadline');}, 245_000);
process.once('SIGTERM', () => {void stop('renderer_terminated');});
browserServer = await chromium.launchServer({host: '127.0.0.1', port: 0, headless: true, timeout: 15_000,
  handleSIGTERM: false, args: ['--autoplay-policy=no-user-gesture-required',
  '--disable-background-timer-throttling', '--disable-renderer-backgrounding']});
const owner = `${target}.owner`;
writeFileSync(`${owner}.tmp`, JSON.stringify({browserPID: browserServer.process().pid}), {mode: 0o600});
renameSync(`${owner}.tmp`, owner);
const ownershipDeadline = Date.now() + 5_000;
while (!existsSync(`${owner}.ack`)) {
  if (Date.now() >= ownershipDeadline) await stop('renderer_owner_unverified');
  await new Promise(resolve => setTimeout(resolve, 25));
}
const browser = await chromium.connect(browserServer.wsEndpoint(), {timeout: 15_000});
result.browserVersion = browser.version();

async function capture(id, compatible) {
  const context = await browser.newContext({viewport: {width: 1280, height: 800}, deviceScaleFactor: 1});
  // Synthetic authorization reaches only the disposable Server, including redirects.
  await context.route('**/*', route => {
    if (new URL(route.request().url()).origin !== input.url) return route.abort();
    return route.continue({headers: {...route.request().headers(), Authorization: `Bearer ${input.token}`}});
  });
  const page = await context.newPage();
  const network = {master: 0, variant: 0, initializationSHA256s: [], successfulFragments: 0,
    unexpectedMediaRequests: 0, failedMediaResponses: 0};
  const successful = new Set();
  const pending = [];
  const base = input.hls.replace(/index\.m3u8$/, '');
  page.on('response', response => {
    const path = new URL(response.url()).pathname;
    if (!compatible || !path.startsWith('/hls/') && !path.startsWith('/media/')) return;
    const relative = path.startsWith(base) ? path.slice(base.length) : '';
    if (path === input.hls) network.master = response.status();
    else if (/^[0-9]{3,4}p\/index\.m3u8$/.test(relative)) network.variant = response.status();
    else if (/^[0-9]{3,4}p\/init\.mp4$/.test(relative)) {
      pending.push(response.body().then(body => {
        if (body.length > 1024 * 1024) throw new Error('renderer_init_bound');
        if (network.initializationSHA256s.length >= 32) throw new Error('renderer_init_count_bound');
        network.initializationSHA256s.push(createHash('sha256').update(body).digest('hex'));
      }).catch(() => {network.failedMediaResponses++;}));
    } else if (/^[0-9]{3,4}p\/segment-[0-9]{5}\.m4s$/.test(relative)) {
      if (response.status() === 200) successful.add(relative);
    } else network.unexpectedMediaRequests++;
    if (response.status() !== 200) network.failedMediaResponses++;
  });
  await page.addInitScript(() => {
    const proof = window.__hlsProof = {rows: [], ended: false, errorCode: 0, captureErrors: 0,
      width: 0, height: 0, nativeTime: null, reportedTime: null, duration: null,
      buffered: [], events: [], hashes: [], videoPlaybackQuality: null};
    const nativeTime = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'currentTime').get;
    const nativeDuration = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'duration').get;
    const attach = media => {
      if (!(media instanceof HTMLVideoElement) || media.dataset.proofAttached) return;
      media.dataset.proofAttached = '1';
      media.muted = true;
      media.playbackRate = 0.25;
      const canvas = document.createElement('canvas');
      const pixels = canvas.getContext('2d', {willReadFrequently: true});
      const record = (_, metadata) => {
        if (proof.rows.length >= 4096) {proof.captureErrors++; media.pause(); return;}
        const row = [metadata.mediaTime, metadata.presentedFrames, ''];
        proof.rows.push(row);
        try {
          canvas.width = proof.width = media.videoWidth;
          canvas.height = proof.height = media.videoHeight;
          if (canvas.width !== 640 || canvas.height !== 360) throw new Error('renderer_dimensions');
          pixels.drawImage(media, 0, 0);
          const buffer = pixels.getImageData(0, 0, canvas.width, canvas.height).data;
          proof.hashes.push(crypto.subtle.digest('SHA-256', buffer).then(hash => {
            row[2] = [...new Uint8Array(hash)].map(byte => byte.toString(16).padStart(2, '0')).join('');
          }).catch(() => {proof.captureErrors++;}));
        } catch {proof.captureErrors++;}
        media.requestVideoFrameCallback(record);
      };
      media.requestVideoFrameCallback(record);
      for (const name of ['loadedmetadata', 'playing', 'waiting', 'seeking', 'seeked', 'ended', 'error']) {
        media.addEventListener(name, () => {
          if (proof.events.length < 128) proof.events.push([name, nativeTime.call(media), media.currentTime]);
          if (name === 'ended') proof.ended = true;
          if (name === 'error') proof.errorCode = media.error?.code || 0;
        });
      }
      const sample = () => {
        proof.nativeTime = nativeTime.call(media);
        proof.reportedTime = media.currentTime;
        const duration = nativeDuration.call(media);
        proof.duration = Number.isFinite(duration) ? duration : null;
        proof.buffered = Array.from({length: Math.min(media.buffered.length, 16)}, (_, n) =>
          [media.buffered.start(n), media.buffered.end(n)]);
        const quality = media.getVideoPlaybackQuality?.();
        proof.videoPlaybackQuality = quality ? {total: quality.totalVideoFrames,
          dropped: quality.droppedVideoFrames, corrupted: quality.corruptedVideoFrames ?? null} : null;
      };
      media.addEventListener('timeupdate', sample);
      media.addEventListener('ended', sample);
    };
    new MutationObserver(() => document.querySelectorAll('video').forEach(attach))
      .observe(document, {childList: true, subtree: true});
    document.addEventListener('DOMContentLoaded', () => document.querySelectorAll('video').forEach(attach));
  });
  let failureClass;
  try {
    const timeout = () => Math.max(1, deadline - Date.now());
    const response = await page.goto(`${input.url}/watch/${id}?${compatible ? 'compatible' : 'direct'}=1`,
      {waitUntil: 'domcontentloaded', timeout: Math.min(15_000, timeout())});
    if (response.status() !== 200) throw new Error('renderer_watch_status');
    await page.waitForFunction(() => document.querySelector('video')?.readyState >= 2,
      null, {timeout: Math.min(20_000, timeout())});
    await page.locator('video').evaluate(media => media.play());
    await page.waitForFunction(() => window.__hlsProof.ended || window.__hlsProof.errorCode,
      null, {timeout: timeout()});
  } catch {failureClass = Date.now() >= deadline - 1000 ? 'renderer_deadline' : 'renderer_watch_failed';}
  let facts = {};
  try {
    facts = await page.evaluate(async () => {
      const proof = window.__hlsProof;
      await Promise.all(proof.hashes);
      const {hashes, ...safe} = proof;
      return safe;
    });
  } catch {failureClass = 'renderer_observation_unavailable';}
  await Promise.all(pending);
  network.successfulFragments = successful.size;
  await context.close();
  return {...facts, network, ...(failureClass ? {failureClass} : {})};
}

try {
  result.reference = await capture(input.referenceID, false);
  save();
  result.public = await capture(input.itemID, true);
  save();
} finally {
  save();
  await browser.close();
  await browserServer.kill();
  clearTimeout(watchdog);
}
