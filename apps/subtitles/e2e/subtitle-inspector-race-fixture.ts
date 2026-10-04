import { readFileSync } from "node:fs";
import vm from "node:vm";

const source = ["subtitle-source-cues.js", "subtitle-inspector.js"].map(name => readFileSync(new URL(`../internal/server/static/${name}`, import.meta.url), "utf8")).join("\n");
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
  let timer = 0, created = 0;
  function node(id = ""): any {
    if (nodes.has(id)) return nodes.get(id);
    const n: any = { id, dataset: { id: "synthetic" }, style: {}, value: "", files: [], checked: false, disabled: false, hidden: false, children: [], listeners: {}, textContent: "", attrs: {},
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
    fetch(url: string, options: object) { return new Promise((resolve, reject) => requests.push({ url, options, resolve, reject })); },
    setTimeout(run: () => void, delay: number) { const id = ++timer; timers.set(id, { run, delay }); return id; }, clearTimeout(id: number) { timers.delete(id); },
  });
  const respond = (request: any, value: any) => request.resolve({ status: 200, ok: true, json: async () => value });
  const review = (language: string, marker: string) => ({ language, fingerprint: marker, role: "translation", source: marker, warnings: [], restorable: true, current: { cues: [], quality: {} }, proposed: null });
  const draft = (state = "idle") => ({ id: "draft", language: form.elements.language.value, state, message: state, words: [] });
  return { node, form, requests, events, timers, exports, respond, review, draft, observers,
    async ready(state = "idle") { respond(requests[0], review("en", "EN current")); respond(requests[1], draft(state)); await flush(); },
    async preview() { const pending = form.listeners.submit({ preventDefault() {} }); await flush(); respond(requests.at(-1), review(form.elements.language.value, "preview")); await pending; },
    changeLanguage(language: string) { form.elements.language.value = language; form.listeners.input(); form.elements.language.listeners.change(); },
    poll() { const [id, next] = [...timers][0]; timers.delete(id); next.run(); },
  };
}
