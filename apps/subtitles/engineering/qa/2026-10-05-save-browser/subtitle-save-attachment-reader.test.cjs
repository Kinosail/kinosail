"use strict";

const assert = require("node:assert/strict");
const { test } = require("node:test");
const { readAttachment } = require("../../../e2e/subtitle-save-attachment-reader.cjs");

function fixture(options = {}) {
  const body = Buffer.from(options.body ?? '{"schema":"fixed"}');
  const calls = [];
  let position = 0, stats = 0;
  const info = { dev: 1, ino: 2, size: body.length, mtimeMs: 3, ctimeMs: 4, isFile: () => true };
  const operations = {
    constants: { O_RDONLY: 0, O_NOFOLLOW: 256, O_NONBLOCK: 2048 },
    openSync(path, flags) {
      calls.push(["open", path, flags]);
      if (options.openError) throw options.openError;
      return 17;
    },
    fstatSync(descriptor) {
      calls.push(["stat", descriptor]);
      stats++;
      return { ...info, ...(stats === 1 ? options.before : options.after) };
    },
    readSync(descriptor, bytes, offset, length, filePosition) {
      calls.push(["read", descriptor, offset, length, filePosition]);
      if (options.readError) throw options.readError;
      if (options.count !== undefined) return options.count;
      const count = Math.min(length, body.length - position, options.chunk ?? Infinity);
      body.copy(bytes, offset, position, position + count);
      position += count;
      return count;
    },
    closeSync(descriptor) {
      calls.push(["close", descriptor]);
      if (options.closeError) throw options.closeError;
    },
  };
  Object.defineProperties(operations, {
    lstatSync: { get() { throw new Error("pathname-stat-forbidden"); } },
    readFileSync: { get() { throw new Error("pathname-read-forbidden"); } },
  });
  return { body, calls, operations };
}
function rejected(options, expected) {
  const view = fixture(options);
  assert.throws(() => readAttachment("/owned/attachment", view.operations), expected);
  assert.equal(view.calls.filter(call => call[0] === "open").length, 1);
  assert.deepEqual(view.calls.filter(call => call[0] === "close"), [["close", 17]]);
  return view;
}

test("attachment reader uses one non-following nonblocking descriptor through partial reads", () => {
  const view = fixture({ chunk: 2 });
  assert.deepEqual(readAttachment("/owned/attachment", view.operations), view.body);
  assert.deepEqual(view.calls[0], ["open", "/owned/attachment", 2304]);
  assert.deepEqual(view.calls.at(-1), ["close", 17]);
  assert.equal(view.calls.filter(call => call[0] === "open").length, 1);
  assert.equal(view.calls.filter(call => call[0] === "stat").length, 2);
  assert.ok(view.calls.filter(call => call[0] === "read").every(call => call[1] === 17 && call[4] === null));
});
test("attachment reader rejects unavailable symlink protection before opening", () => {
  const view = fixture();
  delete view.operations.constants.O_NOFOLLOW;
  assert.throws(() => readAttachment("/owned/attachment", view.operations), /attachment-open-boundary/);
  assert.deepEqual(view.calls, []);
});
test("attachment reader rejects unavailable nonblocking protection before opening", () => {
  const view = fixture();
  delete view.operations.constants.O_NONBLOCK;
  assert.throws(() => readAttachment("/owned/attachment", view.operations), /attachment-open-boundary/);
  assert.deepEqual(view.calls, []);
});
test("attachment reader propagates symlink open rejection without a descriptor to close", () => {
  const error = Object.assign(new Error("symlink"), { code: "ELOOP" });
  const view = fixture({ openError: error });
  assert.throws(() => readAttachment("/owned/attachment", view.operations), value => value === error);
  assert.deepEqual(view.calls, [["open", "/owned/attachment", 2304]]);
});
test("attachment reader rejects nonregular descriptors before reading and closes them", () => {
  const view = rejected({ before: { isFile: () => false } }, /attachment-file-shape/);
  assert.equal(view.calls.filter(call => call[0] === "read").length, 0);
});
test("attachment reader rejects oversized descriptors before reading and closes them", () => {
  const view = rejected({ before: { size: 32769 } }, /attachment-file-shape/);
  assert.equal(view.calls.filter(call => call[0] === "read").length, 0);
});
test("attachment reader rejects negative and fractional descriptor sizes", () => {
  for (const size of [-1, 1.5]) rejected({ before: { size } }, /attachment-file-shape/);
});
test("attachment reader bounds a growing file to one byte beyond the cap", () => {
  const view = rejected({ body: "x".repeat(40000), before: { size: 1 }, after: { size: 1 } }, /attachment-file-changed/);
  assert.deepEqual(view.calls.filter(call => call[0] === "read").map(call => call[3]), [32769]);
});
test("attachment reader rejects short EOF against the opened descriptor size", () => {
  rejected({ before: { size: 100 } }, /attachment-file-changed/);
});
test("attachment reader rejects size changes after consumption", () => {
  rejected({ after: { size: 0 } }, /attachment-file-changed/);
});
test("attachment reader rejects descriptor identity and timestamp changes", () => {
  for (const key of ["dev", "ino", "mtimeMs", "ctimeMs"]) {
    rejected({ after: { [key]: 999 } }, /attachment-file-changed/);
  }
});
test("attachment reader rejects invalid read counts and closes its descriptor", () => {
  for (const count of [-1, 1.5, 32770]) rejected({ count }, /attachment-read-boundary/);
});
test("attachment reader closes its descriptor when a read fails", () => {
  const error = new Error("read failed");
  rejected({ readError: error }, value => value === error);
});
test("attachment reader refuses evidence when descriptor closure fails", () => {
  const error = new Error("close failed");
  rejected({ closeError: error }, value => value === error);
});
test("attachment reader accepts the exact cap and an empty regular file", () => {
  for (const body of ["", "x".repeat(32768)]) {
    const view = fixture({ body });
    assert.deepEqual(readAttachment("/owned/attachment", view.operations), view.body);
    assert.deepEqual(view.calls.at(-1), ["close", 17]);
  }
});
