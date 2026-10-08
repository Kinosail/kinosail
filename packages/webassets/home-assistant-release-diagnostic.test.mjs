// Isolated diagnostic safety controls: prove pass-through, privacy and truthful
// counter persistence. These do not prove native transport, release loss or expiry.
import assert from 'node:assert/strict';
import test from 'node:test';
import vm from 'node:vm';
import {installNativeReleaseDiagnostic, createDepartingConsoleCounters} from './home-assistant-release-diagnostic-fixture.mjs';

const prefix = 'KINOSAIL_R18_RELEASE_DIAGNOSTIC:';
const path = '/api/v1/home-assistant/players/fixture-target/release';
const flush = async () => {for (let index = 0; index < 5; index++) await Promise.resolve();};
function diagnostic(fetch, shared = {name: ''}, denied = false, consoleThrows = false) {
  const events = new Map(), signals = [], window = {fetch};
  Object.defineProperty(window, 'name', {get() {if (denied) throw new Error('synthetic-private-error'); return shared.name;},
    set(value) {if (denied) throw new Error('synthetic-private-error'); shared.name = value;}});
  const context = vm.createContext({window, location: new URL('https://fixture.invalid/watch/fictional'),
    URL, Response, Promise, JSON, Object, Array, Number, Reflect, document: {readyState: 'complete'},
    console: {debug(...args) {if (consoleThrows) throw new Error('synthetic-private-console'); signals.push(args);}},
    addEventListener: (name, callback) => events.set(name, callback)});
  vm.runInContext('(' + installNativeReleaseDiagnostic.toString() + ')()', context);
  window.__kinosailReleaseDiagnosticTarget(path);
  return {window, shared, signals, snapshot: () => JSON.parse(JSON.stringify(window.__kinosailReleaseDiagnostic())),
    event: name => events.get(name)?.()};
}

test('release diagnostics preserve the exact native promise, receiver and all arguments', async () => {
  const promise = Promise.resolve(new Response(null, {status: 204})), receiver = {};
  let received;
  const f = diagnostic(function (...args) {received = {receiver: this, args}; return promise;});
  const init = {method: 'POST', keepalive: true, headers: {'X-Private': 'synthetic-private-claim'}, body: '{}'};
  assert.equal(f.window.fetch.call(receiver, path, init), promise);
  assert.equal(received.receiver, receiver); assert.equal(received.args[0], path); assert.equal(received.args[1], init);
  await flush();
  const s = f.snapshot();
  assert.equal(s.counters.calls, 1); assert.equal(s.counters.returned, 1); assert.equal(s.counters.response204, 1);
  assert.equal(s.counters.pending, 0); assert.equal(s.methodUnknown, true);
  assert.ok(!JSON.stringify(s).includes('synthetic-private')); assert.ok(!f.shared.name.includes('fixture-target'));
});

test('only the exact same-origin target endpoint is counted and method remains unknown', async () => {
  const promise = Promise.resolve(new Response(null, {status: 200}));
  let calls = 0;
  const f = diagnostic(() => {calls++; return promise;});
  assert.equal(f.window.fetch('/api/v1/me'), promise);
  assert.equal(f.window.fetch('https://other.invalid' + path, {method: 'POST'}), promise);
  assert.equal(f.window.fetch(path, {method: 'GET'}), promise);
  await flush();
  assert.equal(calls, 3); assert.equal(f.snapshot().counters.calls, 1);
  assert.equal(f.snapshot().methodUnknown, true);
  assert.equal(f.snapshot().counters.response204, 0);
});

test('pending native outcomes survive a new document without becoming blocked-release proof', async () => {
  let resolve;
  const promise = new Promise(done => resolve = done), shared = {name: ''};
  const first = diagnostic(() => promise, shared);
  first.event('pagehide');
  assert.equal(first.window.fetch(path, {method: 'POST', keepalive: true}), promise);
  const second = diagnostic(() => Promise.resolve(new Response(null, {status: 204})), shared);
  let s = second.snapshot();
  assert.equal(s.counters.documents, 2); assert.equal(s.counters.pagehides, 1);
  assert.equal(s.counters.afterHide, 1); assert.equal(s.counters.pending, 1);
  assert.ok(s.codes.includes('NATIVE_OUTCOME_PENDING'));
  assert.ok(!s.codes.some(code => code.includes('BLOCKED') || code.includes('SUCCESS')));
  resolve(new Response(null, {status: 204})); await flush();
  s = second.snapshot();
  assert.equal(s.counters.documents, 2); assert.equal(s.counters.pending, 0); assert.equal(s.counters.response204, 1);
});

