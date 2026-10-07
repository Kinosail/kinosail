import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createRequire, stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';
import {EventEmitter} from 'node:events';
import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {dirname, join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {navigationDiagnostics} from './navigation-diagnostics.mjs';

// Exercise the public journey registration with a controlled runner, not a
// browser. Observation must preserve its original actions and thrown error.
const source = stripTypeScriptTypes(readFileSync(new URL('../../apps/player/e2e/happy-path.spec.ts', import.meta.url), 'utf8'))
  .replace(/^import .*;\n/gm, '');
function journey(start, complete) {
  let run, budget, retry;
  const register = (_, options, callback) => {assert.equal(options.tag, '@smoke'); run = callback;};
  register.describe = {configure: options => {retry = options.retries;}};
  register.setTimeout = value => {budget = value;};
  runInNewContext(source, {test: register, navigationDiagnostics, startHappyPath: start, completeHappyPath: complete, setTimeout, clearTimeout, Buffer});
  return {run: (...args) => run(...args), settings: () => ({budget, retry})};
}
class Page extends EventEmitter {
  calls = [];
  async addInitScript(callback, argument) {this.calls.push('init'); this.script = callback; this.argument = argument;}
  async evaluate() {return {readyState:'complete'};}
  url() {return 'https://owned.fixture/watch/0123456789abcdef';}
}
for (const attachment of ['ok', 'reject', 'stall']) test(`WebKit playback failure stays original when attachment ${attachment}`, async () => {
  const page = new Page(), failure = new Error('private-synthetic-marker'), attached = [];
  const owner = journey(async target => {target.calls.push('start'); return {state:true};}, async target => {target.calls.push('complete'); throw failure;});
  const info = {project:{name:'webkit', use:{baseURL:'https://owned.fixture', defaultBrowserType:'webkit'}},
    attach: async (name, options) => {attached.push({name, options}); if(attachment === 'reject') throw new Error('private'); if(attachment === 'stall') await new Promise(() => {});}};
  let timer;
  try {await Promise.race([assert.rejects(owner.run({page}, info), error => error === failure),
    new Promise((_, reject) => {timer=setTimeout(() => reject(new Error('observation did not settle')), 1500);})]);}
  finally {clearTimeout(timer);}
  assert.deepEqual(page.calls, ['init','start','complete']); assert.deepEqual(owner.settings(), {budget:90000,retry:0});
  assert.equal(attached.length,1); assert.equal(page.eventNames().length,0);
  const value=JSON.parse(attached[0].options.body); assert.equal(value.errorCategory,'other');
  assert.doesNotMatch(JSON.stringify(attached), /private-synthetic-marker|0123456789abcdef/);
});
test('playback observer remains opt-in and successful Owner actions still complete once', async () => {
 for(const defaultBrowserType of ['chromium','webkit']) {
  const page=new Page(),attached=[];
  const owner=journey(async target=>{target.calls.push('start');return {};},async target=>{target.calls.push('complete');});
  await owner.run({page},{project:{use:{baseURL:'https://owned.fixture',defaultBrowserType}},attach:async(name, options)=>attached.push({name,options})});
  assert.deepEqual(page.calls,['init','start','complete']);assert.equal(attached.length,defaultBrowserType==='webkit'?1:0);assert.equal(page.eventNames().length,0);
 }
});
test('injected playback observations are bounded and idempotent per document', async () => {
 const page=new Page(),owner=journey(async()=>({}),async()=>{});
 await owner.run({page},{project:{use:{baseURL:'https://owned.fixture',defaultBrowserType:'webkit'}},attach:async()=>{}});
 const events=new Map(),records=[];
 const addEventListener=(name,callback)=>{const list=events.get(name)||[];list.push(callback);events.set(name,list);};
 class Video {}
 const video=Object.assign(new Video(),{currentSrc:'blob:private-synthetic-marker',currentTime:2,readyState:3,networkState:2,paused:false});
 const context={window:{},document:{querySelector:()=>video,visibilityState:'visible',addEventListener},addEventListener,
  HTMLVideoElement:Video,location:{href:'https://owned.fixture/watch/0123456789abcdef'},URL,console:{debug:(...values)=>records.push(values.join(' '))}};
 const script=`(${page.script.toString()})(${JSON.stringify(page.argument)})`;
 runInNewContext(script, context);runInNewContext(script, context);
 for(const callback of events.get('DOMContentLoaded')||[])callback();
 const invoke=name=>{for(const callback of events.get(name)||[])callback({target:video});};
 invoke('playing');invoke('kinosail:navigation');invoke('pagehide');
 assert.equal(records.length,3);assert.ok(records.some(value=>value.includes('navigation')));
 for(let n=0;n<1000;n++)invoke('playing');assert.equal(records.length,64);
 assert.doesNotMatch(JSON.stringify(records),/private-synthetic-marker/);
});

test('Owner observer captures the actual non-bubbling player navigation event', async () => {
 const page=new Page(),owner=journey(async()=>({}),async()=>{});
 await owner.run({page},{project:{use:{baseURL:'https://owned.fixture',defaultBrowserType:'webkit'}},attach:async()=>{}});
 const listeners=[],records=[];
 class Video extends EventTarget {}
 const video=Object.assign(new Video(),{currentSrc:'blob:private-synthetic-marker',currentTime:2,readyState:3,networkState:2,paused:false});
 const context={window:{},document:{querySelector:()=>video,visibilityState:'visible',addEventListener:()=>{}},
  addEventListener:(name,callback,options)=>listeners.push({name,callback,capture:options?.capture===true}),
  HTMLVideoElement:Video,location:{href:'https://owned.fixture/watch/0123456789abcdef'},URL,
  console:{debug:(...values)=>records.push(values.join(' '))}};
 runInNewContext(`(${page.script.toString()})(${JSON.stringify(page.argument)})`,context);
 // Node supplies the real Event/target dispatch; this controlled ancestor path
 // models capture versus bubbling only. Actual DOM propagation is a browser gate.
 const event=new Event('kinosail:navigation');
 assert.equal(event.bubbles,false);
 for(const listener of listeners.filter(row=>row.name===event.type&&row.capture))listener.callback(event);
 video.dispatchEvent(event);
 if(event.bubbles)for(const listener of listeners.filter(row=>row.name===event.type&&!row.capture))listener.callback(event);
 assert.equal(records.length,1);
 assert.equal(JSON.parse(records[0].slice('kinosail-playback-lifecycle '.length)).event,'navigation');
 assert.doesNotMatch(JSON.stringify(records),/private-synthetic-marker|0123456789abcdef/);
});

// Loading the pinned runner's resolved configuration starts no tests or browsers.
// Its transform cache is owned by this control, never the dependency tree.
test('actual resolved Safari project activates playback observation without browserName', async () => {
 const directory=fileURLToPath(new URL('../../apps/player/e2e/',import.meta.url));
 const require=createRequire(join(directory,'package.json'));
 const packagePath=createRequire(require.resolve('@playwright/test')).resolve('playwright/package.json');
 const cache=mkdtempSync(join(tmpdir(),'kino-pw-config-'));
 const keys=['KINOSAIL_BROWSER_MATRIX','KINOSAIL_BROWSER_PROJECT','PWTEST_CACHE_DIR'];
 const before=Object.fromEntries(keys.map(key=>[key,process.env[key]]));
 let project;
 try {
  process.env.KINOSAIL_BROWSER_MATRIX='full';delete process.env.KINOSAIL_BROWSER_PROJECT;process.env.PWTEST_CACHE_DIR=cache;
  const {configLoader}=require(join(dirname(packagePath),'lib/common/index.js'));
  const resolved=await configLoader.loadConfig({configDir:directory,resolvedConfigFile:join(directory,'playwright.config.ts')});
  project=resolved.config.projects.find(value=>value.name==='webkit');
 } finally {
  for(const key of keys){if(before[key]===undefined)delete process.env[key];else process.env[key]=before[key];}
  rmSync(cache,{recursive:true,force:true});
 }
 assert.ok(project);assert.equal(project.use.defaultBrowserType,'webkit');assert.equal(project.use.browserName,undefined);
 const page=new Page(),attached=[],owner=journey(async()=>({}),async()=>{});
 await owner.run({page},{project,attach:async(name,options)=>attached.push({name,options})});
 assert.equal(attached.length,1);assert.equal(page.argument,true);assert.equal(page.eventNames().length,0);
});

// A provisional native navigation blocks page evaluation while its response
// remains held. Departure observations must already be delivered to Node.
function departureObserver() {
 const prefix=stripTypeScriptTypes(readFileSync(new URL('../../apps/player/e2e/player-hls-navigation.spec.ts', import.meta.url),'utf8')).replace(/^import .*;\n/gm,'').split('async function movingVideo',1)[0];
 return runInNewContext(`(()=>{${prefix};return {captureDepartures,departureFacts};})()`,{});
}
function departurePage(rejectSetup=false) {
 const page=new EventEmitter(),listeners=[];
 page.pending=false;page.evaluations=0;
 const video={},records=[];
 page.evaluate=async(callback,argument)=>{
  page.evaluations++;
  if(page.pending)throw new Error('controlled provisional navigation prevents page evaluation');
  if(rejectSetup)throw new Error('controlled observation setup failure');
  return runInNewContext(`(${callback.toString()})(${JSON.stringify(argument) ?? 'undefined'})`,{
   window:{addEventListener:(name,callback,options)=>listeners.push({name,callback,capture:options?.capture===true})},
   document:{querySelector:()=>video},console:{debug:text=>{records.push(text);page.emit('console',{text:()=>text});}}});
 };
 return {page,records,dispatch(){
  const event={target:video,bubbles:false};
  for(const row of listeners.filter(row=>row.capture))row.callback(event);
 }};
}
test('held native departure is observable without evaluating the provisional document',async()=>{
 const api=departureObserver(),control=departurePage();
 const cleanup=await api.captureDepartures(control.page);
 control.dispatch();control.page.pending=true;
 assert.deepEqual(JSON.parse(JSON.stringify(await api.departureFacts(control.page))),{capture:1,bubble:0,player:1,bubbling:0});
 assert.equal(control.page.evaluations,1);
 cleanup();assert.equal(control.page.listenerCount('console'),0);
});
test('pending or cancelled departure stays zero; malformed console markers cannot change it',async()=>{
 const api=departureObserver(),control=departurePage();
 const cleanup=await api.captureDepartures(control.page);control.page.pending=true;
 for(const text of ['__kinosail_hls_departure__[1,0,1,0,99]','__kinosail_hls_departure__[999,0,1,0]',
  '__kinosail_hls_departure__[1,0,1,"private"]','__kinosail_hls_departure__'+ 'x'.repeat(1024)])control.page.emit('console',{text:()=>text});
 assert.deepEqual(JSON.parse(JSON.stringify(await api.departureFacts(control.page))),{capture:0,bubble:0,player:0,bubbling:0});
 assert.equal(control.page.evaluations,1);cleanup();assert.equal(control.page.listenerCount('console'),0);
});
test('rejected departure observation setup releases its outer console listener',async()=>{
 const api=departureObserver(),control=departurePage(true);
 await assert.rejects(api.captureDepartures(control.page),/controlled observation setup failure/);
 assert.equal(control.page.evaluations,1);assert.equal(control.page.listenerCount('console'),0);
});
