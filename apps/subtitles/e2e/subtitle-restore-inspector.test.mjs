import assert from "node:assert/strict";
import test from "node:test";
import { flush, inspectorFixture } from "./subtitle-inspector-race-fixture.ts";

// Actual delivered scripts, existing synthetic DOM/controlled external HTTP.
// Written-first lifecycle gaps are documented in the R06 frontend design.
// No browser rendering, layout or populated-server evidence is claimed here.
async function heldRestore(view) {
  const pending = view.node("restore-subtitle").listeners.click(); await flush();
  view.respond(view.requests.at(-1), view.receipt("prepared", undefined, undefined, "restore"), 201);
  await flush(); return { pending };
}

test("An inactive page rejects queued Save and Restore clicks before preparation", async () => {
  const view = inspectorFixture(); await view.ready(); await view.preview(); view.events.pagehide();
  const count = view.requests.length;
  const restore = view.node("restore-subtitle").listeners.click();
  const save = view.node("apply-subtitle").listeners.click(); await flush();
  try { assert.equal(view.requests.length, count); }
  finally { view.events.pagehide(); await Promise.all([restore, save]); }
});

test("An inactive page's explicit Restore status button stays read-only and idle", async () => {
  const view = inspectorFixture(); await view.ready(); const { pending } = await heldRestore(view);
  view.requests.at(-1).reject(new Error("offline")); await pending;
  const check = view.node("inspector-status").children.findLast(node => node.textContent === "Check Restore status");
  assert.ok(check); view.events.pagehide(); const count = view.requests.length;
  const checking = check.listeners.click(); await flush();
  try { assert.equal(view.requests.length, count); }
  finally { view.events.pagehide(); await checking; }
});

test("An invalidated owned Restore releases with finite truthful text and retains corrections", async () => {
  const view = inspectorFixture(); await view.ready(); view.form.elements.text.value = "Keep this correction";
  const { pending } = await heldRestore(view); view.form.listeners.input(); view.poll(); await pending;
  assert.equal(view.form.elements.text.disabled, false);
  assert.equal(view.form.elements.text.value, "Keep this correction");
  assert.match(view.node("inspector-status").textContent, /restore completion is unknown/i);
});

test("Pagehide invalidates painting before releasing an owned Restore", async () => {
  const view = inspectorFixture(); await view.ready(); const { pending } = await heldRestore(view);
  const message = view.node("inspector-status").textContent;
  view.events.pagehide(); await pending;
  assert.equal(view.node("inspector-status").textContent, message);
  assert.equal(view.form.elements.text.disabled, false);
  const count = view.requests.filter(request => request.options.method === "POST").length;
  view.events.pageshow(); await flush();
  assert.equal(view.requests.filter(request => request.options.method === "POST").length, count);
  const receipt = view.requests.findLast(request => request.url === "/api/v1/subtitle-operations/" + view.operation);
  view.respond(receipt, view.receipt("completed", "success", 204, "restore")); await flush();
  view.respond(view.requests.findLast(request => request.url.includes("/inspect?")), view.review("en", "Restored"));
  view.respond(view.requests.findLast(request => request.url.includes("/draft?")), view.draft()); await flush();
  assert.match(view.node("inspector-status").textContent, /previous subtitle restored/i);
  assert.equal(view.form.elements.text.disabled, false);
});

test("Back after pagehide during preparation releases the old pending message without a mutation", async () => {
  const view = inspectorFixture(); await view.ready(); view.form.elements.text.value = "Keep this correction";
  const pending = view.node("restore-subtitle").listeners.click(); await flush();
  assert.equal(view.requests.at(-1).url, "/api/v1/subtitle-operations");
  view.events.pagehide(); await pending;
  const mutations = view.requests.filter(request => request.options.method === "POST").length;
  view.events.pageshow(); await flush();
  assert.equal(view.requests.filter(request => request.options.method === "POST").length, mutations);
  assert.equal(view.requests.filter(request => request.url.endsWith("/restore")).length, 0);
  assert.equal(view.form.elements.text.disabled, false);
  assert.equal(view.form.elements.text.value, "Keep this correction");
  try { assert.match(view.node("inspector-status").textContent, /restore completion is unknown/i); }
  finally { view.events.pagehide(); }
});
