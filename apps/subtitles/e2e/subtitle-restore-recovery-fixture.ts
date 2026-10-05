import { spawn, type ChildProcess } from "node:child_process";
import { mkdtemp } from "node:fs/promises";
import { join, isAbsolute } from "node:path";
import type { Page } from "@playwright/test";
import { owner } from "./subtitle-save-recovery-auth";

export type JSONValue = null | boolean | number | string | JSONObject | JSONValue[];
export type JSONObject = { [key: string]: JSONValue };
export function isObject(value: JSONValue): value is JSONObject {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}
export type Snapshot = {
  protocol: "unreached" | "legacy" | "prepared";
  setupSaveAttempts: number; restoreAttempts: number; prepareAttempts: number;
  responseStatus: number; inspectionResponseStatus: number; activeHolds: number;
  browserCompletedReceiptReads: number; browserRestoredInspections: number;
  actualRestored: boolean; historyOnce: boolean; recoverySwapped: boolean;
  inspectionMatches: boolean; receiptCompleted: boolean; receiptSucceeded: boolean;
  holdEligible: boolean; headersReleased: boolean; bodyReleased: boolean;
  restoreResponseDelivered: boolean; inspectionResponseDelivered: boolean;
  restoreClientCancelled: boolean; inspectionClientCancelled: boolean;
  holdExpired: boolean; boundaryFailed: boolean;
};
export const SNAPSHOT_COUNTERS = [
  "setupSaveAttempts", "restoreAttempts", "prepareAttempts", "responseStatus",
  "inspectionResponseStatus", "activeHolds", "browserCompletedReceiptReads", "browserRestoredInspections",
] as const;
export const SNAPSHOT_FLAGS = [
  "actualRestored", "historyOnce", "recoverySwapped", "inspectionMatches", "receiptCompleted",
  "receiptSucceeded", "holdEligible", "headersReleased", "bodyReleased", "restoreResponseDelivered",
  "inspectionResponseDelivered", "restoreClientCancelled", "inspectionClientCancelled", "holdExpired", "boundaryFailed",
] as const;
export async function bounded<T>(promise: Promise<T>, milliseconds: number): Promise<T> {
  if (!Number.isFinite(milliseconds) || milliseconds <= 0) throw new Error("fixed-phase-budget");
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([promise, new Promise<T>((_, reject) => {
      timer = setTimeout(() => reject(new Error("fixed-phase-budget")), milliseconds);
    })]);
  } finally { if (timer !== undefined) clearTimeout(timer); }
}
export const pause = (milliseconds: number) => new Promise<void>(resolve => setTimeout(resolve, milliseconds));
export type Fixture = { child: ChildProcess; origin: string; closed: Promise<boolean> };
export type PhaseScope = ReturnType<typeof phaseScope>;
export function phaseScope() {
  const controller = new AbortController(), pending = new Set<Promise<void>>();
  const guard = () => { if (controller.signal.aborted) throw new Error("fixed-phase-stopped"); };
  const abort = () => controller.abort();
  const run = async <T>(work: () => Promise<T>, milliseconds: number): Promise<T> => {
    guard();
    const task = work(), joined = task.then(() => {}, () => {});
    pending.add(joined);
    try { const value = await bounded(task, milliseconds); guard(); return value; }
    catch (error) { abort(); throw error; }
    finally { void joined.then(() => pending.delete(joined)); }
  };
  const join = async () => { await Promise.all([...pending]); return true; };
  return { signal: controller.signal, guard, abort, run, join };
}
export async function stopFixture(fixture: Fixture, end: number): Promise<boolean> {
  const remaining = () => end - performance.now();
  if (fixture.child.exitCode === null && fixture.child.signalCode === null) {
    try { fixture.child.kill("SIGTERM"); } catch { return false; }
  }
  try { return await bounded(fixture.closed, Math.min(3000, remaining())); }
  catch {
    try { fixture.child.kill("SIGKILL"); } catch { return false; }
    try { await bounded(fixture.closed, remaining()); } catch { return false; }
    return false; // Forced termination never proves graceful Server cleanup.
  }
}
export async function cleanupCase(scope: PhaseScope, page: Page, fixture: Fixture | undefined): Promise<boolean> {
  const end = performance.now() + 5000;
  scope.abort();
  const success = (task: Promise<void>) => task.then(() => true, () => false);
  const joined = scope.join();
  const pageClosed = success(page.close());
  const requestsClosed = success(page.request.dispose());
  const fixtureClosed = fixture ? stopFixture(fixture, end) : Promise.resolve(false);
  try {
    const results = await bounded(Promise.all([joined, pageClosed, requestsClosed, fixtureClosed]), end - performance.now());
    return results.every(value => value);
  } catch { return false; }
}
export async function startFixture(mode: "headers" | "inspect-body", created: (fixture: Fixture) => void, scope: PhaseScope): Promise<Fixture> {
  scope.guard();
  const binary = process.env.R06_RESTORE_FIXTURE_BINARY;
  const directory = process.env.R06_RESTORE_PRIVATE_ROOT;
  if (!binary || !directory || !isAbsolute(binary) || !isAbsolute(directory)) throw new Error("fixed-fixture-boundary");
  const root = await mkdtemp(join(directory, "r06-restore-case-"));
  scope.guard();
  const child = spawn(binary, ["-r06-restore-serve", "-r06-restore-root", root, "-r06-restore-fault", mode], {
    stdio: ["ignore", "pipe", "pipe"], detached: false,
    env: { PATH: process.env.PATH || "", HOME: process.env.HOME || "", TMPDIR: process.env.TMPDIR || "", LANG: "C.UTF-8" },
  });
  const fixture: Fixture = {
    child, origin: "", closed: new Promise<boolean>(resolve => child.once("close", (code, signal) => resolve(code === 0 && signal === null))),
  };
  created(fixture);
  let buffer = "", stderrBytes = 0;
  child.stderr?.on("data", (chunk: Buffer) => { stderrBytes += chunk.length; if (stderrBytes > 65536) child.kill("SIGTERM"); });
  const ready = new Promise<string>((resolve, reject) => {
    scope.signal.addEventListener("abort", () => reject(new Error("fixed-fixture-boundary")), { once: true });
    child.once("error", () => reject(new Error("fixed-fixture-boundary")));
    child.once("exit", () => reject(new Error("fixed-fixture-boundary")));
    child.stdout?.on("data", (chunk: Buffer) => {
      buffer += chunk.toString("utf8");
      if (Buffer.byteLength(buffer) > 65536) { child.kill("SIGTERM"); reject(new Error("fixed-fixture-boundary")); return; }
      const newline = buffer.indexOf("\n");
      if (newline < 0) return;
      try {
        const value: JSONValue = JSON.parse(buffer.slice(0, newline));
        if (!isObject(value) || Object.keys(value).sort().join(",") !== "Kind,Origin" ||
          value.Kind !== "r06-restore-fixture-v1" || typeof value.Origin !== "string" ||
          !/^https:\/\/127\.0\.0\.1:\d{1,5}$/.test(value.Origin)) throw new Error("fixed-fixture-boundary");
        resolve(value.Origin);
      } catch { child.kill("SIGTERM"); reject(new Error("fixed-fixture-boundary")); }
    });
  });
  const origin = await bounded(ready, 15000);
  scope.guard(); fixture.origin = origin; return fixture;
}
export function validSnapshot(value: JSONValue): value is Snapshot {
  if (!isObject(value) || Object.keys(value).sort().join(",") !== ["protocol", ...SNAPSHOT_COUNTERS, ...SNAPSHOT_FLAGS].sort().join(",") ||
    typeof value.protocol !== "string" || !["unreached", "legacy", "prepared"].includes(value.protocol)) return false;
  return SNAPSHOT_COUNTERS.every(key => {
    const n = value[key]; return typeof n === "number" && Number.isSafeInteger(n) && n >= 0 && n <= 1000;
  }) && SNAPSHOT_FLAGS.every(key => typeof value[key] === "boolean") &&
    typeof value.responseStatus === "number" && value.responseStatus <= 599 &&
    typeof value.inspectionResponseStatus === "number" && value.inspectionResponseStatus <= 599;
}
export async function snapshot(page: Page, origin: string, timeout = 3000): Promise<Snapshot> {
  const response = await page.request.get(origin + "/__r06_restore/witness", { timeout });
  if (response.status() !== 200) throw new Error("fixed-fixture-boundary");
  const data: JSONValue = await response.json();
  if (!validSnapshot(data)) throw new Error("fixed-fixture-boundary");
  return data;
}
export async function setupPage(page: Page, origin: string, scope: PhaseScope): Promise<string> {
  scope.guard();
  await owner(page, origin); scope.guard();
  const catalog = await page.request.get(origin + "/api/v1/subtitle-library?view=library", { timeout: 5000 });
  scope.guard();
  const data: JSONValue = await catalog.json(); scope.guard();
  if (catalog.status() !== 200 || !isObject(data) || !Array.isArray(data.items) || data.items.length !== 1 ||
    !isObject(data.items[0]) || typeof data.items[0].id !== "string" || !/^[a-f0-9]{16}$/.test(data.items[0].id)) throw new Error("fixed-catalog-boundary");
  const item = data.items[0].id, base = origin + "/api/v1/subtitle-library/" + item;
  const inspected = await page.request.get(base + "/inspect?language=en", { timeout: 5000 });
  scope.guard();
  const review: JSONValue = await inspected.json(); scope.guard();
  if (inspected.status() !== 200 || !isObject(review) || review.id !== item || review.language !== "en" ||
    typeof review.fingerprint !== "string" || !/^[a-f0-9]{64}$/.test(review.fingerprint)) throw new Error("fixed-setup-boundary");
  const csrf = await page.locator('meta[name="kinosail-csrf"]').getAttribute("content");
  scope.guard();
  if (!csrf) throw new Error("fixed-auth-boundary");
  const applied = await page.request.post(base + "/apply", {
    data: { language: "en", fingerprint: review.fingerprint, offsetMilliseconds: 500,
      automaticSync: false, removeCredits: false, mergeRepeated: false },
    headers: { Origin: origin, "X-Kinosail-CSRF": csrf }, maxRedirects: 0, timeout: 5000,
  });
  scope.guard();
  if (applied.status() !== 200 || (await applied.body()).length > 65536) throw new Error("fixed-setup-boundary");
  scope.guard();
  await page.goto(origin + "/subtitles/inspect/" + item, { waitUntil: "domcontentloaded", timeout: 5000 });
  scope.guard();
  await page.waitForFunction(() => {
    const cues = document.getElementById("subtitle-cues")?.textContent;
    const text = document.querySelector('textarea[name="text"]');
    return typeof cues === "string" && cues.includes("Fictional original line") && cues.includes("0:01.500") &&
      text instanceof HTMLTextAreaElement && !text.disabled;
  }, undefined, { timeout: 5000 });
  scope.guard();
  await page.getByText("Correct subtitle text", { exact: true }).click({ timeout: 5000 });
  scope.guard(); return item;
}
