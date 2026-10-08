import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createContext, runInContext} from 'node:vm';
import {randomBytes} from 'node:crypto';

const controls = readFileSync(new URL('../../packages/webassets/static/downloads-control.js', import.meta.url), 'utf8');
const transfers = readFileSync(new URL('../../packages/webassets/static/downloads-transfer.js', import.meta.url), 'utf8');
const locks = navigator.locks;
const tick = () => new Promise(resolve => setImmediate(resolve));
function signal() {let resolve; const promise = new Promise(yes => {resolve = yes;}); return {promise, resolve};}
async function peer(capability = 'held') {
  const job = randomBytes(8).toString('hex'), key = 'kinosail-offline:' + job;
  const entered = signal(), release = signal(), event = new EventTarget(), document = new EventTarget();
  const owner = locks.request(key, async () => {entered.resolve(); await release.promise;});
  await entered.promise;
  const button = {dataset:{jobId:job}, isConnected:true, textContent:'Download to this device', disabled:false};
  let profile = 'viewer', requests = 0;
  document.querySelector = () => button;
  const api = {request: (name, options, callback) => {
    requests++; assert.equal(name,key); assert.equal(options.mode,'shared');
    if (capability === 'rejected-request') return Promise.reject(new Error('unavailable'));
    return locks.request(name,options,callback);
  }};
  if (capability !== 'missing-query') api.query = capability === 'rejected-query' ?
    () => Promise.reject(new Error('query unavailable')) : () => locks.query();
  if (capability === 'missing-request') api.request = undefined;
  const context = createContext({navigator:{locks:api}, document, AbortController, AbortSignal,
    addEventListener:event.addEventListener.bind(event), offlineTransfers:new Map(),
    currentOfflineProfile: () => profile, activeOfflineProfile: () => profile,
    offlineJobLockName: id => 'kinosail-offline:' + id, offlineMessage: (_, value) => value,
    offlineNow: () => performance.timeOrigin + performance.now(), console});
  runInContext(transfers + '\n' + controls,context,{timeout:1000});
  const statuses = runInContext('offlineStatuses',context), owners = context.offlineTransfers;
  const sync = runInContext('syncOfflineDownloadButton',context);
  const pending = new Set();
  const deliver = state => {
    const detail = {state,error:'paused'}; statuses.set(job,detail);
    const result = sync(button,detail); pending.add(result); result.finally(() => pending.delete(result));
    return result;
  };
  return {button,job,statuses,owners,sync,deliver,event,requests: () => requests,
    profile: value => {profile = value;}, release:release.resolve,
    async close() {release.resolve(); event.dispatchEvent(new Event('pagehide')); await owner; await Promise.all(pending);}};
}
test('twenty equivalent notifications retain exactly one real shared lock waiter', async () => {
  const p = await peer();
  try {
    p.deliver('needs_attention'); await tick();
    for (let n=0;n<20;n++) p.deliver('needs_attention');
    await tick(); assert.equal(p.requests(),1);
    assert.notEqual(p.button.textContent,'Resume on this device');
    p.release(); await tick(); await tick();
    assert.equal(p.button.textContent,'Resume on this device'); assert.equal(p.button.disabled,false);
  } finally {await p.close();}
});
test('missing and rejected query still await the real exclusive lock release', async () => {
  for (const mode of ['missing-query','rejected-query']) {
    const p = await peer(mode);
    try {
      const completed = p.deliver('needs_attention'); await tick();
      assert.equal(p.requests(),1); assert.notEqual(p.button.textContent,'Resume on this device');
      p.release(); await completed;
      assert.equal(p.button.textContent,'Resume on this device');
    } finally {await p.close();}
  }
});
test('missing or rejected request cannot enable Resume', async () => {
  for (const mode of ['missing-request','rejected-request']) {
    const p = await peer(mode);
    try {await p.deliver('needs_attention'); p.release(); await tick(); assert.notEqual(p.button.textContent,'Resume on this device');}
    finally {await p.close();}
  }
});
test('later state, owner, identity and lifecycle never release stale Resume UI', async () => {
  for (const boundary of ['ready','transferring','unknown','owner','profile','pagehide','detached','same-profile']) {
    const p = await peer();
    try {
      p.deliver('needs_attention'); await tick();
      if (['ready','transferring','unknown'].includes(boundary)) p.deliver(boundary);
      else if (boundary === 'owner') {p.owners.set(p.job,{controller:new AbortController()}); p.deliver('needs_attention');}
      else if (boundary === 'profile' || boundary === 'same-profile') {
        if (boundary === 'profile') p.profile('other');
        p.event.dispatchEvent(new Event('kinosail:offline-profile'));
      } else if (boundary === 'detached') p.button.isConnected = false;
      else p.event.dispatchEvent(new Event('pagehide'));
      p.release(); await tick(); await tick();
      assert.equal(p.button.textContent === 'Resume on this device', boundary === 'same-profile', boundary);
      if (boundary === 'owner') assert.equal(p.button.textContent,'Pause download');
    } finally {await p.close();}
  }
});
test('malformed job or profile causes no lock or button side effects', async () => {
  for (const value of [undefined,'','ABCDEF0123456789','../foreign','a'.repeat(17)]) {
    const p = await peer();
    try {p.button.dataset.jobId = value; await p.deliver('needs_attention'); assert.equal(p.requests(),0); assert.equal(p.button.textContent,'Download to this device');}
    finally {await p.close();}
  }
  for (const value of [undefined,'','../foreign','x'.repeat(257)]) {
    const p = await peer();
    try {p.profile(value); await p.deliver('needs_attention'); assert.equal(p.requests(),0); assert.equal(p.button.textContent,'Download to this device');}
    finally {await p.close();}
  }
});
test('invalidating a pending identity cancels its real shared waiter', async () => {
  for (const boundary of ['job','profile','detached']) {
    const p = await peer();
    try {
      let completed = false;
      const waiting = p.deliver('needs_attention').then(() => {completed = true;});
      await tick(); assert.equal(completed,false);
      if (boundary === 'job') p.button.dataset.jobId = '../foreign';
      else if (boundary === 'profile') p.profile('../foreign');
      else p.button.isConnected = false;
      await p.deliver('needs_attention'); await tick();
      assert.equal(completed,true,boundary); await waiting;
      assert.equal(p.requests(),1); assert.notEqual(p.button.textContent,'Resume on this device');
    } finally {await p.close();}
  }
});
