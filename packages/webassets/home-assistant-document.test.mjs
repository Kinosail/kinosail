import assert from 'node:assert/strict';
import test from 'node:test';
import {fixture, json, flush, states} from './home-assistant-document-fixture.mjs';

test('claims document authority without randomUUID, Web Locks or localStorage', async () => {
  const f = fixture();
  await f.client.sync();
  assert.ok(states(f).length > 0);
  const id = states(f)[0].path.split('/').at(-1);
  assert.equal(f.storage.get('kinosail-home-assistant-document:viewer-a'), id);
  assert.equal(f.storage.size, 1);
  assert.ok(states(f).every(call => call.init.headers['X-Kinosail-Player-Claim']?.length >= 20));
  assert.ok(states(f).every(call => !Object.hasOwn(JSON.parse(call.init.body), 'claim')));
  assert.ok(!JSON.stringify([...f.storage]).includes('claim-'));
  f.client.close();
});

test('denied storage keeps an ephemeral document target with memory-only authority', async () => {
  const f = fixture({deniedStorage: true});
  await f.client.sync();
  assert.ok(states(f).length > 0);
  assert.equal(f.storage.size, 0);
  f.client.close();
});

test('cloned fresh-navigation candidate forks without taking over a live claim', async () => {
  const f = fixture({storage: [['kinosail-home-assistant-document:viewer-a', 'live']],
    fetch: async (path, init) => path.endsWith('/claims') && JSON.parse(init.body).id === 'live'
      ? json({}, 409, {'Retry-After': '30'}) : undefined});
  await f.client.sync();
  assert.ok(states(f).length > 0);
  assert.ok(states(f).every(call => !call.path.endsWith('/live')));
  assert.notEqual(f.storage.get('kinosail-home-assistant-document:viewer-a'), 'live');
  f.client.close();
});

test('poll and SSE requests coalesce into one flight instead of draining concurrent commands', async () => {
  let hold = false, release;
  const barrier = new Promise(resolve => release = resolve);
  const f = fixture({fetch: async (_, init) => {if (init.method === 'PUT' && hold) await barrier;}});
  await f.client.sync();
  const before = states(f).length;
  const resource = states(f)[0].path;
  hold = true;
  const first = f.client.sync();
  await flush();
  f.sse(resource);
  assert.equal(f.client.sync(), first);
  assert.equal(f.client.sync(), first);
  assert.equal(states(f).length, before + 1);
  release();
  await first;
  assert.ok(states(f).length <= before + 2);
  f.client.close();
});

test('only a matching-resource live SSE subscription starts an idle state flight', async () => {
  const f = fixture();
  await f.client.sync();
  assert.equal(f.streams.filter(stream => !stream.closed).length, 1);
  const before = states(f).length;
  f.sse('/api/v1/home-assistant/players/sibling');
  await flush();
  assert.equal(states(f).length, before);
  f.sse(states(f)[0].path);
  await flush();
  assert.equal(states(f).length, before + 1);
  f.client.close();
});

test('persisted return with an invalid media owner performs no new claim or state', async () => {
  const f = fixture();
  await f.client.sync();
  f.event('pagehide', true);
  await flush();
  f.detach();
  const count = f.calls.length;
  f.event('pageshow', true);
  await f.client.sync();
  await f.tick(30_000);
  assert.equal(f.calls.length, count);
  assert.equal(f.effects.length, 0);
  f.client.close();
});

for (const reason of ['pagehide', 'source-change', 'Profile-change', 'detached']) {
  test(`delayed successful command has no effects after ${reason}`, async () => {
    let hold = false, release;
    const barrier = new Promise(resolve => release = resolve);
    const f = fixture({fetch: async (path, init) => {
      if (init.method === 'PUT' && hold) {await barrier; return json({command: 'seek', position: 8});}
    }});
    await f.client.sync();
    hold = true;
    const pending = f.client.sync();
    await flush();
    if (reason === 'pagehide') f.event('pagehide');
    if (reason === 'source-change') f.changeSource();
    if (reason === 'Profile-change') f.setProfile('viewer-b');
    if (reason === 'detached') f.detach();
    release();
    await pending;
    assert.equal(f.effects.length, 0);
    f.client.close();
  });
}

