import {test} from "node:test";
import assert from "node:assert/strict";
import {EventEmitter} from "node:events";
import {measureSubtitleBackground} from "./layout-stability-subtitle-background.mjs";

// Isolated support seam: an actual navigation failure must retain this flow's
// safe page/network state before context teardown, rather than another page's
// old elapsed time and geometry phase. No product timing is simulated here.
test("background navigation failure snapshots its active page before closure", async () => {
  const page=new EventEmitter(), frame={};
  let closed=false;
  page.mainFrame=()=>frame;
  page.url=()=>"https://owned-layout.invalid/?view=library&token=do-not-record";
  page.evaluate=async()=>{assert.equal(closed,false);return {readyState:"complete",libraryMarker:true};};
  page.goto=async()=>{
    page.emit("response",{status:()=>200,url:()=>page.url(),request:()=>({url:()=>page.url(),resourceType:()=>"document",isNavigationRequest:()=>true,frame:()=>frame}),fromServiceWorker:()=>false});
    const error=new Error("Timeout at https://private.invalid/do-not-record");error.name="TimeoutError";throw error;
  };
  const browser={newContext:async()=>({newPage:async()=>page,close:async()=>{closed=true;}})};
  const probe={stage:"old-geometry",operationPhase:"settled-inspect",navigation:{elapsedMs:600000}}, results=[];
  const prior=process.env.KINOSAIL_LAYOUT_MEDIA_ROOT;
  process.env.KINOSAIL_LAYOUT_MEDIA_ROOT="/owned/.verification/layout/fixture/media";
  try {
    await assert.rejects(measureSubtitleBackground(browser,{baseURL:"https://owned-layout.invalid"},results,probe),{name:"TimeoutError"});
    assert.equal(probe.stage,"subtitle-background-navigation");
    assert.equal(probe.operationPhase,"navigation");
    assert.equal(probe.navigation.path,"/");
    assert.equal(probe.navigation.view,"library");
    assert.equal(probe.navigation.errorCategory,"timeout");
    assert.equal(probe.navigation.libraryMarker,true);
    assert.deepEqual(probe.navigation.mainFrameResponses.map(({path,status})=>({path,status})),[{path:"/",status:200}]);
    assert.equal(probe.navigation.mainFrameResponses[0].requestID,1);
    assert.equal(probe.navigation.mainFrameResponses[0].fromServiceWorker,false);
    assert.ok(probe.navigation.mainFrameResponses[0].timeMs>=probe.navigation.mainFrameResponses[0].firstSeenMs);
    assert.ok(probe.navigation.elapsedMs<5000);
    assert.ok(!JSON.stringify(probe).includes("do-not-record"));
    assert.equal(closed,true);
    assert.deepEqual(results,[]);
    for(const event of ["request","requestfinished","requestfailed","response"])assert.equal(page.listenerCount(event),0);
  } finally {
    if(prior===undefined)delete process.env.KINOSAIL_LAYOUT_MEDIA_ROOT;else process.env.KINOSAIL_LAYOUT_MEDIA_ROOT=prior;
  }
});

for (const observation of ["target", "malformed", "oversized", "rejected", "stalled"]) test(`background ${observation} observation keeps one original navigation and cleanup`, async () => {
  const failure = Object.assign(new Error("private original navigation"), {name:"TimeoutError"});
  let closed = false, timer;
  class Page extends EventEmitter {
    calls = []; installs = 0;
    mainFrame() {return this;}
    url() {return "about:blank";}
    async addInitScript() {
      this.installs++;
      if (observation === "rejected") throw new Error("private observer setup");
      if (observation === "stalled") await new Promise(() => {});
    }
    async evaluate() {return {readyState:"complete",timeOrigin:1000};}
    async goto(target, options) {
      this.calls.push({target, options});
      if (observation === "target") for (const kind of ["start", "dcl"])
        this.emit("console", {text:()=>"KINOSAIL_NAV_DOCUMENT "+JSON.stringify({kind,main:true,loginPath:false,setupPath:false,readyState:kind==="start"?"loading":"interactive",timeOrigin:2000,elapsedMs:0,loginForm:false,setupForm:false})});
      if (observation === "malformed") this.emit("console", {text:()=>'KINOSAIL_NAV_DOCUMENT {"kind":"start","private":"do-not-record"}'});
      if (observation === "oversized") this.emit("console", {text:()=>"KINOSAIL_NAV_DOCUMENT "+"do-not-record".repeat(100)});
      this.emit("domcontentloaded"); throw failure;
    }
  }
  const page = new Page(), probe = {}, results = [];
  const browser = {newContext:async()=>({newPage:async()=>page,close:async()=>{closed=true;}})};
  const prior = process.env.KINOSAIL_LAYOUT_MEDIA_ROOT;
  process.env.KINOSAIL_LAYOUT_MEDIA_ROOT="/owned/.verification/layout/fixture/media";
  try {
    await Promise.race([
      assert.rejects(measureSubtitleBackground(browser,{baseURL:"https://owned.fixture"},results,probe), error=>error===failure),
      new Promise((_,reject)=>{timer=setTimeout(()=>reject(new Error("observation blocked navigation")),1600);}),
    ]);
    assert.equal(page.installs,1);
    assert.deepEqual(page.calls,[{target:"/?view=library",options:{waitUntil:"domcontentloaded"}}]);
    assert.equal(probe.navigation.identity,"blank");
    assert.deepEqual(probe.navigation.lifecycle[0].route,{path:"/",view:"library"});
    assert.deepEqual(probe.navigation.documents.map(value=>value.kind),observation==="target"?["start","dcl"]:[]);
    assert.ok(probe.navigation.documents.every(value=>value.source==="unverified-console"));
    assert.doesNotMatch(JSON.stringify(probe),/private|do-not-record|owned\.fixture/);
    assert.equal(closed,true);assert.deepEqual(results,[]);
    assert.equal(page.eventNames().length,0);
  } finally {
    clearTimeout(timer);
    if(prior===undefined)delete process.env.KINOSAIL_LAYOUT_MEDIA_ROOT;else process.env.KINOSAIL_LAYOUT_MEDIA_ROOT=prior;
  }
});
