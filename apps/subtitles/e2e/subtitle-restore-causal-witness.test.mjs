// Isolated gap: a populated journey cannot prove native Promise/argument identity
// or absence of response-body reads by its passive diagnostic wrapper.
import assert from 'node:assert/strict';
import { installFetchWitness, equalDiagnosticHeaders } from './subtitle-restore-causal-witness.mjs';
import * as authority from './subtitle-restore-causal-witness.mjs';
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
  resolve({ status: 204, url:'https://127.0.0.1:1'+input, json() { throw Error('body must not be read'); }, get body() { throw Error('body must not be read'); } });
  await called; await Promise.resolve();
  assert.equal(witness.read().restore.outcome, 'fulfilled');
  assert.equal(witness.read().restore.status, 204);
  assert.deepEqual(witness.read().completion,{requestMatched:true,responseMatched:true});
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

// Isolated inverse gap: populated journeys cannot force every stale/mismatched witness.
assert.equal(typeof authority.nativeBodylessCompletion,'function');
const native204={outcome:'fulfilled',status:204,signalPresent:false,signalAborted:false,pagehide:false};
const aborted={terminal:'request-failed',failureCode:'aborted',resourceType:'Fetch',cancelled:true,navigation:false};
const delivered={seen:true,held:false,delivered:true,cancelled:false,timedOut:false,settled:true};
const diagnostic={schema:'r06-causal-v2',valid:true,reason:'none',headersEqual:true,framingEqual:true,stopped:true,
 probes:['direct','captured'].map(mode=>({mode,native:{...native204},network:{...aborted},server:{...delivered}})),
 restoreNative:{...native204},restoreNetwork:{...aborted},completion:{scope:'r06-native-bodyless-204-v1',
 caseID:'r06-restore-headers-desktop',browserVersion:'153.0.8010.12',pinnedBrowserMatched:true,
 requestMatched:true,responseMatched:true,bodyless:true,fixtureMatched:true}};
diagnostic.probes.push({mode:'held',native:{...native204,outcome:'rejected',status:0,signalPresent:true,signalAborted:true},
 network:{...aborted},server:{...delivered,held:true,delivered:false,cancelled:true}});
const state={caseID:'r06-restore-headers-desktop',protocol:'legacy',responseStatus:204,prepareAttempts:0,
 setupSaveAttempts:1,restoreAttempts:1,restoreRequestObserved:true,restoreResponseObserved:true,
 restoreTerminal:'request-failed',restoreFailureCode:'aborted',restoreResponseDelivered:true,restoreClientCancelled:false,
 eligible:true,actualRestored:true,historyOnce:true,recoverySwapped:true,inspectionMatches:true,holdEligible:true,
 holdExpired:false,boundaryFailed:false,causalDiagnostic:diagnostic};
assert.equal(authority.nativeBodylessCompletion(state),true);
for(const field of ['pinnedBrowserMatched','requestMatched','responseMatched','bodyless','fixtureMatched']){
 const bad=structuredClone(state);bad.causalDiagnostic.completion[field]=false;assert.equal(authority.nativeBodylessCompletion(bad),false);
}
for(const [key,value]of [['protocol','prepared'],['responseStatus',202],['restoreAttempts',2],['holdExpired',true],
 ['restoreClientCancelled',true],['restoreResponseObserved',false],['restoreFailureCode','unclassified']]){
 const bad=structuredClone(state);bad[key]=value;assert.equal(authority.nativeBodylessCompletion(bad),false);
}
for(const [key,value]of [['outcome','rejected'],['status',200],['signalPresent',true],['signalAborted',true],['pagehide',true]]){
 const bad=structuredClone(state);bad.causalDiagnostic.restoreNative[key]=value;assert.equal(authority.nativeBodylessCompletion(bad),false);
}
const badControl=structuredClone(state);badControl.causalDiagnostic.probes[2].server.timedOut=true;
assert.equal(authority.nativeBodylessCompletion(badControl),false);
const privateData=structuredClone(state);privateData.causalDiagnostic.restoreNative.rawURL='private';
assert.equal(authority.nativeBodylessCompletion(privateData),false);
const old=structuredClone(state);old.causalDiagnostic.schema='r06-causal-v1';delete old.causalDiagnostic.completion;
assert.equal(authority.nativeBodylessCompletion(old),false);
console.log('bodyless204 authority positive and inverse controls pass without changing raw terminals');
