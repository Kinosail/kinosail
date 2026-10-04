import { createHmac } from 'node:crypto';
import type { Browser } from '@e2e-dev/web';

export function totp(encoded: string) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  const bits = [...encoded].map(c => alphabet.indexOf(c).toString(2).padStart(5, '0')).join('');
  const secret = Buffer.from(bits.match(/.{8}/g)!.map(byte => parseInt(byte, 2)));
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000)));
  const digest = createHmac('sha1', secret).update(counter).digest();
  return ((digest.readUInt32BE(digest[19] & 15) & 0x7fffffff) % 1000000).toString().padStart(6, '0');
}

export async function api(browser: Browser, path: string, method = 'GET', body?: unknown) {
  return browser.evaluate(async ({ path, method, body }) => {
    const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')?.content ?? '';
    const response = await fetch(path, { method, headers: { 'Content-Type': 'application/json', 'X-Kinosail-CSRF': csrf }, ...(body === null ? {} : { body: JSON.stringify(body) }) });
    const text = await response.text();
    return { status: response.status, data: text ? JSON.parse(text) : null };
  }, { path, method, body: body ?? null });
}

export async function movie(browser: Browser) {
  const response = await api(browser, '/api/v1/library?view=movies');
  if (response.status !== 200) throw new Error(`library HTTP ${response.status}`);
  const item = response.data.items.find((item: { title: string }) => item.title === 'Example Movie');
  if (!item) throw new Error('generated movie missing');
  return item;
}
