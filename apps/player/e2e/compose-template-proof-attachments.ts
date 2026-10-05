import type { TestResult } from "@playwright/test/reporter";
import { createHash } from "node:crypto";
import { basename, dirname, join, resolve } from "node:path";

const titles = [
  "Q47 headers deadline phone Player", "Q47 body deadline desktop Both",
  "Q47 lost request recovers with retained inputs", "Q47 failed HTTP recovers with retained inputs",
  "Q47 input change discards late headers", "Q47 app change discards late body",
  "Q47 Player preserves Compose download and copy", "Q47 Subtitles preserves Compose download and copy",
  "Q47 Both preserves Compose download and copy", "Q47 rejects invalid inputs before template requests",
];
export type Context = {
  outputRoot: string | null; title: string; status: string; retry: number; recognizedCount: number;
};
function canonicalAbsolute(value: string | undefined | null): value is string {
  return typeof value === "string" && value.length > 1 && value.length <= 4096 &&
    Buffer.byteLength(value) <= 4096 && value.startsWith("/") &&
    !/[\\\x00-\x1f\x7f]/.test(value) && resolve(value) === value;
}
export function ownedOutputRoot(configured: string | undefined, requested: string | undefined,
  destination: string | null): string | null {
  if (!canonicalAbsolute(destination) || basename(destination) !== "q47-proof.json" ||
    !canonicalAbsolute(requested) || configured !== requested ||
    requested !== join(dirname(destination), "browser-private")) return null;
  // destination already passed the reporter's realpath and private-parent mode checks.
  return requested;
}
export function privateContextPath(root: string | null, title: string): string | null {
  if (!canonicalAbsolute(root) || !titles.includes(title)) return null;
  // Pinned v1.63.0 TestInfoImpl.outputDir; these ten tests are top-level, retry/repeat zero.
  // https://github.com/microsoft/playwright/blob/v1.63.0/packages/playwright/src/worker/testInfo.ts#L185-L196
  // https://github.com/microsoft/playwright/blob/v1.63.0/packages/utils/fileUtils.ts#L79-L93
  const sanitized = title.replace(/[\x00-\x2c\x2e-\x2f\x3a-\x40\x5b-\x60\x7b-\x7f]+/g, "-");
  const full = "compose-template-recovery.journey.ts-" + sanitized;
  const middle = "-" + createHash("sha1").update(full).digest("hex").slice(0, 5) + "-";
  const start = Math.floor((60 - middle.length) / 2), end = 60 - middle.length - start;
  const child = (full.length <= 60 ? full : full.slice(0, start) + middle + full.slice(-end)) + "-chromium";
  return join(root, child, "error-context.md");
}
export function privateContextValid(attachment: TestResult["attachments"][number], context: Context): boolean {
  if (context.status !== "failed" || context.retry !== 0 || context.recognizedCount !== 0 ||
    attachment.name !== "error-context" || attachment.contentType !== "text/markdown" ||
    attachment.body !== undefined || !canonicalAbsolute(attachment.path)) return false;
  const keys = Object.keys(attachment);
  if (keys.length > 4 || keys.some(key => !["name", "contentType", "path", "body"].includes(key))) return false;
  const expected = privateContextPath(context.outputRoot, context.title);
  // Metadata recognition only: never stat/open/read or export the ancillary file or its path.
  return expected !== null && attachment.path === expected;
}
