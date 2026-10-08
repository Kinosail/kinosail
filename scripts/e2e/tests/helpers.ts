import { createHmac } from 'node:crypto';
import type { Browser } from '@e2e-dev/web';
import { requireFixtureURL, fixtureRequest, fixtureBrowserFetch, decodeFixtureJSON, fixtureItem } from '../fixture-response.mjs';

export function totp(encoded: string) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  const bits = [...encoded].map(c => alphabet.indexOf(c).toString(2).padStart(5, '0')).join('');
  const secret = Buffer.from(bits.match(/.{8}/g)!.map(byte => parseInt(byte, 2)));
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000)));
  const digest = createHmac('sha1', secret).update(counter).digest();
  return ((digest.readUInt32BE(digest[19] & 15) & 0x7fffffff) % 1000000).toString().padStart(6, '0');
}

export async function api(browser: Browser, baseURL: string | undefined, path: string, method = 'GET', body?: unknown) {
  const request = fixtureRequest(baseURL, path, method, body);
  const response = await browser.evaluate(fixtureBrowserFetch, request);
  return {status: response.status, data: response.status === 204 ? null : decodeFixtureJSON(response.raw)};
}

export async function movie(browser: Browser, baseURL: string | undefined) {
  const response = await api(browser, baseURL, '/api/v1/library?view=movies');
  if (response.status !== 200) throw new Error(`library HTTP ${response.status}`);
  return fixtureItem(response.data, 'Example Movie', 'video');
}

export { requireFixtureURL };
