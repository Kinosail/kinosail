import {test} from "node:test";
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";

// Run the literal credential/factor submit seam with an unfinished redirect.
// This controlled test does not launch an app/browser or claim cookie/TLS proof.
const source = readFileSync(new URL("./layout-stability-local.mjs", import.meta.url), "utf8");
const start = source.indexOf('if (await factor.isVisible()) await factor.fill(code);');
const end = source.indexOf('phase = "library-request";', start);
assert.ok(start > 0 && end > start);
const run = new (async function(){}).constructor("page", "factor", "code", "app", 'let phase;' + source.slice(start,end));
const baseURL = "https://owned.fixture.invalid";

function fixture({visible=true, app="subtitles", clickFailure}={}) {
  const calls=[];
  let current=baseURL+"/login", waiter, urlReads=0;
  const failure=Object.assign(new Error("synthetic redirect timeout"),{name:"TimeoutError"});
  const page={
    url:()=>{urlReads++;return current;},
    getByRole(role, options) {
      if(role==="link")return {isVisible:async()=>false};
      assert.deepEqual(options,{name:"Sign in",exact:true});
      return {async click() {calls.push("click");if(clickFailure)throw clickFailure;}};
    },
    waitForURL(predicate) {calls.push("redirect");return new Promise((resolve,reject)=>{waiter={predicate,resolve,reject};});},
  };
  const factor={isVisible:async()=>visible,fill:async code=>{assert.equal(code,"123456");calls.push("factor");}};
  return {calls,failure,urlReads:()=>urlReads,start:()=>run(page,factor,"123456",app),
    finish(status) {
      // Current handlers return 303 for successful credential+factor login.
      // Invalid responses leave /login; the original redirect waiter fails.
      if(status===303) {current=baseURL+"/account";assert.ok(waiter.predicate(new URL(current)));waiter.resolve();}
      else {assert.equal(waiter.predicate(new URL(current)),false);waiter.reject(failure);}
    }};
}
const settle = ()=>new Promise(resolve=>setImmediate(resolve));

test("pending successful first redirect never refills the still-visible factor form", async () => {
  const f=fixture(),pending=f.start();await settle();
  const before=[...f.calls];f.finish(303);await pending;
  assert.deepEqual(before,["factor","click","redirect"]);
  assert.deepEqual(f.calls,before);
  assert.equal(f.urlReads(),0);
});

test("both apps submit the initial visible factor exactly once", async () => {
  for(const app of ["player","subtitles"])for(const visible of [true,false]) {
    const f=fixture({app,visible}),pending=f.start();await settle();f.finish(303);await pending;
    assert.deepEqual(f.calls,visible?["factor","click","redirect"]:["click","redirect"]);
  }
});

test("invalid or unsupported responses preserve the redirect failure without form replay", async () => {
  for(const status of [200,400,401,403,429,500]) {
    const f=fixture(),pending=f.start();const rejected=assert.rejects(pending,error=>error===f.failure);
    await settle();f.finish(status);await rejected;
    assert.deepEqual(f.calls,["factor","click","redirect"]);
  }
});

test("original submit failure prevents later factor or redirect actions", async () => {
  const failure=Object.assign(new Error("synthetic click timeout"),{name:"TimeoutError"});
  const f=fixture({clickFailure:failure});await assert.rejects(f.start(),error=>error===failure);
  assert.deepEqual(f.calls,["factor","click"]);
});
