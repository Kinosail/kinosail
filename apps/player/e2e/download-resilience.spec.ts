import {expect, test, type Page} from '@playwright/test';
import {downloadsSource} from './static-sources';

async function setup(page: Page) {
  await page.route('https://offline.test/', route => route.fulfill({contentType: 'text/html', body: '<body data-viewer-profile="viewer"></body>'}));
  await page.goto('https://offline.test/');
  await page.evaluate(() => {
    const active = {state: 'activated', scriptURL: new URL('/service-worker.js?v=43', location.href).href};
    Object.defineProperty(navigator, 'serviceWorker', {configurable: true, value: {controller: active, getRegistration: async () => ({active})}});
  });
  await page.addScriptTag({content: downloadsSource});
}

test('admission retains concurrent progress and keeps the cached connection usable', async ({page}) => {
  await setup(page);
  const result = await page.evaluate(`(async () => {
    const progress = {seconds: 10, watched: false, session: 'old', revision: 1};
    const record = {id: 'aaaaaaaaaaaaaaaa', itemID: 'bbbbbbbbbbbbbbbb', profileID: 'viewer', sha256: 'a'.repeat(64), transferID: '1'.repeat(32), state: 'ready', readyOffline: true, progressBaseline: progress};
    await saveOfflineJob(record);
    const detached = {...record, transferID: '2'.repeat(32), state: 'transferring', readyOffline: false};
    await updateOfflineProgress(record, current => { current.pendingProgress = {...progress, seconds: 42, revision: 2}; });
    await beginOfflineTransfer(detached, record.transferID);
    const saved = await getOfflineJob(record.id);
    let rejected = false;
    try { await updateOfflineProgress(record, current => { current.pendingProgress.seconds = 99; }); } catch { rejected = true; }
    return {seconds: saved.pendingProgress.seconds, revision: saved.pendingProgress.revision, rejected, count: (await getOfflineJobs()).length};
  })()`);
  expect(result).toEqual({seconds: 42, revision: 2, rejected: true, count: 1});
});

test('removal cancels the owner before waiting for its lock and preserves a newer transfer', async ({page}) => {
  await setup(page);
  const result = await page.evaluate(`(async () => {
    const id = 'aaaaaaaaaaaaaaaa', transferID = '1'.repeat(32);
    const owner = {transferID, startedAt: offlineNow(), controller: new AbortController()};
    offlineTransfers.set(id, owner);
    await saveOfflineJob({id, storage: 'indexeddb', state: 'transferring', transferID, transferStartedAt: owner.startedAt});
    let entered;
    const ready = new Promise(resolve => entered = resolve);
    const held = withOfflineJobLock(id, async () => { entered(); await new Promise(resolve => owner.controller.signal.addEventListener('abort', resolve, {once: true})); });
    await ready;
    const removed = await removeOfflineJobSafely(id, true);
    await held;
    const absent = !await getOfflineJob(id);
    offlineSourceTransfers.set(id, transferID);
    await saveOfflineJob({id, storage: 'indexeddb', state: 'ready', readyOffline: true, transferID: '2'.repeat(32), transferStartedAt: offlineNow()});
    const staleRemoval = await removeOfflineJobSafely(id);
    return {removed, absent, aborted: owner.controller.signal.aborted, staleRemoval, retained: Boolean(await getOfflineJob(id))};
  })()`);
  expect(result).toEqual({removed: true, absent: true, aborted: true, staleRemoval: false, retained: true});
});

test('the first available shared slot starts queued work and queued cancellation is immediate', async ({page}) => {
  await setup(page);
  const result = await page.evaluate(`(async () => {
    const releases = [], entered = [];
    const held = [0, 1].map(index => navigator.locks.request('kinosail-offline-slot:' + index, () => new Promise(resolve => { releases[index] = resolve; entered[index]?.(); })));
    await Promise.all([0, 1].map(index => releases[index] ? Promise.resolve() : new Promise(resolve => entered[index] = resolve)));
    let started = false;
    const next = withOfflineTransferSlot(async () => { started = true; return 42; }, new AbortController().signal);
    releases[1]();
    const value = await next;
    const cancelled = new AbortController(); cancelled.abort();
    let invoked = false, rejected = false;
    try { await withOfflineTransferSlot(() => { invoked = true; }, cancelled.signal); } catch { rejected = true; }
    releases[0](); await Promise.all(held);
    return {started, value, invoked, rejected};
  })()`);
  expect(result).toEqual({started: true, value: 42, invoked: false, rejected: true});
});

