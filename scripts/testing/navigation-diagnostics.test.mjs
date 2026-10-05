import {test} from "node:test";
import assert from "node:assert/strict";
import {EventEmitter} from "node:events";
import {navigationDiagnostics} from "./navigation-diagnostics.mjs";

// Failure modes: credentials in URLs; stalled/failed requests; unbounded
// event streams; page destruction; and listeners retained after completion.
class Page extends EventEmitter {
  async evaluate() {return "interactive";}
  url() {return "http://localhost:39060/login?token=do-not-record";}
}
const request = (url, type="script") => ({url:()=>url,resourceType:()=>type});
test("navigation diagnostics omit credentials and queries", async () => {
  const page=new Page(), probe=navigationDiagnostics(page);
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
  const probe=navigationDiagnostics(page);
  page.emit("request",request("http://localhost:39060/api/v1/private?credential=do-not-record","fetch"));
  const value=await probe.snapshot();
  assert.equal(value.readyState,"unavailable");
  assert.deepEqual(value.pending,[{path:"other",type:"fetch"}]);
  probe.stop();
});

test("malformed, oversized and unknown URL or resource values are omitted", async () => {
  const page=new Page(), probe=navigationDiagnostics(page);
  for (const url of ["not-a-url","file://localhost/login","http://localhost/login?"+"x".repeat(3000)]) page.emit("requestfailed",request(url,"unknown-resource"));
  const value=await probe.snapshot();
  assert.deepEqual(value.failed,Array(3).fill({path:"other",type:"other"}));
  probe.stop();
});

test("stalled renderers cannot stall timeout diagnostics", async () => {
  const page=new Page(); page.evaluate=()=>new Promise(()=>{});
  const probe=navigationDiagnostics(page);
  let timer;
  try {
    const value=await Promise.race([probe.snapshot(),new Promise(resolve=>{timer=setTimeout(()=>resolve("not-bounded"),1100);})]);
    assert.notEqual(value,"not-bounded");
    assert.equal(value.readyState,"unavailable");
    assert.equal(value.path,"/login");
  } finally {clearTimeout(timer); probe.stop();}
});
