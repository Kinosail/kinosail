import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import {withOPFSPage, writeOrphanedOPFS} from '../../apps/player/e2e/offline-opfs-fixture.mjs';

const spec = readFileSync(new URL('../../apps/player/e2e/test-instance-large-offline-b.spec.ts', import.meta.url), 'utf8');
function workerPeer({constructorFails = false} = {}) {
  const events = []; let timeout, worker;
  const context = {Blob, URL:{createObjectURL:()=>{events.push('url');return 'blob:owned';},revokeObjectURL:()=>events.push('revoke')},
    setTimeout:(callback,ms)=>{assert.equal(ms,10000);timeout=callback;events.push('timer');return 1;},clearTimeout:()=>events.push('clear'),
    Worker:class {constructor(){events.push('worker');if(constructorFails)throw new Error('constructor failed');worker=this;}terminate(){events.push('terminate');}}};
  return {events, timeout:()=>timeout, worker:()=>worker,
    invoke:id=>runInNewContext('(' + writeOrphanedOPFS.toString() + ')(id)', {...context,id})};
}
test('worker constructor rejection still revokes the owned Blob URL',async()=>{
  const peer=workerPeer({constructorFails:true});await assert.rejects(peer.invoke('0123456789abcdef'));
  assert.deepEqual(peer.events,['url','worker','clear','revoke']);
});
test('a worker that never responds has an owned bounded deadline',async()=>{
  const peer=workerPeer();const pending=peer.invoke('0123456789abcdef');pending.catch(()=>{});
  assert.equal(typeof peer.timeout(),'function');peer.timeout()();await assert.rejects(pending,/deadline/);
  assert.deepEqual(peer.events,['url','worker','timer','clear','terminate','revoke']);
});
test('only the orphaned OPFS row registers the derived page fixture',async()=>{
  let fixture;
  const base={extend:values=>{fixture=values.page;return {};},use:()=>{}};
  const prefix=spec.slice(spec.indexOf('configureTestInstance();'),spec.indexOf('test.describe('));
  runInNewContext(prefix,{test:base,configureTestInstance:()=>{},withOPFSPage});
  assert.equal(typeof fixture,'function');
  assert.ok(spec.includes('opfsTest("offline resume does not count an orphaned OPFS write twice against quota"'));
  assert.ok(spec.includes('  test("offline resume accounts for replaced IndexedDB chunks near quota"'));
  const peer=contextPeer();let page;
  await fixture({...peer.input,playwright:{webkit:peer.input.webkit}},async value=>{page=value;}, {attach:async(name,body)=>{
    assert.equal(name,'offline-opfs-context');assert.deepEqual(JSON.parse(body.body),{schemaVersion:1,engine:'webkit',mode:'persistent-owned',serviceWorkers:'block',tlsBypass:false});
  }});
  assert.equal(page,peer.ownedPage);assert.deepEqual(peer.events,['launch','page','close']);
});
test('missing or malformed canonical OPFS job ID rejects before allocation or writes',async()=>{
  for(const value of [undefined,null,'','ABCDEF0123456789','../private','1'.repeat(17),'1'.repeat(4097)]) {
    const peer=workerPeer();await assert.rejects(peer.invoke(value));assert.deepEqual(peer.events,[]);
  }
});
test('worker messages keep strict boolean capability and reject malformed or oversized data',async()=>{
  for(const data of [null,[],{}, {supported:'false'}, {supported:true,extra:1}, {supported:true,error:'UnknownError'},
    {error:'private path or credential'}, {error:'x'.repeat(4097)}]) {
    const peer=workerPeer(), pending=peer.invoke('0123456789abcdef');peer.worker().onmessage({data});
    await assert.rejects(pending);assert.ok(peer.events.includes('terminate'));assert.ok(peer.events.includes('revoke'));
  }
  for(const supported of [true,false]) {
    const peer=workerPeer(), pending=peer.invoke('0123456789abcdef');peer.worker().onmessage({data:{supported}});
    assert.equal(await pending,supported);assert.equal(peer.worker().onmessage,null);assert.equal(peer.worker().onerror,null);assert.equal(peer.worker().onmessageerror,null);
  }
});
test('UnknownError and worker event failures cannot become unsupported skips',async()=>{
  const peer=workerPeer(), pending=peer.invoke('0123456789abcdef');peer.worker().onmessage({data:{error:'UnknownError'}});
  await assert.rejects(pending,error=>error.name==='UnknownError' && error.message==='OPFS orphan write failed');
  for(const name of ['onerror','onmessageerror']) {
    const other=workerPeer(), result=other.invoke('0123456789abcdef');other.worker()[name]({message:'private'});
    await assert.rejects(result,/OPFS worker failed/);assert.ok(other.events.includes('terminate'));assert.ok(other.events.includes('revoke'));
  }
});
test('actual worker writes the orphan prefix and closes; only API absence reports unsupported',async()=>{
  let source;
  const capture={Blob:class{constructor(parts){source=parts.join('');}},URL:{createObjectURL:()=> 'blob:owned',revokeObjectURL:()=>{}},
    setTimeout:()=>1,clearTimeout:()=>{},Worker:class{constructor(){queueMicrotask(()=>this.onmessage({data:{supported:true}}));}terminate(){}}};
  await runInNewContext('('+writeOrphanedOPFS.toString()+')(id)', {...capture,id:'0123456789abcdef'});
  const events=[], messages=[];
  const writer={write:bytes=>{assert.deepEqual(Array.from(bytes),Array(16).fill(1));events.push('write');},flush:()=>events.push('flush'),close:()=>events.push('close')};
  const environment={FileSystemFileHandle:class{},navigator:{storage:{getDirectory:async()=>({getFileHandle:async(id,options)=>{
    assert.equal(id,'0123456789abcdef');assert.equal(options.create,true);events.push('file');return {createSyncAccessHandle:async()=>writer};}})}},
    self:{postMessage:value=>messages.push(JSON.parse(JSON.stringify(value)))},Uint8Array};
  environment.FileSystemFileHandle.prototype.createSyncAccessHandle=()=>{};
  await runInNewContext(source,environment);assert.deepEqual(events,['file','write','flush','close']);assert.deepEqual(messages,[{supported:true}]);
  delete environment.FileSystemFileHandle.prototype.createSyncAccessHandle;events.length=0;messages.length=0;
  await runInNewContext(source,environment);assert.deepEqual(events,[]);assert.deepEqual(messages,[{supported:false}]);
  environment.FileSystemFileHandle.prototype.createSyncAccessHandle=()=>{};messages.length=0;
  environment.navigator.storage.getDirectory=async()=>{throw Object.assign(new Error('private'),{name:'UnknownError'});};
  await runInNewContext(source,environment);assert.deepEqual(messages,[{error:'UnknownError'}]);
  messages.length=0;events.length=0;
  environment.navigator.storage.getDirectory=async()=>({getFileHandle:async()=>({createSyncAccessHandle:async()=>writer})});
  writer.write=()=>{events.push('write');throw Object.assign(new Error('private'),{name:'QuotaExceededError'});};
  await runInNewContext(source,environment);assert.deepEqual(events,['write','close']);assert.deepEqual(messages,[{error:'QuotaExceededError'}]);
});
function contextPeer() {
  const events=[],ownedPage={owned:true};
  const context={pages:()=>{events.push('page');return [ownedPage];},close:async()=>events.push('close')};
  const input={page:{ordinary:true},browserName:'webkit',webkit:{launchPersistentContext:async(path,options)=>{
    assert.equal(path,'');assert.deepEqual(options,{baseURL:'http://127.0.0.1:38127',viewport:{width:1280,height:720},
      userAgent:'fixture-browser',deviceScaleFactor:1,isMobile:false,hasTouch:false,serviceWorkers:'block',ignoreHTTPSErrors:false,timeout:10000});
    events.push('launch');return context;}},baseURL:'http://127.0.0.1:38127',viewport:{width:1280,height:720},userAgent:'fixture-browser',deviceScaleFactor:1,isMobile:false,hasTouch:false};
  return {input,events,context,ownedPage};
}
test('persistent WebKit context closes after success or failure without replacing the body error',async()=>{
  const peer=contextPeer();await withOPFSPage(peer.input,async page=>{assert.equal(page,peer.ownedPage);peer.events.push('body');});
  assert.deepEqual(peer.events,['launch','page','body','close']);
  const other=contextPeer(), original=new Error('original body');other.context.close=async()=>{other.events.push('close');throw new Error('cleanup');};
  await assert.rejects(withOPFSPage(other.input,async()=>{throw original;}),error=>error===original);assert.ok(other.events.includes('close'));
  const failed=contextPeer();failed.context.close=async()=>{throw new Error('owned close failed');};
  await assert.rejects(withOPFSPage(failed.input,async()=>{}),/owned close failed/);
});
test('Chromium and Firefox keep their exact page and never launch another context',async()=>{
  for(const browserName of ['chromium','firefox']) {
    const peer=contextPeer();peer.input.browserName=browserName;let page;
    await withOPFSPage(peer.input,async value=>{page=value;});assert.equal(page,peer.input.page);assert.deepEqual(peer.events,[]);
  }
});
test('invalid options reject before launch, body, page allocation or attachment',async()=>{
  const bad=[{browserName:'unknown'},{baseURL:undefined},{baseURL:'http://127.0.0.1:0'}, {baseURL:'https://foreign.example'},
    {baseURL:'http://user@localhost:38127'},{baseURL:'http://localhost:38127/private'},{baseURL:'http://localhost:38127/?unknown'},
    {baseURL:'x'.repeat(2049)},{viewport:{width:-1,height:720}},{viewport:{width:1280,height:Infinity}},
    {viewport:{width:1280,height:720,unknown:1}},{deviceScaleFactor:0},{deviceScaleFactor:Infinity},{isMobile:'false'},
    {hasTouch:undefined},{userAgent:''},{userAgent:'x'.repeat(1025)},{unknown:true}];
  for(const values of bad) {
    const peer=contextPeer();let body=0;await assert.rejects(withOPFSPage({...peer.input,...values},async()=>body++));
    assert.equal(body,0);assert.deepEqual(peer.events,[]);
  }
});
test('CI invokes this exact focused control once',()=>{
  const workflow=readFileSync(new URL('../../.github/workflows/app.yml',import.meta.url),'utf8');
  assert.equal(workflow.split('\n').filter(line=>line.includes('run: node --test')&&line.includes('scripts/testing/offline-opfs-fixture.test.mjs')).length,1);
  const layout=readFileSync(new URL('../../.github/workflows/layout-stability.yml',import.meta.url),'utf8').split('  workflow_dispatch:',1)[0];
  for(const path of ['apps/player/e2e/offline-opfs-fixture.mjs','scripts/testing/offline-opfs-fixture.test.mjs'])
    assert.equal(layout.split('\n').filter(line=>line.includes('"'+path+'"')).length,1);
});



