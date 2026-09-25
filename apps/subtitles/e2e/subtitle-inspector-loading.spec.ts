import { readFileSync } from "node:fs";
import vm from "node:vm";
import { expect, test } from "@playwright/test";

const source = readFileSync(new URL("../internal/server/static/subtitle-inspector.js", import.meta.url), "utf8");
const loadSource = source.slice(source.indexOf("  async function load() {"), source.indexOf("  async function input() {"));

function fixture() {
  const attributes = new Map<string, string>();
  const classes = new Set<string>();
  const status = {
    textContent: "Previous result",
    classList: { contains: (name: string) => classes.has(name), add: (name: string) => classes.add(name), remove: (name: string) => classes.delete(name) },
    setAttribute: (name: string, value: string) => attributes.set(name, value),
    removeAttribute: (name: string) => attributes.delete(name),
  };
  let resolveRequest!: (value: object) => void;
  let rejectRequest!: (error: Error) => void;
  const context = vm.createContext({
    status, apply: { disabled: false }, form: { elements: { language: { value: "en" }, role: { value: "" } } },
    request: () => new Promise((resolve, reject) => { resolveRequest = resolve; rejectRequest = reject; }),
    render: () => {},
  });
  vm.runInContext(`let revision = 0, prepared, review, page; ${loadSource}; globalThis.loadUnderTest = load;`, context);
  return { status, attributes, classes, context, resolve: (value: object) => resolveRequest(value), reject: (error: Error) => rejectRequest(error) };
}

test("inspector status stays one line while details load and settles after success", { tag: "@smoke" }, async () => {
  const view = fixture();
  const pending = (view.context as { loadUnderTest: () => Promise<void> }).loadUnderTest();
  expect(view.status.textContent).toBe("Loading subtitle details…");
  expect(view.attributes.get("aria-busy")).toBe("true");
  expect(view.classes.has("request-skeleton")).toBe(false);
  view.resolve({ current: null, role: "translation" });
  await pending;
  expect(view.attributes.has("aria-busy")).toBe(false);
  expect(view.status.textContent).toBe("Choose a subtitle file to begin.");
});

test("inspector clears busy state after a failed request", { tag: "@smoke" }, async () => {
  const view = fixture();
  const pending = (view.context as { loadUnderTest: () => Promise<void> }).loadUnderTest();
  view.reject(new Error("unavailable"));
  await pending.then(() => { throw new Error("Request unexpectedly succeeded"); }, error => expect(error.message).toBe("unavailable"));
  expect(view.attributes.has("aria-busy")).toBe(false);
  expect(view.classes.has("request-skeleton")).toBe(false);
});
