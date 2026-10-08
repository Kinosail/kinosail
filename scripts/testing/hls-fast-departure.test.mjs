import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { runInNewContext } from 'node:vm';
import test from 'node:test';

const nonce = '01234567-89ab-4cde-8fab-0123456789ab';
const key = 'kinosail-fixture:departure';
function control({ storageFailure = false } = {}) {
  const storage = new Map(), listeners = new Map();
  const video = {}, location = { pathname: '/watch/movie' };
  const context = {
    location, crypto: { randomUUID: () => nonce }, console: { debug: () => {} },
    document: { querySelector: () => video },
    sessionStorage: {
      setItem: (name, value) => { if (storageFailure) throw new Error('controlled storage failure'); storage.set(name, value); },
      getItem: name => storage.get(name) ?? null,
      removeItem: name => storage.delete(name),
    },
    window: { addEventListener: (name, handler) => listeners.set(name, handler) },
  };
  const prefix = stripTypeScriptTypes(readFileSync(new URL('../../apps/player/e2e/player-hls-navigation.spec.ts', import.meta.url), 'utf8'))
    .replace(/^import .*;\n/gm, '').split('async function movingVideo', 1)[0];
  const api = runInNewContext(`(()=>{${prefix}; return {recordFastDeparture, readFastDeparture};})()`, context);
  const page = { evaluate: async (fn, value) => fn(value) };
  return { api, page, storage, listeners, location,
    navigation: (target = video) => listeners.get('kinosail:navigation')?.({ target, bubbles: false }),
    hide: () => listeners.get('pagehide')?.({}),
  };
}

test('actual old-document callbacks retain navigation then pagehide when console delivery is discarded', async () => {
  const c = control(); const id = await c.api.recordFastDeparture(c.page);
  c.navigation(); c.hide(); c.location.pathname = '/';
  const value = await c.api.readFastDeparture(c.page, id);
  assert.deepEqual(JSON.parse(JSON.stringify(value)), { nonce, path: '/watch/movie', events: ['navigation', 'pagehide'] });
  assert.equal(c.storage.size, 0);
});

test('missing pagehide or wrong target cannot pass and the owned record is consumed', async () => {
  for (const wrongTarget of [false, true]) {
    const c = control(); const id = await c.api.recordFastDeparture(c.page);
    c.navigation(wrongTarget ? {} : undefined); if (wrongTarget) c.hide(); c.location.pathname = '/';
    await assert.rejects(c.api.readFastDeparture(c.page, id), /invalid fast departure witness/);
    assert.equal(c.storage.size, 0);
  }
});

test('closed witness rejects stale nonce, order, unknown, duplicate, malformed and oversized records', async () => {
  const base = { nonce, path: '/watch/movie', events: ['navigation', 'pagehide'] };
  const values = [null, '{}', '{', JSON.stringify({ ...base, nonce: 'ffffffff-ffff-4fff-8fff-ffffffffffff' }),
    JSON.stringify({ ...base, path: '/private' }), JSON.stringify({ ...base, events: ['pagehide', 'navigation'] }),
    JSON.stringify({ ...base, events: ['navigation', 'other'] }), JSON.stringify({ ...base, events: ['navigation', 'pagehide', 'pagehide'] }),
    JSON.stringify({ ...base, extra: true }), JSON.stringify(base).replace('"path":', '"path":"/watch/movie","path":'), 'x'.repeat(513)];
  for (const raw of values) {
    const c = control(); const id = await c.api.recordFastDeparture(c.page);
    if (raw === null) c.storage.delete(key); else c.storage.set(key, raw);
    c.location.pathname = '/';
    await assert.rejects(c.api.readFastDeparture(c.page, id), /invalid fast departure witness/);
    assert.equal(c.storage.size, 0);
  }
});

test('unavailable storage rejects before listeners and no event witness is fabricated', async () => {
  const c = control({ storageFailure: true });
  await assert.rejects(c.api.recordFastDeparture(c.page), /controlled storage failure/);
  assert.equal(c.storage.size, 0); assert.equal(c.listeners.size, 0);
});


test('repeated events remain bounded and cannot satisfy the exact departure order', async () => {
  const c = control(); const id = await c.api.recordFastDeparture(c.page);
  for (let count = 0; count < 100; count++) c.navigation();
  assert.ok(c.storage.get(key).length <= 512);
  c.hide(); c.location.pathname = '/';
  await assert.rejects(c.api.readFastDeparture(c.page, id), /invalid fast departure witness/);
  assert.equal(c.storage.size, 0);
});

test('wrong document or nonce rejects before reading or clearing an owned record', async () => {
  const c = control(); const id = await c.api.recordFastDeparture(c.page);
  await assert.rejects(c.api.readFastDeparture(c.page, id), /invalid fast departure document/);
  assert.equal(c.storage.size, 1);
  c.location.pathname = '/';
  for (const invalid of [undefined, null, false, '', 'x'.repeat(10000)]) {
    await assert.rejects(c.api.readFastDeparture(c.page, invalid), /invalid fast departure witness/);
    assert.equal(c.storage.size, 1);
  }
  const invalidDocument = control(); invalidDocument.location.pathname = '/private';
  await assert.rejects(invalidDocument.api.recordFastDeparture(invalidDocument.page), /invalid fast departure document/);
  assert.equal(invalidDocument.listeners.size, 0); assert.equal(invalidDocument.storage.size, 0);
});


test('storage failure inside the real callback cannot fabricate completed pagehide', async () => {
  const c = control(); const id = await c.api.recordFastDeparture(c.page);
  // Fail the browser storage seam after setup, while keeping initial bytes.
  const callback = c.listeners.get('pagehide');
  const before = c.storage.get(key);
  c.storage.set = () => { throw new Error('controlled later storage failure'); };
  assert.throws(() => callback({}), /controlled later storage failure/);
  assert.equal(c.storage.get(key), before);
  c.location.pathname = '/';
  await assert.rejects(c.api.readFastDeparture(c.page, id), /invalid fast departure witness/);
  assert.equal(c.storage.size, 0);
});
