import { execFileSync } from "node:child_process";
import { expect, test } from "@playwright/test";
import { flush, inspectorFixture, type FixtureNode } from "./subtitle-inspector-race-fixture";

const revision = process.env.KINOSAIL_TEST_REVISION ?? execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim();
test.afterEach(async ({}, testInfo) => {
  await testInfo.attach("verification-context", { contentType: "application/json", body: JSON.stringify({ revision, command: "playwright test subtitle-inspector-races.spec.ts", fixture: "Complete production inspector script; synthetic DOM, controlled English/French responses, fake timers", environment: "Node VM logic verification; no browser rendering", result: testInfo.status }) });
});

for (const operation of ["apply", "restore"]) {
  test(`${operation} locks editing until the reviewed language operation settles`, { tag: "@smoke" }, async () => {
    const view = inspectorFixture(); await view.ready(); await view.preview();
    view.form.elements.text.value = "reviewed text";
    view.form.elements.encoding.disabled = true;
    const pending = view.node(operation === "apply" ? "apply-subtitle" : "restore-subtitle").listeners.click();
    await flush();
    let request = view.requests.at(-1);
    expect(JSON.parse(request.options.body)).toEqual({ action: operation, item: view.item });
    view.respond(request, view.receipt("prepared", undefined, undefined, operation), 201); await flush();
    request = view.requests.at(-1);
    expect(request.options.headers["X-Kinosail-Operation"]).toBe(view.operation);
    expect(JSON.parse(request.options.body).language).toBe("en");
    for (const control of view.form.querySelectorAll()) expect(control.disabled).toBe(true);
    expect(view.node("restore-subtitle").disabled).toBe(true);
    expect(view.node("apply-subtitle").disabled).toBe(true);
    view.respond(request, view.receipt("running", undefined, undefined, operation), 202); await flush();
    expect(view.requests.at(-1).url).toBe("/api/v1/subtitle-operations/" + view.operation);
    view.respond(view.requests.at(-1), view.receipt("completed", "success", operation === "restore" ? 204 : 200, operation)); await flush();
    view.respond(view.requests.at(-1), view.review("en", operation === "restore" ? "EN restored" : "EN saved"));
    await pending;
    expect(view.form.elements.language.disabled).toBe(false);
    expect(view.form.elements.text.disabled).toBe(false);
    expect(view.form.elements.encoding.disabled).toBe(true);
    expect(view.node("restore-subtitle").disabled).toBe(false);
    expect(view.node("apply-subtitle").disabled).toBe(true);
    expect(view.exports.every(link => new URL(link.href).searchParams.get("language") === "en")).toBe(true);
  });
}

test("failed save unlocks controls and preserves the user's text", { tag: "@smoke" }, async () => {
  const view = inspectorFixture(); await view.ready(); await view.preview();
  view.form.elements.text.value = "keep this correction";
  const saveBegin = view.requests.length;
  const pending = view.node("apply-subtitle").listeners.click();
  await flush(); view.respond(view.requests.at(-1), view.receipt("prepared"), 201); await flush();
  const activation = view.requests.at(-1);
  expect(activation.options.headers["X-Kinosail-Operation"]).toBe(view.operation);
  activation.reject(new Error("Save failed")); await flush();
  view.respond(view.requests.at(-1), view.receipt("completed", "failed", 500)); await flush();
  view.respond(view.requests.at(-2), view.review("en", "EN current"));
  view.respond(view.requests.at(-1), { pageSize: 20, matched: 0, history: [] }); await pending;
  expect(view.requests.slice(saveBegin).filter(request => request.url.endsWith("/apply"))).toHaveLength(1);
  expect(view.requests.slice(saveBegin).filter(request => request.options.method === "POST")).toHaveLength(2);
  expect(view.form.elements.text.value).toBe("keep this correction");
  expect(view.form.elements.text.disabled).toBe(false);
  expect(view.node("inspector-status").textContent).toBe("Save completion is unknown. Changes may have been written. Your edit is kept; check current subtitle and History before trying again.");
});