test('chunk cancellation aborts an active body without retrying', async ({page}) => {
  await setup(page);
  const result = await page.evaluate(`(async () => {
    let calls = 0, bodyCancelled = false;
    const controller = new AbortController();
    const previous = window.fetch;
    window.fetch = async (_url, options) => {
      calls++;
      const body = new ReadableStream({start(stream) { options.signal.addEventListener('abort', () => { bodyCancelled = true; stream.error(new DOMException('cancelled', 'AbortError')); }); }});
      queueMicrotask(() => controller.abort());
      return new Response(body, {status: 206, headers: {'Content-Range': 'bytes 0-3/4'}});
    };
    let rejected = false;
    try { await fetchOfflineChunk({id: 'aaaaaaaaaaaaaaaa', size: 4}, 0, 4, controller.signal); } catch { rejected = true; }
    finally { window.fetch = previous; }
    return {calls, bodyCancelled, rejected};
  })()`);
  expect(result).toEqual({calls: 1, bodyCancelled: true, rejected: true});
});

test('malformed manifests and oversized JSON are rejected before transfer effects', async ({page}) => {
  await setup(page);
  const result = await page.evaluate(`(async () => {
    const expected = {jobID: 'aaaaaaaaaaaaaaaa', itemID: 'bbbbbbbbbbbbbbbb', profileID: 'viewer', title: 'Movie', quality: '720p'};
    const valid = {id: expected.jobID, itemId: expected.itemID, profileId: expected.profileID, title: expected.title, quality: expected.quality, extension: '.mp4', state: 'ready', readyOffline: true, sha256: 'a'.repeat(64), size: 4};
    const invalid = [{size: 0}, {size: 128 * 1024 ** 3 + 1}, {size: 1.5}, {profileId: 'other'}, {itemId: '../x'}, {quality: 'unknown'}, {sha256: ''}];
    const rejected = invalid.every(change => !validOfflineManifest({...valid, ...change}, expected));
    let oversized = false;
    try { await offlineProgressJSON(new Response('x'.repeat(65537))); } catch { oversized = true; }
    return {rejected, oversized, count: (await getOfflineJobs()).length};
  })()`);
  expect(result).toEqual({rejected: true, oversized: true, count: 0});
});

for (const quality of ['720p', 'compatible']) test(`${quality} storage verifies multiple chunks, resumes without network, and repairs corruption`, async ({page}) => {
  await setup(page);
  const result = await page.evaluate(`(async () => {
    const bytes = new Uint8Array(chunkSize + 17); bytes.fill(73);
    const job = {id: 'aaaaaaaaaaaaaaaa', itemId: 'bbbbbbbbbbbbbbbb', profileId: 'viewer', title: 'Movie', quality: '${quality}', extension: '.mp4', state: 'ready', readyOffline: true, sha256: await offlineDigest(bytes), size: bytes.length};
    const expected = {jobID: job.id, itemID: job.itemId, profileID: job.profileId, title: job.title, quality: job.quality};
    const previous = window.fetch;
    let ranges = 0;
    window.fetch = async (url, options) => {
      if (String(url).includes('/api/v1/items/')) return Response.json({profileId: 'viewer', item: {id: job.itemId, progress: {seconds: 0, watched: false, revision: 0, session: ''}}});
      const [, start, end] = options.headers.Range.match(/^bytes=(\\d+)-(\\d+)$/);
      const body = bytes.slice(Number(start), Number(end) + 1);
      const hash = new Uint8Array(await crypto.subtle.digest('SHA-256', body));
      ranges++;
      return new Response(body, {status: 206, headers: {'Content-Range': 'bytes ' + start + '-' + end + '/' + job.size, 'Content-Digest': 'sha-256=:' + btoa(String.fromCharCode(...hash)) + ':'}});
    };
    try {
      const transfer = () => withOfflineJobLock(job.id, () => withOfflineTransferSlot(() => transferOfflineJob(job, expected), new AbortController().signal));
      const first = await transfer();
      const initialRanges = ranges;
      await transfer();
      const resumedRanges = ranges;
      if (first.storage === 'opfs') await writeOfflineFile(job.id, 0, new Uint8Array([0]).buffer);
      else { const chunk = await getOfflineChunk(job.id + ':0'); new Uint8Array(chunk.data)[0] = 0; await saveOfflineChunk(chunk); }
      const repaired = await transfer();
      return {initialRanges, resumedRanges, repairedRanges: ranges, state: repaired.state, bytes: repaired.bytes, ready: repaired.readyOffline, playbackVerified: Boolean(repaired.playbackAgent)};
    } finally { window.fetch = previous; }
  })()`);
  expect(result).toEqual({initialRanges: 2, resumedRanges: 2, repairedRanges: 3, state: 'ready', bytes: 8 * 1024 * 1024 + 17, ready: true, playbackVerified: false});
});

