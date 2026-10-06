import { readFileSync } from "node:fs";
import vm from "node:vm";
import { createHash } from "node:crypto";

const source = ["subtitle-source-cues.js", "subtitle-save-operation.js", "subtitle-inspector.js"].map(name => readFileSync(new URL(`../internal/server/static/${name}`, import.meta.url), "utf8")).join("\n");
export const flush = () => new Promise<void>(resolve => setImmediate(resolve));

// Execute the complete shipped script with controlled requests and lifecycle.
// This proves JS ordering, not layout, keyboard access, or browser rendering.
export function inspectorFixture(withObservers = false) {
  const observers: any[] = [];
  class IntersectionObserver {
    target: any; disconnected = false;
    constructor(public callback: (entries: any[]) => void) { observers.push(this); }
    observe(target: any) { this.target = target; }
    disconnect() { this.disconnected = true; }
  }
  const nodes = new Map<string, any>(), requests: any[] = [], events: Record<string, any> = {}, timers = new Map<number, { run: () => void; delay: number }>();
  let timer = 0, created = 0, clock = 0;
  const item = "0000000000000001", operation = "a".repeat(64);
  function node(id = ""): any {
    if (nodes.has(id)) return nodes.get(id);
    const n: any = { id, dataset: { id: item }, style: {}, value: "", files: [], checked: false, disabled: false, hidden: false, children: [], listeners: {}, textContent: "", attrs: {},
      addEventListener(name: string, listener: any) { this.listeners[name] = listener; },
      setAttribute(name: string, value: string) { this.attrs[name] = value; }, removeAttribute(name: string) { delete this.attrs[name]; },
      append(...children: any[]) { this.children.push(...children); }, prepend() {}, replaceChildren(...children: any[]) { this.children = children; }, focus() {},
      querySelector() { return node(id + "-child"); }, querySelectorAll() { return []; },
      addTextTrack() { return { cues: [], addCue(cue: any) { this.cues.push(cue); }, removeCue(cue: any) { this.cues = this.cues.filter((value: any) => value !== cue); } }; },
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
  vm.runInNewContext(source, { document, IntersectionObserver, window: { ...(withObservers ? { IntersectionObserver } : {}), addEventListener: (name: string, handler: any) => { events[name] = handler; } }, location: { pathname: "/synthetic", search: "" }, URL,
    VTTCue: function(start: number, end: number, text: string) { return { start, end, text }; },
    AbortController, performance: { now: () => clock },
    fetch(url: string, options: object) { return new Promise((resolve, reject) => requests.push({ url, options, resolve, reject })); },
    setTimeout(run: () => void, delay: number) { const id = ++timer; timers.set(id, { run, delay }); return id; }, clearTimeout(id: number) { timers.delete(id); },
  });
  const respond = (request: any, value: any, status = 200) => request.resolve({ status, ok: status >= 200 && status < 300, json: async () => value });
  const receipt = (state: string, outcome?: string, status?: number) => ({ id: operation, action: "apply", item, state, outcome, status });
  const review = (language: string, marker: string) => ({ id: item, title: "Fictional review", language, fingerprint: createHash("sha256").update(marker).digest("hex"), role: "translation", source: marker, matchEvidence: "Synthetic logic fixture", warnings: [], originalAvailable: false, restorable: true, duration: 2,
    current: { cues: [{ start: 1, end: 2, text: "Fictional reviewed line", warnings: [] }], quality: { cueCount: 1, fastCues: 0, overlaps: 0, longLines: 0, maxCPS: 23, firstCue: 1, lastCue: 2, timing: "Synthetic", completeness: "Synthetic" } } });
  const draft = (state = "idle") => ({ id: "draft", language: form.elements.language.value, state, message: state, words: [] });
  return { node, form, requests, events, timers, exports, respond, review, draft, observers, receipt, operation, item,
    async ready(state = "idle") { respond(requests[0], review("en", "EN current")); respond(requests[1], draft(state)); await flush(); },
    async preview() { const pending = form.listeners.submit({ preventDefault() {} }); await flush(); const result = review(form.elements.language.value, "preview"); respond(requests.at(-1), { ...result, proposed: result.current }); await pending; },
    changeLanguage(language: string) { form.elements.language.value = language; form.listeners.input(); form.elements.language.listeners.change(); },
    poll() { const [id, next] = [...timers][0]; timers.delete(id); clock += next.delay; next.run(); },
  };
}