test("older inspect failure cannot replace the newer language's success", { tag: "@smoke" }, async () => {
  const view = inspectorFixture(), old = view.requests[0];
  view.respond(view.requests[1], view.draft()); view.changeLanguage("fr");
  view.respond(view.requests.at(-2), view.review("fr", "FR current")); view.respond(view.requests.at(-1), view.draft()); await flush();
  const status = view.node("inspector-status").textContent;
  old.reject(new Error("Old English request failed")); await flush();
  expect(view.node("inspector-status").textContent).toBe(status);
  expect(view.node("inspector-status").attrs["aria-busy"]).toBeUndefined();
});

test("older preview failure does not overwrite a newer language", { tag: "@smoke" }, async () => {
  const view = inspectorFixture(); await view.ready();
  const pending = view.form.listeners.submit({ preventDefault() {} }); await flush(); const old = view.requests.at(-1);
  view.changeLanguage("fr"); view.respond(view.requests.at(-2), view.review("fr", "FR current")); view.respond(view.requests.at(-1), view.draft()); await flush();
  const status = view.node("inspector-status").textContent;
  old.reject(new Error("Old preview failed")); await pending;
  expect(view.node("inspector-status").textContent).toBe(status);
});

test("draft observation retries a transient error and settles when ready", { tag: "@smoke" }, async () => {
  const view = inspectorFixture(); await view.ready("running"); view.poll();
  view.requests.at(-1).reject(new Error("temporary network failure")); await flush();
  expect(view.timers.size).toBe(1);
  expect(view.node("draft-status").textContent).toContain("Retrying");
  expect(view.node("start-draft").disabled).toBe(true);
  view.poll(); view.respond(view.requests.at(-1), view.draft("ready")); await flush();
  expect(view.timers.size).toBe(0); expect(view.node("review-draft").hidden).toBe(false); expect(view.node("start-draft").disabled).toBe(false);
});

for (const failed of [false, true]) {
  test(`pagehide discards an in-flight ${failed ? "failed" : "running"} draft and pageshow starts one chain`, { tag: "@smoke" }, async () => {
    const view = inspectorFixture(); await view.ready("running"); view.poll(); const old = view.requests.at(-1);
    view.events.pagehide();
    if (failed) old.reject(new Error("offline")); else view.respond(old, view.draft("running"));
    await flush(); expect(view.timers.size).toBe(0);
    view.events.pageshow(); const fresh = view.requests.at(-1); const count = view.requests.length; view.events.pageshow();
    expect(view.requests.length).toBe(count);
    view.respond(fresh, view.draft("running")); await flush(); expect(view.timers.size).toBe(1);
  });
}

test("a superseded draft response cannot regress the current state or polling", { tag: "@smoke" }, async () => {
  const view = inspectorFixture(); await view.ready("running"); view.poll(); const old = view.requests.at(-1);
  view.changeLanguage("fr"); view.respond(view.requests.at(-2), view.review("fr", "FR current")); view.respond(view.requests.at(-1), view.draft("ready")); await flush();
  view.respond(old, { ...view.draft("running"), language: "en" }); await flush();
  expect(view.node("draft-status").textContent).toBe("ready"); expect(view.timers.size).toBe(0);
});


test("pageshow preserves previously appended cue pages", { tag: "@smoke" }, async () => {
  const view = inspectorFixture(true);
  const review = view.review("en", "English");
  review.current.cues = Array.from({ length: 120 }, (_, index) => ({ start: index, end: index + 1, text: "Cue " + index, warnings: [] }));
  view.respond(view.requests[0], review); view.respond(view.requests[1], view.draft()); await flush();
  for (let page = 0; page < 2; page++) view.observers.findLast(observer => observer.target?.id === "cue-page" && !observer.disconnected).callback([{ isIntersecting: true }]);
  expect(view.node("subtitle-cues").children).toHaveLength(120);
  view.events.pagehide(); view.events.pageshow();
  expect(view.node("subtitle-cues").children).toHaveLength(120);
  expect(view.node("cue-page").textContent).toBe("120 of 120 cues shown");
});