test('an unavailable file writer preserves existing downloads and allows new IndexedDB transfers', async ({page}) => {
  await setup(page);
  const result = await page.evaluate(`(async () => {
    const bytes = new Uint8Array([1, 2, 3, 4]);
    const job = {id: 'aaaaaaaaaaaaaaaa', itemId: 'bbbbbbbbbbbbbbbb', profileId: 'viewer', title: 'Movie', quality: '720p', extension: '.mp4', state: 'ready', readyOffline: true, sha256: await offlineDigest(bytes), size: bytes.length};
    const expected = {jobID: job.id, itemID: job.itemId, profileID: job.profileId, title: job.title, quality: job.quality};
    const record = {id: job.id, integrityVersion: offlineIntegrityVersion, profileID: job.profileId, itemID: job.itemId, sha256: job.sha256, size: job.size, storage: 'opfs', state: 'ready', readyOffline: true};
    await saveOfflineJob(record);
    openOfflineWriter = async () => { throw new Error('file storage unavailable'); };
    offlineFileSize = async () => 4;
    let requests = 0;
    window.fetch = async (url) => {
      requests++;
      if (String(url).includes('/api/v1/items/')) return Response.json({profileId: 'viewer', item: {id: job.itemId, progress: {seconds: 0, watched: false, revision: 0, session: ''}}});
      const hash = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes));
      return new Response(bytes, {status: 206, headers: {'Content-Range': 'bytes 0-3/4', 'Content-Digest': 'sha-256=:' + btoa(String.fromCharCode(...hash)) + ':'}});
    };
    let rejected = false;
    try { await transferOfflineJob(job, expected); } catch { rejected = true; }
    const retained = JSON.stringify(await getOfflineJob(job.id)) === JSON.stringify(record);
    const rejectedRequests = requests;
    job.id = expected.jobID = "cccccccccccccccc";
    const saved = await transferOfflineJob(job, expected);
    return {rejected, retained, rejectedRequests, storage: saved.storage, ready: saved.readyOffline, bytes: saved.bytes};
  })()`);
  expect(result).toEqual({rejected: true, retained: true, rejectedRequests: 0, storage: 'indexeddb', ready: true, bytes: 4});
});

for (const permission of ['pending', 'denied', 'rejected', 'granted']) {
  test(`offline download proceeds when storage persistence is ${permission}`, async ({page}) => {
    await setup(page);
    const result = await page.evaluate(async (permission) => {
      Object.defineProperty(navigator.storage, 'persist', {configurable: true, value: () => permission === 'pending' ? new Promise(() => {}) : permission === 'rejected' ? Promise.reject(new Error('unavailable')) : Promise.resolve(permission === 'granted')});
      return eval(`(async () => {
        const job = {id: 'aaaaaaaaaaaaaaaa', itemId: 'bbbbbbbbbbbbbbbb', profileId: 'viewer', title: 'Movie', quality: '720p', extension: '.mp4', state: 'ready', readyOffline: true, sha256: 'a'.repeat(64), size: 4};
        const button = document.createElement('button');
        Object.assign(button.dataset, {jobId: job.id, itemId: job.itemId, title: job.title, quality: job.quality});
        let transferred = 0;
        window.fetch = async () => new Response(JSON.stringify(job));
        transferOfflineJob = async () => { transferred++; };
        await downloadOfflineJob(button);
        return {transferred, disabled: button.disabled, active: offlineTransfers.size};
      })()`);
    }, permission);
    expect(result).toEqual({transferred: 1, disabled: false, active: 0});
  });
}

