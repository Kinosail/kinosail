import assert from 'node:assert/strict';
import test from 'node:test';
import {fixture, json, flush, states} from './home-assistant-document-fixture.mjs';

test('authenticated Profile mismatch stops before claim or state effects', async () => {
  const f = fixture({fetch: async path => path === '/api/v1/me' ? json({viewer: {id: 'viewer-b'}}) : undefined});
  await f.client.sync();
  assert.equal(f.calls.filter(call => call.path.endsWith('/claims')).length, 0);
  assert.equal(states(f).length, 0);
  f.client.close();
});

test('occupied reload candidate settles at the 35-second reconnect bound without takeover', async () => {
  const f = fixture({navigationType: 'reload',
    storage: [['kinosail-home-assistant-document:viewer-a', 'live']],
    fetch: async path => path.endsWith('/claims') ? json({}, 409, {'Retry-After': '30'}) : undefined});
  let settled = false;
  const pending = f.client.sync().then(() => settled = true);
  await flush();
  await f.tick(34_999);
  assert.equal(settled, false);
  await f.tick(1);
  assert.equal(settled, true);
  await pending;
  assert.equal(states(f).length, 0);
  assert.ok(f.calls.filter(call => call.path.endsWith('/claims')).length <= 2);
  assert.equal(f.storage.get('kinosail-home-assistant-document:viewer-a'), 'live');
  f.client.close();
});

test('hung claim request aborts and settles without publishing state', async () => {
  const f = fixture({fetch: async (path, init) => path.endsWith('/claims')
    ? new Promise((_, reject) => init.signal.addEventListener('abort', () => reject(new Error('aborted')))) : undefined});
  let settled = false;
  const pending = f.client.sync().then(() => settled = true);
  await flush();
  await f.tick(5000);
  assert.equal(settled, true);
  await pending;
  assert.equal(states(f).length, 0);
  f.client.close();
});

test('expired authority reacquires the candidate and does not reuse the rejected token', async () => {
  let expired = false, rejected = false;
  const f = fixture({fetch: async (_, init) => {
    if (init.method === 'PUT' && expired && !rejected) {rejected = true; return json({}, 403);}
  }});
  await f.client.sync();
  const first = states(f)[0];
  expired = true;
  await f.client.sync();
  await f.client.sync();
  const latest = states(f).at(-1);
  assert.equal(latest.path, first.path);
  assert.notEqual(latest.init.headers['X-Kinosail-Player-Claim'], first.init.headers['X-Kinosail-Player-Claim']);
  assert.equal(f.effects.length, 0);
  f.client.close();
});

test('authenticated Profile change between successful polls stops before a new state or command', async () => {
  let changed = false;
  const f = fixture({fetch: async path => changed && path === '/api/v1/me'
    ? json({viewer: {id: 'viewer-b'}}) : undefined});
  await f.client.sync();
  const before = states(f).length;
  changed = true;
  await f.client.sync();
  assert.equal(states(f).length, before);
  assert.equal(f.effects.length, 0);
  f.client.close();
});

test('a fetch ignoring abort still settles at its owned timeout', async () => {
  const f = fixture({fetch: async path => path === '/api/v1/me' ? new Promise(() => {}) : undefined});
  let settled = false;
  const pending = f.client.sync().then(() => settled = true);
  await flush();
  await f.tick(5000);
  assert.equal(settled, true);
  await pending;
  assert.equal(states(f).length, 0);
  f.client.close();
});

test('hung successful body cancels its reader at the owned request deadline', async () => {
  let cancelled = false;
  const f = fixture({fetch: async path => path.endsWith('/claims') ? new Response(new ReadableStream({
    pull() {return new Promise(() => {});}, cancel() {cancelled = true;},
  }), {status: 201, headers: {'Content-Type': 'application/json'}}) : undefined});
  let settled = false;
  const pending = f.client.sync().then(() => settled = true);
  await flush();
  await f.tick(5000);
  assert.equal(settled, true);
  assert.equal(cancelled, true);
  await pending;
  assert.equal(states(f).length, 0);
  f.client.close();
});

test('failure diagnostics contain safe operation and level without claim, URL or response text', async () => {
  const secret = 'synthetic-private-claim-xxxxxxxx';
  const f = fixture({fetch: async path => path.endsWith('/claims')
    ? json({id: 'valid', claim: secret, expiresIn: 30, extra: 'https://private.example/path'}, 201,
      {'X-Request-ID': 'bounded-request'}) : undefined});
  await f.client.sync();
  assert.ok(f.logs.some(record => record.operation === 'claim' && record.failure === 'response' && record.level === 'warn'));
  assert.ok(f.logs.some(record => record.requestID === 'bounded-request'));
  assert.ok(!JSON.stringify(f.logs).includes(secret));
  assert.ok(!JSON.stringify(f.logs).includes('private.example'));
  assert.ok(f.logs.every(record => typeof record.generation === 'number'));
  f.client.close();
});

for (const requestID of ['untrusted/path', 'x'.repeat(65)]) {
  test(`invalid diagnostic request ID is omitted: ${requestID.length}`, async () => {
    const f = fixture({fetch: async path => path.endsWith('/claims')
      ? json({}, 500, {'X-Request-ID': requestID}) : undefined});
    await f.client.sync();
    assert.ok(f.logs.length > 0);
    assert.ok(f.logs.every(record => record.requestID === ''));
    f.client.close();
  });
}