for (const body of [{id: '../target', claim: 'x'.repeat(24), expiresIn: 30},
  {id: 'valid', claim: 'short', expiresIn: 30}, {id: 'valid', claim: 'x'.repeat(24), expiresIn: 31},
  {id: 'valid', claim: 'x'.repeat(24), expiresIn: 30, extra: true}]) {
  test(`invalid successful claim cannot publish state: ${body.id}/${body.claim.length}/${body.expiresIn}/${Object.keys(body).length}`, async () => {
    const f = fixture({fetch: async path => path.endsWith('/claims') ? json(body, 201) : undefined});
    await f.client.sync();
    assert.equal(states(f).length, 0);
    assert.equal(f.storage.size, 0);
    assert.ok(!JSON.stringify(f.logs).includes(body.claim));
    f.client.close();
  });
}

for (const body of [{command: 'seek', position: -1}, {command: 'seek', position: 1e10},
  {command: 'volume', volume: 2}, {command: 'play_media', itemId: '\ninvalid'},
  {command: 'mute', muted: 'true'}, {command: 'pause', extra: true},
  {command: 'unknown'}, {command: null, extra: true}]) {
  test(`invalid command has no media effects: ${body.command}/${body.position ?? body.volume ?? body.muted ?? body.itemId ?? Object.keys(body).length}`, async () => {
    const f = fixture({fetch: async (_, init) => init.method === 'PUT' ? json(body) : undefined});
    await f.client.sync();
    assert.equal(f.effects.length, 0);
    f.client.close();
  });
}

test('oversized successful body is rejected without command effects', async () => {
  const f = fixture({fetch: async (_, init) => init.method === 'PUT'
    ? new Response(' '.repeat(4097) + '{"command":"pause"}',
      {headers: {'Content-Type': 'application/json'}}) : undefined});
  await f.client.sync();
  assert.equal(f.effects.length, 0);
  f.client.close();
});

test('pagehide releases only its own claim and virtual persisted pageshow acquires fresh authority', async () => {
  const f = fixture();
  await f.client.sync();
  const first = states(f)[0], oldCount = states(f).length;
  f.event('pagehide', true);
  await flush();
  const release = f.calls.find(call => call.path.endsWith('/release'));
  assert.equal(release.init.headers['X-Kinosail-Player-Claim'], first.init.headers['X-Kinosail-Player-Claim']);
  assert.equal(release.init.keepalive, true);
  assert.equal(release.init.body, '{}');
  assert.ok(f.streams.every(stream => stream.closed));
  f.event('pageshow', true);
  await f.client.sync();
  assert.ok(states(f).length > oldCount);
  assert.equal(states(f).at(-1).path, first.path);
  assert.notEqual(states(f).at(-1).init.headers['X-Kinosail-Player-Claim'], first.init.headers['X-Kinosail-Player-Claim']);
  f.client.close();
});

for (const reason of ['pagehide', 'Profile-change', 'detached']) {
  test(`delayed claim cannot persist authority or publish state after ${reason}`, async () => {
    let release;
    const barrier = new Promise(resolve => release = resolve);
    const f = fixture({fetch: async path => {if (path.endsWith('/claims')) {
      await barrier; return json({id: 'late', claim: 'x'.repeat(24), expiresIn: 30}, 201);
    }}});
    const pending = f.client.sync();
    await flush();
    if (reason === 'pagehide') f.event('pagehide');
    if (reason === 'Profile-change') f.setProfile('viewer-b');
    if (reason === 'detached') f.detach();
    release();
    await pending;
    assert.equal(f.storage.size, 0);
    assert.equal(states(f).length, 0);
    f.client.close();
  });
}

test('terminal close remains inert across later sync, persisted pageshow and timers', async () => {
  const f = fixture();
  await f.client.sync();
  f.client.close();
  await flush();
  const count = f.calls.length;
  f.event('pageshow', true);
  await f.client.sync();
  await f.tick(60_000);
  assert.equal(f.calls.length, count);
  assert.ok(f.streams.every(stream => stream.closed));
});
