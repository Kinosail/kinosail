import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

// Actual delivered controller, isolated external HTTP/clock. Coverage gaps and
// failure modes were written first in engineering/qa/2026-10-06-r06-restore-frontend.md.
// These controls are not populated-server or browser rendering evidence.
const source = readFileSync(new URL("../internal/server/static/subtitle-save-operation.js", import.meta.url), "utf8");
const item = "0000000000000001", base = "/api/v1/subtitle-library/" + item;
const flush = async () => { for (let i = 0; i < 30; i++) await Promise.resolve(); };
const deferred = () => { let resolve; const promise = new Promise(done => { resolve = done; }); return { promise, resolve }; };
const document = text => ({ cues: [{ start: 1, end: 2, text }], quality: {
  cueCount: 1, fastCues: 0, overlaps: 0, longLines: 0, maxCPS: 4, firstCue: 1, lastCue: 2, timing: "Known", completeness: "Complete",
} });

function rig({ abort = true } = {}) {
  const requests = [], timers = new Map(), records = new Map(), window = {};
  let clock = 0, timerID = 0, sequence = 0, fingerprint = "c".repeat(64);
  const state = { hold: "", preparation: undefined, receipt: undefined, inspection: undefined, lateInspection: undefined };
  const review = (language = "en", text = "Current line") => ({ id: item, language, fingerprint,
    role: "translation", title: "Fictional title", source: "Fictional source", matchEvidence: "Fixture identity",
    originalAvailable: true, restorable: true, warnings: [], duration: 6, current: document(text) });
  const response = (status, data) => ({ status, ok: status >= 200 && status < 300, json: async () => data });
  const pending = signal => new Promise((_, reject) => signal?.addEventListener("abort", () => reject(new Error("cancelled")), { once: true }));
  async function fetch(url, options) {
    const request = { url, options }; requests.push(request);
    if (url === "/api/v1/subtitle-operations") {
      const prepared = { id: (++sequence).toString(16).padStart(64, "0"), ...JSON.parse(options.body), state: "prepared" };
      records.set(prepared.id, prepared);
      if (state.hold === "prepare") return pending(options.signal);
      return response(201, state.preparation ? state.preparation(prepared) : prepared);
    }
    if (options.method === "POST") {
      const id = options.headers["X-Kinosail-Operation"], record = records.get(id);
      assert.ok(record, "activation must use a prepared identity");
      assert.equal(url, base + "/" + record.action);
      records.set(id, { ...record, state: "completed", outcome: "success", status: 204 });
      if (state.hold === "headers") return pending(options.signal);
      if (state.hold === "activation-body") return { status: 202, ok: true, json: () => pending(options.signal) };
      return response(202, { ...record, state: "running" });
    }
    if (url.startsWith("/api/v1/subtitle-operations/")) {
      const record = records.get(url.split("/").at(-1));
      return state.receipt ? state.receipt(record) : response(200, record);
    }
    if (url.startsWith(base + "/inspect?")) {
      const language = new URL(url, "https://fixture.invalid").searchParams.get("language");
      if (state.lateInspection) return { status: 200, ok: true, json: () => state.lateInspection.promise };
      if (state.hold === "inspection-body") return { status: 200, ok: true, json: () => pending(options.signal) };
      return response(200, state.inspection ?? review(language));
    }
    assert.equal(url, "/api/v1/subtitle-library?view=history");
    return response(200, { pageSize: 20, matched: 1, history: [] });
  }
  vm.runInNewContext(source, { window, fetch, ...(abort ? { AbortController } : {}), performance: { now: () => clock },
    setTimeout(run, delay) { const id = ++timerID; timers.set(id, { run, at: clock + delay }); return id; },
    clearTimeout(id) { timers.delete(id); },
  });
  const controller = window.kinosailSubtitleSave({ item, base, csrf: () => "fixture-csrf" });
  const released = [];
  const restore = (language = "en") => controller.restore(language, result => released.push({ result, at: clock }));
  const save = (language = "en") => controller.save({ language, fingerprint }, document("Current line"), result => released.push({ result, at: clock }));
  return { controller, state, requests, released, restore, save, review, response, records, timers,
    changeFingerprint() { fingerprint = "d".repeat(64); },
    activations(action = "restore") { return requests.filter(request => request.url === base + "/" + action); },
    async tick(milliseconds) {
      await flush(); const end = clock + milliseconds;
      for (;;) {
        const entry = [...timers].filter(([, value]) => value.at <= end).sort((a, b) => a[1].at - b[1].at)[0];
        if (!entry) break;
        const [id, value] = entry; clock = value.at; timers.delete(id); value.run(); await flush();
      }
      clock = end; await flush();
    },
  };
}

