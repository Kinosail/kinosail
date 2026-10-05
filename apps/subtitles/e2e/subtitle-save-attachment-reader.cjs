"use strict";

const fs = require("node:fs");
const LIMIT = 32768;

function checkedStat(operations, descriptor) {
  const info = operations.fstatSync(descriptor);
  if (!info.isFile() || !Number.isSafeInteger(info.size) || info.size < 0 || info.size > LIMIT ||
      !Number.isFinite(info.mtimeMs) || !Number.isFinite(info.ctimeMs)) {
    throw new Error("attachment-file-shape");
  }
  return info;
}

function readAttachment(path, operations = fs) {
  const { O_RDONLY, O_NOFOLLOW, O_NONBLOCK } = operations.constants;
  if (typeof path !== "string" || path.length === 0 ||
      !Number.isInteger(O_RDONLY) || !Number.isInteger(O_NOFOLLOW) || O_NOFOLLOW <= 0 ||
      !Number.isInteger(O_NONBLOCK) || O_NONBLOCK <= 0) {
    throw new Error("attachment-open-boundary");
  }
  const descriptor = operations.openSync(path, O_RDONLY | O_NOFOLLOW | O_NONBLOCK);
  try {
    const before = checkedStat(operations, descriptor);
    const bytes = Buffer.alloc(LIMIT + 1);
    let length = 0;
    while (length < bytes.length) {
      const count = operations.readSync(descriptor, bytes, length, bytes.length - length, null);
      if (!Number.isSafeInteger(count) || count < 0 || count > bytes.length - length) {
        throw new Error("attachment-read-boundary");
      }
      if (count === 0) break;
      length += count;
    }
    const after = checkedStat(operations, descriptor);
    if (length > LIMIT || length !== before.size || length !== after.size ||
        ["dev", "ino", "mtimeMs", "ctimeMs"].some(key => before[key] !== after[key])) {
      throw new Error("attachment-file-changed");
    }
    return bytes.subarray(0, length);
  } finally {
    operations.closeSync(descriptor);
  }
}

module.exports = { readAttachment };
