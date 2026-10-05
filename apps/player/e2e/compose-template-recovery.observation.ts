import type { Page, TestInfo } from "@playwright/test";

export const ASSERTION_IDS = [
  "control-status", "witness-status", "prior-handler-idle",
  "served-helper-equal", "builder-visible", "original-links-present",
  "template-request-observed", "result-visible", "preview-equal",
  "download-name-equal", "native-blob-digest-equal", "copy-status-equal",
  "native-copy-equal", "retry-keyboard-focused", "primary-one-request",
  "primary-one-hold", "primary-body-prefix", "primary-template-digest",
  "primary-pending-label", "primary-pending-disabled", "primary-no-output",
  "deadline-retry-label", "deadline-enabled", "deadline-error-visible",
  "deadline-retry-visible", "deadline-error-still-visible", "fallback-visible",
  "deadline-elapsed-at-least20s", "fallback-canonical", "primary-input-retention",
  "primary-peer-canceled", "primary-new-completed", "primary-two-requests",
  "primary-settled", "primary-privacy", "recovery-error-visible",
  "recovery-retry-visible", "recovery-input-retention", "recovery-fallback-canonical",
  "recovery-new-completed", "recovery-new-request", "recovery-prior-failed",
  "recovery-settled", "recovery-privacy", "supersession-one-hold",
  "supersession-no-active", "supersession-peer-canceled", "supersession-preview-current",
  "contract-one-completed", "contract-one-request", "contract-settled",
  "contract-privacy", "invalid-path-error", "invalid-path-no-request",
  "invalid-port-error", "invalid-port-no-request", "invalid-no-result",
  "invalid-no-download", "pending-confirmed-before-deadline", "absolute-clock-window-eligible",
  "recovery-observed-by-product-deadline",
] as const;
export type AssertionID = typeof ASSERTION_IDS[number];
export type Json = null | boolean | number | string | Json[] | { [key: string]: Json };
export type Dictionary = { [key: string]: Json };
export type Assertion = { attempted: boolean; completed: boolean; passed: boolean | null; attempts: number };
export type Ledger = { assertions: Record<AssertionID, Assertion>; clock: BrowserClock | null };
export type BrowserClock = { elapsedMs: number | null; trustedClick: boolean; clicks: number;
  pendingObserved: boolean; firstEnabledMs: number | null; firstRecoveryMs: number | null;
  sample: { elapsedMs: number; action: string; createDisabled: boolean; errorVisible: boolean } | null };
