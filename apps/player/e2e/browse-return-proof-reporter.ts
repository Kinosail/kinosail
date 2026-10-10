import type {JSONValue, JSONObject} from "../../../scripts/testing/json-value";
import { createHash } from "node:crypto";
import type { FullConfig, FullResult, Reporter, Suite, TestCase, TestError, TestResult } from "@playwright/test/reporter";

// Opt-in fictional-Server proof only. Normal E2E reporters remain unchanged.
const files = ["browse-return.spec.ts", "browse-return-cold.spec.ts", "browse-return-bfcache.spec.ts", "browse-return-safety.spec.ts", "browse-return-home.spec.ts"];
const modes = ["primary", "cold", "bfcache", "htmx", "shows", "search", "safety", "home", "all"];
const requested = process.env.KINOSAIL_BROWSE_RETURN_CASES || "primary";
const suiteName = modes.includes(requested) ? requested : "invalid";
const safetyNames = ["external origin", "protocol-relative origin", "non-browse route", "duplicate query", "unknown query", "oversized query", "excessive extent", "different profile", "different destination"];
const titles: Record<string, string[]> = {
  [files[0]]: [
    ...[390, 1440].map(width => "visible Player Back preserves Movies query, offset, extent, focus and scroll at " + width + "px"),
    "HTMX title-letter Back fetches current browse data and restores extent without a native pageshow",
    ...["direct Play", "details and episode"].map(action => "visible Player Back restores original Shows action via " + action),
  ],
  [files[1]]: [
    ...[390, 1440].map(width => "cold native Back restores later Movie cards at " + width + "px"),
    "live query uses current URL rather than the document's initial browse key",
  ],
  [files[2]]: ["native BFCache preserves loaded Movie DOM without repeated continuation"],
  [files[3]]: [
    ...safetyNames.map(name => "saved return rejects " + name + " before navigation or continuation"),
    "a direct Player opened in another tab does not inherit browse return state",
  ],
  [files[4]]: [
    "visible Player Back restores the Home Continue watching action without a Library grid",
    "cold native Back restores the Home Continue watching action without a Library grid",
    "cold native Back restores the scrolled Home Movies destination without a Library grid",
  ],
};
const attachmentNames = ["before-state", "player-state", "returned-state", "served-browse-asset", "original-url-state", "letter-state", "cold-boundary-state", "native-cache-boundary-state", "htmx-boundary-state", "home-before-state", "home-player-state", "home-cold-boundary-state", "home-returned-state", "safe-rejection"];
const navigationTypes = ["navigate", "reload", "back_forward", "prerender"];
const digest = (value: Buffer) => createHash("sha256").update(value).digest("hex");
const object = (value: JSONValue | undefined): value is JSONObject => Boolean(value) && typeof value === "object" && !Array.isArray(value);
const exact = (value: JSONValue | undefined, fields: string[]): value is JSONObject => object(value) && Object.keys(value).sort().join(",") === [...fields].sort().join(",");
const link = (value: JSONValue | undefined) => typeof value === "string" && /^\/(?:watch|show)\/[a-f0-9]{16}$/.test(value) ? value : null;
const number = (value: JSONValue | undefined) => typeof value === "number" && Number.isFinite(value) && Math.abs(value) <= 10_000_000 ? value : null;
const integer = (value: JSONValue | undefined, low: number, high: number): value is number => typeof value === "number" && Number.isSafeInteger(value) && value >= low && value <= high;
const basename = (value: string) => value.split(/[\\/]/).at(-1)!;
const known = (file: string, title: string) => Boolean(titles[file]?.includes(title));
type SelectedObservation = { href: string | null; browse: Record<string, string> | null; top: number | null; bottom: number | null };