test("source expansion and page jumps retain appended comparisons and only one expanded group", { tag: "@smoke" }, async () => {
  const view = inspectorFixture(true), review = view.review("en", "Synthetic merged groups");
  review.current.cues = Array.from({ length: 861 }, (_, index) => ({ start: index, end: index + 1, text: `Group ${Math.floor(index / 21)}`, warnings: [] }));
  review.proposed = { cues: Array.from({ length: 41 }, (_, index) => ({ start: index * 21, end: (index + 1) * 21, text: `Group ${index}`, warnings: [] })), quality: {} };
  review.comparison = Array.from({ length: 41 }, (_, index) => ({ current: Array.from({ length: 21 }, (_, cue) => index * 21 + cue), proposed: [index], kind: "merged" }));
  view.respond(view.requests[0], review); view.respond(view.requests[1], view.draft()); await flush();
  expect(view.node("subtitle-cues").children).toHaveLength(40);
  view.observers.findLast(observer => observer.target?.id === "cue-page" && !observer.disconnected).callback([{ isIntersecting: true }]);
  const rows = view.node("subtitle-cues").children, first = rows[0].children[0], last = rows[40].children[0];
  first.isConnected = last.isConnected = true;
  const seekCount = (column: FixtureNode) => column.children.filter((node) => node.attrs["aria-label"]?.startsWith("Seek ")).length;
  const toggle = (column: FixtureNode) => column.children.find((node) => node.dataset.sourceToggle !== undefined);
  expect(rows).toHaveLength(41); expect(seekCount(first)).toBe(2);
  toggle(first).listeners.click(); expect(seekCount(first)).toBe(20);
  toggle(last).listeners.click(); expect(seekCount(first)).toBe(2); expect(seekCount(last)).toBe(20);
  expect(toggle(first).attrs["aria-expanded"]).toBe("false");
  const input = last.children.at(-1).children[0].children[0]; input.value = "2";
  input.listeners.keydown({ key: "Enter", preventDefault() {} });
  expect(seekCount(last)).toBe(1);
  const finalCue = last.children.find((node) => node.attrs["aria-label"] === "Seek Current cue 861: 14:20.000 to 14:21.000");
  finalCue.listeners.click(); expect(view.node("subtitle-preview-video").currentTime).toBe(859);
  expect(view.node("subtitle-cues").children).toEqual(rows);
  toggle(last).listeners.click(); expect(seekCount(last)).toBe(2); expect(toggle(last).attrs["aria-expanded"]).toBe("false");
});

test("switching language hides the previous draft and cannot discard newer corrections", { tag: "@smoke" }, async () => {
  const view = inspectorFixture(); await view.ready("ready");
  view.changeLanguage("fr");
  view.respond(view.requests.at(-2), view.review("fr", "FR current")); await flush();
  view.form.elements.text.value = "New French corrections";
  expect(view.node("review-draft").hidden).toBe(true);
  expect(view.node("cancel-draft").hidden).toBe(true);
  const requests = view.requests.length;
  view.node("review-draft").listeners.click();
  expect(view.form.elements.text.value).toBe("New French corrections");
  expect(view.requests.length).toBe(requests);
  view.respond(view.requests.at(-1), view.draft("ready")); await flush();
  expect(view.node("review-draft").hidden).toBe(false);
});

