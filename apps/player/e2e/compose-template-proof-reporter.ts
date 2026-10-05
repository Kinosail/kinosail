import type { FullConfig, FullResult, Reporter, Suite, TestCase, TestError, TestResult } from "@playwright/test/reporter";
import { closeSync, openSync, realpathSync, statSync, writeFileSync } from "node:fs";
import { basename, dirname, isAbsolute, join, resolve, sep } from "node:path";
import { ASSERTION_IDS, ASSERTION_SOURCE, object, integer, safeLedger, scope, type AssertionID, type Ledger, type Json } from "./compose-template-recovery.observation";

const registry = {
  primary: ["Q47 headers deadline phone Player", "Q47 body deadline desktop Both"],
  recovery: ["Q47 lost request recovers with retained inputs", "Q47 failed HTTP recovers with retained inputs"],
  supersession: ["Q47 input change discards late headers", "Q47 app change discards late body"],
  contracts: ["Q47 Player preserves Compose download and copy", "Q47 Subtitles preserves Compose download and copy",
    "Q47 Both preserves Compose download and copy", "Q47 rejects invalid inputs before template requests"],
};
const names = Object.values(registry).flat();
const modes = ["valid", "headers", "body", "loss", "late_headers", "late_body", "http_error"];
const stages = ["q47-asset", "q47-started", "q47-pending", "q47-deadline", "q47-failed", "q47-recovered", "q47-stale", "q47-rejected"];
const bools = ["createDisabled", "resultVisible", "errorVisible", "fallbackVisible", "fallbackCanonical", "downloadReady",
  "retainedApp", "retainedPath", "retainedPort", "retainedSecondPort"];
const checks = ["cookieSeen", "authorizationSeen", "bodySeen", "querySeen", "bodyPrefixSent"];
const counts = ["requests", "active", "holds", "canceled", "completed", "failures", "expired", "templateBytes"];
type State = {
  action: string; createDisabled: boolean; resultVisible: boolean; errorVisible: boolean;
  fallbackVisible: boolean; fallbackCanonical: boolean; downloadReady: boolean;
  retainedApp: boolean; retainedPath: boolean; retainedPort: boolean; retainedSecondPort: boolean;
  blobMatches: boolean | null; copyMatches: boolean | null; blobSHA256: string | null;
  blobBytes: number | null; elapsedMs: number; responses: number;
};
type Peer = {
  mode: string; app: string; templateSHA256: string;
  counts: { requests: number; active: number; holds: number; canceled: number; completed: number; failures: number; expired: number; templateBytes: number };
  checks: { cookieSeen: boolean; authorizationSeen: boolean; bodySeen: boolean; querySeen: boolean; bodyPrefixSent: boolean };
};
type Observation = { asset: { bytes: number; sha256: string; sourceMatches: boolean } } | { state: State; peer: Peer };
type ErrorAdmission = { totalErrorCount: number; knownAssertionErrorIDs: AssertionID[]; unknownErrorCount: number; assertionErrorsExact: boolean };
type Case = ErrorAdmission & { name: string; status: TestResult["status"]; retry: number; durationMs: number;
  outcome: "passed" | "failed" | "incomplete"; failure: "none" | "known-assertion" | "unclassified";
  ledger: Ledger | null; failedAssertions: AssertionID[]; unattemptedAssertions: AssertionID[]; incompleteAssertions: AssertionID[];
  deadlineDisposition: "within-product-deadline" | "observation-grace" | "no-recovery-observed" | "ineligible" | "not-applicable";
  observations: { stage: string; observation: Observation }[] };
