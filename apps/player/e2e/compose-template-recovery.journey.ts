import { test, expect, type APIRequestContext, type Page, type TestInfo } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { begin, finish, verify, installClock, clock, deadline, type BrowserClock } from "./compose-template-recovery.observation";

type App = "player" | "subtitles" | "both";
type Mode = "valid" | "headers" | "body" | "loss" | "late_headers" | "late_body" | "http_error";
type Fields = { app: App; media: string; port: string; secondPort: string };
type Integrity = { blobMatches: boolean; copyMatches: boolean; blobSHA256: string; blobBytes: number };
const origin = "http://127.0.0.1:41847";
const suite = process.env.KINOSAIL_Q47_SUITE ?? "primary";
const templates = {
  player: readFileSync(resolve(import.meta.dirname, "../packaging/platform-compose.yaml"), "utf8"),
  subtitles: readFileSync(resolve(import.meta.dirname, "../../subtitles/packaging/platform-compose.yaml"), "utf8"),
  both: readFileSync(resolve(import.meta.dirname, "../packaging/platform-compose-both.yaml"), "utf8"),
};
const helper = readFileSync(resolve(import.meta.dirname, "../docs/assets/js/platform-install.js"));
const originals = [
  "https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/player/packaging/platform-compose.yaml",
  "https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/subtitles/packaging/platform-compose.yaml",
  "https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/player/packaging/platform-compose-both.yaml",
];
test.beforeEach(async ({}, info) => { begin(info); });
test.afterEach(async ({}, info) => { await info.attach("q47-assertions", { body: JSON.stringify(finish(info)), contentType: "application/json" }); });
const clocks = new WeakMap<Page, { responses: number }>();
const sha = (value: string | Buffer) => createHash("sha256").update(value).digest("hex");
const field = (page: Page, name: string) => page.locator("[data-install-" + name + "]");
function inputs(app: App): Fields {
  return { app, media: "/fictional/q47/media", port: app === "subtitles" ? "49128" : "49127", secondPort: app === "both" ? "49128" : "38128" };
}
async function control(request: APIRequestContext, operation: string, mode: Mode, app: App) {
  const response = await request.post("/__q47__/control", {
    headers: { "X-Q47-Control": "1" }, data: { operation, mode, app }, timeout: 2_000,
  });
  await verify(test.info(), "control-status", async () => expect(response.status(), "Q47 prerequisite:control").toBe(204));
}
async function witness(request: APIRequestContext) {
  const response = await request.get("/__q47__/witness", { timeout: 2_000 });
  await verify(test.info(), "witness-status", async () => expect(response.status(), "Q47 prerequisite:witness").toBe(200));
  return response.json();
}
async function start(page: Page, request: APIRequestContext, info: TestInfo, mode: Mode, values: Fields, width: number) {
  await verify(test.info(), "prior-handler-idle", async () => expect.poll(async () => (await witness(request)).counts.active, { timeout: 3_000, message: "Q47 prerequisite:idle" }).toBe(0));
  await control(request, "start", mode, values.app);
  await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 });
  await page.context().addCookies([{ name: "q47_fixture_marker", value: "fictional", url: origin + "/", sameSite: "Lax" }]);
  const counter = { responses: 0 }; clocks.set(page, counter);
  page.on("response", response => { if (response.url().startsWith(origin + "/assets/install/")) counter.responses++; });
  const delivered = page.waitForResponse(response => response.url() === origin + "/assets/js/platform-install.js");
  await page.goto("/getting-started/platforms/", { waitUntil: "load" });
  const bytes = await (await delivered).body();
  await verify(test.info(), "served-helper-equal", async () => expect(sha(bytes), "Q47 prerequisite:helper").toBe(sha(helper)));
  await info.attach("q47-asset", { body: JSON.stringify({ asset: { bytes: bytes.length, sha256: sha(bytes), sourceMatches: true } }), contentType: "application/json" });
  await verify(test.info(), "builder-visible", async () => expect(field(page, "builder"), "Q47 prerequisite:builder").toBeVisible());
  for (const href of originals) await verify(test.info(), "original-links-present", async () => expect(await page.locator("a[href]").evaluateAll((links, target) => links.some(link => link.getAttribute("href") === target), href), "Q47 prerequisite:originals").toBe(true));
  await fill(page, values);
  await installClock(page);
}
async function fill(page: Page, values: Fields) {
  await field(page, "app").selectOption(values.app);
  await field(page, "media").fill(values.media);
  await field(page, "port").fill(values.port);
  if (values.app === "both") await field(page, "subtitles-port").fill(values.secondPort);
}
async function state(page: Page, values: Fields, extra: Partial<Integrity> = {}) {
  const observed = await page.evaluate(({ values, originals }) => {
    const f = (name: string) => document.querySelector<HTMLElement>("[data-install-" + name + "]");
    const shown = (element: HTMLElement | null) => Boolean(element && !element.closest("[hidden]") && element.getBoundingClientRect().width > 0 && element.getBoundingClientRect().height > 0);
    const action = f("create") as HTMLButtonElement;
    const label = action.textContent?.trim();
    const links = [...(f("fallback")?.querySelectorAll<HTMLAnchorElement>("a") ?? [])];
    return {
      action: label === "Retry" ? "retry" : label === "Preparing file\u2026" ? "preparing" : label === "Make Compose file" ? "make" : "unknown",
      createDisabled: action.disabled, resultVisible: shown(f("result")), errorVisible: shown(f("error")),
      fallbackVisible: shown(f("fallback")), fallbackCanonical: links.length === 3 && links.every((link, index) => link.href === originals[index]),
      downloadReady: Boolean((f("download") as HTMLAnchorElement).getAttribute("href")?.startsWith("blob:")),
      retainedApp: (f("app") as HTMLSelectElement).value === values.app,
      retainedPath: (f("media") as HTMLInputElement).value === values.media,
      retainedPort: (f("port") as HTMLInputElement).value === values.port,
      retainedSecondPort: (f("subtitles-port") as HTMLInputElement).value === values.secondPort,
      blobMatches: null, copyMatches: null, blobSHA256: null, blobBytes: null,
      elapsedMs: Math.floor((window as Window & { __q47Clock: () => BrowserClock }).__q47Clock().elapsedMs ?? 0),
    };
  }, { values, originals });
  return { ...observed, responses: clocks.get(page)!.responses, ...extra };
}
async function attach(name: string, page: Page, request: APIRequestContext, info: TestInfo, values: Fields, extra: Partial<Integrity> = {}) {
  await info.attach(name, { body: JSON.stringify({ state: await state(page, values, extra), peer: await witness(request) }), contentType: "application/json" });
}
async function make(page: Page, request: APIRequestContext, info: TestInfo, values: Fields) {
  await field(page, "create").click();
  await verify(test.info(), "template-request-observed", async () => expect.poll(async () => (await witness(request)).counts.requests > 0, { timeout: 3_000, message: "Q47 prerequisite:request" }).toBe(true));
  await attach("q47-started", page, request, info, values);
}
function expectedFile(values: Fields) {
  const media = 'source: "' + '$' + '{KINOSAIL_MEDIA_PATH:?Set an existing absolute media path}' + '"';
  const first = values.app === "subtitles" ? "38128" : "38127";
  return templates[values.app].replaceAll(media, "source: " + JSON.stringify(values.media))
    .replace('- "' + first + ":" + first + '"', '- "' + values.port + ":" + first + '"')
    .replace('- "38128:38128"', values.app === "both" ? '- "' + values.secondPort + ':38128"' : '- "38128:38128"');
}
async function ready(page: Page, values: Fields) {
  const expected = expectedFile(values);
  await verify(test.info(), "result-visible", async () => expect(field(page, "result"), "Q47 acceptance:ready").toBeVisible());
  await verify(test.info(), "preview-equal", async () => expect(field(page, "preview"), "Q47 acceptance:integrity").toHaveText(expected));
  await verify(test.info(), "download-name-equal", async () => expect(field(page, "download"), "Q47 acceptance:integrity").toHaveAttribute("download", "kinosail-" + values.app + "-compose.yaml"));
  const blob = await field(page, "download").evaluate(async (element: HTMLAnchorElement) => {
    const bytes = await (await fetch(element.href)).arrayBuffer();
    const digest = await crypto.subtle.digest("SHA-256", bytes);
    return { bytes: bytes.byteLength, sha256: [...new Uint8Array(digest)].map(value => value.toString(16).padStart(2, "0")).join("") };
  });
  await verify(test.info(), "native-blob-digest-equal", async () => expect(blob.sha256, "Q47 acceptance:integrity").toBe(sha(expected)));
  await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
  await field(page, "copy").click();
  await verify(test.info(), "copy-status-equal", async () => expect(field(page, "status"), "Q47 acceptance:copy").toHaveText("Compose file copied."));
  await verify(test.info(), "native-copy-equal", async () => expect(await page.evaluate(() => navigator.clipboard.readText()), "Q47 acceptance:copy").toBe(expected));
  return { blobMatches: true, copyMatches: true, blobSHA256: blob.sha256, blobBytes: blob.bytes };
}
async function retry(page: Page, request: APIRequestContext, values: Fields, keyboard: boolean) {
  await control(request, "recover", "valid", values.app);
  const button = page.getByRole("button", { name: "Retry", exact: true });
  if (keyboard) {
    await field(page, values.app === "both" ? "subtitles-port" : "port").focus();
    await page.keyboard.press("Tab");
    await verify(test.info(), "retry-keyboard-focused", async () => expect(button, "Q47 acceptance:focus").toBeFocused());
    await page.keyboard.press("Enter");
  } else await button.click();
}

