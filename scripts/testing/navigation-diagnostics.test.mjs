import {test} from "node:test";
import assert from "node:assert/strict";
import {EventEmitter} from "node:events";
import {navigationDiagnostics} from "./navigation-diagnostics.mjs";

// Failure modes: credentials in URLs; stalled/failed requests; unbounded
// event streams; page destruction; and listeners retained after completion.
class Page extends EventEmitter {
  async evaluate() {return {readyState:"interactive",libraryMarker:false};}
  url() {return "http://localhost:39060/login?token=do-not-record";}
}
const request = (url, type="script") => ({url:()=>url,resourceType:()=>type});
test("owned watch and inspector routes expose only a route class, never item identity", async () => {
  for (const [route, expected] of [["/watch/0123456789abcdef", "/watch"], ["/subtitles/inspect/0123456789abcdef", "/subtitles/inspect"]]) {
    const page = new Page();
    page.url = ()=>"http://localhost:39060" + route + "?token=do-not-record";
    const probe = navigationDiagnostics(page,"http://localhost:39060");
    const value = await probe.snapshot();
    assert.equal(value.path, expected);
    assert.equal(value.identity, "owned");
    assert.doesNotMatch(JSON.stringify(value), /0123456789abcdef|do-not-record|token/);
    probe.stop();
  }
});
test("malformed, conflicting and oversized item paths cannot claim owned identity", async () => {
  for (const path of ["/watch/", "/watch/unknown", "/watch/0123456789abcdef/extra", "/watch/0123456789abcdef%2fextra", "/watch/"+"a".repeat(2049), "/subtitles/inspect/unknown"]) {
    const page = new Page(); page.url = ()=>"http://localhost:39060" + path;
    const probe = navigationDiagnostics(page,"http://localhost:39060"), value = await probe.snapshot();
    assert.equal(value.path, "other"); assert.equal(value.identity, "other"); probe.stop();
  }
});
test("navigation diagnostics omit credentials and queries", async () => {
  const page=new Page(), probe=navigationDiagnostics(page,"http://localhost:39060");
  const first=request("http://localhost:39060/static/main.js?token=do-not-record");
  page.emit("request",first);
  page.emit("requestfailed",request("https://user:do-not-record@remote.invalid/private/do-not-record"));
  const value=await probe.snapshot();
  assert.deepEqual(value.pending,[{path:"/static/main.js",type:"script"}]);
  assert.deepEqual(value.failed,[{path:"other",type:"script"}]);
  assert.equal(value.path,"/login");
  assert.ok(!JSON.stringify(value).includes("do-not-record"));
  probe.stop();
  assert.equal(page.listenerCount("request"),0);
  assert.equal(page.listenerCount("requestfailed"),0);
});
test("navigation diagnostics bound and settle event streams", async () => {
  const page=new Page(), probe=navigationDiagnostics(page), done=request("http://localhost:39060/static/done.js");
  page.emit("request",done); page.emit("requestfinished",done);
  for(let i=0;i<100;i++) {
    const item=request("http://localhost:39060/static/test-"+i+".js");
    page.emit("request",item); page.emit("requestfailed",item);
  }
  const value=await probe.snapshot();
  assert.equal(value.pending.length,0); assert.equal(value.failed.length,20);
  assert.ok(!value.failed.some(item=>item.path.includes("done")));
  probe.stop();
});
test("destroyed pages still retain bounded transport diagnostics", async () => {
  const page=new Page(); page.evaluate=async()=>{throw new Error("destroyed");};
  const probe=navigationDiagnostics(page,"http://localhost:39060");
  page.emit("request",request("http://localhost:39060/api/v1/private?credential=do-not-record","fetch"));
  const value=await probe.snapshot();
  assert.equal(value.readyState,"unavailable");
  assert.deepEqual(value.pending,[{path:"other",type:"fetch"}]);
  probe.stop();
});