test('native rejection is preserved and only its fixed failure class enters diagnostics', async () => {
  const error = new TypeError('synthetic-private-url-or-body'), promise = Promise.reject(error);
  const f = diagnostic(() => promise);
  assert.equal(f.window.fetch(path, {method: 'POST', keepalive: true}), promise);
  await assert.rejects(promise, value => value === error); await flush();
  const s = f.snapshot();
  assert.equal(s.counters.rejected, 1); assert.equal(s.counters.pending, 0);
  assert.ok(!JSON.stringify(s).includes('synthetic-private'));
});

test('synchronous native errors keep exact identity without a fabricated return or response', () => {
  const error = new Error('synthetic-private-error'), f = diagnostic(() => {throw error;});
  assert.throws(() => f.window.fetch(path, {method: 'POST'}), value => value === error);
  const s = f.snapshot();
  assert.equal(s.counters.calls, 1); assert.equal(s.counters.syncThrows, 1);
  assert.equal(s.counters.returned, 0); assert.equal(s.counters.response204, 0);
});

test('denied diagnostic persistence cannot break native fetch or claim cross-document evidence', async () => {
  const promise = Promise.resolve(new Response(null, {status: 204}));
  const f = diagnostic(() => promise, {name: ''}, true);
  assert.equal(f.window.fetch(path, {method: 'POST', keepalive: true}), promise); await flush();
  const s = f.snapshot();
  assert.equal(s.counters.persistence, 0); assert.ok(s.codes.includes('DIAGNOSTIC_PERSISTENCE_DENIED'));
  assert.ok(!JSON.stringify(s).includes('synthetic-private'));
});

test('unowned window names remain untouched and cannot claim persistent evidence', () => {
  const shared = {name: 'synthetic-private-window-name'}, f = diagnostic(() => Promise.resolve(), shared);
  assert.equal(shared.name, 'synthetic-private-window-name');
  assert.equal(f.snapshot().counters.persistence, 0);
  assert.ok(f.snapshot().codes.includes('DIAGNOSTIC_NAME_UNOWNED'));
  assert.ok(!JSON.stringify(f.snapshot()).includes('synthetic-private'));
});

for (const name of [prefix + '{"documents":1,"private":"synthetic-private-claim"}',
  prefix + 'x'.repeat(513)]) {
  test('untrusted diagnostic state is discarded without leaking private values: ' + name.length, () => {
    const f = diagnostic(() => Promise.resolve(new Response(null, {status: 204})), {name});
    const s = f.snapshot();
    assert.equal(s.counters.documents, 1); assert.equal(s.counters.corrupt, 1);
    assert.ok(!JSON.stringify(s).includes('synthetic-private')); assert.ok(f.shared.name.length <= 512);
  });
}

test('diagnostic counters saturate and mark overflow without retaining a request inventory', async () => {
  const promise = Promise.resolve(new Response(null, {status: 204})), f = diagnostic(() => promise);
  for (let index = 0; index < 20; index++) f.window.fetch(path, {method: 'POST', keepalive: true});
  await flush();
  const s = f.snapshot();
  assert.ok(Object.values(s.counters).every(value => Number.isInteger(value) && value >= 0 && value <= 8));
  assert.equal(s.counters.calls, 8); assert.equal(s.counters.saturated, 1); assert.ok(f.shared.name.length <= 512);
  const perCode = new Map(); for (const [, code] of f.signals) perCode.set(code, (perCode.get(code) || 0) + 1);
  assert.ok([...perCode.values()].every(count => count <= 8)); assert.ok(f.signals.length <= 120);
});

test('telemetry never reads request init getters while preserving the native result', async () => {
  let reads = 0;
  const init = {get method() {reads++; throw new Error('synthetic-private-method');},
    get keepalive() {reads++; throw new Error('synthetic-private-keepalive');}};
  const promise = Promise.resolve(new Response(null, {status: 204})), f = diagnostic(() => promise);
  assert.equal(f.window.fetch(path, init), promise); await flush();
  assert.equal(reads, 0); assert.equal(f.snapshot().counters.response204, 1);
});

test('non-string request inputs are passed through without getter or proxy inspection', async () => {
  let reads = 0;
  const input = new Proxy({}, {get() {reads++; throw new Error('synthetic-private-input');}});
  const promise = Promise.resolve(new Response(null, {status: 200})), f = diagnostic(() => promise);
  assert.equal(f.window.fetch(input, {method: 'POST'}), promise); await flush();
  assert.equal(reads, 0); assert.equal(f.snapshot().counters.calls, 0);
});

