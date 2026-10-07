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