test("Restore prepares, activates once, confirms exact 204 receipt and bounded inspection", async () => {
  const view = rig(), result = await view.restore();
  assert.equal(result.kind, "restored");
  assert.match(result.message, /previous subtitle restored/i);
  assert.equal(view.released.length, 1);
  assert.equal(view.activations().length, 1);
  assert.deepEqual(JSON.parse(view.requests[0].options.body), { action: "restore", item });
  assert.deepEqual(JSON.parse(view.activations()[0].options.body), { language: "en" });
  assert.equal(view.activations()[0].options.credentials, "same-origin");
  assert.equal(view.activations()[0].options.headers["X-Kinosail-CSRF"], "fixture-csrf");
  assert.equal(result.review.language, "en");
  assert.equal(view.timers.size, 0);
});

for (const hold of ["headers", "activation-body", "inspection-body"]) {
  for (const abort of [true, false]) {
    test(`Restore ${hold} has finite truthful release with AbortController=${abort}`, async () => {
      const view = rig({ abort }); view.state.hold = hold;
      const pending = view.restore(); await view.tick(44000); const result = await pending;
      assert.equal(result.kind, "unconfirmed"); assert.match(result.message, /restore.*unknown/i);
      assert.equal(view.released.length, 1); assert.ok(view.released[0].at <= 44000);
      assert.equal(view.activations().length, 1); assert.equal(view.timers.size, 0);
      const count = view.requests.length; await view.tick(60000);
      assert.equal(view.requests.length, count, "no automatic late inspection or activation");
      view.state.hold = ""; const checked = await view.controller.checkRestore("en");
      assert.equal(checked.kind, "restored"); assert.equal(view.activations().length, 1);
      await view.restore(); assert.equal(view.activations().length, 1, "status confirmation does not replay swap");
    });
  }
}

for (const [name, mutate] of [
  ["wrong action", receipt => ({ ...receipt, action: "apply" })],
  ["wrong item", receipt => ({ ...receipt, item: "0000000000000002" })],
  ["invalid identity", receipt => ({ ...receipt, id: "bad" })],
  ["nonprepared state", receipt => ({ ...receipt, state: "running" })],
]) {
  test(`Restore rejects ${name} preparation before activation`, async () => {
    const view = rig(); view.state.preparation = mutate;
    const result = await view.restore(); assert.equal(result.kind, "not-submitted");
    assert.match(result.message, /restore/i); assert.equal(view.activations().length, 0);
  });
}

for (const [name, receipt] of [
  ["wrong action", record => ({ ...record, action: "apply" })],
  ["wrong item", record => ({ ...record, item: "0000000000000002" })],
  ["wrong identity", record => ({ ...record, id: "f".repeat(64) })],
  ["wrong success status", record => ({ ...record, status: 200 })],
  ["failed", record => ({ ...record, outcome: "failed", status: 500 })],
  ["unknown", record => ({ ...record, state: "unknown", outcome: "uncertain", status: undefined })],
]) {
  test(`Restore retains ${name} receipt without replay even after fingerprint drift`, async () => {
    const view = rig(); view.state.receipt = record => view.response(200, receipt(record));
    assert.equal((await view.restore()).kind, "unconfirmed");
    view.changeFingerprint(); await view.restore(); await view.controller.checkRestore("en");
    assert.equal(view.activations().length, 1);
    assert.equal(view.requests.filter(request => request.options.method === "POST").length, 2);
  });
}

