import {test} from "node:test";
import assert from "node:assert/strict";
import {EventEmitter} from "node:events";
import {measureFlows} from "./layout-stability-flows.mjs";
import {observeLayoutFlow} from "./layout-stability-flow-page.mjs";

// A later flow can fail after earlier layout pages have closed. Its transport
// witness must come from the actual failed page, before that context closes.
test("theater navigation failure retains its own page and closes owned contexts", async () => {
  const contexts = [];
  const failure = Object.assign(new Error("private navigation detail"), {name:"TimeoutError"});
  class Page extends EventEmitter {
    constructor(fail) {super(); this.fail = fail; this.current = "about:blank";}
    mainFrame() {return this;}
    url() {return this.current;}
    async goto(path, options) {
      assert.equal(options.waitUntil, "domcontentloaded");
      this.current = "https://owned.fixture.invalid" + path;
      const request = {url:()=>this.current, resourceType:()=>"document", isNavigationRequest:()=>true, frame:()=>this};
      this.emit("response", {request:()=>request, url:()=>this.current, status:()=>200, fromServiceWorker:()=>false});
      if (this.fail) throw failure;
    }
    locator() {return {count:async()=>0};}
    async evaluate() {return {readyState:"interactive", libraryMarker:false};}
  }
  const browser = {async newContext() {
    const page = new Page(contexts.length === 2);
    const context = {closed:false, newPage:async()=>page, async close() {this.closed=true; page.emit("close");}};
    contexts.push({context,page});
    return context;
  }};
  const probe = {};
  await assert.rejects(measureFlows(browser, {baseURL:"https://owned.fixture.invalid"}, "/watch/0123456789abcdef?token=private", undefined, [], probe), error=>error===failure);
  assert.equal(probe.stage, "theater-idle-exit");
  assert.equal(probe.operationPhase,"navigation");
  assert.equal(probe.navigation?.errorCategory, "timeout");
  assert.equal(probe.navigation?.readyState, "interactive");
  assert.equal(probe.navigation?.path, "/watch");
  assert.deepEqual(probe.navigation.lifecycle.find(record=>record.kind==="navigation-start")?.route, {path:"/watch",view:"other"});
  assert.deepEqual(probe.navigation?.mainFrameResponses.map(({path,status})=>({path,status})), [{path:"/watch",status:200}]);
  assert.equal(probe.navigation.mainFrameResponses[0].requestID,1);
  assert.equal(probe.navigation.mainFrameResponses[0].fromServiceWorker,false);
  assert.ok(probe.navigation.mainFrameResponses[0].timeMs>=probe.navigation.mainFrameResponses[0].firstSeenMs);
  assert.doesNotMatch(JSON.stringify(probe), /private|0123456789abcdef|fixture.invalid/);
  assert.ok(contexts.every(({context})=>context.closed));
  assert.ok(contexts.every(({page})=>["request","requestfailed","response"].every(event=>page.listenerCount(event)===0)));
});

test("flow page creation failures clear stale witnesses and still close the context", async () => {
  let closed = false, invoked = false;
  const error = new Error("private page creation failure");
  const probe = {navigation:{path:"/",errorCategory:"none"},operationPhase:"stale",elapsedMs:100};
  const context = {newPage:async()=>{throw error;},close:async()=>{closed=true;}};
  await assert.rejects(observeLayoutFlow(context,"https://owned.fixture.invalid",probe,async()=>{invoked=true;}), value=>value===error);
  assert.equal(closed,true); assert.equal(invoked,false);
  assert.deepEqual(probe,{});
});

test("cleanup failure cannot overwrite a failed flow's transport witness", async () => {
  const page = new EventEmitter();
  page.url = ()=>"about:blank";
  page.evaluate = async()=>({readyState:"loading"});
  const context = {newPage:async()=>page,close:async()=>{throw new Error("private cleanup detail");}};
  const error = Object.assign(new Error("private navigation detail"),{name:"TimeoutError"}), probe = {};
  await assert.rejects(observeLayoutFlow(context,"https://owned.fixture.invalid",probe,async()=>{throw error;}),value=>value===error);
  assert.equal(probe.navigation.errorCategory,"timeout");
  assert.equal(probe.navigation.identity,"blank");
  assert.equal(page.listenerCount("response"),0);
  assert.doesNotMatch(JSON.stringify(probe),/private/);
});