if (suite === "primary") {
  for (const scenario of [
    { name: "Q47 headers deadline phone Player", mode: "headers" as Mode, app: "player" as App, width: 390 },
    { name: "Q47 body deadline desktop Both", mode: "body" as Mode, app: "both" as App, width: 1440 },
  ]) test(scenario.name, async ({ page, request }, info) => {
    const values = inputs(scenario.app);
    await start(page, request, info, scenario.mode, values, scenario.width);
    await make(page, request, info, values);
    const pending = await witness(request);
    await verify(test.info(), "primary-one-request", async () => expect(pending.counts.requests, "Q47 prerequisite:hold").toBe(1));
    await verify(test.info(), "primary-one-hold", async () => expect(pending.counts.holds, "Q47 prerequisite:hold").toBe(1));
    await verify(test.info(), "primary-body-prefix", async () => expect(pending.checks.bodyPrefixSent, "Q47 prerequisite:hold").toBe(scenario.mode === "body"));
    await verify(test.info(), "primary-template-digest", async () => expect(pending.templateSHA256, "Q47 prerequisite:template").toBe(sha(templates[values.app])));
    const loading = await state(page, values);
    await verify(test.info(), "primary-pending-label", async () => expect(loading.action, "Q47 prerequisite:hold").toBe("preparing"));
    await verify(test.info(), "primary-pending-disabled", async () => expect(loading.createDisabled, "Q47 prerequisite:hold").toBe(true));
    await verify(test.info(), "primary-no-output", async () => expect(loading.resultVisible || loading.errorVisible || loading.downloadReady, "Q47 prerequisite:hold").toBe(false));
    await attach("q47-pending", page, request, info, values);
    await verify(info, "pending-confirmed-before-deadline", async () => expect(loading.elapsedMs < 20_000 && pending.counts.active === 1 && pending.counts.expired === 0 && pending.counts.canceled === 0, "Q47 prerequisite:clock").toBe(true));
    const captured = await deadline(page, info);
    await verify(info, "absolute-clock-window-eligible", async () => expect(captured.trustedClick && captured.clicks === 1 && captured.pendingObserved && captured.sample !== null && captured.sample.elapsedMs >= 20_000 && captured.sample.elapsedMs <= 20_200, "Q47 prerequisite:clock").toBe(true));
    const atDeadline = { ...await state(page, values), ...captured.sample!, elapsedMs: Math.floor(captured.sample!.elapsedMs) };
    await info.attach("q47-deadline", { body: JSON.stringify({ state: atDeadline, peer: await witness(request) }), contentType: "application/json" });
    await verify(test.info(), "deadline-retry-label", async () => expect(atDeadline.action, "Q47 acceptance:deadline").toBe("retry"));
    await verify(test.info(), "deadline-enabled", async () => expect(atDeadline.createDisabled, "Q47 acceptance:deadline").toBe(false));
    await verify(test.info(), "deadline-error-visible", async () => expect(atDeadline.errorVisible, "Q47 acceptance:deadline").toBe(true));
    await verify(info, "recovery-observed-by-product-deadline", async () => expect(captured.firstEnabledMs !== null && captured.firstEnabledMs <= 20_000 && captured.firstRecoveryMs !== null && captured.firstRecoveryMs <= 20_000, "Q47 acceptance:deadline").toBe(true));
    await verify(test.info(), "deadline-retry-visible", async () => expect(page.getByRole("button", { name: "Retry", exact: true }), "Q47 acceptance:deadline").toBeVisible({ timeout: 1_000 }));
    await verify(test.info(), "deadline-error-still-visible", async () => expect(field(page, "error"), "Q47 acceptance:deadline").toBeVisible());
    await verify(test.info(), "fallback-visible", async () => expect(field(page, "fallback"), "Q47 acceptance:originals").toBeVisible());
    const failed = await state(page, values);
    await verify(test.info(), "deadline-elapsed-at-least20s", async () => expect(failed.elapsedMs, "Q47 acceptance:deadline").toBeGreaterThanOrEqual(20_000));
    await verify(test.info(), "fallback-canonical", async () => expect(failed.fallbackCanonical, "Q47 acceptance:originals").toBe(true));
    for (const key of ["retainedApp", "retainedPath", "retainedPort", "retainedSecondPort"] as const) await verify(test.info(), "primary-input-retention", async () => expect(failed[key], "Q47 acceptance:retention").toBe(true));
    await verify(test.info(), "primary-peer-canceled", async () => expect.poll(async () => (await witness(request)).counts.canceled, { timeout: 3_000, message: "Q47 acceptance:cancellation" }).toBe(1));
    await attach("q47-failed", page, request, info, values);
    await retry(page, request, values, scenario.mode === "body");
    const integrity = await ready(page, values);
    await verify(test.info(), "primary-new-completed", async () => expect.poll(async () => (await witness(request)).counts.completed, { timeout: 3_000, message: "Q47 acceptance:new_attempt" }).toBe(1));
    const final = await witness(request);
    await verify(test.info(), "primary-two-requests", async () => expect(final.counts.requests, "Q47 acceptance:new_attempt").toBe(2));
    await verify(test.info(), "primary-settled", async () => expect(final.counts.active || final.counts.expired, "Q47 acceptance:cancellation").toBe(0));
    await verify(test.info(), "primary-privacy", async () => expect(final.checks.cookieSeen || final.checks.authorizationSeen || final.checks.bodySeen || final.checks.querySeen, "Q47 acceptance:privacy").toBe(false));
    await attach("q47-recovered", page, request, info, values, integrity);
  });
}