test("malformed, oversized and unknown URL or resource values are omitted", async () => {
  const page=new Page(), probe=navigationDiagnostics(page,"http://localhost:39060");
  for (const url of ["not-a-url","file://localhost/login","http://localhost/login?"+"x".repeat(3000)]) page.emit("requestfailed",request(url,"unknown-resource"));
  const value=await probe.snapshot();
  assert.deepEqual(value.failed,Array(3).fill({path:"other",type:"other"}));
  probe.stop();
});

test("stalled renderers cannot stall timeout diagnostics", async () => {
  const page=new Page(); page.evaluate=()=>new Promise(()=>{});
  const probe=navigationDiagnostics(page,"http://localhost:39060");
  let timer;
  try {
    const value=await Promise.race([probe.snapshot(),new Promise(resolve=>{timer=setTimeout(()=>resolve("not-bounded"),1100);})]);
    assert.notEqual(value,"not-bounded");
    assert.equal(value.readyState,"unavailable");
    assert.equal(value.path,"/login");
  } finally {clearTimeout(timer); probe.stop();}
});

test("owned TLS aliases retain page identity and main-frame status without query secrets",async()=>{
 const page=new Page();page.url=()=>"https://layout.fixture.invalid:443/?view=library&token=do-not-record";
 page.evaluate=async()=>({readyState:"complete",libraryMarker:true});
 page.mainFrame=()=>"main";
 const probe=navigationDiagnostics(page,"https://layout.fixture.invalid/");
 const documentRequest={...request(page.url(),"document"),isNavigationRequest:()=>true,frame:()=>"main"};
 page.emit("response",{request:()=>documentRequest,url:()=>page.url(),status:()=>303});
 page.emit("response",{request:()=>documentRequest,url:()=>page.url(),status:()=>200});
 const value=await probe.snapshot({name:"TimeoutError",message:"goto timed out at do-not-record"});
 assert.equal(value.identity,"owned");assert.equal(value.view,"library");assert.equal(value.libraryMarker,true);
 assert.equal(value.readyState,"complete");assert.equal(value.errorCategory,"timeout");
 assert.deepEqual(value.mainFrameResponses,[{path:"/",status:303},{path:"/",status:200}]);
 assert.equal(value.redirectCount,1);assert.ok(value.elapsedMs>=0&&value.elapsedMs<=600000);
 assert.doesNotMatch(JSON.stringify(value),/do-not-record|fixture.invalid|token/);
 probe.stop();assert.equal(page.listenerCount("response"),0);
});
test("blank, foreign, credential-bearing and unknown pages cannot claim owned Library identity",async()=>{
 for(const url of ["about:blank","https://other.invalid/?view=library","https://user:do-not-record@layout.fixture.invalid/?view=library","https://layout.fixture.invalid/private?view=library"]){
 const page=new Page();page.url=()=>url;page.evaluate=async()=>({readyState:"complete",libraryMarker:true});
 const probe=navigationDiagnostics(page,"https://layout.fixture.invalid/");
 const value=await probe.snapshot({name:"UnknownPrivateClass",message:"do-not-record"});
 assert.equal(value.identity,url==="about:blank"?"blank":"other");assert.equal(value.view,"other");assert.equal(value.errorCategory,"other");
 assert.doesNotMatch(JSON.stringify(value),/do-not-record|Private|fixture.invalid/);probe.stop();
 }
});
test("invalid owned origins, unknown views and error messages remain bounded",async()=>{
 for(const base of [undefined,"not-a-url","https://user:do-not-record@localhost","https://localhost/"+"x".repeat(2049)]){
 const page=new Page();page.url=()=>typeof base==="string"&&base.startsWith("https://")?"https://localhost/?view=library":"http://localhost:39060/?view=library";const probe=navigationDiagnostics(page,base),value=await probe.snapshot({message:"ERR_CONNECTION_REFUSED"+"do-not-record".repeat(1000)});
 assert.equal(value.identity,"other");assert.equal(value.errorCategory,"other");assert.equal(value.view,"other");assert.doesNotMatch(JSON.stringify(value),/do-not-record/);probe.stop();
 }
 const page=new Page();page.url=()=>"http://localhost:39060/?view=private-do-not-record";
 const probe=navigationDiagnostics(page,"http://localhost:39060");
 assert.equal((await probe.snapshot({message:"net::ERR_CONNECTION_REFUSED at do-not-record"})).errorCategory,"refused");
 const value=await probe.snapshot();assert.equal(value.view,"other");assert.doesNotMatch(JSON.stringify(value),/do-not-record/);probe.stop();
});