test("Unavailable receipt and cross-language checks cannot issue another Restore", async () => {
  const view = rig(); await view.restore();
  view.state.receipt = () => view.response(404, { error: "gone" });
  assert.equal((await view.controller.checkRestore("en")).kind, "unconfirmed");
  assert.equal(await view.controller.checkRestore("fr"), null);
  assert.equal((await view.restore("fr")).kind, "unconfirmed");
  assert.equal(view.activations().length, 1);
});

test("Only a new confirmed matching same-language Save rearms a Restore", async () => {
  const view = rig(); await view.restore();
  await view.controller.checkRestore("en"); await view.restore();
  assert.equal(view.activations().length, 1);
  assert.equal((await view.save()).kind, "saved");
  assert.equal((await view.restore()).kind, "restored");
  assert.equal(view.activations().length, 2);
});

test("Another-language Save cannot rearm the retained original-language swap", async () => {
  const view = rig(); await view.restore(); assert.equal((await view.save("fr")).kind, "saved");
  await view.restore(); assert.equal(view.activations().length, 1);
});

test("Read-only Save confirmation and observed drift cannot rearm Restore", async () => {
  const view = rig(); view.state.hold = "headers";
  const save = view.save(); await view.tick(30000); view.state.hold = ""; await save;
  await view.restore(); await view.controller.check("en"); view.changeFingerprint();
  await view.restore(); assert.equal(view.activations().length, 1);
});

test("Mismatched proposed/current Save cannot rearm Restore", async () => {
  const view = rig(); await view.restore(); view.state.inspection = view.review("en", "External change");
  assert.equal((await view.save()).kind, "review-required");
  await view.restore(); assert.equal(view.activations().length, 1);
});

test("An older Save's late matching inspection cannot clear a newer Restore fence", async () => {
  const view = rig(), late = deferred(); view.state.lateInspection = late;
  const save = view.save(); await view.tick(30000);
  view.state.lateInspection = undefined; await view.restore();
  late.resolve(view.review()); await flush(); await save;
  await view.restore(); assert.equal(view.activations().length, 1);
});

test("Stop during preparation prevents activation and settles once", async () => {
  const view = rig(); view.state.hold = "prepare";
  const pending = view.restore(); await flush(); view.controller.stop();
  assert.equal((await pending).kind, "not-submitted");
  assert.equal(view.activations().length, 0); assert.equal(view.released.length, 1); assert.equal(view.timers.size, 0);
});

test("Concurrent Restore calls cannot prepare or activate two swaps", async () => {
  const view = rig(); view.state.hold = "headers";
  const first = view.restore(), second = view.restore();
  await view.tick(44000); await Promise.all([first, second]);
  assert.equal(view.activations().length, 1);
  assert.equal(view.requests.filter(request => request.url === "/api/v1/subtitle-operations").length, 1);
});

test("Stop retains dispatched Restore identity and status remains read-only", async () => {
  const view = rig(); view.state.hold = "headers";
  const pending = view.restore(); await flush(); view.controller.stop();
  assert.equal((await pending).kind, "unconfirmed");
  view.state.hold = ""; assert.equal((await view.controller.checkRestore("en")).kind, "restored");
  await view.restore(); assert.equal(view.activations().length, 1); assert.equal(view.timers.size, 0);
});

test("Existing Save keeps its default apply receipt and successful result", async () => {
  const view = rig(), result = await view.save();
  assert.equal(result.kind, "saved"); assert.match(result.message, /subtitle saved/i);
  assert.equal(view.activations("apply").length, 1);
  assert.deepEqual(JSON.parse(view.requests[0].options.body), { action: "apply", item });
});
