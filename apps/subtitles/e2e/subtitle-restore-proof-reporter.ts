import type { FullConfig, FullResult, Reporter, Suite, TestCase, TestError, TestResult } from "@playwright/test/reporter";
import { constants, closeSync, openSync, writeFileSync } from "node:fs";
import { isAbsolute } from "node:path";
import { readAttachment } from "./subtitle-save-attachment-reader.cjs";
import { ASSERTION_IDS, type AssertionID, type SafeCase } from "./subtitle-restore-recovery-helpers";
import { isObject, SNAPSHOT_COUNTERS, SNAPSHOT_FLAGS, type JSONObject, type JSONValue } from "./subtitle-restore-recovery-fixture";

import { FAILURE_CODES } from "./subtitle-restore-recovery-network";

const SCHEMA = "r06-restore-browser-v1";
const FILE = "apps/subtitles/e2e/subtitle-restore-recovery.journey.ts";
const CASES = {
  "r06-restore-headers-desktop": "R06 Restore held headers releases desktop editor",
  "r06-restore-headers-phone": "R06 Restore held headers releases phone editor",
  "r06-restore-inspect-body-desktop": "R06 Restore held inspection body releases desktop editor",
  "r06-restore-inspect-body-phone": "R06 Restore held inspection body releases phone editor",
} as const;
const STAGES = ["setup", "witness", "deadline", "release", "terminal", "navigation", "cleanup"];
const EXTRA_FLAGS = [
  "eligible", "fixtureStopped", "restoreRequestObserved", "restoreResponseObserved", "inspectRequestObserved",
  "inspectResponseObserved", "restoreBodyDelivered", "inspectionBodyDelivered", "releaseAttempted",
];
const TIMINGS = ["clickToWitnessMs", "clickToUnlockMs", "holdDurationMs", "durationMs"];
const FIELDS = [
  "schema", "kind", "caseID", "stage", "failureStage", "protocol", ...SNAPSHOT_COUNTERS, ...SNAPSHOT_FLAGS,
  ...EXTRA_FLAGS, "restoreTerminal", "inspectTerminal", "restoreFailureCode", "inspectFailureCode", ...TIMINGS, "servedScriptSHA256", "assertions",
];
function exactKeys(value: JSONValue, keys: readonly string[]): value is JSONObject {
  return isObject(value) && Object.keys(value).sort().join(",") === [...keys].sort().join(",");
}
function validRecord(value: JSONValue, id: string): value is SafeCase {
  if (!exactKeys(value, FIELDS) || value.schema !== SCHEMA || value.kind !== "runtime-case" || value.caseID !== id ||
    typeof value.stage !== "string" || !STAGES.includes(value.stage) ||
    value.failureStage !== null && (typeof value.failureStage !== "string" || !STAGES.includes(value.failureStage)) ||
    typeof value.protocol !== "string" || !["unreached", "legacy", "prepared"].includes(value.protocol)) return false;
  if ([...SNAPSHOT_FLAGS, ...EXTRA_FLAGS].some(key => typeof value[key] !== "boolean")) return false;
  if (SNAPSHOT_COUNTERS.some(key => {
    const n = value[key]; return typeof n !== "number" || !Number.isSafeInteger(n) || n < 0 || n > 1000;
  })) return false;
  if (typeof value.responseStatus !== "number" || value.responseStatus > 599 ||
    typeof value.inspectionResponseStatus !== "number" || value.inspectionResponseStatus > 599) return false;
  for (const key of ["restoreTerminal", "inspectTerminal"]) {
    const terminal = value[key];
    if (typeof terminal !== "string" || !["unreached", "pending", "finished", "request-failed"].includes(terminal)) return false;
  }
  for (const prefix of ["restore", "inspect"]) {
    const code = value[prefix + "FailureCode"];
    if (typeof code !== "string" || !FAILURE_CODES.some(candidate => candidate === code) ||
      (value[prefix + "Terminal"] === "request-failed") !== (code !== "none")) return false;
  }
  if (TIMINGS.some(key => {
    const n = value[key]; return n !== null && (typeof n !== "number" || !Number.isFinite(n) || n < 0 || n > 125000);
  }) || typeof value.durationMs !== "number") return false;
  if (!exactKeys(value.servedScriptSHA256, ["inspector"]) ||
    value.servedScriptSHA256.inspector !== null && (typeof value.servedScriptSHA256.inspector !== "string" ||
      !/^[a-f0-9]{64}$/.test(value.servedScriptSHA256.inspector)) || !exactKeys(value.assertions, ASSERTION_IDS)) return false;
  const assertions = value.assertions;
  return ASSERTION_IDS.every(id => {
    const a = assertions[id];
    if (!exactKeys(a, ["attempted", "completed", "passed"]) || typeof a.attempted !== "boolean" || typeof a.completed !== "boolean") return false;
    return a.completed ? a.attempted && typeof a.passed === "boolean" : a.passed === null;
  });
}
type ErrorAdmission = {
  retry: number; totalErrorCount: number; knownAssertionErrorIDs: string[];
  unknownErrorCount: number; assertionErrorsExact: boolean;
};
type CaseResult = ErrorAdmission & {
  caseID: string; outcome: "passed" | "failed" | "incomplete"; data: SafeCase | null;
  failedAssertions: AssertionID[]; unattemptedAssertions: AssertionID[]; incompleteAssertions: AssertionID[];
};
function errorIdentity(error: TestError): string | null {
  const location = error.location;
  if (!location || !location.file.replaceAll("\\", "/").endsWith("/" + FILE) ||
    typeof error.message !== "string" || Buffer.byteLength(error.message) > 65536 ||
    error.cause !== undefined || error.value !== undefined) return null;
  const first = error.message.replace(/\u001b\[[0-9;]*m/g, "").split("\n")[0].replace(/^Error: /, "");
  if (first === "eligible-real-restore-history-transport" && location.line === 20 && location.column === 77) return first;
  if (first === "owned-fixture-stopped" && location.line === 26 && location.column === 65) return first;
  if (location.line === 22 && location.column === 71 && ASSERTION_IDS.some(id => first === id + "-attempted")) return first;
  if (location.line === 23 && location.column === 71 && ASSERTION_IDS.some(id => first === id + "-completed")) return first;
  if (location.line === 24 && location.column === 53 && ASSERTION_IDS.some(id => first === id)) return first;
  return null;
}
function admitErrors(result: TestResult, data: SafeCase | null): ErrorAdmission {
  const knownAssertionErrorIDs: string[] = [];
  let unknownErrorCount = Math.max(0, result.errors.length - 64);
  for (const error of result.errors.slice(0, 64)) {
    const id = errorIdentity(error);
    if (id) knownAssertionErrorIDs.push(id); else unknownErrorCount++;
  }
  const expected: string[] = [];
  if (data) {
    if (!data.eligible) expected.push("eligible-real-restore-history-transport");
    for (const id of ASSERTION_IDS) {
      const a = data.assertions[id];
      if (!a.attempted) expected.push(id + "-attempted");
      if (!a.completed) expected.push(id + "-completed");
      if (a.passed !== true) expected.push(id);
    }
    if (!data.fixtureStopped) expected.push("owned-fixture-stopped");
  }
  const assertionErrorsExact = data !== null && result.errors.length <= 64 && unknownErrorCount === 0 && result.retry === 0 &&
    knownAssertionErrorIDs.slice().sort().join(",") === expected.sort().join(",");
  return { retry: result.retry, totalErrorCount: result.errors.length, knownAssertionErrorIDs, unknownErrorCount, assertionErrorsExact };
}
export default class RestoreReporter implements Reporter {
  private mode = process.env.R06_RESTORE_REPORT_MODE;
  private selected: string[] = [];
  private cases = new Map<string, CaseResult>();
  private malformed = 0;
  private globalErrors = 0;
  private duplicates = 0;
  printsToStdio() { return false; }
  onBegin(_config: FullConfig, suite: Suite) {
    for (const test of suite.allTests()) {
      const id = Object.keys(CASES).find(key => CASES[key as keyof typeof CASES] === test.title);
      if (!id || !test.location.file.replaceAll("\\", "/").endsWith("/" + FILE) || this.selected.includes(id)) {
        this.malformed++; continue;
      }
      this.selected.push(id);
    }
    this.selected.sort();
    if (this.mode === "collection") {
      if (this.selected.join(",") !== Object.keys(CASES).sort().join(",")) this.malformed++;
    } else if (this.mode === "runtime") {
      const headers = ["r06-restore-headers-desktop", "r06-restore-headers-phone"];
      const body = ["r06-restore-inspect-body-desktop", "r06-restore-inspect-body-phone"];
      if (![headers.join(","), body.join(",")].includes(this.selected.join(","))) this.malformed++;
    } else this.malformed++;
  }
  onStdOut() {}
  onStdErr() {}
  onError() { this.globalErrors++; }
  onTestEnd(test: TestCase, result: TestResult) {
    const id = Object.keys(CASES).find(key => CASES[key as keyof typeof CASES] === test.title);
    if (!id || !this.selected.includes(id)) { this.malformed++; return; }
    if (this.cases.has(id)) { this.duplicates++; return; }
    const attachments = result.attachments.filter(a => a.name === SCHEMA && a.contentType === "application/json");
    let data: SafeCase | null = null;
    try {
      if (attachments.length !== 1) throw new Error("fixed-attachment-boundary");
      const attachment = attachments[0];
      if (attachment.body !== undefined && attachment.path !== undefined) throw new Error("fixed-attachment-boundary");
      let bytes: Buffer;
      if (attachment.body !== undefined) bytes = attachment.body;
      else {
        if (!attachment.path) throw new Error("fixed-attachment-boundary");
        bytes = readAttachment(attachment.path);
      }
      if (bytes.length > 32768) throw new Error("fixed-attachment-boundary");
      const parsed: JSONValue = JSON.parse(bytes.toString("utf8"));
      if (!validRecord(parsed, id)) throw new Error("fixed-record-boundary");
      data = parsed;
    } catch { this.malformed++; }
    const admission = admitErrors(result, data);
    const failedAssertions = data ? ASSERTION_IDS.filter(id => data.assertions[id].completed && data.assertions[id].passed === false) : [];
    const unattemptedAssertions = data ? ASSERTION_IDS.filter(id => !data.assertions[id].attempted) : [...ASSERTION_IDS];
    const incompleteAssertions = data ? ASSERTION_IDS.filter(id => !data.assertions[id].completed) : [...ASSERTION_IDS];
    const complete = data && data.eligible && data.fixtureStopped && data.failureStage === null &&
      incompleteAssertions.length === 0 && admission.assertionErrorsExact;
    const outcome = complete && result.status === "passed" && admission.totalErrorCount === 0 && failedAssertions.length === 0 ?
      "passed" : complete && result.status === "failed" && admission.totalErrorCount > 0 ? "failed" : "incomplete";
    this.cases.set(id, { caseID: id, outcome, data, failedAssertions, unattemptedAssertions, incompleteAssertions, ...admission });
  }
  onEnd(result: FullResult) {
    const collection = this.mode === "collection";
    const projection = {
      schema: SCHEMA, kind: collection ? "collection" : "runtime",
      collection: this.selected.map(caseID => ({ caseID, file: FILE, title: CASES[caseID as keyof typeof CASES] })),
      cases: collection ? [] : this.selected.map(caseID => this.cases.get(caseID) || {
        caseID, outcome: "incomplete", data: null, failedAssertions: [], unattemptedAssertions: [...ASSERTION_IDS],
        incompleteAssertions: [...ASSERTION_IDS], retry: 0, totalErrorCount: 0, knownAssertionErrorIDs: [],
        unknownErrorCount: 0, assertionErrorsExact: false,
      }),
      malformedRecords: this.malformed, duplicateTerminals: this.duplicates, globalErrorCount: this.globalErrors,
      runnerStatus: ["passed", "failed", "timedout", "interrupted"].includes(result.status) ? result.status : "unclassified",
    };
    const output = process.env.R06_RESTORE_SAFE_RESULTS;
    const bytes = JSON.stringify(projection) + "\n";
    if (!output || !isAbsolute(output) || Buffer.byteLength(bytes) > 131072) throw new Error("fixed-safe-projection-boundary");
    const descriptor = openSync(output, constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL | constants.O_NOFOLLOW, 0o600);
    try { writeFileSync(descriptor, bytes); } finally { closeSync(descriptor); }
  }
}
