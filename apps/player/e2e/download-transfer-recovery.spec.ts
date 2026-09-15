import { readFile } from 'node:fs/promises';
import { expect, test } from '@playwright/test';

const source = await readFile(new URL('../../../packages/webassets/static/downloads-transfer.js', import.meta.url), 'utf8');
const helper = source.slice(source.indexOf('async function fetchOfflineChunk'), source.indexOf('async function transferOfflineJob'));

for (const failure of ['network', '503', '401', 'range', 'digest', 'oversized', 'short', 'exhausted']) {
  test(`download recovery handles ${failure} without accepting invalid bytes`, async ({ page }) => {
    await page.goto('about:blank');
    const result = await page.evaluate(async ({ helper, failure }) => {
      let calls = 0;
      const notices: string[] = [];
      const fetch = async (_url: string, options: RequestInit) => {
        calls++;
        if ((failure === 'network' && calls === 1) || failure === 'exhausted') throw new TypeError('disconnected');
        if (failure === '503' && calls === 1) return new Response(null, { status: 503 });
        if (failure === '401') return new Response(null, { status: 401 });
        if ((options.headers as Record<string, string>).Range !== 'bytes=4-7') throw new Error('wrong range');
        return new Response(new Uint8Array(failure === 'oversized' ? 5 : failure === 'short' && calls === 1 ? 2 : 4), {
          status: 206, headers: { 'Content-Range': failure === 'range' ? 'bytes 0-3/8' : 'bytes 4-7/8', 'Content-Digest': failure === 'digest' ? 'bad' : 'good' },
        });
      };
      const offlineMessage = (_key: string, fallback: string) => fallback;
      const offlineDigest = async () => 'good';
      const contentDigest = (value: string) => value;
      const notifyOffline = (_id: string, detail: { state: string }) => notices.push(detail.state);
      // Skip backoff in the fixture, but do not trigger idle deadlines.
      const setTimeout = (callback: () => void, delay: number) => { if (delay !== 30000) queueMicrotask(callback); return 1; };
      const clearTimeout = () => {};
      const run = new Function('fetch', 'offlineMessage', 'offlineDigest', 'contentDigest', 'notifyOffline', 'setTimeout', 'clearTimeout', 'offlineDelay', `${helper}; return fetchOfflineChunk({id: 'aaaaaaaaaaaaaaaa', size: 8}, 4, 4);`);
      try {
        const data = await run(fetch, offlineMessage, offlineDigest, contentDigest, notifyOffline, setTimeout, clearTimeout, async () => {});
        return { calls, notices, bytes: data.byteLength, failed: false };
      } catch { return { calls, notices, bytes: 0, failed: true }; }
    }, { helper, failure });
    const temporary = ['network', '503', 'short'].includes(failure);
    expect(result.calls).toBe(temporary ? 2 : failure === 'exhausted' ? 6 : 1);
    expect(result.failed).toBe(!temporary);
    expect(result.bytes).toBe(temporary ? 4 : 0);
    expect(result.notices.length).toBe(temporary ? 1 : failure === 'exhausted' ? 5 : 0);
  });
}
