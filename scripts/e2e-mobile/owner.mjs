import { createHmac, randomBytes } from 'node:crypto';
import { registerSecrets } from './privacy.mjs';
import { requireFixtureURL, decodeFixtureJSON } from '../e2e/fixture-response.mjs';
import { validateFixtureSetup } from '../e2e/fixture-setup.mjs';
export async function request(base, path, { method = 'GET', body, token, cookie, csrf } = {}) {
  requireFixtureURL(base);
  const url = new URL(path, base);
  if (url.origin !== base || !path.startsWith('/') || path.length > 4096) throw new Error('invalid fixture API path');
  const response = await fetch(url, { method, redirect: 'error', signal: AbortSignal.timeout(10000), headers: { ...(body === undefined ? {} : { 'Content-Type': 'application/json' }), ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(cookie ? { Cookie: cookie } : {}), ...(csrf ? { 'X-Kinosail-CSRF': csrf } : {}) }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) });
  if (response.url !== url.href) throw new Error('foreign fixture response');
  if (!response.ok) { await response.body?.cancel(); throw new Error(`fixture API HTTP ${response.status}`); }
  const reader = response.body?.getReader();
  let bytes = 0; const chunks = [];
  if (reader) try { for (;;) { const { value, done } = await reader.read(); if (done) break; if ((bytes += value.length) > 1024 * 1024) throw new Error('fixture response too large'); chunks.push(Buffer.from(value)); } } finally { await reader.cancel(); }
  const text = new TextDecoder('utf-8', {fatal: true}).decode(Buffer.concat(chunks));
  let data = null;
  if (response.headers.get('content-type')?.includes('application/json') && text) {
    try { data = decodeFixtureJSON(text); } catch { throw new Error('invalid fixture JSON response'); }
  }
  return { status: response.status, text, data };
}
function totp(encoded) {
  if (typeof encoded !== 'string' || !/^[A-Z2-7]{16,128}$/.test(encoded)) throw new Error('invalid fixture MFA enrollment');
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  const bits = [...encoded].map(c => alphabet.indexOf(c).toString(2).padStart(5, '0')).join('');
  const secret = Buffer.from(bits.match(/.{8}/g).map(byte => parseInt(byte, 2)));
  const counter = Buffer.alloc(8); counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000)));
  const digest = createHmac('sha1', secret).update(counter).digest();
  return ((digest.readUInt32BE(digest[19] & 15) & 0x7fffffff) % 1000000).toString().padStart(6, '0');
}
export async function owner(base, root) {
  const password = randomBytes(24).toString('hex');
  registerSecrets(root, [password], 'started');
  const created = await request(base, '/api/v1/setup', { method: 'POST', body: { name: 'Owner', password, device: 'Disposable native QA Owner', totp: true, automaticUpdates: false } });
  if (created.status !== 201) throw new Error('invalid fixture setup status');
  const setup = validateFixtureSetup(created.data);
  const token = setup.token, code = totp(setup.totp.secret);
  registerSecrets(root, [token, setup.totp.secret, code], 'started');
  await request(base, '/api/v1/me/mfa', { method: 'PUT', token, body: { code } });
  await request(base, '/api/v1/settings/onboarding', { method: 'PUT', token, body: { enabled: false } });
  const cookie = `__Host-kinosail_session=${token}`;
  const settings = await request(base, '/settings', { cookie });
  const csrf = /<meta name="kinosail-csrf" content="([A-Za-z0-9_-]{43})">/.exec(settings.text)?.[1];
  if (!csrf) throw new Error('fixture Owner CSRF unavailable');
  registerSecrets(root, [csrf], 'started');
  return {
    read: async path => (await request(base, path, { token })).data,
    approve: async approval => {
      if (!/^\d{6}$/.test(approval)) throw new Error('invalid fixture approval');
      registerSecrets(root, [approval], 'started');
      const result = await request(base, `/api/v1/quick-connect/${approval}`, { method: 'POST', cookie, csrf });
      if (result.status !== 204) throw new Error('fixture approval failed');
      registerSecrets(root, [], 'paired');
    },
  };
}
