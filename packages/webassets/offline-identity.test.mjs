import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';
const identity = readFileSync(new URL('./static/offline-identity.js', import.meta.url), 'utf8');
const media = readFileSync(new URL('./static/offline-media.js', import.meta.url), 'utf8');
function page(profile, storage, failWrites = false) {
  const window = new EventTarget(), document = new EventTarget(), worker = new EventTarget();
  document.body = {dataset:{}};
  document.querySelector = selector => selector === '#downloads' ? {dataset:{viewerProfile:profile}} : null;
  class Form {action='https://kino.test/logout';}
  worker.postMessage = data => {worker.sent=data;};
  worker.controller = worker;
  vm.runInNewContext(identity, {window, document, navigator:{serviceWorker:worker}, localStorage:{getItem:key=>storage.get(key) ?? null,setItem:(key,value)=>{if(failWrites)throw new Error("quota");storage.set(key,value);}}, Event, HTMLFormElement:Form, URL, location:{href:'https://kino.test/'}});
  return {window, worker, document, Form};
}
function message(target, data) {
  const event = new Event('message');
  event.data = data;
  target.dispatchEvent(event);
}
test('existing tabs follow a new profile and ignore delayed old broadcasts', () => {
  const storage = new Map();
  const a=page('A',storage), old=a.window.kinosailOfflineIdentity.state();
  const b=page('B',storage);
  assert.equal(a.window.kinosailOfflineIdentity.current(),'B');
  message(a.worker,{type:'offline-profile',...old});
  assert.equal(a.window.kinosailOfflineIdentity.current(),'B');
  assert.equal(b.window.kinosailOfflineIdentity.current(),'B');
  const next=b.window.kinosailOfflineIdentity.state();
  message(a.worker,{type:'offline-profile',profile:'',revision:next.revision+1});
  assert.equal(a.window.kinosailOfflineIdentity.current(),'');
  assert.equal(b.window.kinosailOfflineIdentity.current(),'');
  assert.equal(page(undefined,storage).window.kinosailOfflineIdentity.current(),'');
});
test('authenticated downloads-only entry initializes the active profile',()=>{
  assert.equal(page('DownloadsViewer',new Map()).window.kinosailOfflineIdentity.current(),'DownloadsViewer');
});
function worker(storage) {
  const handlers=new Map(), fetched=[], broadcast=[];
  const self={location:{origin:'https://kino.test'},clients:{matchAll:async()=>[{postMessage:value=>broadcast.push(value)}]},addEventListener:(name,callback)=>handlers.set(name,callback)};
  const context=vm.createContext({self, URL, Date, Number, Promise, openOfflineDatabaseConnection(){}, offlineProfileState:storage, fetch:async request=>{fetched.push(request);return new Response(null,{status:204});}});
  vm.runInContext(media+'\nregisterOfflineLifecycle("cache","retired",()=>{});globalThis.current=selectedProfile;',context);
  return {context,handlers,fetched,broadcast};
}
for(const mode of ['rejected','blocked']) test(`logout reaches the Server when offline storage is ${mode}`,async()=>{
  const f=worker(()=>mode==='rejected'?Promise.reject(new Error('unavailable')):new Promise(()=>{}));
  let response;
  f.handlers.get('fetch')({request:new Request('https://kino.test/api/v1/session',{method:'DELETE'}),waitUntil:promise=>promise.catch(()=>{}),respondWith:value=>{response=value;}});
  assert.equal((await response).status,204);
  assert.equal(f.fetched.length,1);
  assert.equal(await f.context.current(),'');
});
test('worker rejects delayed profile selections after a newer transition',async()=>{
  const persisted=[]; let saved; const f=worker(async value=>{if(value!==undefined){persisted.push(value);saved=value;}return saved;});
  let pending;
  const send=data=>{f.handlers.get('message')({data,waitUntil:value=>{pending=value;}});return pending;};
  const revision=Date.now()-10;
  await send({type:'profile',profile:'B',revision:revision+1});
  await send({type:'profile',profile:'A',revision});
  assert.equal(await f.context.current(),'B');
  assert.deepEqual(persisted.map(value=>value.profile),['B']);
  await send({type:'logout',profile:'',revision:revision+2});
  assert.equal(await f.context.current(),'');
});

test('readable stale storage cannot override a newer in-memory selection',()=>{
  const storage=new Map();
  page('A',storage);
  const b=page('B',storage,true);
  assert.equal(b.window.kinosailOfflineIdentity.current(),'B');
  const current=b.window.kinosailOfflineIdentity.state();
  message(b.worker,{type:'offline-profile',profile:'',revision:current.revision+1});
  assert.equal(b.window.kinosailOfflineIdentity.current(),'');
});
test('a restarted worker preserves a newer durable logout',async()=>{
  const revision=Date.now()-10;
  let saved={profile:'',revision:revision+1}, writes=0;
  const f=worker(async value=>{if(value!==undefined){saved=value;writes++;}return saved;});
  let pending;
  f.handlers.get('message')({data:{type:'profile',profile:'A',revision},waitUntil:value=>{pending=value;}});
  assert.equal(await f.context.current(),'');
  await pending;
  assert.equal(await f.context.current(),'');
  assert.equal(writes,0);
});

test('server logout supersedes a durable revision ahead of the clock',async()=>{
  let saved={profile:'A',revision:Date.now()+1000};
  const f=worker(async value=>{if(value!==undefined)saved=value;return saved;});
  let pending, response;
  f.handlers.get('fetch')({request:new Request('https://kino.test/api/v1/session',{method:'DELETE'}),waitUntil:value=>{pending=value;},respondWith:value=>{response=value;}});
  await response; await pending;
  assert.equal(saved.profile,'');
  assert.equal(await f.context.current(),'');
});
