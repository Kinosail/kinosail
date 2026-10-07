import {test} from "node:test";
import assert from "node:assert/strict";
import {EventEmitter} from "node:events";
import {navigationDiagnostics} from "./navigation-diagnostics.mjs";

// Failure modes: tracing changes outcomes; unknown console payloads leak;
// transport events lose ownership; markers are unbounded or survive stop.
class Page extends EventEmitter {
  url() {return "https://owned.fixture/watch/0123456789abcdef";}
  async evaluate() {return {readyState:"complete"};}
  mainFrame() {return this;}
}
const item = (page, url, method="GET") => ({url:()=>url,method:()=>method,resourceType:()=>"media",isNavigationRequest:()=>false,frame:()=>page,failure:()=>({errorText:"Load request cancelled private-synthetic-marker"})});
const emit = (page,type,text) => page.emit("console",{type:()=>type,text:()=>text});
const marker = {event:"pagehide-capture",source:"/hls/0123456789abcdef/p/t-a0-s0-none-t0-b0/1080p/segment-00075.m4s",visible:"hidden",position:4.25,ready:3,network:2,paused:false,pip:false};

test("playback transport is opt-in and correlates actual bounded HLS paths over owned HTTP and HTTPS",async()=>{
 for(const scheme of ["http","https"]){
  const page=new Page(),origin=scheme+"://owned.fixture",probe=navigationDiagnostics(page,origin);
  const request=item(page,origin+marker.source+"?token=private-synthetic-marker");
  page.emit("request",request);assert.deepEqual((await probe.snapshot()).playback,[]);
  probe.observePlayback();page.emit("request",request);
  page.emit("response",{request:()=>request,url:()=>request.url(),status:()=>200,fromServiceWorker:()=>true});
  page.emit("requestfinished",request);page.emit("requestfailed",request);
  const rows=(await probe.snapshot()).playback;
  assert.deepEqual(rows.map(r=>r.kind),["request","response","finished","failed"]);
  assert.ok(rows.every(r=>r.path==="/hls/segment"&&r.requestID===rows[0].requestID));
  assert.equal(rows[1].status,200);assert.equal(rows[3].failure,"cancelled");
  assert.ok(rows.every(r=>r.timeMs>=0&&r.timeMs<=600000));
  assert.doesNotMatch(JSON.stringify(rows),/0123456789abcdef|1080p|t-a0|token|private-synthetic-marker|owned.fixture/);
  probe.stop();assert.equal(page.eventNames().length,0);
 }
});
test("playback console classifies spaced access control without retaining messages or remote URLs",async()=>{
 const page=new Page(),probe=navigationDiagnostics(page,"https://owned.fixture");probe.observePlayback();
 for(const text of ["blocked by access control https://foreign.invalid/private-synthetic-marker","access-control blocked","CORS policy blocked"]){emit(page,"error",text);}
 const rows=(await probe.snapshot()).playback;assert.equal(rows.length,3);assert.ok(rows.every(r=>r.category==="access-control"));
 assert.doesNotMatch(JSON.stringify(rows),/private-synthetic-marker|foreign.invalid|blocked/);probe.stop();
});
test("playback lifecycle markers reject missing, unknown, conflicting, malformed and oversized input",async()=>{
 const page=new Page(),probe=navigationDiagnostics(page,"https://owned.fixture");probe.observePlayback();
 const send=value=>emit(page,"debug","kinosail-playback-lifecycle "+value);
 const {ready,...missing}=marker;
 for(const value of [missing,{...marker,secret:"private-synthetic-marker"},{...marker,event:"unknown"},{...marker,position:-1},{...marker,position:31536001},{...marker,position:"4"},{...marker,paused:"false"},{...marker,ready:5},{...marker,network:4},{...marker,visible:"unknown"},{...marker,source:"x".repeat(2049)}])send(JSON.stringify(value));
 send("malformed private-synthetic-marker");send("x".repeat(4097));send(JSON.stringify(marker).replace('"ready":3','"ready":3,"ready":2'));
 assert.equal((await probe.snapshot()).playback.length,0);
 send(JSON.stringify(marker));const row=(await probe.snapshot()).playback[0];
 assert.equal(row.source,"unverified-console");assert.equal(row.mediaSource,"/hls/segment");assert.equal(row.position,4.25);
 assert.doesNotMatch(JSON.stringify(row),/0123456789abcdef|segment-00075|1080p/);probe.stop();
});
test("playback events bound cardinality, reject foreign routes and stop collecting",async()=>{
 const page=new Page(),probe=navigationDiagnostics(page,"https://owned.fixture");probe.observePlayback();probe.observePlayback();
 for(const url of ["https://foreign.invalid/hls/0123456789abcdef/index.m3u8","https://user:private-synthetic-marker@owned.fixture/hls/0123456789abcdef/index.m3u8","https://owned.fixture/hls/unknown/index.m3u8","https://owned.fixture/hls/0123456789abcdef/p/"+"x".repeat(2049)+"/index.m3u8"]){page.emit("request",item(page,url));}
 assert.equal((await probe.snapshot()).playback.length,0);
 for(let n=0;n<1000;n++)emit(page,"debug","kinosail-playback-lifecycle "+JSON.stringify(marker));
 assert.equal((await probe.snapshot()).playback.length,64);assert.equal(page.listenerCount("console"),1);
 probe.stop();assert.equal(page.eventNames().length,0);emit(page,"error","access control");assert.equal((await probe.snapshot()).playback.length,64);
});
test("native list and watched mutations expose only route classes, never item identity or query",async()=>{
 const page=new Page(),probe=navigationDiagnostics(page,"https://owned.fixture");probe.observePlayback();
 for(const route of ["list","watched"]){page.emit("request",item(page,"https://owned.fixture/"+route+"/0123456789abcdef?token=private-synthetic-marker","POST"));}
 const rows=(await probe.snapshot()).playback;assert.deepEqual(rows.map(r=>r.path),["/list","/watched"]);assert.ok(rows.every(r=>r.method==="POST"));
 assert.doesNotMatch(JSON.stringify(rows),/0123456789abcdef|private-synthetic-marker|token/);probe.stop();
});
