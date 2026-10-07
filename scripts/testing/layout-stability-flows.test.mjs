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
      this.emit("response", {request:()=>request, url:()=>this.current, status:()=>200});
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
  assert.deepEqual(probe.navigation?.mainFrameResponses, [{path:"/watch",status:200}]);
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