if (suite === "recovery") {
  for (const [name, mode] of [
    ["Q47 lost request recovers with retained inputs", "loss"],
    ["Q47 failed HTTP recovers with retained inputs", "http_error"],
  ] as const) test(name, async ({ page, request }, info) => {
    const values = inputs("both");
    await start(page, request, info, mode, values, 390);
    await make(page, request, info, values);
    await verify(test.info(), "recovery-error-visible", async () => expect(field(page, "error"), "Q47 prerequisite:fault").toBeVisible());
    await attach("q47-failed", page, request, info, values);
    await verify(test.info(), "recovery-retry-visible", async () => expect(page.getByRole("button", { name: "Retry", exact: true }), "Q47 acceptance:retry").toBeVisible());
    const failed = await state(page, values);
    for (const key of ["retainedApp", "retainedPath", "retainedPort", "retainedSecondPort"] as const) await verify(test.info(), "recovery-input-retention", async () => expect(failed[key], "Q47 acceptance:retention").toBe(true));
    await verify(test.info(), "recovery-fallback-canonical", async () => expect(failed.fallbackCanonical, "Q47 acceptance:originals").toBe(true));
    await retry(page, request, values, false);
    const integrity = await ready(page, values);
    await verify(test.info(), "recovery-new-completed", async () => expect.poll(async () => (await witness(request)).counts.completed, { timeout: 3_000, message: "Q47 acceptance:new_attempt" }).toBe(1));
    const final = await witness(request);
    await verify(test.info(), "recovery-new-request", async () => expect(final.counts.requests, "Q47 acceptance:new_attempt").toBeGreaterThanOrEqual(2));
    await verify(test.info(), "recovery-prior-failed", async () => expect(final.counts.failures, "Q47 acceptance:new_attempt").toBeGreaterThanOrEqual(1));
    await verify(test.info(), "recovery-settled", async () => expect(final.counts.active || final.counts.expired, "Q47 acceptance:cancellation").toBe(0));
    await verify(test.info(), "recovery-privacy", async () => expect(final.checks.cookieSeen || final.checks.authorizationSeen || final.checks.bodySeen || final.checks.querySeen, "Q47 acceptance:privacy").toBe(false));
    await attach("q47-recovered", page, request, info, values, integrity);
  });
}

