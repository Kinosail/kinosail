import type { FullConfig, FullResult, Reporter, Suite, TestCase, TestError, TestResult } from "@playwright/test/reporter";
import type { AssertionID, JSONObject, JSONValue, SafeCase } from "./subtitle-save-recovery-helpers";
import { constants, closeSync, openSync, writeFileSync } from "node:fs";
import { readAttachment } from "./subtitle-save-attachment-reader.cjs";
import { isAbsolute } from "node:path";

const SCHEMA = "r06-save-browser-v1";
const FILE = "apps/subtitles/e2e/subtitle-save-recovery.journey.ts";
const CASES = {
  "save-headers-phone": "R06 Save headers held after completed write - phone",
  "save-headers-desktop": "R06 Save headers held after completed write - desktop",
  "save-body-phone": "R06 Save body held after completed write - phone",
  "save-body-desktop": "R06 Save body held after completed write - desktop",
} as const;
const STAGES = ["auth","preview","witness","deadline","new-edit","late-response","navigation","settlement","release-budget-exhausted"];
const ASSERTIONS: readonly AssertionID[] = [
  "actual-save-completed","actual-history-once","actual-recovery-retained","response-hold-eligible",
  "editing-unlocked-by-45s","finite-truthful-status","original-retained-if-uncertain","newer-edit-accepted",
  "newer-edit-survives-old-response","newer-status-survives-old-response","library-navigation-exercised",
  "browser-back-exercised","no-save-replay","current-server-state","fixture-settled",
];
const FIELDS = [
  "schema","kind","caseID","stage","failureStage","eligible","fixtureStopped","protocol","responseStatus",
  "saveAttempts","prepareAttempts","browserCompletedReads","browserSavedInspections","activeHolds",
  "actualSaved","historyOnce","recoveryMatches","inspectionMatches","receiptCompleted","receiptSucceeded",
  "holdEligible","holdExpired","saveRequestObserved","saveResponseObserved","saveTerminal","releaseAttempted",
  "responseBodyDelivered","fixtureClientCancelled","saveClickToWitnessMs","saveClickToUnlockMs",
  "holdDurationMs","durationMs","servedScriptSHA256","assertions",
];
const BOOLEAN_FIELDS = ["eligible","fixtureStopped","actualSaved","historyOnce","recoveryMatches",
  "inspectionMatches","receiptCompleted","receiptSucceeded","holdEligible","holdExpired",
  "saveRequestObserved","saveResponseObserved","releaseAttempted","responseBodyDelivered","fixtureClientCancelled"];
const COUNTERS = ["responseStatus","saveAttempts","prepareAttempts","browserCompletedReads","browserSavedInspections","activeHolds"];
const TIMINGS = ["saveClickToWitnessMs","saveClickToUnlockMs","holdDurationMs","durationMs"];
function exactKeys(value: JSONValue, keys: readonly string[]): value is JSONObject {
  return value !== null && typeof value === "object" && !Array.isArray(value) &&
    Object.keys(value).sort().join(",") === [...keys].sort().join(",");
}
function validRecord(value: JSONValue, id: string): value is SafeCase {
  if (!exactKeys(value, FIELDS) || value.schema !== SCHEMA || value.kind !== "runtime-case" ||
    value.caseID !== id || typeof value.stage !== "string" || !STAGES.includes(value.stage) ||
    value.failureStage !== null && (typeof value.failureStage !== "string" || !STAGES.includes(value.failureStage)) ||
    typeof value.protocol !== "string" || !["unreached","legacy","prepared"].includes(value.protocol) ||
    typeof value.saveTerminal !== "string" || !["unreached","pending","finished","request-failed"].includes(value.saveTerminal)) return false;
  if (BOOLEAN_FIELDS.some(key => typeof value[key] !== "boolean")) return false;
  if (COUNTERS.some(key => { const n = value[key]; return typeof n !== "number" || !Number.isSafeInteger(n) || n < 0 || n > 1000; })) return false;
  if (typeof value.responseStatus !== "number" || value.responseStatus > 599 ||
    TIMINGS.some(key => { const n = value[key]; return n !== null && (typeof n !== "number" || !Number.isFinite(n) || n < 0 || n > 90000); }) ||
    typeof value.durationMs !== "number") return false;
  if (!exactKeys(value.servedScriptSHA256, ["inspector"]) ||
    value.servedScriptSHA256.inspector !== null && (typeof value.servedScriptSHA256.inspector !== "string" || !/^[a-f0-9]{64}$/.test(value.servedScriptSHA256.inspector)) ||
    !exactKeys(value.assertions, ASSERTIONS)) return false;
  const assertions = value.assertions;
  return ASSERTIONS.every(key => {
    const assertion = assertions[key];
    return exactKeys(assertion, ["attempted","passed"]) && typeof assertion.attempted === "boolean" &&
      (assertion.attempted ? typeof assertion.passed === "boolean" : assertion.passed === null);
  });
}
type ErrorAdmission = { retry: number; totalErrorCount: number; knownAssertionErrorIDs: string[];
  unknownErrorCount: number; assertionErrorsExact: boolean };
