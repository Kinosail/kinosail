// Isolated gap: a populated journey cannot prove native Promise/argument identity
// or absence of response-body reads by its passive diagnostic wrapper.
import assert from 'node:assert/strict';
import { installFetchWitness, equalDiagnosticHeaders } from './subtitle-restore-causal-witness.mjs';
assert.equal(equalDiagnosticHeaders([null],["absent"]), false);
assert.equal(equalDiagnosticHeaders([null],[null]), true);
assert.equal(equalDiagnosticHeaders(["0"],["0"]), true);
assert.equal(equalDiagnosticHeaders(["0"],[null]), false);

const originalFetch = globalThis.fetch;
const listeners = new Map();
const originalAdd = globalThis.addEventListener, originalRemove = globalThis.removeEventListener;
globalThis.addEventListener = (name, fn) => listeners.set(name, fn);
globalThis.removeEventListener = (name, fn) => { if (listeners.get(name) === fn) listeners.delete(name); };
try {
  let resolve, reject, args, receiver;
  const nativePromise = new Promise((yes, no) => { resolve = yes; reject = no; });
  const native = function (...values) { args = values; receiver = this; return nativePromise; };
  globalThis.fetch = native;
  const witness = installFetchWitness({ origin: 'https://127.0.0.1:1', item: 'a'.repeat(16) });
  const input = '/api/v1/subtitle-library/' + 'a'.repeat(16) + '/restore';
  const options = { method: 'POST', credentials: 'same-origin', body: '{"language":"en"}' };
  const called = globalThis.fetch.call(globalThis, input, options);
  assert.equal(called, nativePromise);
  assert.equal(args[0], input); assert.equal(args[1], options); assert.equal(receiver, globalThis);
  assert.equal(Object.hasOwn(options, 'signal'), false);
  resolve({ status: 204, json() { throw Error('body must not be read'); }, get body() { throw Error('body must not be read'); } });
  await called; await Promise.resolve();
  assert.equal(witness.read().restore.outcome, 'fulfilled');
  assert.equal(witness.read().restore.status, 204);
  witness.close(); assert.equal(globalThis.fetch, native); assert.equal(listeners.size, 0);

  const rejected = new Promise((yes, no) => { reject = no; });
  globalThis.fetch = () => rejected;
  const failure = installFetchWitness({ origin: 'https://127.0.0.1:1', item: 'a'.repeat(16) });
  const failed = globalThis.fetch(input, options); assert.equal(failed, rejected);
  const exactError = new Error('private raw URL must not be projected'); reject(exactError);
  await assert.rejects(failed, error => error === exactError); await Promise.resolve();
  const safe = failure.read(); assert.equal(safe.restore.outcome, 'rejected');
  assert.equal(JSON.stringify(safe).includes(exactError.message), false); failure.close();

  for (const config of [{origin:'https://remote.invalid',item:'a'.repeat(16)},
    {origin:'https://127.0.0.1:1',item:'../../private'},
    {origin:'https://127.0.0.1:1',item:'a'.repeat(16),extra:true}]) {
    const before = globalThis.fetch; assert.throws(() => installFetchWitness(config)); assert.equal(globalThis.fetch, before);
  }
  console.log('native Promise/argument/receiver identity, no body reads, rejection identity and safe projection controls pass');
} finally {
  globalThis.fetch = originalFetch; globalThis.addEventListener = originalAdd; globalThis.removeEventListener = originalRemove;
}
