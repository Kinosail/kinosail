import { createHash } from "node:crypto";
import type { FullConfig, FullResult, Reporter, Suite, TestCase, TestError, TestResult } from "@playwright/test/reporter";

// Opt-in fictional-Server proof only. Normal E2E reporters remain unchanged.
const files = ["browse-return.spec.ts", "browse-return-cold.spec.ts", "browse-return-bfcache.spec.ts"];
const digest = (value: Buffer) => createHash("sha256").update(value).digest("hex");
const link = (value: unknown) => typeof value === "string" && /^\/(?:watch|show)\/[a-f0-9]{16}$/.test(value) ? value : null;
const number = (value: unknown) => typeof value === "number" && Number.isFinite(value) ? value : null;
type FixtureObservation = {
  src?: string; bytes?: number; sha256?: string;
  state?: { path: string; values: Record<string, unknown>; profile: string; titles: unknown[]; hrefs: unknown[]; focused: unknown; scroll?: { x: number; y: number }; selected?: { href: unknown; top: number; bottom: number } };
  peer?: { values: Record<string, unknown>; continuation: unknown; history: unknown; htmx: unknown }[];
};
function values(input: Record<string, unknown> = {}) {
  const output: Record<string, unknown> = {};
  const allowed: Record<string, RegExp> = { view: /^(movies|shows)$/, q: /^Return (Movie|Show)$/, sort: /^title$/, offset: /^\d{1,7}$/, limit: /^\d{1,4}$/, letter: /^[A-Z]$/, lang: /^[a-z]{2}(?:-[A-Z]{2})?$/ };
  for (const [key, pattern] of Object.entries(allowed)) if (typeof input[key] === "string" && pattern.test(input[key])) output[key] = input[key];
  return output;
}
function observation(input: FixtureObservation) {
  if (input.src && /^\/static\/main\.kinosail\.bundle\.js\?v=[a-zA-Z0-9._-]{1,80}$/.test(input.src)) {
    return { src: input.src, bytes: number(input.bytes), sha256: typeof input.sha256 === "string" && /^[a-f0-9]{64}$/.test(input.sha256) ? input.sha256 : null };
  }
  if (!input.state || !Array.isArray(input.peer)) return null;
  const state = input.state;
  return {
    state: {
      path: state.path === "/" ? "/" : link(state.path), values: values(state.values),
      profile: state.profile === "local-owner" ? state.profile : null,
      titles: Array.isArray(state.titles) ? state.titles.slice(0, 64).filter((title: unknown) => typeof title === "string" && /^(Return (Movie|Show) \d{2}|Anchor Movie|Zeta Movie)$/.test(title)) : [],
      hrefs: Array.isArray(state.hrefs) ? state.hrefs.slice(0, 64).map(link) : [], focused: link(state.focused),
      scroll: { x: number(state.scroll?.x), y: number(state.scroll?.y) },
      selected: state.selected ? { href: link(state.selected.href), top: number(state.selected.top), bottom: number(state.selected.bottom) } : null,
    },
    peer: input.peer.slice(0, 256).map(request => ({ values: values(request.values), continuation: request.continuation === true, history: request.history === true, htmx: request.htmx === true })),
  };
}
function failure(error: TestError) {
  const message = error.message || "";
  const labels = ["Q14 acceptance: return to the same public browse URL", "Q14 acceptance: selected title action regains focus", "Q14 acceptance: same settled browse position", "fixture prerequisite:", "BFCache prerequisite:", "cold boundary prerequisite:", "HTMX prerequisite:"];
  const label = labels.find(value => message.includes(value));
  const file = error.location?.file.split(/[\\/]/).at(-1);
  return { phase: label?.startsWith("Q14 acceptance:") ? "acceptance" : label ? "prerequisite" : "unclassified",
    label: label || null, location: file && [...files, "browse-return-helpers.ts"].includes(file) ? { file, line: error.location!.line, column: error.location!.column } : null };
}
export default class BrowseReturnProofReporter implements Reporter {
  private collected: { file: string; title: string }[] = [];
  private cases: Record<string, unknown>[] = [];
  private errors: Record<string, unknown>[] = [];
  onBegin(_config: FullConfig, suite: Suite) {
    this.collected = suite.allTests().map(test => ({ file: test.location.file.split(/[\\/]/).at(-1)!, title: test.title }));
    if (this.collected.some(test => !files.includes(test.file) || test.title.length > 256)) throw new Error("unexpected Q14 proof selection");
  }
  onTestEnd(test: TestCase, result: TestResult) {
    const attachments = result.attachments.filter(item => item.body && item.contentType === "application/json" && /^(before-state|player-state|returned-state|served-browse-asset)$/.test(item.name)).map(item => {
      let decoded = null;
      try { decoded = observation(JSON.parse(item.body!.toString("utf8"))); } catch { /* Missing observations do not certify a boundary. */ }
      return { name: item.name, bytes: item.body!.length, sha256: digest(item.body!), observation: decoded };
    });
    this.cases.push({ file: test.location.file.split(/[\\/]/).at(-1), title: test.title,
      status: result.status, retry: result.retry, durationMs: result.duration,
      expectedStatus: test.expectedStatus, failures: result.errors.map(failure), attachments });
  }
  onError(error: TestError) { this.errors.push(failure(error)); }
  onEnd(result: FullResult) {
    console.log(`Q14_PROOF_RESULT ${JSON.stringify({ schemaVersion: 1, status: result.status, collected: this.collected, cases: this.cases, errors: this.errors })}`);
  }
  printsToStdio() { return true; }
}
