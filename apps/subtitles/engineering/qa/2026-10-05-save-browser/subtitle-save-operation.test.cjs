"use strict";
const { test } = require("node:test");
const assert = require("node:assert/strict");
const { readFileSync } = require("node:fs");
const { join } = require("node:path");
const { runInNewContext } = require("node:vm");

const ITEM = "1234567890abcdef", ID = "a".repeat(64), FP = "b".repeat(64);
const cues = [{ start:1, end:2, text:"Fictional reviewed line" }];
const quality = { cueCount:1,fastCues:0,overlaps:0,longLines:0,maxCPS:0,firstCue:1,lastCue:2,timing:"Reviewed",completeness:"Reviewed" };
const review = { id:ITEM,title:"Fictional clip",language:"en",fingerprint:FP,role:"translation",source:"Manual",
  matchEvidence:"Owner supplied",originalAvailable:false,restorable:true,warnings:[],duration:2,current:{ cues,quality } };
const values = { language:"en", fingerprint:FP, text:"Fictional reviewed line" };
const receipt = (state, outcome, status) => ({ id:ID, action:"apply", item:ITEM, state, outcome, status });
const history = { pageSize:20, matched:0 };
const pending = () => {
  let resolve;
  const promise = new Promise(done => { resolve = done; });
  return { promise, resolve };
};
class Clock {
  now = 0;
  next = 0;
  timers = new Map();
  set = (fn, delay) => { const id = ++this.next; this.timers.set(id,{ fn, at:this.now+delay }); return id; };
  clear = id => this.timers.delete(id);
  advance(delay) {
    const end = this.now+delay;
    for (;;) {
      const due = [...this.timers].filter(([,value]) => value.at <= end).sort((a,b) => a[1].at-b[1].at)[0];
      if (!due) break;
      this.now = due[1].at; this.timers.delete(due[0]); due[1].fn();
    }
    this.now = end;
  }
}
async function flush() { for (let i = 0; i < 30; i++) await new Promise(setImmediate); }
function harness(route) {
  const clock = new Clock(), calls = [], releases = [];
  const fetch = async (path, options = {}) => {
    const call = { path, method:options.method || "GET", body:options.body, operation:options.headers?.["X-Kinosail-Operation"], signal:options.signal };
    calls.push(call);
    const answer = await route(call, calls);
    return { ok:answer.status >= 200 && answer.status < 300, status:answer.status,
      json:() => answer.body?.promise || Promise.resolve(answer.data) };
  };
  const context = { window:{}, fetch, AbortController, performance:{ now:() => clock.now },
    setTimeout:clock.set, clearTimeout:clock.clear, JSON, Error, Set, Map };
  runInNewContext(readFileSync(join(__dirname,"../../../internal/server/static/subtitle-save-operation.js"),"utf8"),context);
  assert.equal(typeof context.window.kinosailSubtitleSave,"function");
  const operation = context.window.kinosailSubtitleSave({ item:ITEM, base:"/api/v1/subtitle-library/"+ITEM, csrf:() => "fictional-test-csrf" });
  return { clock, calls, releases, operation,
    save:() => operation.save(values,{ cues,quality },result => releases.push({ at:clock.now, message:result.message })) };
}
function ordinary(call) {
  if (call.path === "/api/v1/subtitle-operations") return { status:201, data:receipt("prepared") };
  if (call.path.endsWith("/apply")) return { status:202, data:receipt("running") };
  if (call.path.endsWith("/inspect?language=en")) return { status:200, data:review };
  if (call.path === "/api/v1/subtitle-library?view=history") return { status:200, data:history };
  return { status:200, data:receipt("completed","success",200) };
}
test("R06 Save preparation body deadline submits no apply and preserves retry input", async () => {
  const body = pending();
  const h = harness(call => call.path === "/api/v1/subtitle-operations" ? { status:201, body } : ordinary(call));
  const work = h.save(); await flush(); h.clock.advance(15000); await flush();
  const result = await work;
  assert.equal(result.kind,"not-submitted");
  assert.equal(h.calls.filter(call => call.method === "POST").length,1);
  assert.equal(h.calls.some(call => call.path.endsWith("/apply")),false);
  assert.equal(h.releases.length,1); assert.equal(h.releases[0].at,15000);
  body.resolve(receipt("prepared")); await flush();
  assert.equal(h.calls.some(call => call.path.endsWith("/apply")),false);
});
test("R06 Save activation body deadline releases once and reconciles only by GET", async () => {
  const body = pending();
  const h = harness(call => call.path.endsWith("/apply") ? { status:202, body } : ordinary(call));
  const work = h.save(); await flush(); h.clock.advance(30000); await flush();
  const result = await work;
  assert.equal(result.kind,"saved");
  assert.equal(h.releases.length,1); assert.equal(h.releases[0].at,30000);
  assert.equal(h.calls.filter(call => call.path.endsWith("/apply")).length,1);
  assert.equal(h.calls.find(call => call.path.endsWith("/apply")).operation,ID);
  assert.equal(h.calls.filter(call => call.method === "POST").length,2);
  assert.ok(h.calls.slice(2).every(call => call.method === "GET"));
  body.resolve(receipt("running")); await flush();
  assert.equal(h.releases.length,1);
});
test("R06 Save admission202 never claims completion for running work", async () => {
  const h = harness(call => call.path === "/api/v1/subtitle-operations/"+ID ? { status:200, data:receipt("running") } : ordinary(call));
  const work = h.save(); await flush();
  assert.equal(h.releases.length,0);
  h.clock.advance(30000); await flush();
  const result = await work;
  assert.equal(result.kind,"unconfirmed");
  assert.equal(h.releases.length,1);
  assert.equal(h.calls.filter(call => call.path.endsWith("/apply")).length,1);
});
test("R06 Save global release reserves headroom under the unchanged45s criterion", async () => {
  const prepared = pending(), body = pending();
  const h = harness(call => call.path === "/api/v1/subtitle-operations" ? { status:201, body:prepared } :
    call.path.endsWith("/apply") ? { status:202, body } : ordinary(call));
  const work = h.save(); await flush(); h.clock.advance(14900); prepared.resolve(receipt("prepared")); await flush();
  h.clock.advance(29100); await flush(); await work;
  assert.equal(h.releases.length,1); assert.equal(h.releases[0].at,44000);
  assert.ok(h.releases[0].at < 45000);
});
test("R06 Save failed receipt can have effects and never claims rollback", async () => {
  const h = harness(call => call.path === "/api/v1/subtitle-operations/"+ID ? { status:200, data:receipt("completed","failed",500) } : ordinary(call));
  const result = await h.save();
  assert.equal(result.kind,"unconfirmed");
  assert.match(result.message,/may have been written/i);
  assert.doesNotMatch(result.message,/nothing.*saved|not saved|rolled back/i);
  assert.ok(h.calls.some(call => call.path.endsWith("/inspect?language=en")));
  assert.ok(h.calls.some(call => call.path.endsWith("?view=history")));
});
test("R06 Save explicit retry checks status current and History before another write", async () => {
  let failed = true;
  const h = harness(call => call.path === "/api/v1/subtitle-operations/"+ID && failed ?
    { status:200, data:receipt("completed","failed",500) } : ordinary(call));
  await h.save(); const count = h.calls.filter(call => call.method === "POST").length;
  const result = await h.save();
  assert.equal(result.kind,"review-required");
  assert.equal(h.calls.filter(call => call.method === "POST").length,count);
  failed = false;
  const begin = h.calls.length; await h.save();
  const next = h.calls.slice(begin), preparation = next.findIndex(call => call.method === "POST");
  assert.ok(preparation >= 3);
  assert.ok(next.slice(0,preparation).some(call => call.path.endsWith("?view=history")));
});
test("R06 Save unavailable receipt is not evidence of zero effects", async () => {
  const h = harness(call => call.path === "/api/v1/subtitle-operations/"+ID ?
    { status:404, data:{ error:"fictional unavailable receipt" } } : ordinary(call));
  const result = await h.save();
  assert.equal(result.kind,"unconfirmed");
  assert.doesNotMatch(result.message,/nothing.*saved|not saved|rolled back/i);
  assert.equal(h.calls.filter(call => call.path.endsWith("/apply")).length,1);
});
test("R06 Save lifecycle stop cancels browser waits without dispatch replay", async () => {
  const body = pending();
  const h = harness(call => call.path.endsWith("/apply") ? { status:202, body } : ordinary(call));
  const work = h.save(); await flush(); h.operation.stop(); await flush();
  await work;
  assert.equal(h.calls.filter(call => call.path.endsWith("/apply")).length,1);
  assert.equal(h.releases.length,1);
  assert.ok(h.calls.find(call => call.path.endsWith("/apply")).signal.aborted);
  const before = h.calls.length; await h.operation.check("en");
  assert.ok(h.calls.slice(before).every(call => call.method === "GET"));
});
test("R06 Save completed receipt with changed current cues retains the edit", async () => {
  const h = harness(call => call.path.endsWith("/inspect?language=en") ?
    { status:200, data:{ ...review,current:{ cues:[{ start:1,end:2,text:"Fictional other change" }],quality } } } : ordinary(call));
  const result = await h.save();
  assert.equal(result.kind,"review-required");
  assert.doesNotMatch(result.message,/subtitle saved/i);
});
test("R06 Save reconciliation has one absolute15s body deadline", async () => {
  const stalled = pending();
  const h = harness(call => call.path === "/api/v1/subtitle-operations/"+ID ? { status:200, body:stalled } : ordinary(call));
  const work = h.save(); await flush(); h.clock.advance(30000); await flush();
  assert.equal(h.releases.length,1);
  h.clock.advance(15000); await flush(); const result = await work;
  assert.equal(result.kind,"unconfirmed");
  assert.equal(h.calls.filter(call => call.method === "POST").length,2);
  assert.equal(h.releases.length,1);
});