type CaseResult = ErrorAdmission & { caseID: string; outcome: "passed" | "failed" | "incomplete";
  data: SafeCase | null; failedAssertions: AssertionID[]; unreachableAssertions: AssertionID[] };
function errorIdentity(error: TestError): string | null {
  const location = error.location;
  if (!location || !location.file.replaceAll("\\", "/").endsWith("/"+FILE) ||
    typeof error.message !== "string" || Buffer.byteLength(error.message) > 65536 ||
    error.cause !== undefined || error.value !== undefined) return null;
  const first = error.message.replace(/\u001b\[[0-9;]*m/g, "").split("\n")[0].replace(/^Error: /, "");
  if (first === "eligible-real-save-history-transport" && location.line === 21 && location.column === 74) return first;
  if (first === "owned-fixture-stopped" && location.line === 26 && location.column === 65) return first;
  if (location.line === 23 && location.column === 71 &&
    ASSERTIONS.some(id => first === id+"-attempted")) return first;
  if (location.line === 24 && location.column === 53 && ASSERTIONS.some(id => first === id)) return first;
  return null;
}
function admitErrors(result: TestResult, data: SafeCase | null): ErrorAdmission {
  const knownAssertionErrorIDs: string[] = [];
  let unknownErrorCount = Math.max(0, result.errors.length-32);
  for (const error of result.errors.slice(0,32)) {
    const id = errorIdentity(error);
    if (id) knownAssertionErrorIDs.push(id); else unknownErrorCount++;
  }
  const expected: string[] = [];
  if (data) {
    if (!data.eligible) expected.push("eligible-real-save-history-transport");
    for (const id of ASSERTIONS) {
      if (!data.assertions[id].attempted) expected.push(id+"-attempted");
      if (data.assertions[id].passed !== true) expected.push(id);
    }
    if (!data.fixtureStopped) expected.push("owned-fixture-stopped");
  }
  const assertionErrorsExact = data !== null && result.errors.length <= 32 && unknownErrorCount === 0 &&
    result.retry === 0 && knownAssertionErrorIDs.slice().sort().join(",") === expected.sort().join(",");
  return { retry:result.retry, totalErrorCount:result.errors.length,
    knownAssertionErrorIDs, unknownErrorCount, assertionErrorsExact };
}
export default class SaveReporter implements Reporter {
  private mode = process.env.R06_SAVE_REPORT_MODE;
  private selected: string[] = [];
  private cases = new Map<string, CaseResult>();
  private malformed = 0;
  private globalErrors = 0;
  private duplicates = 0;
  printsToStdio() { return false; }
  onBegin(_config: FullConfig, suite: Suite) {
    for (const test of suite.allTests()) {
      const id = Object.keys(CASES).find(key => CASES[key as keyof typeof CASES] === test.title);
      if (!id || !test.location.file.replaceAll("\\", "/").endsWith("/"+FILE) || this.selected.includes(id)) {
        this.malformed++; continue;
      }
      this.selected.push(id);
    }
    this.selected.sort();
    if (this.mode === "collection") {
      if (this.selected.join(",") !== Object.keys(CASES).sort().join(",")) this.malformed++;
    } else if (this.mode === "runtime") {
      const headers = ["save-headers-desktop","save-headers-phone"], body = ["save-body-desktop","save-body-phone"];
      if (![headers.join(","),body.join(",")].includes(this.selected.join(","))) this.malformed++;
    } else this.malformed++;
  }
  onStdOut() {}
  onStdErr() {}
  onError() { this.globalErrors++; }
  onTestEnd(test: TestCase, result: TestResult) {
    const id = Object.keys(CASES).find(key => CASES[key as keyof typeof CASES] === test.title);
    if (!id || !this.selected.includes(id)) { this.malformed++; return; }
    if (this.cases.has(id)) { this.duplicates++; return; }
    const attachments = result.attachments.filter(attachment => attachment.name === SCHEMA && attachment.contentType === "application/json");
    let data: SafeCase | null = null;
    try {
      if (attachments.length !== 1) throw new Error();
      const attachment = attachments[0]; let bytes: Buffer;
      if (attachment.body) bytes = attachment.body;
      else {
        if (!attachment.path) throw new Error();
        bytes = readAttachment(attachment.path);
      }
      if (bytes.length > 32768) throw new Error();
      const parsed: JSONValue = JSON.parse(bytes.toString("utf8"));
      if (!validRecord(parsed,id)) throw new Error();
      data = parsed;
    } catch { this.malformed++; }
    const failedAssertions = data ? ASSERTIONS.filter(key => data.assertions[key].attempted && data.assertions[key].passed === false) : [];
    const unreachableAssertions = data ? ASSERTIONS.filter(key => !data.assertions[key].attempted) : [...ASSERTIONS];
    const admission = admitErrors(result, data);
    const complete = data && data.fixtureStopped && data.failureStage === null && admission.assertionErrorsExact;
    const outcome = complete && result.status === "passed" && admission.totalErrorCount === 0 && failedAssertions.length === 0 && unreachableAssertions.length === 0 ?
      "passed" : complete && result.status === "failed" && admission.totalErrorCount > 0 ? "failed" : "incomplete";
    this.cases.set(id, { caseID:id, outcome, data, failedAssertions, unreachableAssertions, ...admission });
  }
  onEnd(result: FullResult) {
    const collection = this.mode === "collection";
    const projection = {
      schema:SCHEMA, kind:collection ? "collection" : "runtime",
      collection:this.selected.map(caseID => ({caseID,file:FILE,title:CASES[caseID as keyof typeof CASES]})),
      cases:collection ? [] : this.selected.map(caseID => this.cases.get(caseID) ||
        {caseID,outcome:"incomplete",data:null,failedAssertions:[],unreachableAssertions:[...ASSERTIONS],
          retry:0,totalErrorCount:0,knownAssertionErrorIDs:[],unknownErrorCount:0,assertionErrorsExact:false}),
      malformedRecords:this.malformed, duplicateTerminals:this.duplicates, globalErrorCount:this.globalErrors,
      runnerStatus:["passed","failed","timedout","interrupted"].includes(result.status) ? result.status : "unclassified",
    };
    const output = process.env.R06_SAVE_SAFE_RESULTS;
    if (!output || !isAbsolute(output) || Buffer.byteLength(JSON.stringify(projection)) > 131072) throw new Error("fixed-safe-projection-boundary");
    const descriptor = openSync(output, constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL, 0o600);
    try { writeFileSync(descriptor, JSON.stringify(projection)+"\n"); } finally { closeSync(descriptor); }
  }
}