test("ambiguous views and excessive main-frame responses remain bounded",async()=>{
 const page=new Page();page.url=()=>"http://localhost:39060/?view=library&view=movies&token=do-not-record";page.mainFrame=()=>"main";
 const probe=navigationDiagnostics(page,"http://localhost:39060");
 const documentRequest={...request(page.url(),"document"),isNavigationRequest:()=>true,frame:()=>"main"};
 for(let n=0;n<100;n++)page.emit("response",{request:()=>documentRequest,url:()=>page.url(),status:()=>200});
 const value=await probe.snapshot();assert.equal(value.view,"other");assert.equal(value.mainFrameResponses.length,20);assert.doesNotMatch(JSON.stringify(value),/do-not-record|token/);probe.stop();assert.equal(page.listenerCount("response"),0);
});

test("error classifications and redirect statuses exclude non-redirect responses",async()=>{
 const page=new Page();page.mainFrame=()=>"main";
 const probe=navigationDiagnostics(page,"http://localhost:39060");
 const documentRequest={...request(page.url(),"document"),isNavigationRequest:()=>true,frame:()=>"main"};
 for(const status of [304,301,302,303,307,308,200])page.emit("response",{request:()=>documentRequest,url:()=>page.url(),status:()=>status});
 assert.equal((await probe.snapshot()).redirectCount,5);
 for(const [message,category] of [["net::ERR_CERT_AUTHORITY_INVALID private","certificate"],["net::ERR_ABORTED private","interrupted"],["Target page, context or browser has been closed private","closed"]]){
  const value=await probe.snapshot({message});assert.equal(value.errorCategory,category);assert.doesNotMatch(JSON.stringify(value),/private/);
 }
 probe.stop();
});

test("navigation lifecycle is ordered, bounded, and records only safe route classes", async()=>{
 const page=new Page();let current="about:blank";page.url=()=>current;
 const main={url:()=>current};page.mainFrame=()=>main;
 const probe=navigationDiagnostics(page,"http://localhost:39060");
 probe.markNavigation("/?view=library&token=do-not-record");
 page.emit("framenavigated",main);
 current="http://localhost:39060/?view=library&token=do-not-record";
 page.emit("framenavigated",main);page.emit("domcontentloaded");page.emit("load");
 current="about:blank";page.emit("framenavigated",main);
 const value=await probe.snapshot();
 assert.ok(value.lifecycle.length<=20);
 assert.equal(value.lifecycle[0].kind,"navigation-start");
 assert.deepEqual(value.lifecycle[0].route,{path:"/",view:"library"});
 const committed=value.lifecycle.findIndex(e=>e.kind==="frame-navigated"&&e.route.path==="/");
 const dcl=value.lifecycle.findIndex(e=>e.kind==="domcontentloaded");
 const blank=value.lifecycle.findIndex((e,i)=>i>dcl&&e.kind==="frame-navigated"&&e.route.path==="blank");
 assert.ok(committed>=0&&committed<dcl&&dcl<blank);
 assert.equal(value.identity,"blank");
 assert.equal(value.readyState,"interactive");
 assert.ok(value.lifecycle.some(e=>e.kind==="domcontentloaded"));
 assert.deepEqual(value.lifecycle.map(e=>e.timeMs),[...value.lifecycle.map(e=>e.timeMs)].sort((a,b)=>a-b));
 assert.doesNotMatch(JSON.stringify(value),/do-not-record|token/);
 for(let i=0;i<100;i++)page.emit("domcontentloaded");
 assert.equal((await probe.snapshot()).lifecycle.length,20);
 probe.stop();
});