function inspectorHarness() {
  const nodes = new Map(), actions = [], events = {};
  let created = 0;
  function node(id) {
    if (nodes.has(id)) return nodes.get(id);
    const value = { dataset:{ id:ITEM }, style:{}, value:"", checked:false, disabled:false, hidden:false,
      files:[], children:[], listeners:{}, textContent:"", attrs:{},
      addEventListener(name, listener) { this.listeners[name] = listener; },
      setAttribute(name, text) { this.attrs[name] = text; }, removeAttribute(name) { delete this.attrs[name]; },
      append(...children) { this.children.push(...children); }, replaceChildren(...children) { this.children = children; },
      focus() {}, prepend() {}, contains() { return false; },
      querySelector() { return node(id+"-child"); }, querySelectorAll() { return []; },
      addTextTrack() { return { cues:[], addCue(cue) { this.cues.push(cue); }, removeCue(cue) { this.cues = this.cues.filter(value => value !== cue); } }; },
    };
    nodes.set(id,value); return value;
  }
  const form = node("subtitle-edit-form");
  form.elements = Object.fromEntries(["role","language","encoding","offset","automaticSync","removeCredits","mergeRepeated","file","text"]
    .map(name => [name,node("field-"+name)]));
  form.elements.language.value = "en"; form.elements.offset.value = "0";
  form.querySelectorAll = () => [...Object.values(form.elements),node("subtitle-edit-form-child")];
  const empty = { cues:[],quality:{ ...quality,cueCount:0,firstCue:0,lastCue:0 } };
  const current = { ...review,source:"fictional",proposed:empty,current:empty };
  const document = { body:node("body"), getElementById:node,
    querySelector(selector) { return selector === ".subtitle-inspector" ? node("root") : selector.startsWith("meta") ? { content:"fictional-csrf" } : node(selector); },
    querySelectorAll() { return []; }, createElement() { return node("created-"+ ++created); } };
  const window = { addEventListener:(name,handler) => { events[name] = handler; },
    kinosailSubtitleSourceCues:() => ({ reset() {}, column:() => node("column") }),
    kinosailSubtitleSave:() => ({ check:async () => null, stop() {}, save(values, proposed, release) {
      const work = pending(); actions.push({ values, proposed, release, work }); return work.promise;
    } }) };
  runInNewContext(readFileSync(join(__dirname,"../../../internal/server/static/subtitle-inspector.js"),"utf8"),{
    document, window, location:{ pathname:"/fictional",search:"" }, URL,
    VTTCue:function(start,end,text) { return { start,end,text }; },
    fetch:async path => ({ ok:true,status:200,json:async () => path.includes("/draft") ?
      { state:"idle",message:"No draft",words:[] } : current }), setTimeout,clearTimeout,
  });
  return { form, actions, events, current, node,
    preview:() => form.listeners.submit({ preventDefault() {} }),
    save:() => node("apply-subtitle").listeners.click() };
}
test("R06 Save old reconciliation cannot release a newer Save action", async () => {
  const h = inspectorHarness(); await flush(); await h.preview();
  const old = h.save(); await flush();
  h.actions[0].release({ kind:"unconfirmed",message:"Completion unknown; edit kept." });
  h.form.elements.text.value = "Fictional newer correction"; h.form.listeners.input(); await h.preview();
  const next = h.save(); await flush();
  assert.equal(h.actions.length,2); assert.equal(h.form.elements.text.disabled,true);
  const status = h.node("inspector-status").textContent;
  h.actions[0].work.resolve({ kind:"saved",review:h.current,message:"Old Save finished" }); await old;
  assert.equal(h.form.elements.text.disabled,true);
  assert.equal(h.form.elements.text.value,"Fictional newer correction");
  assert.equal(h.node("inspector-status").textContent,status);
  h.actions[1].release({ kind:"unconfirmed",message:"New Save completion unknown." });
  h.actions[1].work.resolve({ kind:"unconfirmed",message:"New Save completion unknown." }); await next;
});

