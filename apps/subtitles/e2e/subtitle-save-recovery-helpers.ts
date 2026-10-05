import { createHash } from "node:crypto";
import { spawn, type ChildProcess } from "node:child_process";
import { mkdtemp } from "node:fs/promises";
import { join, isAbsolute } from "node:path";
import type { Page, TestInfo } from "@playwright/test";
import { observeSave } from "./subtitle-save-recovery-network";
import { owner } from "./subtitle-save-recovery-auth";

export const ASSERTION_IDS = [
  "actual-save-completed", "actual-history-once", "actual-recovery-retained",
  "response-hold-eligible", "editing-unlocked-by-45s", "finite-truthful-status",
  "original-retained-if-uncertain", "newer-edit-accepted",
  "newer-edit-survives-old-response", "newer-status-survives-old-response",
  "library-navigation-exercised", "browser-back-exercised", "no-save-replay",
  "current-server-state", "fixture-settled",
] as const;
export type AssertionID = typeof ASSERTION_IDS[number];
export type JSONValue = null | boolean | number | string | JSONObject | JSONValue[];
export type JSONObject = { [key: string]: JSONValue };
export function isObject(value: JSONValue): value is JSONObject { return value !== null && typeof value === "object" && !Array.isArray(value); }
export type Stage = "auth" | "preview" | "witness" | "deadline" | "new-edit" | "late-response" | "navigation" | "settlement" | "release-budget-exhausted";
type Entry = { id: string; title: string; mode: "headers" | "body"; width: 390 | 1440 };
type Snapshot = {
  protocol: "unreached" | "legacy" | "prepared";
  saveAttempts: number; prepareAttempts: number; responseStatus: number;
  actualSaved: boolean; historyOnce: boolean; recoveryMatches: boolean;
  inspectionMatches: boolean; receiptCompleted: boolean; receiptSucceeded: boolean;
  holdEligible: boolean; headersReleased: boolean; bodyReleased: boolean;
  holdExpired: boolean; activeHolds: number; browserCompletedReads: number;
  browserSavedInspections: number; boundaryFailed: boolean;
  responseBodyWritten: boolean; clientCancelled: boolean;
};
export type SafeCase = {
  schema: "r06-save-browser-v1"; kind: "runtime-case"; caseID: string; stage: Stage;
  failureStage: Stage | null; eligible: boolean; fixtureStopped: boolean;
  protocol: Snapshot["protocol"]; responseStatus: number;
  saveAttempts: number; prepareAttempts: number; browserCompletedReads: number;
  browserSavedInspections: number; activeHolds: number;
  actualSaved: boolean; historyOnce: boolean; recoveryMatches: boolean;
  inspectionMatches: boolean; receiptCompleted: boolean; receiptSucceeded: boolean;
  holdEligible: boolean; holdExpired: boolean;
  saveRequestObserved: boolean; saveResponseObserved: boolean;
  saveTerminal: "unreached" | "pending" | "finished" | "request-failed";
  releaseAttempted: boolean; responseBodyDelivered: boolean; fixtureClientCancelled: boolean;
  saveClickToWitnessMs: number | null; saveClickToUnlockMs: number | null;
  holdDurationMs: number | null; durationMs: number;
  servedScriptSHA256: { inspector: string | null };
  assertions: Record<AssertionID, { attempted: boolean; passed: boolean | null }>;
};
const SAVED = "1\n00:00:01,000 --> 00:00:02,000\nFictional reviewed line\n\n2\n00:00:04,000 --> 00:00:05,000\nFictional reviewed later line\n";
const NEWER = "1\n00:00:01,000 --> 00:00:02,000\nFictional newer unsaved correction\n";
const pause = (milliseconds: number) => new Promise<void>(resolve => setTimeout(resolve, milliseconds));
const bounded = async <T>(promise: Promise<T>, milliseconds: number): Promise<T> => {
  let timer: ReturnType<typeof setTimeout>;
  try { return await Promise.race([promise, new Promise<T>((_, reject) => { timer = setTimeout(() => reject(new Error("fixed-fixture-boundary")), milliseconds); })]); }
  finally { clearTimeout(timer!); }
};
type BrowserClock = { elapsed: number; unlocked: number | null };
async function installClock(page: Page) {
  await page.evaluate(() => {
    const button = document.getElementById("apply-subtitle");
    const form = document.getElementById("subtitle-edit-form");
    if (!button || !form) throw new Error("fixed-observation-boundary");
    let clicked: number | null = null, unlocked: number | null = null;
    const sample = () => {
      const text = form.querySelector<HTMLTextAreaElement>('textarea[name="text"]');
      const language = form.querySelector<HTMLSelectElement>('select[name="language"]');
      const file = form.querySelector<HTMLInputElement>('input[name="file"]');
      if (clicked !== null && unlocked === null && text && language && file &&
        !text.disabled && !text.readOnly && !language.disabled && !file.disabled) unlocked = performance.now()-clicked;
    };
    const observer = new MutationObserver(sample);
    observer.observe(form, {subtree:true,attributes:true,attributeFilter:["disabled","readonly"]});
    button.addEventListener("click", () => { clicked = performance.now(); }, {capture:true,once:true});
    (window as Window & {__r06SaveClock:()=>BrowserClock}).__r06SaveClock = () => {
      sample(); return {elapsed:clicked === null ? -1 : performance.now()-clicked,unlocked};
    };
  });
}
async function clock(page: Page): Promise<BrowserClock> {
  const value = await page.evaluate(() => (window as Window & {__r06SaveClock:()=>BrowserClock}).__r06SaveClock());
  if (!Number.isFinite(value.elapsed) || value.elapsed < 0 ||
    value.unlocked !== null && (!Number.isFinite(value.unlocked) || value.unlocked < 0)) throw new Error("fixed-observation-boundary");
  return value;
}
async function startFixture(mode: Entry["mode"]): Promise<{ child: ChildProcess; origin: string; closed: Promise<void> }> {
  const binary = process.env.R06_SAVE_FIXTURE_BINARY, directory = process.env.R06_SAVE_PRIVATE_ROOT;
  if (!binary || !directory || !isAbsolute(binary) || !isAbsolute(directory)) throw new Error("fixed-fixture-boundary");
  const root = await mkdtemp(join(directory, "r06-case-"));
  const child = spawn(binary, ["-r06-serve", "-r06-root", root, "-r06-fault", mode], {
    stdio: ["ignore", "pipe", "pipe"], detached: false,
    env: { PATH: process.env.PATH || "", HOME: process.env.HOME || "", TMPDIR: process.env.TMPDIR || "", LANG: "C.UTF-8" },
  });
  const closed = new Promise<void>(resolve => child.once("close", () => resolve()));
  let buffer = ""; let stderrBytes = 0;
  child.stderr?.on("data", chunk => { stderrBytes += chunk.length; if (stderrBytes > 65536) child.kill("SIGTERM"); });
  const ready = new Promise<string>((resolve, reject) => {
    child.once("error", () => reject(new Error("fixed-fixture-boundary")));
    child.once("exit", () => reject(new Error("fixed-fixture-boundary")));
    child.stdout?.on("data", chunk => {
      buffer += chunk.toString("utf8"); if (Buffer.byteLength(buffer) > 65536) { child.kill("SIGTERM"); reject(new Error("fixed-fixture-boundary")); return; }
      const newline = buffer.indexOf("\n"); if (newline < 0) return;
      try { const value: JSONValue = JSON.parse(buffer.slice(0, newline));
        if (!isObject(value) || Object.keys(value).sort().join(",") !== "Kind,Origin" || value.Kind !== "r06-save-fixture-v1" || typeof value.Origin !== "string" || !/^https:\/\/127\.0\.0\.1:\d{1,5}$/.test(value.Origin)) throw new Error();
        resolve(value.Origin);
      } catch { child.kill("SIGTERM"); reject(new Error("fixed-fixture-boundary")); }
    });
  });
  try { return { child, origin: await bounded(ready, 15000), closed }; }
  catch { await stopFixture(child, closed); throw new Error("fixed-fixture-boundary"); }
}
async function stopFixture(child: ChildProcess, closed: Promise<void>): Promise<boolean> {
  if (child.exitCode === null) child.kill("SIGTERM");
  try { await bounded(closed, 3000); return true; }
  catch { child.kill("SIGKILL"); try { await bounded(closed, 2000); return true; } catch { return false; } }
}
function validSnapshot(data: JSONValue): data is Snapshot {
  const keys = ["protocol","saveAttempts","prepareAttempts","responseStatus","actualSaved","historyOnce","recoveryMatches","inspectionMatches","receiptCompleted","receiptSucceeded","holdEligible","headersReleased","bodyReleased","holdExpired","activeHolds","browserCompletedReads","browserSavedInspections","boundaryFailed","responseBodyWritten","clientCancelled"];
  if (!isObject(data) || Object.keys(data).sort().join(",") !== keys.sort().join(",") ||
    typeof data.protocol !== "string" || !["unreached","legacy","prepared"].includes(data.protocol)) return false;
  return keys.filter(key => key !== "protocol").every(key => {
    const value = data[key];
    return ["saveAttempts","prepareAttempts","responseStatus","activeHolds","browserCompletedReads","browserSavedInspections"].includes(key) ?
      typeof value === "number" && Number.isSafeInteger(value) && value >= 0 && value <= 1000 : typeof value === "boolean";
  });
}
async function snapshot(page: Page, origin: string): Promise<Snapshot> {
  const response = await page.request.get(origin + "/__r06/witness", { timeout: 3000 });
  if (response.status() !== 200) throw new Error("fixed-fixture-boundary");
  const data: JSONValue = await response.json();
  if (!validSnapshot(data)) throw new Error("fixed-fixture-boundary");
  return data;
}
function applySnapshot(result: SafeCase, state: Snapshot) {
  for (const key of ["protocol","saveAttempts","prepareAttempts","responseStatus","actualSaved","historyOnce","recoveryMatches","inspectionMatches","receiptCompleted","receiptSucceeded","holdEligible","holdExpired","activeHolds","browserCompletedReads","browserSavedInspections"] as const) {
    Object.assign(result, { [key]: state[key] });
  }
  result.fixtureClientCancelled = state.clientCancelled;
}
export async function runSaveCase(page: Page, _info: TestInfo, entry: Entry): Promise<SafeCase> {
  const began = performance.now();
  const assertions = Object.fromEntries(ASSERTION_IDS.map(id => [id, { attempted: false, passed: null }])) as SafeCase["assertions"];
  const result: SafeCase = { schema:"r06-save-browser-v1", kind:"runtime-case", caseID:entry.id,
    stage:"auth", failureStage:null, eligible:false, fixtureStopped:false, protocol:"unreached", responseStatus:0,
    saveAttempts:0, prepareAttempts:0, browserCompletedReads:0, browserSavedInspections:0, activeHolds:0,
    actualSaved:false, historyOnce:false, recoveryMatches:false, inspectionMatches:false, receiptCompleted:false,
    receiptSucceeded:false, holdEligible:false, holdExpired:false, saveRequestObserved:false,
    saveResponseObserved:false, saveTerminal:"unreached", releaseAttempted:false, responseBodyDelivered:false,
    fixtureClientCancelled:false, saveClickToWitnessMs:null, saveClickToUnlockMs:null, holdDurationMs:null, durationMs:0, servedScriptSHA256:{inspector:null}, assertions };
  const record = (id: AssertionID, passed: boolean) => { assertions[id] = { attempted:true, passed }; };
  let processFixture: Awaited<ReturnType<typeof startFixture>> | undefined;
  let witnessAt: number | undefined, save: ReturnType<typeof observeSave> | undefined;
  const remaining = (limit: number) => Math.max(1, Math.min(limit, began+65000-performance.now()));
  try {
    processFixture = await startFixture(entry.mode);
    const origin = processFixture.origin;
    await page.setViewportSize({ width:entry.width, height:entry.width === 390 ? 844 : 960 });
    await owner(page, origin);
    const catalogue = await page.request.get(origin+"/api/v1/subtitle-library?view=library", { timeout:5000 });
    const data: JSONValue = await catalogue.json();
    if (catalogue.status() !== 200 || !isObject(data) || !Array.isArray(data.items) || data.items.length !== 1 || !isObject(data.items[0]) || typeof data.items[0].id !== "string" || !/^[a-f0-9]{16}$/.test(data.items[0].id)) throw new Error("fixed-fixture-boundary");
    const item = data.items[0].id;
    await page.goto(origin+"/subtitles/inspect/"+item+"?language=en", { waitUntil:"domcontentloaded", timeout:10000 });
    await page.locator('#subtitle-edit-form button[type="submit"]').waitFor({ state:"visible", timeout:5000 });
    await page.locator('details').filter({ has:page.locator("#edit-preview-text") }).locator("summary").click();
    await page.locator("#edit-preview-text").click({ timeout:5000 });
    await page.locator('textarea[name="text"]').fill(SAVED);
    if (await page.locator('input[name="automaticSync"]').isChecked()) throw new Error("fixed-preview-boundary");
    result.stage = "preview";
    await page.locator('#subtitle-edit-form button[type="submit"]').click({ timeout:5000 });
    await page.locator("#apply-subtitle").waitFor({ state:"visible", timeout:5000 });
    const script = await page.locator('script[src*="/static/subtitle-inspector.js"]').getAttribute("src");
    if (!script || new URL(script, origin).origin !== origin) throw new Error("fixed-fixture-boundary");
    const scriptResponse = await page.request.get(new URL(script, origin).href, { timeout:5000 });
    if (scriptResponse.status() !== 200) throw new Error("fixed-fixture-boundary");
    result.servedScriptSHA256.inspector = createHash("sha256").update(await scriptResponse.body()).digest("hex");
    await installClock(page);
    save = observeSave(page, origin, item, result);
    await page.locator("#apply-subtitle").click({ timeout:5000 });
    result.stage = "witness";
    let state = await snapshot(page, origin);
    const witnessEnd = performance.now()+10000;
    while ((!state.holdEligible || (entry.mode === "body" && !state.headersReleased)) && performance.now() < witnessEnd && !state.boundaryFailed) {
      await pause(100); state = await snapshot(page, origin);
    }
    applySnapshot(result, state);
    result.eligible = state.holdEligible && state.actualSaved && state.historyOnce &&
      state.recoveryMatches && state.inspectionMatches && !state.boundaryFailed && !state.holdExpired &&
      state.saveAttempts === 1 && state.activeHolds === 1 && !state.clientCancelled && result.saveRequestObserved && !state.bodyReleased && !state.responseBodyWritten &&
      (entry.mode === "headers" ? !state.headersReleased : state.headersReleased) &&
      (state.protocol === "legacy" ? state.responseStatus === 200 : state.receiptSucceeded && state.responseStatus === 202);
    record("actual-save-completed", state.actualSaved);
    record("actual-history-once", state.historyOnce);
    record("actual-recovery-retained", state.recoveryMatches);
    record("response-hold-eligible", result.eligible);
    if (!result.eligible) throw new Error("fixed-witness-boundary");
    witnessAt = performance.now(); result.saveClickToWitnessMs = (await clock(page)).elapsed;
    result.stage = "deadline";
    const text = page.locator('textarea[name="text"]');
    let observedClock = await clock(page);
    while (observedClock.elapsed < 55000 && observedClock.unlocked === null) {
      await pause(100); observedClock = await clock(page);
    }
    result.saveClickToUnlockMs = observedClock.unlocked;
    record("editing-unlocked-by-45s", result.saveClickToUnlockMs !== null && result.saveClickToUnlockMs <= 45000);
    const observed = await page.evaluate(() => {
      const status = document.getElementById("inspector-status");
      const text = document.querySelector('textarea[name="text"]');
      if (!(status instanceof HTMLElement) || !(text instanceof HTMLTextAreaElement)) throw new Error("fixed-observation-boundary");
      return { status: status.textContent || "", text: text.value };
    });
    const status = observed.status;
    state = await snapshot(page, origin);
    const savedClaim = /subtitle saved/i.test(status);
    const authoritativeSaved = savedClaim && state.protocol === "prepared" && state.browserCompletedReads > 0 && state.browserSavedInspections > 0;
    const uncertain = /confirm|unknown|unavailable|lost|timed out|taking longer/i.test(status);
    record("finite-truthful-status", authoritativeSaved || (uncertain && !/not saved|rolled back|nothing.*saved/i.test(status)));
    record("original-retained-if-uncertain", authoritativeSaved || observed.text === SAVED);
    result.stage = "new-edit";
    let edited = false, newStatus = "";
    if (await text.isEditable()) { await text.fill(NEWER); edited = (await text.inputValue()) === NEWER;
      newStatus = (await page.locator("#inspector-status").textContent()) || ""; record("newer-edit-accepted", edited); }
    result.stage = "late-response";
    result.releaseAttempted = true;
    const releaseTimeout = remaining(3000);
    try {
      if ((await page.request.get(origin+"/__r06/release", { timeout:releaseTimeout })).status() !== 204) throw new Error("fixed-fixture-boundary");
    } catch (error) {
      if (releaseTimeout === 1) result.failureStage = "release-budget-exhausted";
      throw error;
    }
    result.holdDurationMs = performance.now()-witnessAt;
    let settled = false;
    try {
      const terminal = await bounded(save.terminal, remaining(3000));
      state = await snapshot(page, origin);
      if (terminal === "finished") {
        const response = save.response();
        if (!response || response.status() !== state.responseStatus) throw new Error("fixed-response-boundary");
        const body = await bounded(response.body(), remaining(2000));
        if (body.length === 0 || body.length > 65536) throw new Error("fixed-response-boundary");
        const end = performance.now()+remaining(2000);
        while (state.activeHolds !== 0 && performance.now() < end) { await pause(50); state = await snapshot(page, origin); }
        result.responseBodyDelivered = state.responseBodyWritten && !state.clientCancelled && state.activeHolds === 0;
        settled = result.responseBodyDelivered;
      } else {
        const end = performance.now()+remaining(2000);
        while (!state.clientCancelled && performance.now() < end) { await pause(50); state = await snapshot(page, origin); }
        settled = state.clientCancelled && state.activeHolds === 0;
      }
      applySnapshot(result, state);
      if (settled) await bounded(page.evaluate(() => new Promise<void>(resolve => {
        requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
      })), remaining(2000));
    } catch { settled = false; }
    if (!settled) result.failureStage = "late-response";
    if (edited && settled) {
      record("newer-edit-survives-old-response", (await text.inputValue()) === NEWER);
      record("newer-status-survives-old-response", (await page.locator("#inspector-status").textContent()) === newStatus);
    }
    result.stage = "navigation";
    await page.getByRole("link", { name:"Back to library", exact:true }).click({ timeout:remaining(5000) });
    record("library-navigation-exercised", new URL(page.url()).pathname === "/" && new URL(page.url()).searchParams.get("view") === "library");
    await page.goBack({ waitUntil:"domcontentloaded", timeout:remaining(5000) });
    record("browser-back-exercised", new URL(page.url()).pathname === "/subtitles/inspect/"+item);
    await page.waitForFunction(() => {
      const cues = document.getElementById("subtitle-cues")?.textContent || "";
      return cues.includes("Fictional reviewed line") && cues.includes("Fictional reviewed later line") && !cues.includes("Fictional original line");
    }, undefined, { timeout:remaining(5000) });
    state = await snapshot(page, origin); applySnapshot(result, state);
    record("no-save-replay", state.saveAttempts === 1 && state.prepareAttempts <= 1);
    const shown = (await page.locator("#subtitle-cues").textContent()) || "";
    record("current-server-state", state.actualSaved && state.historyOnce && state.inspectionMatches &&
      shown.includes("Fictional reviewed line") && shown.includes("Fictional reviewed later line") && !shown.includes("Fictional original line"));
    record("fixture-settled", state.activeHolds === 0 && !state.holdExpired && !state.boundaryFailed);
  } catch { if (result.failureStage === null) result.failureStage = result.stage; }
  finally {
    result.stage = "settlement";
    save?.close();
    if (processFixture) result.fixtureStopped = await stopFixture(processFixture.child, processFixture.closed);
    result.durationMs = performance.now()-began;
  }
  return result;
}
