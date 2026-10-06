import { createHash } from "node:crypto";
import type { Page } from "@playwright/test";
import { bounded, pause, snapshot, startFixture, setupPage, phaseScope, cleanupCase, type PhaseScope, type Fixture, type Snapshot } from "./subtitle-restore-recovery-fixture";
import { observeRestore, type Terminal, type FailureCode } from "./subtitle-restore-recovery-network";

import { observeCausal } from "./subtitle-restore-causal-observer";
import type { Diagnostic } from "./subtitle-restore-causal-schema";
import { nativeBodylessCompletion } from "./subtitle-restore-causal-witness.mjs";

export const ASSERTION_IDS = [
  "actual-restore", "history-once", "recovery-swapped", "hold-proven",
  "editor-released-by-45s", "finite-truthful-outcome", "original-correction-kept",
  "original-import-kept", "library-navigation", "back-current-subtitle", "no-restore-replay", "fixture-settled",
] as const;
export type AssertionID = typeof ASSERTION_IDS[number];
export type Stage = "setup" | "witness" | "deadline" | "release" | "terminal" | "navigation" | "cleanup";
export type Entry = { id: string; title: string; mode: "headers" | "inspect-body"; width: 390 | 1280; height: 844 | 900 };
export type Observation = { attempted: boolean; completed: boolean; passed: boolean | null };
export type SafeCase = Snapshot & {
  schema: "r06-restore-browser-v1"; kind: "runtime-case"; caseID: string; stage: Stage;
  failureStage: Stage | null; eligible: boolean; fixtureStopped: boolean;
  restoreRequestObserved: boolean; restoreResponseObserved: boolean; restoreTerminal: Terminal;
  inspectRequestObserved: boolean; inspectResponseObserved: boolean; inspectTerminal: Terminal;
  restoreBodyDelivered: boolean; inspectionBodyDelivered: boolean; releaseAttempted: boolean;
  restoreFailureCode: FailureCode; inspectFailureCode: FailureCode;
  clickToWitnessMs: number | null; clickToUnlockMs: number | null; holdDurationMs: number | null;
  durationMs: number; servedScriptSHA256: { inspector: string | null };
  assertions: Record<AssertionID, Observation>; causalDiagnostic: Diagnostic | null;
};
const CORRECTION = "1\n00:00:01,000 --> 00:00:02,000\nFictional unsaved correction\n";
const IMPORT_NAME = "R06 Fictional Unsaved.srt";
type Clock = { elapsed: number; locked: boolean; unlocked: number | null };
async function installClock(page: Page) {
  await page.evaluate(() => {
    const button = document.getElementById("restore-subtitle");
    const form = document.getElementById("subtitle-edit-form");
    const text = form?.querySelector('textarea[name="text"]');
    const language = form?.querySelector('select[name="language"]');
    const file = form?.querySelector('input[name="file"]');
    if (!(button instanceof HTMLButtonElement) || !(form instanceof HTMLFormElement) ||
      !(text instanceof HTMLTextAreaElement) || !(language instanceof HTMLSelectElement) ||
      !(file instanceof HTMLInputElement)) throw new Error("fixed-observation-boundary");
    let clicked: number | null = null, unlocked: number | null = null, locked = false;
    const sample = () => {
      if (clicked === null) return;
      if (text.disabled && language.disabled && file.disabled) locked = true;
      if (locked && unlocked === null && !text.disabled && !text.readOnly && !language.disabled && !file.disabled) {
        unlocked = performance.now() - clicked;
      }
    };
    new MutationObserver(sample).observe(form, { subtree: true, attributes: true, attributeFilter: ["disabled", "readonly"] });
    button.addEventListener("click", () => { clicked = performance.now(); }, { capture: true, once: true });
    (window as Window & { __r06RestoreClock: () => Clock }).__r06RestoreClock = () => {
      sample(); return { elapsed: clicked === null ? -1 : performance.now() - clicked, locked, unlocked };
    };
  });
}
async function clock(page: Page): Promise<Clock> {
  const value = await page.evaluate(() => (window as Window & { __r06RestoreClock: () => Clock }).__r06RestoreClock());
  if (!Number.isFinite(value.elapsed) || value.elapsed < 0 || typeof value.locked !== "boolean" ||
    value.unlocked !== null && (!Number.isFinite(value.unlocked) || value.unlocked < 0)) throw new Error("fixed-observation-boundary");
  return value;
}
function initialResult(id: string): SafeCase {
  return {
    schema: "r06-restore-browser-v1", kind: "runtime-case", caseID: id, stage: "setup", failureStage: null,
    eligible: false, fixtureStopped: false, protocol: "unreached", setupSaveAttempts: 0, restoreAttempts: 0,
    prepareAttempts: 0, responseStatus: 0, inspectionResponseStatus: 0, activeHolds: 0,
    browserCompletedReceiptReads: 0, browserRestoredInspections: 0, actualRestored: false, historyOnce: false,
    recoverySwapped: false, inspectionMatches: false, receiptCompleted: false, receiptSucceeded: false,
    holdEligible: false, headersReleased: false, bodyReleased: false, restoreResponseDelivered: false,
    inspectionResponseDelivered: false, restoreClientCancelled: false, inspectionClientCancelled: false,
    holdExpired: false, boundaryFailed: false, restoreRequestObserved: false, restoreResponseObserved: false,
    restoreTerminal: "unreached", inspectRequestObserved: false, inspectResponseObserved: false, inspectTerminal: "unreached",
    restoreBodyDelivered: false, inspectionBodyDelivered: false, releaseAttempted: false,
    restoreFailureCode: "none", inspectFailureCode: "none",
    clickToWitnessMs: null, clickToUnlockMs: null, holdDurationMs: null, durationMs: 0,
    servedScriptSHA256: { inspector: null }, causalDiagnostic: null,
    assertions: Object.fromEntries(ASSERTION_IDS.map(id => [id, { attempted: false, completed: false, passed: null }])) as SafeCase["assertions"],
  };
}
async function servedIdentity(page: Page, origin: string): Promise<string> {
  const path = await page.locator('script[src*="/static/subtitle-inspector.js"]').getAttribute("src");
  if (!path || new URL(path, origin).origin !== origin) throw new Error("fixed-script-boundary");
  const response = await page.request.get(new URL(path, origin).href, { timeout: 5000 });
  const body = await response.body();
  if (response.status() !== 200 || body.length === 0 || body.length > 65536) throw new Error("fixed-script-boundary");
  return createHash("sha256").update(body).digest("hex");
}
async function observedDOM(page: Page) {
  return page.evaluate(() => {
    const status = document.getElementById("inspector-status"), text = document.querySelector('textarea[name="text"]');
    const file = document.querySelector('input[name="file"]');
    if (!(status instanceof HTMLElement) || !(text instanceof HTMLTextAreaElement) || !(file instanceof HTMLInputElement)) {
      throw new Error("fixed-observation-boundary");
    }
    return { status: status.textContent || "", text: text.value, imported: file.files?.length === 1 && file.files.item(0)?.name === "R06 Fictional Unsaved.srt" };
  });
}
async function settleHeld(page: Page, fixture: Fixture, network: ReturnType<typeof observeRestore>, result: SafeCase, mode: Entry["mode"], scope: PhaseScope, causal: Awaited<ReturnType<typeof observeCausal>> | undefined, end: number) {
  const remaining = () => Math.max(0, end - performance.now());
  const held = mode === "headers" ? network.restore : network.inspect;
  const terminal = await bounded(held.ended, remaining()); scope.guard();
  let state = await snapshot(page, fixture.origin, Math.min(3000, remaining())); scope.guard();
  while (state.activeHolds !== 0 && remaining() > 0) { await pause(50); scope.guard(); state = await snapshot(page, fixture.origin, Math.min(3000, remaining())); scope.guard(); }
  Object.assign(result, state);
  if(causal)result.causalDiagnostic=await causal.finish(end);
  const native204=nativeBodylessCompletion(result);
  if (terminal === "finished") {
    if (!held.response || held.response.status() !== (mode === "headers" ? state.responseStatus : httpOK)) throw new Error("fixed-terminal-boundary");
    const body = await bounded(held.response.body(), remaining()); scope.guard();
    if (body.length > 65536 || mode === "inspect-body" && body.length === 0) throw new Error("fixed-terminal-boundary");
    if (mode === "headers") {
      result.restoreBodyDelivered = state.restoreResponseDelivered && (state.protocol !== "legacy" || body.length === 0);
    } else result.inspectionBodyDelivered = state.inspectionResponseDelivered;
    if (!(mode === "headers" ? result.restoreBodyDelivered : result.inspectionBodyDelivered)) throw new Error("fixed-terminal-boundary");
  } else if(mode==="headers"&&native204) {
    result.restoreBodyDelivered=true; // Actual captured zero-byte204 plus unchanged native fulfillment; raw terminal retained.
  } else if (!(mode === "headers" ? state.restoreClientCancelled : state.inspectionClientCancelled)) {
    throw new Error("fixed-terminal-boundary");
  }
  if (state.activeHolds !== 0) throw new Error("fixed-terminal-boundary");
  if (mode === "inspect-body" && network.restore.terminal !== "finished" && !native204) throw new Error("fixed-restore-terminal-boundary");
}
const httpOK = 200;
export async function runRestoreCase(page: Page, entry: Entry): Promise<SafeCase> {
  const began = performance.now(), result = initialResult(entry.id), scope = phaseScope();
  const attempt = (...ids: AssertionID[]) => { scope.guard(); for (const id of ids) result.assertions[id] = { attempted: true, completed: false, passed: null }; };
  const record = (id: AssertionID, passed: boolean) => { scope.guard(); result.assertions[id] = { attempted: true, completed: true, passed }; };
  let fixture: Fixture | undefined, network: ReturnType<typeof observeRestore> | undefined;
  let witnessAt: number | null = null, item = "", terminalSettled = false;
  let causal: Awaited<ReturnType<typeof observeCausal>> | undefined;
  const caseRemaining = (limit: number) => {
    const available = began + 105000 - performance.now();
    if (available <= 0) throw new Error("fixed-phase-budget");
    return Math.min(limit, available);
  };
  try {
    await scope.run(async () => {
      fixture = await startFixture(entry.mode, created => { fixture = created; }, scope);
      scope.guard();
      await page.setViewportSize({ width: entry.width, height: entry.height }); scope.guard();
      item = await setupPage(page, fixture.origin, scope); scope.guard();
      causal = observeCausal(page, fixture.origin, item, began + 105000, began + 20000, entry.id);
      await causal.start(); scope.guard();
      await causal.probe(); scope.guard();
      const inspectorSHA = await servedIdentity(page, fixture.origin); scope.guard();
      result.servedScriptSHA256.inspector = inspectorSHA;
      await page.locator('input[name="file"]').setInputFiles({ name: IMPORT_NAME, mimeType: "application/x-subrip", buffer: Buffer.from(CORRECTION) }); scope.guard();
      await page.locator('textarea[name="text"]').fill(CORRECTION); scope.guard();
      await installClock(page); scope.guard();
      network = observeRestore(page, fixture.origin, item, result);
      await page.locator("#restore-subtitle").click({ timeout: 5000 }); scope.guard();
    }, 20000);
    if (!fixture || !network) throw new Error("fixed-setup-boundary");
    result.stage = "witness";
    attempt("actual-restore", "history-once", "recovery-swapped", "hold-proven");
    let observed = await clock(page);
    const eligible = (s: Snapshot, c: Clock) => {
      const nominal = s.protocol === "legacy" ? s.responseStatus === 204 :
        s.protocol === "prepared" && s.responseStatus === 202 && s.receiptCompleted && s.receiptSucceeded;
      const held = entry.mode === "headers" ? !s.headersReleased && !s.restoreResponseDelivered :
        s.inspectionResponseStatus === 200 && s.headersReleased && !s.inspectionResponseDelivered && result.inspectRequestObserved && result.inspectResponseObserved;
      return nominal && s.setupSaveAttempts === 1 && s.restoreAttempts === 1 &&
        s.actualRestored && s.historyOnce && s.recoverySwapped && s.inspectionMatches && s.holdEligible &&
        s.activeHolds === 1 && !s.boundaryFailed && !s.holdExpired && held && c.locked && result.restoreRequestObserved;
    };
    const witnessBudget = () => { const remaining = 10000 - observed.elapsed; if (remaining <= 0) throw new Error("fixed-witness-budget"); return remaining; };
    const witnessed = await scope.run(async () => {
      let state = await snapshot(page, fixture!.origin, Math.min(3000, witnessBudget())); scope.guard();
      observed = await clock(page); scope.guard();
      while (!eligible(state, observed) && observed.elapsed < 10000 && !state.boundaryFailed) {
        await pause(Math.min(100, witnessBudget())); scope.guard();
        observed = await clock(page); scope.guard();
        if (observed.elapsed >= 10000) break;
        state = await snapshot(page, fixture!.origin, Math.min(3000, witnessBudget())); scope.guard();
        observed = await clock(page); scope.guard();
      }
      return { state, eligible: observed.elapsed <= 10000 && eligible(state, observed) };
    }, witnessBudget());
    let state = witnessed.state; Object.assign(result, state); result.eligible = witnessed.eligible;
    record("actual-restore", state.actualRestored && state.inspectionMatches);
    record("history-once", state.historyOnce);
    record("recovery-swapped", state.recoverySwapped);
    record("hold-proven", result.eligible);
    if (!result.eligible) throw new Error("fixed-witness-boundary");
    witnessAt = performance.now(); result.clickToWitnessMs = observed.elapsed;
    result.stage = "deadline";
    attempt("editor-released-by-45s");
    while (observed.elapsed < 55000 && observed.unlocked === null) { await pause(100); observed = await clock(page); }
    result.clickToUnlockMs = observed.unlocked;
    record("editor-released-by-45s", observed.unlocked !== null && observed.unlocked <= 45000);
    attempt("finite-truthful-outcome", "original-correction-kept", "original-import-kept");
    const dom = await observedDOM(page); scope.guard();
    state = await snapshot(page, fixture.origin); scope.guard(); Object.assign(result, state);
    const restoredClaim = /previous subtitle restored|restore completed/i.test(dom.status);
    const authoritative = restoredClaim && state.browserRestoredInspections > 0 && network.inspect.terminal === "finished" &&
      (state.protocol === "legacy" ? network.restore.terminal === "finished" : state.browserCompletedReceiptReads > 0 && state.receiptSucceeded);
    const uncertain = /restor[\s\S]*(unknown|unconfirmed|not confirmed|taking longer|unavailable|timed out)/i.test(dom.status);
    record("finite-truthful-outcome", authoritative || uncertain && !/not restored|rolled back|nothing.*changed/i.test(dom.status));
    record("original-correction-kept", dom.text === CORRECTION);
    record("original-import-kept", dom.imported);
    result.stage = "release"; result.releaseAttempted = true;
    const releaseBudget = caseRemaining(5000);
    const released = await scope.run(() => page.request.get(fixture!.origin + "/__r06_restore/release", { timeout: releaseBudget }), releaseBudget);
    if (released.status() !== 204) throw new Error("fixed-release-boundary");
    result.holdDurationMs = performance.now() - witnessAt;
    result.stage = "terminal";
    const terminalEnd=Math.min(began+105000,performance.now()+5000);
    await scope.run(() => settleHeld(page, fixture!, network!, result, entry.mode, scope, causal, terminalEnd), terminalEnd-performance.now());
    terminalSettled = true;
    result.stage = "navigation";
    await scope.run(async () => {
      attempt("library-navigation");
      await page.getByRole("link", { name: "Back to library", exact: true }).click({ timeout: 5000 }); scope.guard();
      await page.waitForURL(url => url.origin === fixture!.origin && url.pathname === "/" && url.searchParams.get("view") === "library", { timeout: 5000 }); scope.guard();
      await page.locator("main.subtitle-main").waitFor({ state: "visible", timeout: 5000 }); scope.guard();
      const inspectorCount = await page.locator(".subtitle-inspector").count(); scope.guard();
      record("library-navigation", new URL(page.url()).origin === fixture!.origin && inspectorCount === 0);
      attempt("back-current-subtitle");
      await page.goBack({ waitUntil: "domcontentloaded", timeout: 5000 }); scope.guard();
      await page.waitForFunction(() => {
        const cues = document.getElementById("subtitle-cues")?.textContent;
        return typeof cues === "string" && cues.includes("Fictional original line") && cues.includes("Fictional later line") &&
          cues.includes("0:01.000") && cues.includes("0:04.000") && !cues.includes("0:01.500");
      }, undefined, { timeout: 5000 }); scope.guard();
      attempt("no-restore-replay");
      state = await snapshot(page, fixture!.origin); scope.guard(); Object.assign(result, state);
      record("back-current-subtitle", new URL(page.url()).pathname === "/subtitles/inspect/" + item && state.actualRestored && state.inspectionMatches);
      record("no-restore-replay", state.restoreAttempts === 1 && state.setupSaveAttempts === 1 && state.prepareAttempts <= 1 && state.historyOnce && state.recoverySwapped);
    }, caseRemaining(10000));
  } catch { if (result.failureStage === null) result.failureStage = result.stage; }
  finally {
    result.stage = "cleanup";
    if (fixture) result.assertions["fixture-settled"] = { attempted: true, completed: false, passed: null };
    network?.close();
    const cleanupEnd = Math.min(began + 110000, performance.now() + 5000);
    if (causal && result.causalDiagnostic===null) { try { result.causalDiagnostic = await causal.finish(cleanupEnd); } catch { result.causalDiagnostic = causal.invalid(); } }
    result.fixtureStopped = await cleanupCase(scope, page, fixture, cleanupEnd);
    if (fixture) result.assertions["fixture-settled"] = {
      attempted: true, completed: true,
      passed: terminalSettled && result.activeHolds === 0 && !result.holdExpired && !result.boundaryFailed && result.fixtureStopped,
    };
    result.durationMs = performance.now() - began;
  }
  return result;
}