const digest = (value: Json): value is string => typeof value === "string" && /^[0-9a-f]{64}$/.test(value);
function safeState(value: Json): State | null {
  if (!object(value, [...bools, "action", "blobMatches", "copyMatches", "blobSHA256", "blobBytes", "elapsedMs", "responses"])) return null;
  if (typeof value.action !== "string" || !["make", "preparing", "retry", "unknown"].includes(value.action) ||
    !bools.every(key => typeof value[key] === "boolean") ||
    !integer(value.elapsedMs, 70_000) || !integer(value.responses, 32)) return null;
  if (![value.blobMatches, value.copyMatches].every(item => item === null || typeof item === "boolean") ||
    !(value.blobSHA256 === null || digest(value.blobSHA256)) ||
    !(value.blobBytes === null || integer(value.blobBytes, 65_536, 32))) return null;
  return {
    action: value.action, elapsedMs: value.elapsedMs, responses: value.responses,
    createDisabled: value.createDisabled as boolean, resultVisible: value.resultVisible as boolean,
    errorVisible: value.errorVisible as boolean, fallbackVisible: value.fallbackVisible as boolean,
    fallbackCanonical: value.fallbackCanonical as boolean, downloadReady: value.downloadReady as boolean,
    retainedApp: value.retainedApp as boolean, retainedPath: value.retainedPath as boolean,
    retainedPort: value.retainedPort as boolean, retainedSecondPort: value.retainedSecondPort as boolean,
    blobMatches: value.blobMatches as boolean | null, copyMatches: value.copyMatches as boolean | null,
    blobSHA256: value.blobSHA256 as string | null, blobBytes: value.blobBytes as number | null,
  };
}
function safePeer(value: Json): Peer | null {
  if (!object(value, ["mode", "app", "templateSHA256", "counts", "checks"]) ||
    typeof value.mode !== "string" || !modes.includes(value.mode) || typeof value.app !== "string" || !["player", "subtitles", "both"].includes(value.app) ||
    !digest(value.templateSHA256) || !object(value.counts, counts) || !object(value.checks, checks)) return null;
  const quantities = value.counts, flags = value.checks;
  if (!counts.every(key => integer(quantities[key], key === "templateBytes" ? 16_384 : 32, key === "templateBytes" ? 32 : 0)) ||
    !checks.every(key => typeof flags[key] === "boolean")) return null;
  return { mode: value.mode, app: value.app, templateSHA256: value.templateSHA256,
    counts: {
      requests: quantities.requests as number, active: quantities.active as number, holds: quantities.holds as number,
      canceled: quantities.canceled as number, completed: quantities.completed as number, failures: quantities.failures as number,
      expired: quantities.expired as number, templateBytes: quantities.templateBytes as number,
    },
    checks: { cookieSeen: flags.cookieSeen as boolean, authorizationSeen: flags.authorizationSeen as boolean,
      bodySeen: flags.bodySeen as boolean, querySeen: flags.querySeen as boolean, bodyPrefixSent: flags.bodyPrefixSent as boolean },
  };
}
function observation(name: string, value: Json): Observation | null {
  if (name === "q47-asset") {
    if (!object(value, ["asset"]) || !object(value.asset, ["bytes", "sha256", "sourceMatches"]) ||
      !integer(value.asset.bytes, 65_536, 1) || !digest(value.asset.sha256) ||
      typeof value.asset.sourceMatches !== "boolean") return null;
    return { asset: { bytes: value.asset.bytes, sha256: value.asset.sha256, sourceMatches: value.asset.sourceMatches } };
  }
  if (!object(value, ["state", "peer"])) return null;
  const state = safeState(value.state), peer = safePeer(value.peer);
  return state && peer ? { state, peer } : null;
}
function errorIdentity(error: TestError): AssertionID | null {
  const location = error.location;
  if (!location || typeof location.file !== "string" || location.file.length > 4096 ||
    !location.file.replaceAll("\\", "/").endsWith("/" + ASSERTION_SOURCE.file) ||
    location.line !== ASSERTION_SOURCE.line || location.column !== ASSERTION_SOURCE.column ||
    typeof error.message !== "string" || Buffer.byteLength(error.message) > 65_536 ||
    error.cause !== undefined || error.value !== undefined) return null;
  const first = error.message.replace(/\u001b\[[0-9;]*m/g, "").split("\n")[0].replace(/^Error: /, "");
  return ASSERTION_IDS.find(id => first === id) ?? null;
}
function admitErrors(result: TestResult, ledger: Ledger | null): ErrorAdmission {
  const knownAssertionErrorIDs: AssertionID[] = [];
  let unknownErrorCount = Math.max(0, result.errors.length - 32);
  for (const error of result.errors.slice(0, 32)) {
    const id = errorIdentity(error);
    if (id) knownAssertionErrorIDs.push(id); else unknownErrorCount++;
  }
  const expected = ledger ? ASSERTION_IDS.filter(id => ledger.assertions[id].completed && ledger.assertions[id].passed === false) : [];
  const assertionErrorsExact = ledger !== null && result.errors.length <= 32 && unknownErrorCount === 0 && result.retry === 0 &&
    knownAssertionErrorIDs.slice().sort().join(",") === expected.slice().sort().join(",");
  return { totalErrorCount: result.errors.length, knownAssertionErrorIDs, unknownErrorCount, assertionErrorsExact };
}
function deadlineDisposition(ledger: Ledger | null, name: string): Case["deadlineDisposition"] {
  if (!name.startsWith("Q47 headers deadline") && !name.startsWith("Q47 body deadline")) return "not-applicable";
  const clock = ledger?.clock;
  if (!clock || !clock.trustedClick || clock.clicks !== 1 || !clock.pendingObserved || !clock.sample ||
    clock.sample.elapsedMs < 20_000 || clock.sample.elapsedMs > 20_200) return "ineligible";
  if (clock.firstRecoveryMs === null || clock.firstEnabledMs === null) return "no-recovery-observed";
  if (clock.firstRecoveryMs <= 20_000 && clock.firstEnabledMs <= 20_000) return "within-product-deadline";
  return clock.firstRecoveryMs <= 20_200 ? "observation-grace" : "ineligible";
}
function privateDestination() {
  const root = process.env.RUNNER_TEMP ?? "", destination = process.env.KINOSAIL_Q47_REPORT_FILE ?? "";
  if (!root || !destination || root.length > 4096 || destination.length > 4096 ||
    !isAbsolute(root) || !isAbsolute(destination) || root === sep ||
    basename(destination) !== "q47-proof.json" || resolve(root) !== root || resolve(destination) !== destination) return null;
  const realRoot = realpathSync(root), parent = realpathSync(dirname(destination));
  if (realRoot === sep || realRoot !== root || (parent !== realRoot && !parent.startsWith(realRoot + sep)) || parent !== dirname(destination) ||
    (statSync(parent).mode & 0o077) !== 0) return null;
  return join(parent, "q47-proof.json");
}

export default class ComposeTemplateReporter implements Reporter {
  private suite = process.env.KINOSAIL_Q47_SUITE ?? "primary";
  private phase = process.env.KINOSAIL_Q47_COLLECTION_ONLY === "1" ? "collection" : "journey";
  private collected: string[] = [];
  private cases: Case[] = [];
  private errors: string[] = [];
  private runnerErrorCount = 0;
  private problem(code: string) { if (this.errors.length < 16) this.errors.push(code); }
  onBegin(config: FullConfig, suite: Suite) {
    if (!Object.hasOwn(registry, this.suite)) this.problem("suite_invalid");
    if (config.projects.length !== 1 || config.projects[0].name !== "chromium" || config.projects[0].retries !== 0 || config.workers !== 1) this.problem("project_invalid");
    const tests = suite.allTests();
    if (tests.length > 10) this.problem("collection_bound");
    this.collected = tests.slice(0, 10).map(test => names.includes(test.title) ? test.title : "unknown");
    if (this.collected.includes("unknown") || tests.some(test => !test.location.file.replaceAll("\\", "/").endsWith("/apps/player/e2e/compose-template-recovery.journey.ts"))) this.problem("collection_invalid");
  }
  onTestEnd(test: TestCase, result: TestResult) {
    if (this.cases.length >= 10) { this.problem("case_bound"); return; }
    const name = names.includes(test.title) ? test.title : "unknown";
    if (name === "unknown" || !integer(result.retry, 3) || !integer(result.duration, 70_000) || result.errors.length > 1_024 ||
      !["passed", "failed", "timedOut", "skipped", "interrupted"].includes(result.status)) {
      this.problem("case_invalid"); return;
    }
    if (result.attachments.length > 10 || result.errors.length > 32 || result.retry !== 0) this.problem("attachment_or_error_bound");
    const records: Case["observations"] = [], seen = new Set<string>();
    let ledger: Ledger | null = null, malformed = false;
    for (const attachment of result.attachments.slice(0, 10)) {
      if ((!stages.includes(attachment.name) && attachment.name !== "q47-assertions") ||
        attachment.contentType !== "application/json" || seen.has(attachment.name) || attachment.path !== undefined) {
        this.problem("attachment_invalid"); malformed = true; continue;
      }
      seen.add(attachment.name);
      try {
        const body = attachment.body;
        const parsed: Json = body && body.length > 0 && body.length <= 65_536 ? JSON.parse(body.toString("utf8")) : null;
        if (attachment.name === "q47-assertions") {
          ledger = safeLedger(parsed, name);
          if (!ledger) { this.problem("assertions_invalid"); malformed = true; }
        } else {
          const safe = observation(attachment.name, parsed);
          if (!safe) { this.problem("observation_invalid"); malformed = true; }
          else records.push({ stage: attachment.name, observation: safe });
        }
      } catch { this.problem("observation_invalid"); malformed = true; }
    }
    if (!ledger || result.attachments.length > 10) { this.problem("assertions_missing_or_bound"); malformed = true; }
    const selected = scope(name);
    const failedAssertions = ledger ? selected.filter(id => ledger!.assertions[id].completed && ledger!.assertions[id].passed === false) : [];
    const unattemptedAssertions = ledger ? selected.filter(id => !ledger!.assertions[id].attempted) : selected;
    const incompleteAssertions = ledger ? selected.filter(id => ledger!.assertions[id].attempted && !ledger!.assertions[id].completed) : [];
    const admission = admitErrors(result, ledger);
    const outcome = !malformed && admission.assertionErrorsExact && result.status === "passed" &&
      admission.totalErrorCount === 0 && failedAssertions.length === 0 && unattemptedAssertions.length === 0 && incompleteAssertions.length === 0 ?
      "passed" : !malformed && admission.assertionErrorsExact && result.status === "failed" && admission.totalErrorCount > 0 ? "failed" : "incomplete";
    this.cases.push({ name, status: result.status, retry: result.retry, durationMs: result.duration, outcome,
      failure: outcome === "passed" ? "none" : outcome === "failed" ? "known-assertion" : "unclassified",
      ledger, failedAssertions, unattemptedAssertions, incompleteAssertions, ...admission,
      deadlineDisposition: deadlineDisposition(ledger, name), observations: records });
  }
  onError() { this.runnerErrorCount++; this.problem("runner_error"); }
  onEnd(result: FullResult) {
    const valid = ["passed", "failed", "timedout", "interrupted"].includes(result.status);
    if (!valid) this.problem("terminal_invalid");
    const expected = registry[this.suite as keyof typeof registry] ?? [];
    if (this.collected.join("\n") !== expected.join("\n")) this.problem("selection_invalid");
    if (this.phase === "collection") {
      if (this.cases.length !== 0) this.problem("collection_case_emitted");
    } else {
      if (this.cases.length !== expected.length || this.cases.map(entry => entry.name).join("\n") !== expected.join("\n")) this.problem("case_incomplete");
      if (result.status === "passed" && (!this.cases.length || this.cases.some(entry => entry.status !== "passed" || entry.retry !== 0 || entry.outcome !== "passed"))) this.problem("empty_or_inconsistent_green");
    }
    if (this.runnerErrorCount !== 0 || !["passed", "failed"].includes(result.status)) {
      for (const entry of this.cases) { entry.outcome = "incomplete"; entry.failure = "unclassified"; }
    }
    const report = { schemaVersion: 2, campaign: "Q47", phase: this.phase,
      suite: Object.hasOwn(registry, this.suite) ? this.suite : "invalid", status: valid ? result.status : "failed",
      collected: this.collected, cases: this.cases, errors: this.errors, runnerErrorCount: this.runnerErrorCount };
    try {
      const destination = privateDestination(), serialized = JSON.stringify(report) + "\n";
      if (!destination || Buffer.byteLength(serialized) > 262_144) { process.exitCode = 2; return; }
      const descriptor = openSync(destination, "wx", 0o600);
      try { writeFileSync(descriptor, serialized); } finally { closeSync(descriptor); }
    } catch { process.exitCode = 2; }
  }
  printsToStdio() { return false; }
}