function query(input: JSONValue | undefined): Record<string, string> | null {
  if (!object(input)) return null;
  const output: Record<string, string> = {};
  const allowed: Record<string, RegExp> = { view: /^(all|movies|shows)$/, q: /^Return (Movie|Show)$/, sort: /^title$/, offset: /^\d{1,7}$/, limit: /^\d{1,4}$/, letter: /^[A-Z]$/, lang: /^[a-z]{2}(?:-[A-Z]{2})?$/ };
  for (const [key, value] of Object.entries(input)) {
    if (!Object.hasOwn(allowed, key) || typeof value !== "string" || !allowed[key].test(value)) return null;
    if (key === "offset" && Number(value) > 1_000_000 || key === "limit" && (Number(value) < 1 || Number(value) > 200)) return null;
    output[key] = value;
  }
  return output;
}
function browse(value: JSONValue | undefined) {
  if (typeof value !== "string" || value.length > 2048 || !/^\/(?:\?|$)/.test(value) || /[\\#\x00-\x20]/.test(value)) return null;
  try {
    const url = new URL(value, "https://fictional.invalid");
    if (url.origin !== "https://fictional.invalid" || url.pathname !== "/" || url.hash) return null;
    decodeURIComponent(url.search.replace(/\+/g, " "));
    for (const name of url.searchParams.keys()) if (url.searchParams.getAll(name).length !== 1) return null;
    return query(Object.fromEntries(url.searchParams));
  } catch { return null; }
}
function documentObservation(value: JSONValue | undefined) {
  if (!exact(value, ["document", "shows", "histories"]) || !Array.isArray(value.shows) || value.shows.length > 16 || !integer(value.histories, 0, 256)) return null;
  if (!value.shows.every(show => exact(show, ["persisted"]) && typeof show.persisted === "boolean")) return null;
  const id = typeof value.document === "string" && /^[a-f0-9]{8}-(?:[a-f0-9]{4}-){3}[a-f0-9]{12}$/.test(value.document) ? value.document : null;
  return { idSHA256: id ? digest(Buffer.from(id, "utf8")) : null, shows: value.shows.map(show => ({ persisted: show.persisted as boolean })), histories: value.histories };
}
function stateObservation(input: JSONValue | undefined) {
  if (!exact(input, ["path", "values", "profile", "document", "navigation", "titles", "hrefs", "focused", "scroll", "selected"])) return null;
  const values = query(input.values), document = documentObservation(input.document);
  if (values === null || document === null || input.path !== "/" && !link(input.path)) return null;
  if (!Array.isArray(input.navigation) || input.navigation.length > 4 || !input.navigation.every(value => typeof value === "string" && navigationTypes.includes(value))) return null;
  if (!Array.isArray(input.titles) || input.titles.length > 64 || !input.titles.every(title => typeof title === "string" && /^(Return (Movie|Show) \d{2}|Anchor Movie|Zeta Movie)$/.test(title))) return null;
  if (!Array.isArray(input.hrefs) || input.hrefs.length > 64 || !input.hrefs.every(value => value === null || link(value))) return null;
  if (!exact(input.scroll, ["x", "y"]) || number(input.scroll.x) === null || number(input.scroll.y) === null) return null;
  const focused = link(input.focused), focusedBrowse = browse(input.focused);
  if (input.focused !== null && focused === null && focusedBrowse === null) return null;
  let selected: SelectedObservation | null = null;
  if (input.selected !== null) {
    if (!exact(input.selected, ["href", "top", "bottom"]) || number(input.selected.top) === null || number(input.selected.bottom) === null) return null;
    const href = link(input.selected.href), selectedBrowse = browse(input.selected.href);
    if (href === null && selectedBrowse === null) return null;
    selected = { href, browse: selectedBrowse, top: number(input.selected.top), bottom: number(input.selected.bottom) };
  }
  return {
    path: input.path === "/" ? "/" : link(input.path), values, profile: input.profile === "local-owner" ? "local-owner" : null,
    document, navigation: input.navigation, titles: input.titles, hrefs: input.hrefs.map(link), focused, focusedBrowse,
    scroll: { x: number(input.scroll.x), y: number(input.scroll.y) }, selected,
  };
}
function observation(input: JSONValue | undefined, name: string) {
  if (name === "served-browse-asset") {
    if (!exact(input, ["src", "bytes", "sha256"]) || typeof input.src !== "string" || !/^\/static\/main\.kinosail\.bundle\.js\?v=[a-zA-Z0-9._-]{1,80}$/.test(input.src) || !integer(input.bytes, 1, 10_000_000) || typeof input.sha256 !== "string" || !/^[a-f0-9]{64}$/.test(input.sha256)) return null;
    return { src: input.src, bytes: input.bytes, sha256: input.sha256 };
  }
  if (name === "safe-rejection") {
    if (!exact(input, ["name", "href", "noBrowseRequests"]) || typeof input.name !== "string" || !safetyNames.includes(input.name) || !link(input.href) || typeof input.noBrowseRequests !== "boolean") return null;
    return { name: input.name, href: link(input.href), noBrowseRequests: input.noBrowseRequests };
  }
  if (!exact(input, ["state", "peer"]) || !Array.isArray(input.peer) || input.peer.length > 256) return null;
  const state = stateObservation(input.state);
  if (!state) return null;
  const peer = [];
  for (const request of input.peer) {
    if (!exact(request, ["values", "continuation", "history", "htmx"]) || ![request.continuation, request.history, request.htmx].every(value => typeof value === "boolean")) return null;
    const values = query(request.values);
    if (values === null) return null;
    peer.push({ values, continuation: request.continuation, history: request.history, htmx: request.htmx });
  }
  return { state, peer };
}

function cacheDiagnosticObservation(value: JSONValue | undefined) {
  const allowed = ["unload-listener","unload-handler","response-cache-control-no-store","response-cache-control-no-store-with-cookie-modification","related-active-contents","masked","websocket","outstanding-network-request","other"];
  if (!exact(value, ["schemaVersion", "supported", "present", "frameCount", "reasons", "truncated", "navigationType"])) return null;
  if (value.schemaVersion !== 1 || typeof value.supported !== "boolean" || typeof value.present !== "boolean" || typeof value.truncated !== "boolean" || !integer(value.frameCount, 0, 64)) return null;
  if (!Array.isArray(value.reasons) || value.reasons.length > 16 || !value.reasons.every(reason => typeof reason === "string" && allowed.includes(reason))) return null;
  if (JSON.stringify(value.reasons) !== JSON.stringify([...new Set(value.reasons)].sort()) || typeof value.navigationType !== "string" || ![...navigationTypes, "unknown"].includes(value.navigationType)) return null;
  if (value.present && (!value.supported || value.frameCount < 1) || !value.present && (value.frameCount !== 0 || value.reasons.length)) return null;
  return { schemaVersion: 1, supported: value.supported, present: value.present, frameCount: value.frameCount, reasons: value.reasons, truncated: value.truncated, navigationType: value.navigationType };
}

function failure(error: TestError) {
  const message = error.message || "";
  const labels = ["Q14 acceptance: return to the same public browse URL", "Q14 acceptance: selected title action regains focus", "Q14 acceptance: same settled browse position", "Q14 Home acceptance: original action regains focus", "Q14 Home acceptance: settled horizontal position", "Q14 Home acceptance: settled vertical position", "fixture prerequisite:", "BFCache prerequisite:", "cold boundary prerequisite:", "cold search prerequisite:", "HTMX prerequisite:", "Home prerequisite:", "Home cold prerequisite:", "destination prerequisite:"];
  const label = labels.find(value => message.includes(value));
  const file = error.location ? basename(error.location.file) : undefined;
  return { phase: label?.startsWith("Q14 ") ? "acceptance" : label ? "prerequisite" : "unclassified",
    label: label || null, location: file && [...files, "browse-return-helpers.ts"].includes(file) && integer(error.location?.line, 1, 1000) && integer(error.location?.column, 1, 1000) ? { file, line: error.location!.line, column: error.location!.column } : null };
}
export default class BrowseReturnProofReporter implements Reporter {
  private collected: { file: string; title: string }[] = [];
  private cases: JSONObject[] = [];
  private errors: JSONObject[] = [];
  private invalid() { if (this.errors.length < 16) this.errors.push({ phase: "unclassified", label: null, location: null }); }
  onBegin(_config: FullConfig, suite: Suite) {
    const tests = suite.allTests();
    if (suiteName === "invalid" || tests.length > 22 || tests.some(test => !known(basename(test.location.file), test.title))) { this.invalid(); return; }
    this.collected = tests.map(test => ({ file: basename(test.location.file), title: test.title }));
  }
  onTestEnd(test: TestCase, result: TestResult) {
    const file = basename(test.location.file);
    if (!known(file, test.title) || this.cases.length >= 22 || !integer(result.duration, 0, 60_000) || !integer(result.retry, 0, 3)) { this.invalid(); return; }
    const attachments: { name: string; bytes: number; sha256: string; observation: ReturnType<typeof observation> }[] = [], names = new Set<string>();
    if (result.attachments.length > 64 || result.errors.length > 16) this.invalid();
    for (const item of result.attachments.slice(0, 64)) {
      if (suiteName === "bfcache" && file === files[2] && item.name === "native-cache-diagnostic" && item.contentType === "application/json" && item.body && item.body.length <= 4096) {
        try {
          const diagnostic = cacheDiagnosticObservation(JSON.parse(item.body.toString("utf8")));
          if (diagnostic) console.log("Q14_CACHE_DIAGNOSTIC " + JSON.stringify(diagnostic));
        } catch { /* Diagnostic rejection never changes product assertions. */ }
      }
      if (!item.body || item.contentType !== "application/json" || !attachmentNames.includes(item.name)) continue;
      if (item.body.length < 1 || item.body.length > 1_000_000 || names.has(item.name) || attachments.length >= 16) { this.invalid(); continue; }
      names.add(item.name);
      let decoded: ReturnType<typeof observation> = null;
      try { decoded = observation(JSON.parse(item.body.toString("utf8")), item.name); } catch { /* Missing observations never certify a boundary. */ }
      attachments.push({ name: item.name, bytes: item.body.length, sha256: digest(item.body), observation: decoded });
    }
    this.cases.push({ file, title: test.title, status: result.status, retry: result.retry, durationMs: result.duration,
      expectedStatus: test.expectedStatus, failures: result.errors.slice(0, 16).map(failure), attachments });
  }
  onError(error: TestError) { if (this.errors.length < 16) this.errors.push(failure(error)); }
  onEnd(result: FullResult) {
    console.log("Q14_PROOF_RESULT " + JSON.stringify({ schemaVersion: 2, suite: suiteName, status: result.status, collected: this.collected, cases: this.cases, errors: this.errors }));
  }
  printsToStdio() { return true; }
}
