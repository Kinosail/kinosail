import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';
const source = readFileSync(new URL('./static/watch-progress.js', import.meta.url), 'utf8');
const flush = () => new Promise(resolve => setImmediate(resolve));
function fixture(id = 'movie') {
  const requests = [], listeners = new Map();
  const bar = { setAttribute(name, value) { this[name] = value; } }, label = {};
  const main = { addEventListener(name, fn) { listeners.set(name, fn); } };
  const container = { dataset: { watchProgress: id }, isConnected: true, hidden: true, closest: () => main, querySelector: name => name === 'progress' ? bar : label };
  vm.runInNewContext(source, {
    document: { readyState: 'complete', querySelectorAll: () => [container], addEventListener() {} },
    AbortController, setTimeout, clearTimeout,
    fetch: (url, options) => new Promise(resolve => requests.push({ url, options, resolve })),
  });
  return { requests, bar, label, container, select(id) { listeners.get('kinosail:feature')({ detail: { id } }); } };
}
function respond(request, value) { request.resolve({ ok: true, headers: { get: () => null }, text: async () => JSON.stringify(value) }); }
test('renders true progress with remaining time', async () => {
  const f = fixture(); respond(f.requests[0], { seconds: 1200, duration: 3600 }); await flush();
  assert.equal(f.container.hidden, false); assert.equal(f.bar.hidden, false);
  assert.ok(Math.abs(f.bar.value - 100 / 3) < 0.0001); assert.equal(f.label.textContent, '40 min left');
  assert.equal(f.requests[0].options.credentials, 'same-origin');
});
test('unknown runtime reports elapsed time without a percentage', async () => {
  const f = fixture(); respond(f.requests[0], { seconds: 1200, duration: 0 }); await flush();
  assert.equal(f.label.textContent, '20 min watched'); assert.equal(f.bar.hidden, true);
});
for (const value of [null, [], {}, { seconds: 10, duration: 100, extra: 1 }, { seconds: -1, duration: 100 }, { seconds: '10', duration: 100 }, { seconds: 101, duration: 100 }, { seconds: 0, duration: 315360001 }]) {
  test(`rejects invalid progress ${JSON.stringify(value)}`, async () => {
    const f = fixture(); respond(f.requests[0], value); await flush(); assert.equal(f.container.hidden, true);
  });
}
test('title changes immediately clear the old bar and ignore stale responses', async () => {
  const f = fixture(); f.select('next'); assert.equal(f.requests[0].options.signal.aborted, true);
  respond(f.requests[1], { seconds: 60, duration: 600 }); await flush();
  respond(f.requests[0], { seconds: 1200, duration: 3600 }); await flush();
  assert.equal(f.label.textContent, '9 min left');
});
test('invalid identifiers cause no request', () => { assert.equal(fixture('../secret').requests.length, 0); });

test('unstarted titles have no resume progress even when runtime is known', async () => {
  const f = fixture(); respond(f.requests[0], { seconds: 0, duration: 3600 }); await flush();
  assert.equal(f.container.hidden, true);
});