test('native console signals contain only fixed codes and preserve actual endpoint outcomes', async () => {
  const promise = Promise.resolve(new Response(null, {status: 204})), f = diagnostic(() => promise);
  f.event('pagehide');
  assert.equal(f.window.fetch(path, {method: 'POST', headers: {'X-Private': 'synthetic-private-claim'},
    body: 'synthetic-private-body'}), promise); await flush();
  const allowed = new Set(['DOCUMENT_READY', 'PAGE_HIDE', 'PAGE_SHOW', 'ENDPOINT_CALLED', 'AFTER_HIDE',
    'RETURNED', 'RESPONSE_204', 'RESPONSE_OTHER', 'REJECTED', 'SYNC_THROW', 'OBSERVER_ERROR',
    'COUNTER_SATURATED', 'STATE_DISCARDED', 'TARGET_SELECTED', 'READY_COMPLETE', 'READY_INTERACTIVE', 'READY_LOADING', 'READY_UNKNOWN']);
  assert.ok(f.signals.every(args => args.length === 2 && args[0] === 'KINOSAIL_R18_NATIVE_EVENT' && allowed.has(args[1])));
  for (const code of ['TARGET_SELECTED', 'READY_COMPLETE', 'PAGE_HIDE', 'ENDPOINT_CALLED', 'RETURNED', 'RESPONSE_204'])
    assert.ok(f.signals.some(args => args[1] === code));
  assert.ok(!JSON.stringify(f.signals).includes('synthetic-private'));
  assert.ok(!JSON.stringify(f.signals).includes('fixture-target'));
});

test('console diagnostic failure cannot alter native promise or synchronous exception identity', async () => {
  const promise = Promise.resolve(new Response(null, {status: 204})), f = diagnostic(() => promise, {name: ''}, false, true);
  assert.equal(f.window.fetch(path, {method: 'POST'}), promise); await flush();
  assert.equal(f.snapshot().counters.response204, 1);
  const error = new Error('synthetic-private-error'), throwing = diagnostic(() => {throw error;}, {name: ''}, false, true);
  assert.throws(() => throwing.window.fetch(path, {method: 'POST'}), value => value === error);
});

test('direct console counters accept only the exact selected execution context', () => {
  const c = createDepartingConsoleCounters(() => 7);
  const event = (context, code) => ({executionContextId: context, args: [
    {type: 'string', value: 'KINOSAIL_R18_NATIVE_EVENT'}, {type: 'string', value: code}]});
  c.observe(event(9, 'PAGE_HIDE')); c.observe(event(9, 'ENDPOINT_CALLED'));
  c.observe(event(7, 'TARGET_SELECTED')); c.observe(event(7, 'READY_COMPLETE')); c.observe(event(7, 'PAGE_HIDE'));
  assert.equal(c.snapshot().targetSelected, 1); assert.equal(c.snapshot().readyComplete, 1);
  assert.equal(c.snapshot().pageHide, 1); assert.equal(c.snapshot().endpointCalls, 0);
});

test('direct console counters ignore malformed and prototype-name codes without retaining data', () => {
  const c = createDepartingConsoleCounters(() => 7);
  for (const args of [[], [{type: 'string', value: 'synthetic-private-prefix'}],
    [{type: 'string', value: 'KINOSAIL_R18_NATIVE_EVENT'}, {type: 'object', value: 'synthetic-private-claim'}],
    ...['__proto__', 'constructor', 'synthetic-private-body'].map(value => [
      {type: 'string', value: 'KINOSAIL_R18_NATIVE_EVENT'}, {type: 'string', value}])])
    c.observe({executionContextId: 7, args});
  const s = c.snapshot();
  assert.ok(Object.values(s).every(value => value === 0)); assert.ok(!Object.hasOwn(s, 'constructor'));
  assert.ok(!JSON.stringify(s).includes('synthetic-private'));
});

test('direct console counters saturate without retaining event arguments or context identifiers', () => {
  const c = createDepartingConsoleCounters(() => 731);
  for (let index = 0; index < 100; index++) c.observe({executionContextId: 731, args: [
    {type: 'string', value: 'KINOSAIL_R18_NATIVE_EVENT'}, {type: 'string', value: 'ENDPOINT_CALLED'}]});
  const s = c.snapshot();
  assert.equal(s.endpointCalls, 8); assert.equal(s.saturated, 1);
  assert.ok(Object.values(s).every(value => Number.isInteger(value) && value >= 0 && value <= 8));
  assert.ok(!JSON.stringify(s).includes('731'));
});
