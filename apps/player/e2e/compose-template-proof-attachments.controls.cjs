"use strict";
// Source-only controls until the bounded hosted Node 26 phase.
// Native Node type stripping loads the actual helper; no transformed VM copy.
const assert = require("node:assert/strict");
const { test } = require("node:test");
const { ownedOutputRoot, privateContextPath, privateContextValid } =
  require("./compose-template-proof-attachments.ts");

const ROOT = "/fictional/private/repeat-one/browser-private";
const DESTINATION = "/fictional/private/repeat-one/q47-proof.json";
const NAMES = [
  ["Q47 headers deadline phone Player", "compose-template-recovery.-3be76-aders-deadline-phone-Player-chromium"],
  ["Q47 body deadline desktop Both", "compose-template-recovery.-e90c1--body-deadline-desktop-Both-chromium"],
  ["Q47 lost request recovers with retained inputs", "compose-template-recovery.-d4d1a-covers-with-retained-inputs-chromium"],
  ["Q47 failed HTTP recovers with retained inputs", "compose-template-recovery.-ba874-covers-with-retained-inputs-chromium"],
  ["Q47 input change discards late headers", "compose-template-recovery.-76724-hange-discards-late-headers-chromium"],
  ["Q47 app change discards late body", "compose-template-recovery.-effca-p-change-discards-late-body-chromium"],
  ["Q47 Player preserves Compose download and copy", "compose-template-recovery.-76da6-s-Compose-download-and-copy-chromium"],
  ["Q47 Subtitles preserves Compose download and copy", "compose-template-recovery.-d4877-s-Compose-download-and-copy-chromium"],
  ["Q47 Both preserves Compose download and copy", "compose-template-recovery.-bbf08-s-Compose-download-and-copy-chromium"],
  ["Q47 rejects invalid inputs before template requests", "compose-template-recovery.-9c17a-ts-before-template-requests-chromium"],
];
const TITLE = NAMES[0][0];
const PATH = ROOT + "/" + NAMES[0][1] + "/error-context.md";
const context = () => ({ outputRoot: ROOT, title: TITLE, status: "failed", retry: 0, recognizedCount: 0 });
const attachment = () => ({ name: "error-context", contentType: "text/markdown", path: PATH });

test("configured private root is exact", () => {
  assert.equal(ownedOutputRoot(ROOT, ROOT, DESTINATION), ROOT);
});
test("all ten per-case directories match pinned Playwright vectors", () => {
  for (const [title, child] of NAMES)
    assert.equal(privateContextPath(ROOT, title), ROOT + "/" + child + "/error-context.md");
});
test("one failed case path-only context metadata is recognizable", () => {
  assert.equal(privateContextValid(attachment(), context()), true);
  assert.equal(privateContextValid({ ...attachment(), body: undefined }, context()), true);
});
test("wrong configured root never becomes owned", () => {
  for (const [configured, requested, destination] of [
    [ROOT + "-sibling", ROOT, DESTINATION], [ROOT, ROOT + "-sibling", DESTINATION],
    [ROOT, undefined, DESTINATION], [ROOT, ROOT, null],
    [ROOT, ROOT, "/fictional/private/another/q47-proof.json"],
    [ROOT + "/../browser-private", ROOT + "/../browser-private", DESTINATION],
    [ROOT + "//", ROOT + "//", DESTINATION],
    [ROOT.replaceAll("/", "\\"), ROOT.replaceAll("/", "\\"), DESTINATION],
    ["browser-private", "browser-private", DESTINATION],
    [ROOT + "\n", ROOT + "\n", DESTINATION],
    [ROOT, ROOT, "/fictional/private/repeat-one/another.json"],
  ]) assert.equal(ownedOutputRoot(configured, requested, destination), null);
});
test("unknown names MIME bodies and extra keys are rejected", () => {
  for (const mutation of [
    { name: "trace" }, { name: "error-context-extra" }, { contentType: "text/plain" },
    { contentType: "text/markdown; charset=utf-8" }, { body: Buffer.alloc(0) },
    { body: Buffer.from("fictional-private-canary") }, { body: null },
    { extra: "fictional-private-canary" }, { path: undefined },
  ]) assert.equal(privateContextValid({ ...attachment(), ...mutation }, context()), false);
});
test("relative escape sibling backslash control and noncanonical paths are rejected", () => {
  for (const path of [
    "error-context.md", PATH.replace(ROOT, ROOT + "-sibling"), ROOT + "/../" + PATH.split("/").slice(-2).join("/"),
    PATH.replaceAll("/", "\\"), PATH + "\n", PATH + "\0", PATH.replace("/error-context.md", "/../error-context.md"),
    PATH.replace("/error-context.md", "/nested/error-context.md"), PATH.replace("/error-context.md", "/other.md"),
    PATH.replace("/error-context.md", "//error-context.md"), "/" + PATH,
    "/" + "x".repeat(4096),
  ]) assert.equal(privateContextValid({ ...attachment(), path }, context()), false);
});
test("another real test directory is not this case", () => {
  for (const [, child] of NAMES.slice(1))
    assert.equal(privateContextValid({ ...attachment(), path: ROOT + "/" + child + "/error-context.md" }, context()), false);
});
test("nonfailed interrupted timedout skipped and retried cases are rejected", () => {
  for (const status of ["passed", "interrupted", "timedOut", "skipped", "unknown"])
    assert.equal(privateContextValid(attachment(), { ...context(), status }), false);
  for (const retry of [1, -1, true, null])
    assert.equal(privateContextValid(attachment(), { ...context(), retry }), false);
});
test("duplicates and invalid prior recognition counts are rejected", () => {
  for (const recognizedCount of [1, 2, -1, true, null])
    assert.equal(privateContextValid(attachment(), { ...context(), recognizedCount }), false);
});
test("unknown title and absent ownership are rejected", () => {
  for (const title of ["Q47 arbitrary", TITLE + " extra", ""])
    assert.equal(privateContextValid(attachment(), { ...context(), title }), false);
  assert.equal(privateContextValid(attachment(), { ...context(), outputRoot: null }), false);
});
test("metadata recognition performs no attachment content acquisition", () => {
  const record = attachment();
  Object.defineProperty(record, "body", { get() { throw new Error("fictional-acquisition"); } });
  assert.throws(() => privateContextValid(record, context()), /fictional-acquisition/);
  // The reporter catches acquisition failures as attachment_invalid, never known errors.
});