const labels = [
  "Q47 prerequisite:control",
  "Q47 prerequisite:witness",
  "Q47 prerequisite:idle",
  "Q47 prerequisite:helper",
  "Q47 prerequisite:builder",
  "Q47 prerequisite:originals",
  "Q47 prerequisite:request",
  "Q47 acceptance:ready",
  "Q47 acceptance:integrity",
  "Q47 acceptance:integrity",
  "Q47 acceptance:integrity",
  "Q47 acceptance:copy",
  "Q47 acceptance:copy",
  "Q47 acceptance:focus",
  "Q47 prerequisite:hold",
  "Q47 prerequisite:hold",
  "Q47 prerequisite:hold",
  "Q47 prerequisite:template",
  "Q47 prerequisite:hold",
  "Q47 prerequisite:hold",
  "Q47 prerequisite:hold",
  "Q47 acceptance:deadline",
  "Q47 acceptance:deadline",
  "Q47 acceptance:deadline",
  "Q47 acceptance:deadline",
  "Q47 acceptance:deadline",
  "Q47 acceptance:originals",
  "Q47 acceptance:deadline",
  "Q47 acceptance:originals",
  "Q47 acceptance:retention",
  "Q47 acceptance:cancellation",
  "Q47 acceptance:new_attempt",
  "Q47 acceptance:new_attempt",
  "Q47 acceptance:cancellation",
  "Q47 acceptance:privacy",
  "Q47 prerequisite:fault",
  "Q47 acceptance:retry",
  "Q47 acceptance:retention",
  "Q47 acceptance:originals",
  "Q47 acceptance:new_attempt",
  "Q47 acceptance:new_attempt",
  "Q47 acceptance:new_attempt",
  "Q47 acceptance:cancellation",
  "Q47 acceptance:privacy",
  "Q47 prerequisite:hold",
  "Q47 acceptance:cancellation",
  "Q47 acceptance:cancellation",
  "Q47 acceptance:stale",
  "Q47 acceptance:ready",
  "Q47 acceptance:privacy",
  "Q47 acceptance:ready",
  "Q47 acceptance:privacy",
  "Q47 acceptance:validation",
  "Q47 acceptance:validation",
  "Q47 acceptance:validation",
  "Q47 acceptance:validation",
  "Q47 acceptance:validation",
  "Q47 acceptance:validation",
  "Q47 prerequisite:clock",
  "Q47 prerequisite:clock",
  "Q47 acceptance:deadline"
];
const ledgers = new WeakMap<TestInfo, Ledger>();
class ProofAssertionError extends Error {}
export function scope(title: string): AssertionID[] {
  const common = ASSERTION_IDS.slice(0, 6), ready = ASSERTION_IDS.slice(7, 13);
  if (title.startsWith("Q47 headers deadline") || title.startsWith("Q47 body deadline")) {
    return [...common, ASSERTION_IDS[6], ...ready, ...(title.startsWith("Q47 body") ? [ASSERTION_IDS[13]] : []),
      ...ASSERTION_IDS.slice(14, 35), ...ASSERTION_IDS.slice(58)];
  }
  if (title.includes("recovers with retained inputs")) return [...common, ASSERTION_IDS[6], ...ready, ...ASSERTION_IDS.slice(35, 44)];
  if (title.includes("discards late")) return [...common, ASSERTION_IDS[6], ...ready, ...ASSERTION_IDS.slice(44, 48)];
  if (title.includes("preserves Compose download and copy")) return [...common, ASSERTION_IDS[6], ...ready, ...ASSERTION_IDS.slice(48, 52)];
  if (title === "Q47 rejects invalid inputs before template requests") return [...common, ...ASSERTION_IDS.slice(52, 58)];
  return [];
}
export function begin(info: TestInfo) {
  ledgers.set(info, { assertions: Object.fromEntries(ASSERTION_IDS.map(id => [id,
    { attempted: false, completed: false, passed: null, attempts: 0 }])) as Ledger["assertions"], clock: null });
}
export async function verify(info: TestInfo, id: AssertionID, action: () => Promise<void>) {
  const ledger = ledgers.get(info);
  if (!ledger || !scope(info.title).includes(id)) throw new Error("fixed-assertion-boundary");
  const assertion = ledger.assertions[id];
  Object.assign(assertion, { attempted: true, completed: false, passed: null, attempts: assertion.attempts + 1 });
  try { await action(); Object.assign(assertion, { completed: true, passed: true }); }
  catch (error) {
    if (error instanceof ProofAssertionError) throw error;
    if (!(error instanceof Error) || error.message.length > 65_536 || !("matcherResult" in error) ||
      !error.matcherResult || typeof error.matcherResult !== "object" || !("pass" in error.matcherResult) ||
      error.matcherResult.pass !== false ||
      error.message.replace(/\u001b\[[0-9;]*m/g, "").split("\n")[0] !== labels[ASSERTION_IDS.indexOf(id)]) throw error;
    Object.assign(assertion, { completed: true, passed: false });
    throw new ProofAssertionError(id);
  }
}
export function finish(info: TestInfo): Ledger {
  const ledger = ledgers.get(info);
  if (!ledger) throw new Error("fixed-assertion-boundary");
  return ledger;
}
export async function installClock(page: Page) {
  await page.evaluate(() => {
    const button = document.querySelector<HTMLButtonElement>("[data-install-create]");
    if (!button) throw new Error("fixed-click-clock-boundary");
    let clicked: number | null = null, trustedClick = false, clicks = 0, pendingObserved = false;
    let firstEnabledMs: number | null = null, firstRecoveryMs: number | null = null, sample: BrowserClock["sample"] = null;
    const read = () => {
      const error = document.querySelector<HTMLElement>("[data-install-error]");
      const label = button.textContent?.trim();
      const action = label === "Retry" ? "retry" : label === "Preparing file\u2026" ? "preparing" : label === "Make Compose file" ? "make" : "unknown";
      const createDisabled = button.disabled;
      const errorVisible = Boolean(error && !error.closest("[hidden]") && error.getBoundingClientRect().width > 0 && error.getBoundingClientRect().height > 0);
      return { action, createDisabled, errorVisible };
    };
    const observe = () => {
      if (clicked === null) return;
      const state = read(), elapsed = performance.now() - clicked;
      if (state.action === "preparing" && state.createDisabled) pendingObserved = true;
      if (!pendingObserved) return;
      if (!state.createDisabled && firstEnabledMs === null) firstEnabledMs = elapsed;
      if (state.action === "retry" && !state.createDisabled && state.errorVisible && firstRecoveryMs === null) firstRecoveryMs = elapsed;
    };
    const observer = new MutationObserver(observe);
    observer.observe(document.documentElement, { attributes: true, childList: true, characterData: true, subtree: true });
    button.addEventListener("click", event => {
      clicks++;
      if (clicked !== null) return;
      clicked = performance.now(); trustedClick = event.isTrusted;
      queueMicrotask(observe);
      setTimeout(() => requestAnimationFrame(() => requestAnimationFrame(() => {
        observe();
        sample = { ...read(), elapsedMs: performance.now() - clicked! };
      })), 20_000);
    }, { capture: true });
    window.addEventListener("pagehide", () => observer.disconnect(), { once: true });
    (window as Window & { __q47Clock: () => BrowserClock }).__q47Clock = () => ({
      elapsedMs: clicked === null ? null : performance.now() - clicked, trustedClick, clicks,
      pendingObserved, firstEnabledMs, firstRecoveryMs, sample,
    });
  });
}
export async function clock(page: Page): Promise<BrowserClock> {
  return page.evaluate(() => (window as Window & { __q47Clock: () => BrowserClock }).__q47Clock());
}
export async function deadline(page: Page, info: TestInfo): Promise<BrowserClock> {
  const current = await clock(page), ledger = ledgers.get(info);
  if (!ledger || current.elapsedMs === null || !Number.isFinite(current.elapsedMs) || current.elapsedMs < 0) throw new Error("fixed-click-clock-boundary");
  ledger.clock = current;
  try {
    await page.waitForFunction(() => (window as Window & { __q47Clock: () => BrowserClock }).__q47Clock().sample !== null,
      undefined, { timeout: Math.max(1, 20_200 - current.elapsedMs) });
  } finally { ledger.clock = await clock(page); }
  return ledger.clock!;
}
export function object(value: Json, keys: readonly string[]): value is Dictionary {
  return Boolean(value && typeof value === "object" && !Array.isArray(value) &&
    Object.keys(value).sort().join(",") === [...keys].sort().join(","));
}
export function integer(value: Json, maximum: number, minimum = 0): value is number {
  return typeof value === "number" && Number.isSafeInteger(value) && value >= minimum && value <= maximum;
}
function milliseconds(value: Json): value is number {
  return typeof value === "number" && Number.isFinite(value) && value >= 0 && value <= 70_000;
}
function safeClock(value: Json): value is BrowserClock {
  if (!object(value, ["elapsedMs", "trustedClick", "clicks", "pendingObserved", "firstEnabledMs", "firstRecoveryMs", "sample"]) ||
    !(value.elapsedMs === null || milliseconds(value.elapsedMs)) || typeof value.trustedClick !== "boolean" ||
    !integer(value.clicks, 32) || typeof value.pendingObserved !== "boolean" ||
    ![value.firstEnabledMs, value.firstRecoveryMs].every(item => item === null || milliseconds(item))) return false;
  const elapsed = value.elapsedMs as number | null, enabled = value.firstEnabledMs as number | null, recovered = value.firstRecoveryMs as number | null;
  if ((elapsed === null && (value.clicks !== 0 || value.pendingObserved || enabled !== null || recovered !== null || value.sample !== null)) ||
    (elapsed !== null && (value.clicks === 0 || [enabled, recovered].some(item => item !== null && item > elapsed)))) return false;
  if (recovered !== null && (enabled === null || !value.pendingObserved || recovered < enabled)) return false;
  return value.sample === null || (object(value.sample, ["elapsedMs", "action", "createDisabled", "errorVisible"]) &&
    milliseconds(value.sample.elapsedMs) && typeof value.elapsedMs === "number" && value.sample.elapsedMs <= value.elapsedMs &&
    typeof value.sample.action === "string" && ["make", "preparing", "retry", "unknown"].includes(value.sample.action) &&
    typeof value.sample.createDisabled === "boolean" && typeof value.sample.errorVisible === "boolean");
}
export function safeLedger(value: Json, title: string): Ledger | null {
  if (!object(value, ["assertions", "clock"]) || !object(value.assertions, ASSERTION_IDS) || !(value.clock === null || safeClock(value.clock))) return null;
  const selected = scope(title);
  if (selected.length === 0) return null;
  for (const id of ASSERTION_IDS) {
    const entry = value.assertions[id];
    if (!object(entry, ["attempted", "completed", "passed", "attempts"]) || typeof entry.attempted !== "boolean" ||
      typeof entry.completed !== "boolean" || !(entry.passed === null || typeof entry.passed === "boolean") ||
      !integer(entry.attempts, 1_024)) return null;
    if ((!entry.attempted && (entry.completed || entry.passed !== null || entry.attempts !== 0)) ||
      (entry.attempted && entry.attempts === 0) || (entry.completed !== (entry.passed !== null)) ||
      (!selected.includes(id) && entry.attempted)) return null;
  }
  return value as Ledger;
}
export const ASSERTION_SOURCE = { file: "apps/player/e2e/compose-template-recovery.observation.ts", line: 128, column: 11 };