test('compatible downloads retain strict identity checks before any transfer side effects', async ({page}) => {
  await setup(page);
  const result = await page.evaluate(`(async () => {
    const expected = {jobID: 'aaaaaaaaaaaaaaaa', itemID: 'bbbbbbbbbbbbbbbb', profileID: 'viewer', title: 'Movie', quality: 'compatible'};
    const job = {id: expected.jobID, itemId: expected.itemID, profileId: expected.profileID, title: expected.title, quality: expected.quality, extension: '.mp4', state: 'ready', readyOffline: true, sha256: 'a'.repeat(64), size: 4};
    let requests = 0, writers = 0, rejected = 0;
    window.fetch = async () => { requests++; throw new Error('unexpected request'); };
    openOfflineWriter = async () => { writers++; throw new Error('unexpected write'); };
    const invalid = [{quality: ''}, {quality: 'Compatible'}, {quality: 'unknown'}, {quality: 'x'.repeat(1024)}, {quality: '720p'}, {profileId: 'other'}, {itemId: 'cccccccccccccccc'}, {id: 'dddddddddddddddd'}, {size: 0}, {sha256: ''}];
    for (const change of invalid) {
      try { await transferOfflineJob({...job, ...change}, expected); } catch { rejected++; }
    }
    const button = document.createElement('button');
    Object.assign(button.dataset, {jobId: expected.jobID, itemId: expected.itemID, title: expected.title, quality: 'unknown'});
    await downloadOfflineJob(button);
    return {accepted: validOfflineManifest(job, expected), rejected, requests, writers, count: (await getOfflineJobs()).length, disabled: button.disabled};
  })()`);
  expect(result).toEqual({accepted: true, rejected: 10, requests: 0, writers: 0, count: 0, disabled: false});
});

test('local Play appears after verification, survives page refresh, and hides during recovery', async ({page}) => {
  await setup(page);
  await page.evaluate(`document.body.innerHTML = '<main id="downloads" data-viewer-profile="viewer"><article data-download-job="aaaaaaaaaaaaaaaa"><button data-download-device data-job-id="aaaaaaaaaaaaaaaa">Download to this device</button><a data-download-play href="/offline?job=aaaaaaaaaaaaaaaa" hidden>Play offline</a><span data-download-device-status></span></article></main>'`);
  const play = page.getByRole('link', {name: 'Play offline'});
  await expect(play).toBeHidden();
  await page.evaluate(`showOfflineStatus('aaaaaaaaaaaaaaaa', {state: 'ready', playbackVerified: false})`);
  await expect(play).toBeVisible();
  await expect(play).toHaveAttribute('href', '/offline?job=aaaaaaaaaaaaaaaa');
  await expect(page.locator('[data-download-device-status]')).toContainText('Play to check compatibility');
  await page.evaluate(`showOfflineStatus('aaaaaaaaaaaaaaaa', {state: 'transferring', percent: 50})`);
  await expect(play).toBeHidden();
  await page.evaluate(`(async () => {
    offlineStatuses.clear();
    await saveOfflineJob({id: 'aaaaaaaaaaaaaaaa', profileID: 'viewer', state: 'ready', readyOffline: true, sha256: 'a'.repeat(64)});
    await bindOfflineDownloads();
  })()`);
  await expect(play).toBeVisible();
  await page.evaluate(`showOfflineStatus('aaaaaaaaaaaaaaaa', {state: 'needs_attention', error: 'Resume to continue'})`);
  await expect(play).toBeHidden();
  await page.evaluate(`(async () => {
    offlineStatuses.clear();
    await saveOfflineJob({id: 'aaaaaaaaaaaaaaaa', profileID: 'other', state: 'ready', readyOffline: true, sha256: 'a'.repeat(64)});
    await bindOfflineDownloads();
  })()`);
  await expect(play).toBeHidden();
});