for (const observation of ['target', 'missing', 'rejected', 'stalled']) test(`Library ${observation} document observation preserves original DCL failure and cleanup`, async () => {
  const failure = Object.assign(new Error('private synthetic navigation detail'), {name: 'TimeoutError'});
  let closed = false;
  class Page extends EventEmitter {
    calls = []; installs = 0;
    mainFrame() {return this;}
    url() {return 'about:blank';}
    async addInitScript() {
      this.installs++;
      if (observation === 'rejected') throw new Error('private observer setup failure');
      if (observation === 'stalled') await new Promise(() => {});
    }
    async evaluate() {return {readyState: 'unavailable', timeOrigin: 3000};}
    async goto(target, options) {
      this.calls.push({target, options}); this.emit('domcontentloaded'); this.emit('load');
      if (observation === 'target') for (const value of [{kind: 'start', timeOrigin: 2000}, {kind: 'dcl', timeOrigin: 3000}])
        this.emit('console', {text: () => 'KINOSAIL_NAV_DOCUMENT ' + JSON.stringify({main: true, loginPath: false, setupPath: false, readyState: 'loading', elapsedMs: 0, loginForm: false, setupForm: false, ...value})});
      throw failure;
    }
  }
  const page = new Page(), probe = {};
  const browser = {newContext: async () => ({newPage: async () => page, close: async () => {closed = true;}})};
  let timer;
  try {
    await Promise.race([
      assert.rejects(measureFlows(browser, {baseURL: 'https://owned.fixture'}, '/watch/0123456789abcdef', '/inspect/0123456789abcdef', [], probe), value => value === failure),
      new Promise((_, reject) => {timer = setTimeout(() => reject(new Error('observer blocked original navigation')), 1600);}),
    ]);
  } finally {clearTimeout(timer);}
  assert.equal(closed, true); assert.equal(page.eventNames().length, 0);
  assert.equal(page.installs, 1);
  assert.deepEqual(page.calls, [{target: '/?view=library', options: {waitUntil: 'domcontentloaded'}}]);
  assert.equal(probe.navigation.identity, 'blank'); assert.equal(probe.navigation.readyState, 'unavailable');
  assert.deepEqual(probe.navigation.lifecycle[0].route, {path: '/', view: 'library'});
  assert.equal(probe.navigation.lifecycle.find(value => value.kind === 'domcontentloaded').route.path, 'blank');
  assert.deepEqual(probe.navigation.documents.map(value => [value.kind, value.timeOrigin]), observation === 'target' ? [['start', 2000], ['dcl', 3000]] : []);
  assert.ok(probe.navigation.documents.every(value => value.source === 'unverified-console'));
  assert.doesNotMatch(JSON.stringify(probe), /private|fixture|0123456789abcdef/);
});