if (suite === "supersession") {
  for (const [name, mode] of [
    ["Q47 input change discards late headers", "late_headers"],
    ["Q47 app change discards late body", "late_body"],
  ] as const) test(name, async ({ page, request }, info) => {
    const first = inputs("player");
    await start(page, request, info, mode, first, 1440);
    await make(page, request, info, first);
    await verify(test.info(), "supersession-one-hold", async () => expect.poll(async () => (await witness(request)).counts.holds, { timeout: 3_000, message: "Q47 prerequisite:hold" }).toBe(1));
    const next = mode === "late_body" ? inputs("subtitles") : { ...first, media: "/fictional/q47/changed", port: "50127" };
    if (mode === "late_body") await fill(page, next);
    else {
      await field(page, "media").fill(next.media);
      await field(page, "port").fill(next.port);
    }
    await field(page, "create").click();
    const integrity = await ready(page, next);
    await control(request, "release", mode, first.app);
    await verify(test.info(), "supersession-no-active", async () => expect.poll(async () => (await witness(request)).counts.active, { timeout: 3_000, message: "Q47 acceptance:cancellation" }).toBe(0));
    await verify(test.info(), "supersession-peer-canceled", async () => expect((await witness(request)).counts.canceled, "Q47 acceptance:cancellation").toBe(1));
    await attach("q47-stale", page, request, info, next, integrity);
    await verify(test.info(), "supersession-preview-current", async () => expect(field(page, "preview"), "Q47 acceptance:stale").toHaveText(expectedFile(next)));
  });
}