// Execute the actual registered quota callbacks with a controlled browser peer.
// This proves public action selection/negative side effects, not browser storage.
const quotaCases = new Map();
const register = (title, callback) => quotaCases.set(title, callback);
register.extend = () => register;
register.use = () => {};
register.describe = (_, body) => body();
register.skip = () => {};
register.info = () => ({attach: async()=>{}});
const quotaExpect = value => ({
  toBe: expected => assert.equal(value,expected),
  toEqual: expected => assert.deepEqual(value,expected),
  toBeTruthy: () => assert.ok(value),
  toMatch: pattern => assert.match(value,pattern),
  toHaveCount: async expected => assert.equal(value.count(),expected),
  toHaveAccessibleName: async pattern => assert.match(value.label(),pattern),
  toBeEnabled: async()=>assert.equal(value.enabled(),true),
  toBeVisible: async()=>assert.equal(value.visible,true),
});
globalThis.kinosailQuotaControl = {test:register,expect:quotaExpect};
const {registerHooks} = await import('node:module');
const quotaRoot = new URL('../../apps/player/e2e/',import.meta.url).href;
const hooks = registerHooks({resolve(specifier,context,next) {
  if(context.parentURL === quotaRoot+'test-instance-large-offline-b.spec.ts') {
    if(specifier === '@playwright/test') return {url:'data:text/javascript,'+encodeURIComponent('export const {test,expect}=globalThis.kinosailQuotaControl;'),shortCircuit:true};
    if(specifier === './test-instance-helpers') return {url:'data:text/javascript,'+encodeURIComponent('export const configureTestInstance=()=>{}, login=async()=>{}, firstPlayable=async()=>"/watch/0123456789abcdef", downloadsSource="const chunkSize = 8 * 1024 * 1024;";'),shortCircuit:true};
  }
  return next(specifier,context);
}});
try {await import('../../apps/player/e2e/test-instance-large-offline-b.spec.ts');}
finally {hooks.deregister();delete globalThis.kinosailQuotaControl;}
class QuotaActionPage {
  id='0123456789abcdef'; label='Resume on this device'; controlID=this.id;
  controls=1; disabled=false; writes=0; orphanWrites=0; fixtureRoutes=0; clicks=0;
  routes=[];
  async route(matcher,handler) {this.routes.push({matcher,handler});if(this.routes.length>1)this.fixtureRoutes++;}
  async addInitScript() {}
  async goto() {}
  getByText() {return {visible:true,click:async()=>{}};}
  getByRole(_, {name}) {
    if(name==='Prepare 720p offline')return {click:async()=>{}};
    assert.equal(name,'Download to this device');
    return {getAttribute:async key=>this.metadata(key),click:async()=>{assert.equal(this.label,'Download to this device','old action name became stale');await this.transfer();}};
  }
  metadata(key) {return {'data-job-id':this.id,'data-item-id':'fedcba9876543210','data-title':'Fixture movie','data-quality':'720p','data-viewer-profile':'owner'}[key];}
  locator(selector) {
    if(selector==='#downloads')return {getAttribute:async key=>this.metadata(key)};
    const matched=selector.match(/^\[data-download-device\]\[data-job-id="([a-f0-9]{16})"\]$/);
    assert.ok(matched,'only canonical exact job selector allowed');
    return {count:()=>this.controlID===matched[1]?this.controls:0,label:()=>this.label,enabled:()=>!this.disabled,click:async()=>this.transfer()};
  }
  async evaluate(callback,args) {
    if(callback===writeOrphanedOPFS){this.orphanWrites++;return true;}
    if(args && typeof args==='object' && 'id' in args){this.writes++;return;}
    return 33;
  }
  async transfer() {
    this.clicks++;
    const url=new URL('http://127.0.0.1:38127/api/v1/downloads/'+this.id+'/file');
    const route=this.routes.find(row=>row.matcher(url));
    assert.ok(route);
    for(const range of ['bytes=0-15','bytes=16-31','bytes=32-32'])
      await route.handler({request:()=>({headers:()=>({range})}),fulfill:async()=>{},abort:()=>{throw new Error('range rejected');}});
  }
}
for(const [title,body] of quotaCases) {
  for(const label of ['Download to this device','Resume on this device'])
    test('actual quota callback clicks the same enabled canonical job with '+label+': '+title,async()=>{
      const page=new QuotaActionPage();page.label=label;
      await body({page});assert.equal(page.clicks,1);assert.equal(page.writes,1);
    });
  for(const invalid of ['',null,'ABCDEF0123456789','../private','a'.repeat(17),'a'.repeat(4097)])
    test('actual quota callback rejects noncanonical job before local writes: '+title+' / '+String(invalid).length,async()=>{
      const page=new QuotaActionPage();page.id=invalid;
      await assert.rejects(body({page}));assert.equal(page.writes,0);assert.equal(page.orphanWrites,0);assert.equal(page.fixtureRoutes,0);assert.equal(page.clicks,0);
    });
  for(const values of [{label:'Pause download'},{label:'unknown'},{label:'Download to this device extra'},{controls:2},{controlID:'aaaaaaaaaaaaaaaa'},{disabled:true}])
    test('actual quota callback refuses an ambiguous/replaced/disabled public action: '+title+' / '+JSON.stringify(values),async()=>{
      const page=new QuotaActionPage();Object.assign(page,values);
      await assert.rejects(body({page}));assert.equal(page.clicks,0);
    });
}