for (const action of ["start", "cancel"]) {
  test(`late ${action} failure cannot replace another language's preview status`, { tag: "@smoke" }, async () => {
    const view = inspectorFixture(); await view.ready(action === "cancel" ? "running" : "idle");
    const button = view.node(action + "-draft"), pending = button.listeners.click({ currentTarget: button }), old = view.requests.at(-1);
    view.changeLanguage("fr"); view.respond(view.requests.at(-2), view.review("fr", "FR current")); view.respond(view.requests.at(-1), view.draft()); await flush(); await view.preview();
    const status = view.node("inspector-status").textContent;
    old.reject(new Error("Old English draft action failed")); await pending;
    expect(view.node("inspector-status").textContent).toBe(status);
    expect(view.node("apply-subtitle").disabled).toBe(false);
  });
}

test("late draft start success preserves the new language's reviewed correction", { tag: "@smoke" }, async () => {
  const view = inspectorFixture(); await view.ready();
  const button = view.node("start-draft"), pending = button.listeners.click({ currentTarget: button }), old = view.requests.at(-1);
  view.changeLanguage("fr"); view.respond(view.requests.at(-2), view.review("fr", "FR current")); view.respond(view.requests.at(-1), view.draft()); await flush();
  view.form.elements.text.value = "Reviewed French correction"; await view.preview();
  const requestCount = view.requests.length;
  view.respond(old, view.draft("running")); await flush();
  if (view.requests.length > requestCount) view.respond(view.requests.at(-1), view.draft());
  await pending;
  expect(view.node("apply-subtitle").disabled).toBe(false);
  expect(view.form.elements.text.value).toBe("Reviewed French correction");
  expect(view.requests).toHaveLength(requestCount);
});

for (const action of ["start", "cancel"]) {
  test(`pagehide ignores a pending draft ${action} failure`, { tag: "@smoke" }, async () => {
    const view = inspectorFixture(); await view.ready(action === "cancel" ? "running" : "idle");
    const button = view.node(action + "-draft"), pending = button.listeners.click({ currentTarget: button }), old = view.requests.at(-1);
    const status = view.node("inspector-status").textContent;
    view.events.pagehide(); old.reject(new Error("Expired hidden-page action")); await pending;
    expect(view.node("inspector-status").textContent).toBe(status);
    expect(view.timers.size).toBe(0);
  });
}

test("an obsolete start completion cannot enable Generate during a newer start", { tag: "@smoke" }, async () => {
  const view = inspectorFixture(); await view.ready();
  const button = view.node("start-draft"), oldPending = button.listeners.click({ currentTarget: button }), old = view.requests.at(-1);
  view.changeLanguage("fr"); view.respond(view.requests.at(-2), view.review("fr", "FR current")); view.respond(view.requests.at(-1), view.draft()); await flush();
  const pending = button.listeners.click({ currentTarget: button }), current = view.requests.at(-1);
  old.reject(new Error("Old start failed")); await oldPending;
  expect(button.disabled).toBe(true);
  view.respond(current, view.draft("running")); await flush(); view.respond(view.requests.at(-1), view.draft("running")); await pending;
  expect(button.disabled).toBe(true);
  expect(view.timers.size).toBe(1);
});

test("a draft start settling after pageshow reconciles an earlier idle observation", { tag: "@smoke" }, async () => {
  const view = inspectorFixture(); await view.ready();
  const button = view.node("start-draft"), pending = button.listeners.click({ currentTarget: button }), start = view.requests.at(-1);
  view.events.pagehide(); view.events.pageshow();
  view.respond(view.requests.at(-1), view.draft()); await flush();
  expect(button.disabled).toBe(true);
  await view.preview();
  const count = view.requests.length;
  view.respond(start, view.draft("running")); await flush();
  expect(view.requests.length).toBe(count + 1);
  view.respond(view.requests.at(-1), view.draft("running")); await pending;
  expect(view.node("draft-status").textContent).toBe("running");
  expect(button.disabled).toBe(true);
  expect(view.node("apply-subtitle").disabled).toBe(false);
  expect(view.timers.size).toBe(1);
});