for (const renderer of ['ready', 'rejected', 'stalled', 'invalid']) test(`nonthrowing theater ${renderer} miss retains witness and original actions/stable gate`, async () => {
  const calls = [], contexts = [], results = [], probe = {};
  class Page extends EventEmitter {
    constructor(theaterPage) {super(); this.theaterPage = theaterPage;}
    mainFrame() {return this;}
    url() {return 'https://owned.fixture/watch/0123456789abcdef';}
    async goto(path, options) {calls.push(['goto', path, options]);}
    async evaluate(callback) {
      // The independent navigation snapshot is not the media idle snapshot.
      if (String(callback).includes('libraryMarker')) return {readyState:'complete'};
      if (this.theaterPage) calls.push(['witness']);
      if (this.theaterPage && renderer === 'rejected') throw new Error('private renderer failure');
      if (this.theaterPage && renderer === 'stalled') await new Promise(() => {});
      if (this.theaterPage && renderer === 'invalid') return {documentReady: 'synthetic-secret', paused: 'synthetic-secret', ended: 'synthetic-secret', readyState: 99, errorCode: 9, currentTime: Infinity, theaterActive: 'synthetic-secret', theaterPressed: 'synthetic-secret', toolbarHidden: 'synthetic-secret'};
      return {documentReady: 'complete', paused: true, ended: false, readyState: 1, errorCode: 3, currentTime: 4, theaterActive: calls.some(value => value[0] === 'Theater'), theaterPressed: 'false', toolbarHidden: false, private: 'synthetic-secret'};
    }
    locator(selector) {
      if (selector === '.app-header input[name=q]') return {count: async () => 0};
      if (selector === '[data-theater]') return {isVisible: async () => true, click: async () => calls.push(['Theater'])};
      if (selector === '.media-stage') return {boundingBox: async () => ({x: 16, y: 200, width: 358, height: 272})};
      if (selector === '.player-stage-toolbar') return {evaluate: async () => {calls.push(['hidden-state']); return false;}, isVisible: async () => true};
      if (selector === 'video') return {evaluate: async fn => fn({readyState: 1, error: {code: 3}, canPlayType: () => 'probably', paused: true, loop: false})};
      throw new Error('unexpected public fixture selector');
    }
    getByRole(role, options) {assert.equal(role, 'button'); assert.equal(options.name, 'Play'); return {first: () => ({click: async () => calls.push(['Play'])})};}
    mouse = {move: async (x, y) => calls.push(['mouse', x, y])};
    keyboard = {press: async key => calls.push(['key', key])};
    async waitForTimeout(milliseconds) {calls.push(['wait', milliseconds]);}
  }
  const browser = {newContext: async () => {
    const context = {closed: false, newPage: async () => new Page(contexts.length === 3), close: async () => {context.closed = true;}};
    contexts.push(context); return context;
  }};
  await measureFlows(browser, {baseURL: 'https://owned.fixture'}, '/watch/0123456789abcdef', undefined, results, probe);
  const value = results.find(value => value.flow === 'theater-idle-exit');
  assert.equal(value.stable, false); assert.equal(value.hiddenAfterIdle, false); assert.equal(value.visibleAfterExit, true);
  if (renderer === 'ready') {
    assert.equal(value.witness.before.documentReady, 'complete'); assert.equal(value.witness.before.paused, true);
    assert.equal(value.witness.afterIdle.readyState, 1); assert.equal(value.witness.afterIdle.errorCode, 3);
    assert.equal(value.witness.afterIdle.currentTime, 4); assert.equal(value.witness.afterIdle.toolbarHidden, false);
    assert.equal(value.witness.afterIdle.theaterActive, true);
  } else {
    assert.equal(value.witness.before.documentReady, 'unavailable');
    assert.equal(value.witness.afterIdle.currentTime, 'unavailable');
    assert.equal(value.witness.afterIdle.readyState, 'unavailable');
    assert.equal(value.witness.afterIdle.toolbarHidden, 'unavailable');
  }
  assert.ok(Number.isFinite(value.witness.elapsedMs) && value.witness.elapsedMs >= 0);
  assert.ok(calls.findIndex(value => value[0] === 'hidden-state') < calls.findLastIndex(value => value[0] === 'witness'), 'idle gate is read before the after-idle diagnostic snapshot');
  assert.ok(calls.findLastIndex(value => value[0] === 'witness') < calls.findIndex(value => value[0] === 'key'), 'snapshot is dispatched before Escape without blocking it');
  assert.deepEqual(calls.filter(value => ['Play', 'Theater', 'mouse', 'wait', 'key'].includes(value[0])), [['Play'], ['Theater'], ['mouse', 0, 0], ['wait', 2700], ['key', 'Escape'], ['wait', 100], ['mouse', 31, 215], ['wait', 200]]);
  assert.ok(contexts.every(value => value.closed)); assert.doesNotMatch(JSON.stringify(value), /synthetic-secret|fixture|0123456789abcdef/);
});

