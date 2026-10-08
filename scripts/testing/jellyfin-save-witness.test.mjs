import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {stripTypeScriptTypes} from "node:module";
import {runInNewContext} from "node:vm";
import {EventEmitter} from "node:events";
import {createHmac} from "node:crypto";
import test from "node:test";

const source = stripTypeScriptTypes(readFileSync(new URL("../../apps/player/e2e/jellyfin-setup.spec.ts", import.meta.url), "utf8")).replace(/^import .*;\n/gm, "");
const origin = "https://localhost:38127", original = new Error("original exact destination assertion");
function fixture({mode = "put", attachment = "ok"} = {}) {
  const registrations = [], values = [], effects = [];
  const pw = (_, callback) => registrations.push(callback);
  for (const key of ["skip", "use", "afterEach", "setTimeout"]) pw[key] = () => {};
  let saves = 0, saveRoute;
  const req = (method, path) => ({url: () => origin + path, method: () => method, isNavigationRequest: () => method === "POST", frame: () => page.mainFrame()});
  class Page extends EventEmitter {
    current = origin + "/login";
    frame = {url: () => this.current};
    url() {return this.current;}
    mainFrame() {return this.frame;}
    async goto(path) {this.current = new URL(path, origin).href;}
    async reload() {}
    async setViewportSize() {}
    async screenshot() {}
    async emulateMedia() {}
    async evaluate() {return true;}
    keyboard = {press: async () => {}};
    locator() {return locator;}
    getByRole() {return locator;}
    getByLabel() {return locator;}
    getByText() {return locator;}
  }
  const page = new Page();
  const locator = new Proxy({}, {get(_, key) {
    if (["locator", "getByRole", "getByLabel", "getByText", "first"].includes(key)) return () => locator;
    if (key === "evaluate") return async () => true;
    if (key === "click") return async () => {
      if (!saveRoute) return;
      saves++; effects.push("save-" + saves);
      if (saves === 1) await saveRoute({request: () => ({method: () => "PUT"}), fulfill: async () => {}});
      else {
        await saveRoute({request: () => ({method: () => "PUT"}), continue: async () => {
          page.current = origin + "/onboarding/connection#trusted-https-configuration";
          if (mode === "foreign") {
            for (const raw of ["https://foreign.invalid/api/v1/settings/trusted-https", origin + "/api/v1/settings/trusted-https?private=value", "x".repeat(5000)]) {
              const request = {url: () => raw, method: () => "PUT"}; page.emit("request", request);
              page.emit("response", {url: () => raw, status: () => 200, request: () => request});
            }
            return;
          }
          const request = req(mode === "post" ? "POST" : "PUT", mode === "post" ? "/onboarding/trusted-https" : "/api/v1/settings/trusted-https");
          page.emit("request", request);
          if (mode === "failed") page.emit("requestfailed", request);
          else page.emit("response", {url: request.url, status: () => mode === "post" ? 303 : 200, request: () => request});
          if (mode === "post") {
            page.emit("request", {url: () => origin + "/onboarding/connection", method: () => "GET", isNavigationRequest: () => true, frame: () => page.mainFrame()});
            page.emit("framenavigated", page.mainFrame());
          }
        }});
      }
    };
    return async () => {};
  }});
  const expect = value => new Proxy({}, {get(target, key, receiver) {
    if (key === "not") return receiver;
    if (key === "toHaveURL") return async expected => {
      if (expected === "/" && saves === 0) {page.current = origin + "/"; return;}
      if (expected === "/onboarding/connection" && saves === 2) throw original;
    };
    return () => {};
  }});
  class AxeBuilder {include() {return this;} async analyze() {return {violations: []};}}
  runInNewContext(source, {test: pw, expect, configureProviderProfile() {}, finishRootSignIn: async () => {},
    providerRoute: async (_, pattern, callback) => {if (pattern === "**/api/v1/settings/trusted-https") saveRoute = callback;},
    AxeBuilder, createHmac, Buffer, URL, setTimeout, clearTimeout, process: {env: {}}});
  const info = {project: {use: {baseURL: origin}}, outputPath: () => "/unused", attach: async (name, value) => {
    if (attachment === "reject") throw new Error("diagnostic-only");
    if (attachment === "stall") return new Promise(() => {});
    values.push({name, value});
  }};
  return {run: () => registrations[0]({page, browserName: "chromium"}, info), page, values, effects};
}
function facts(control) {
  assert.equal(control.values.length, 1); const {name, value} = control.values[0];
  assert.equal(name, "jellyfin-save-stages"); assert.equal(value.contentType, "application/json");
  assert.ok(Buffer.byteLength(value.body) < 2048); assert.doesNotMatch(value.body, /https?:|private|localhost|token|header|body/);
  return JSON.parse(value.body);
}
for (const mode of ["put", "post", "failed", "foreign"]) test("actual Save callback distinguishes " + mode + " without changing its failed exact destination", async () => {
  const control = fixture({mode}); await assert.rejects(control.run(), error => error === original);
  const value = facts(control);
  assert.equal(value.putRequests, mode === "put" || mode === "failed" ? 1 : 0);
  assert.equal(value.putResponses, mode === "put" ? 1 : 0); assert.equal(value.putFailures, mode === "failed" ? 1 : 0);
  assert.equal(value.putStatus, mode === "put" ? 200 : null);
  assert.equal(value.nativePostRequests, mode === "post" ? 1 : 0);
  assert.equal(value.nativePostResponses, mode === "post" ? 1 : 0); assert.equal(value.nativePostFailures, 0);
  assert.equal(value.nativePostStatus, mode === "post" ? 303 : null);
  assert.equal(value.documentStarted, mode === "post"); assert.equal(value.documentCommitted, mode === "post");
  assert.equal(value.currentOriginOwned, true); assert.equal(value.currentPath, "connection"); assert.equal(value.currentFragment, "trusted-https");
  assert.deepEqual(control.effects, ["save-1", "save-2"]); assert.equal(control.page.eventNames().length, 0);
});
for (const attachment of ["reject", "stall"]) test("diagnostic " + attachment + " preserves exact assertion cause and listener cleanup", async () => {
  const control = fixture({attachment}); await assert.rejects(control.run(), error => error === original);
  assert.deepEqual(control.effects, ["save-1", "save-2"]); assert.equal(control.values.length, 0); assert.equal(control.page.eventNames().length, 0);
});