if (suite === "contracts") {
  for (const app of ["player", "subtitles", "both"] as const) test("Q47 " + (app === "player" ? "Player" : app === "subtitles" ? "Subtitles" : "Both") + " preserves Compose download and copy", async ({ page, request }, info) => {
    const values = inputs(app);
    await start(page, request, info, "valid", values, 390);
    await make(page, request, info, values);
    const integrity = await ready(page, values);
    await verify(test.info(), "contract-one-completed", async () => expect.poll(async () => (await witness(request)).counts.completed, { timeout: 3_000, message: "Q47 acceptance:ready" }).toBe(1));
    const final = await witness(request);
    await verify(test.info(), "contract-one-request", async () => expect(final.counts.requests, "Q47 acceptance:privacy").toBe(1));
    await verify(test.info(), "contract-settled", async () => expect(final.counts.active || final.counts.expired, "Q47 acceptance:ready").toBe(0));
    await verify(test.info(), "contract-privacy", async () => expect(final.checks.cookieSeen || final.checks.authorizationSeen || final.checks.bodySeen || final.checks.querySeen, "Q47 acceptance:privacy").toBe(false));
    await attach("q47-recovered", page, request, info, values, integrity);
  });
  test("Q47 rejects invalid inputs before template requests", async ({ page, request }, info) => {
    const values = inputs("both");
    await start(page, request, info, "valid", values, 1440);
    for (const media of ["", "/", "relative/media", "/fictional/../media", "/fictional/$MEDIA", "/fictional/\u007fmedia", "/media "]) {
      await field(page, "media").fill(media);
      await field(page, "create").click();
      await verify(test.info(), "invalid-path-error", async () => expect(field(page, "error"), "Q47 acceptance:validation").toBeVisible());
      await verify(test.info(), "invalid-path-no-request", async () => expect((await witness(request)).counts.requests, "Q47 acceptance:validation").toBe(0));
    }
    await field(page, "media").fill(values.media);
    for (const ports of [["80", "49128"], ["01024", "49128"], ["65536", "49128"], ["49127", "49127"]]) {
      await field(page, "port").fill(ports[0]); await field(page, "subtitles-port").fill(ports[1]);
      await field(page, "create").click();
      await verify(test.info(), "invalid-port-error", async () => expect(field(page, "error"), "Q47 acceptance:validation").toBeVisible());
      await verify(test.info(), "invalid-port-no-request", async () => expect((await witness(request)).counts.requests, "Q47 acceptance:validation").toBe(0));
    }
    await attach("q47-rejected", page, request, info, values);
    await verify(test.info(), "invalid-no-result", async () => expect(field(page, "result"), "Q47 acceptance:validation").toBeHidden());
    await verify(test.info(), "invalid-no-download", async () => expect(await field(page, "download").getAttribute("href"), "Q47 acceptance:validation").toBeNull());
  });
}
