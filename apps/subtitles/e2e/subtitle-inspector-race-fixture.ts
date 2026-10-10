import { readFileSync } from "node:fs";
import vm from "node:vm";
import { createHash } from "node:crypto";

type Cue = {start: number; end: number; text: string; warnings?: string[]};
export type FixtureNode = {
  id: string; dataset: Record<string, string>; style: Record<string, string>; value: string;
  files: File[]; checked: boolean; disabled: boolean; hidden: boolean; children: FixtureNode[];
  listeners: Record<string, (event?: {preventDefault?: () => void; key?: string; currentTarget?: FixtureNode}) => void | Promise<void>>;
  textContent: string; currentTime?: number; attrs: Record<string, string>; href?: string; elements?: Record<string, FixtureNode>;
  addEventListener(name: string, listener: FixtureNode["listeners"][string]): void;
  setAttribute(name: string, value: string): void; removeAttribute(name: string): void;
  append(...children: FixtureNode[]): void; prepend(): void; replaceChildren(...children: FixtureNode[]): void;
  focus(): void; querySelector(): FixtureNode; querySelectorAll(): FixtureNode[];
  addTextTrack(): {cues: Cue[]; addCue(cue: Cue): void; removeCue(cue: Cue): void};
};
type FixtureRequest = {url: string; options: RequestInit; reject: (error: Error) => void;
  resolve: (response: {status: number; ok: boolean; json: () => Promise<object>}) => void};
type Review = {id: string; title: string; language: string; fingerprint: string; role: string;
  source: string; matchEvidence: string; warnings: string[]; originalAvailable: boolean; restorable: boolean;
  duration: number; current: {cues: Cue[]; quality: object}; proposed?: {cues: Cue[]; quality: object};
  comparison?: Array<{current: number[]; proposed: number[]; kind: string}>};

const source = ["subtitle-source-cues.js", "subtitle-save-operation.js", "subtitle-inspector.js"].map(name => readFileSync(new URL(`../internal/server/static/${name}`, import.meta.url), "utf8")).join("\n");
export const flush = () => new Promise<void>(resolve => setImmediate(resolve));

// Execute the complete shipped script with controlled requests and lifecycle.
// This proves JS ordering, not layout, keyboard access, or browser rendering.
export function inspectorFixture(withObservers = false) {
  const observers: IntersectionObserver[] = [];
  class IntersectionObserver {
    target?: FixtureNode; disconnected = false;
    callback: (entries: Array<{isIntersecting: boolean}>) => void;
    constructor(callback: (entries: Array<{isIntersecting: boolean}>) => void) { this.callback = callback; observers.push(this); }
    observe(target: FixtureNode) { this.target = target; }
    disconnect() { this.disconnected = true; }
  }
  const nodes = new Map<string, FixtureNode>(), requests: FixtureRequest[] = [], events: Record<string, () => void> = {}, timers = new Map<number, { run: () => void; delay: number }>();
  let timer = 0, created = 0, clock = 0;
  const item = "0000000000000001", operation = "a".repeat(64);
  function node(id = ""): FixtureNode {
    if (nodes.has(id)) return nodes.get(id);
    const n: FixtureNode = { id, dataset: { id: item }, style: {}, value: "", files: [], checked: false, disabled: false, hidden: false, children: [], listeners: {}, textContent: "", attrs: {},
      addEventListener(name: string, listener: FixtureNode["listeners"][string]) { this.listeners[name] = listener; },
      setAttribute(name: string, value: string) { this.attrs[name] = value; }, removeAttribute(name: string) { delete this.attrs[name]; },
      append(...children: FixtureNode[]) { this.children.push(...children); }, prepend() {}, replaceChildren(...children: FixtureNode[]) { this.children = children; }, focus() {},
      querySelector() { return node(id + "-child"); }, querySelectorAll() { return []; },
      addTextTrack() { return { cues: [] as Cue[], addCue(cue: Cue) { this.cues.push(cue); }, removeCue(cue: Cue) { this.cues = this.cues.filter((value: Cue) => value !== cue); } }; },
    };
    nodes.set(id, n); return n;
  }
  const form = node("subtitle-edit-form");
  form.elements = Object.fromEntries(["role", "language", "encoding", "offset", "automaticSync", "removeCredits", "mergeRepeated", "file", "text"].map(name => [name, node("field-" + name)]));
  form.elements.language.value = "en"; form.elements.offset.value = "0";
  form.querySelectorAll = () => [...Object.values(form.elements), node("add-anchor"), node("edit-preview-text"), node("clear-preview-text"), node("subtitle-edit-form-child")];
  const exports = [node("export-srt"), node("export-vtt")];
  for (const link of exports) link.href = "http://inspector.test/export?language=en";
  const document = { getElementById: node, querySelector(selector: string) { if (selector === ".subtitle-inspector") return node("root"); if (selector.startsWith("meta")) return { content: "synthetic-csrf" }; return node(selector); },
    querySelectorAll(selector: string) { return selector === ".subtitle-inspector-exports a" ? exports : []; }, createElement() { return node("created-" + ++created); }, createTextNode: () => ({}),
  };
  vm.runInNewContext(source, { document, IntersectionObserver, window: { ...(withObservers ? { IntersectionObserver } : {}), addEventListener: (name: string, handler: () => void) => { events[name] = handler; } }, location: { pathname: "/synthetic", search: "" }, URL,
    VTTCue: function(start: number, end: number, text: string) { return { start, end, text }; },
    AbortController, performance: { now: () => clock },
    fetch(url: string, options: object) { return new Promise((resolve, reject) => requests.push({ url, options, resolve, reject })); },
    setTimeout(run: () => void, delay: number) { const id = ++timer; timers.set(id, { run, delay }); return id; }, clearTimeout(id: number) { timers.delete(id); },
  });
  const respond = (request: FixtureRequest, value: object, status = 200) => request.resolve({ status, ok: status >= 200 && status < 300, json: async () => value });
  const receipt = (state: string, outcome?: string, status?: number, action = "apply") => ({ id: operation, action, item, state, outcome, status });
  const review = (language: string, marker: string): Review => ({ id: item, title: "Fictional review", language, fingerprint: createHash("sha256").update(marker).digest("hex"), role: "translation", source: marker, matchEvidence: "Synthetic logic fixture", warnings: [], originalAvailable: false, restorable: true, duration: 2,
    current: { cues: [{ start: 1, end: 2, text: "Fictional reviewed line", warnings: [] }], quality: { cueCount: 1, fastCues: 0, overlaps: 0, longLines: 0, maxCPS: 23, firstCue: 1, lastCue: 2, timing: "Synthetic", completeness: "Synthetic" } } });
  const draft = (state = "idle") => ({ id: "draft", language: form.elements.language.value, state, message: state, words: [] });
  return { node, form, requests, events, timers, exports, respond, review, draft, observers, receipt, operation, item,
    async ready(state = "idle") { respond(requests[0], review("en", "EN current")); respond(requests[1], draft(state)); await flush(); },
    async preview() { const pending = form.listeners.submit({ preventDefault() {} }); await flush(); const result = review(form.elements.language.value, "preview"); respond(requests.at(-1), { ...result, proposed: result.current }); await pending; },
    changeLanguage(language: string) { form.elements.language.value = language; form.listeners.input(); form.elements.language.listeners.change(); },
    poll() { const [id, next] = [...timers][0]; timers.delete(id); clock += next.delay; next.run(); },
  };
}