import vm from "node:vm";
import {parseControlMarker,installPlaybackObservation,observedTheaterFlow} from "./layout-stability-theater-witness.mjs";
const marker={event:"toggle-capture",label:"Play",paused:true,ready:0,network:2,position:0,visible:"visible",focusVisible:false};
const send=value=>"kinosail-theater-control "+JSON.stringify(value);
test("valid control observation retains only fixed public state and explicitly unverified source",()=>{
 assert.deepEqual(parseControlMarker(send(marker)),{...marker,source:"unverified-console"});
});
test("missing unknown malformed oversized out-of-range and conflicting fields reject",()=>{
 const missing={...marker};delete missing.ready;
 for(const value of [missing,{...marker,event:"secret"},{...marker,label:"secret"},{...marker,paused:"true"},{...marker,ready:5},{...marker,network:-1},{...marker,position:-1},{...marker,position:31536001},{...marker,visible:"secret"},{...marker,focusVisible:1},{...marker,extra:"secret"}])assert.equal(parseControlMarker(send(value)),undefined);
 for(const value of ["kinosail-theater-control {","kinosail-theater-control "+"x".repeat(1025),send(marker).slice(0,-1)+',"ready":1}',send(marker).slice(0,-1)+',"\\u0072eady":1}'])assert.equal(parseControlMarker(value),undefined);
});
test("observer bounds records and releases its only listener",async()=>{
 const page=new EventEmitter();page.addInitScript=async()=>{};const records=[],navigation={observePlayback(){}};
 const stop=await installPlaybackObservation(page,navigation,records);
 for(let n=0;n<1000;n++)page.emit("console",{text:()=>send(marker)});
 assert.equal(records.length,64);stop();assert.equal(page.listenerCount("console"),0);
 page.emit("console",{text:()=>send(marker)});assert.equal(records.length,64);
});
test("observer setup rejection releases owned listener",async()=>{
 const page=new EventEmitter(),error=new Error("synthetic setup failure");page.addInitScript=async()=>{throw error;};
 await assert.rejects(installPlaybackObservation(page,{observePlayback(){}},[]),value=>value===error);assert.equal(page.listenerCount("console"),0);
});
test("rejected and stalled observation cannot replace the single original rejected navigation",async()=>{
 for(const kind of ["reject","stall"]){
  const page=new EventEmitter(),failure=new Error("original navigation failure");let calls=0,actions=0;
  page.addInitScript=()=>kind==="reject"?Promise.reject(new Error("observer failure")):new Promise(()=>{});
  const started=performance.now();
  await assert.rejects(observedTheaterFlow(page,{observePlayback(){}},[],async()=>{calls++;throw failure;}),error=>error===failure);
  assert.equal(calls,1);assert.equal(actions,0);assert.equal(page.listenerCount("console"),0);assert.ok(performance.now()-started<1000);
 }
});
test("double installation adds one passive listener per native event and never calls playback methods",async()=>{
 const page=new EventEmitter();let script;
 page.addInitScript=async value=>{script=value;};await installPlaybackObservation(page,{observePlayback(){}},[]);
 const registrations=[];let actions=0;const video={play(){actions++;},pause(){actions++;}};
 const context={window:{},document:{addEventListener:(name,listener,options)=>registrations.push({name,listener,options}),querySelector:()=>video},Symbol,console};
 vm.runInNewContext(`(${script.toString()})()`,context);vm.runInNewContext(`(${script.toString()})()`,context);
 assert.equal(registrations.length,9);assert.equal(new Set(registrations.map(record=>record.name)).size,9);assert.ok(registrations.every(record=>record.options.capture));assert.equal(actions,0);
});

for(const [event,label] of [["toggle-capture","Theater"],["toggle-capture","Exit theater"],["theater-capture","Play"],["theater-capture","Pause"]])test(`contradictory ${event}/${label} produces no record`,async()=>{
 const text=send({...marker,event,label});assert.equal(parseControlMarker(text),undefined);
 const page=new EventEmitter();page.addInitScript=async()=>{};const records=[];const stop=await installPlaybackObservation(page,{observePlayback(){}},records);page.emit("console",{text:()=>text});assert.equal(records.length,0);stop();assert.equal(page.listenerCount("console"),0);
});