async function draftLease(newerStatus) {
  const h = inspectorHarness(); await flush(); await h.preview();
  h.form.elements.text.value = "Fictional retained input";
  const work = h.save(); await flush();
  const start = h.node("start-draft");
  await start.listeners.click({ currentTarget:start });
  assert.equal(h.form.elements.text.disabled,true);
  if (newerStatus) h.node("inspector-status").textContent = newerStatus;
  h.actions[0].release({ kind:"unconfirmed",message:"Completion unknown; edit kept." });
  assert.equal(h.form.elements.text.disabled,false);
  assert.equal(h.form.elements.text.value,"Fictional retained input");
  if (newerStatus) assert.equal(h.node("inspector-status").textContent,newerStatus);
  else assert.doesNotMatch(h.node("inspector-status").textContent,/^Saving subtitle/);
  h.actions[0].work.resolve({ kind:"unconfirmed",message:"Completion unknown; edit kept." }); await work;
}
test("R06 Save owns busy release independently of an actual draft revision", () => draftLease(null));
test("R06 Save busy release preserves a newer draft status", () => draftLease("Fictional newer draft notice"));
test("R06 Save malformed inspection cannot authorize payload cleanup", async () => {
  const broken = [
    { ...review,current:{ cues } }, { ...review,warnings:null },
    { ...review,current:{ cues,quality:{ ...quality,cueCount:"1" } } },
    { ...review,current:{ cues:[{ ...cues[0],warnings:"invalid" }],quality } },
  ];
  for (const data of broken) {
    const h = harness(call => call.path.endsWith("/inspect?language=en") ? { status:200,data } : ordinary(call));
    const result = await h.save();
    assert.equal(result.kind,"unconfirmed");
    assert.equal(h.calls.filter(call => call.method === "POST").length,2);
    assert.doesNotMatch(result.message,/subtitle saved/i);
  }
});
test("R06 Save requires expected proposed cues before any write", async () => {
  const h = harness(ordinary);
  const result = await h.operation.save(values,undefined,() => {});
  assert.equal(result.kind,"not-submitted");
  assert.equal(h.calls.filter(call => call.method === "POST").length,0);
});

test("R06 Save rejects contradictory completed receipt outcome and status", async () => {
  for (const [outcome,status] of [["success",500],["failed",200]]) {
    const h = harness(call => call.path === "/api/v1/subtitle-operations/"+ID ?
      { status:200,data:receipt("completed",outcome,status) } : ordinary(call));
    const result = await h.save();
    assert.equal(result.kind,"unconfirmed");
    assert.equal(h.calls.some(call => call.path.includes("/inspect")),false);
    assert.equal(h.calls.filter(call => call.method === "POST").length,2);
    assert.doesNotMatch(result.message,/subtitle saved/i);
  }
});
