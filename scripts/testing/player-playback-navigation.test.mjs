import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';
import {EventEmitter} from 'node:events';
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
  const info = {project:{name:'webkit', use:{baseURL:'https://owned.fixture', browserName:'webkit'}},
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
 for(const browserName of ['chromium','webkit']) {
  const page=new Page(),attached=[];
  const owner=journey(async target=>{target.calls.push('start');return {};},async target=>{target.calls.push('complete');});
  await owner.run({page},{project:{use:{baseURL:'https://owned.fixture',browserName}},attach:async(name, options)=>attached.push({name,options})});
  assert.deepEqual(page.calls,['init','start','complete']);assert.equal(attached.length,browserName==='webkit'?1:0);assert.equal(page.eventNames().length,0);
 }
});
test('injected playback observations are bounded and idempotent per document', async () => {
 const page=new Page(),owner=journey(async()=>({}),async()=>{});
 await owner.run({page},{project:{use:{baseURL:'https://owned.fixture',browserName:'webkit'}},attach:async()=>{}});
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
