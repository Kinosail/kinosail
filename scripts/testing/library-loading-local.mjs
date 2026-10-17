import {createRequire} from "node:module";
import {writeFile} from "node:fs/promises";
import {join} from "node:path";

const require = createRequire(new URL("../../apps/player/e2e/package.json", import.meta.url));
const load = name => { try { return require(name); } catch { return null; } };
const {chromium} = load("@playwright/test") ?? load("playwright") ?? (() => { throw new Error("Playwright is not installed"); })();
const baseURL = process.env.KINOSAIL_E2E_URL, token = process.env.KINOSAIL_LOADING_TOKEN;
const app = process.env.KINOSAIL_LOADING_APP, phase = process.env.KINOSAIL_LOADING_PHASE, run = process.env.KINOSAIL_LOADING_RUN;
const path = app === "player" ? "/" : "/?view=library";
const viewports = {phone: {width: 390, height: 844, reducedMotion: "reduce"}, desktop: {width: 1440, height: 900, reducedMotion: "no-preference"}, tv: {width: 1920, height: 1080, reducedMotion: "no-preference"}};
const evidence = {revision: process.env.KINOSAIL_TEST_REVISION, app, phase, path, viewports: {}, api: {}};
const failures = [];
const check = (condition, message) => { if (!condition) failures.push(message); };
const box = async (page, selector) => page.locator(selector).first().boundingBox();

async function api(context, label) {
  const response = await context.request.get("/api/v1/library?limit=1", {failOnStatusCode: false});
  evidence.api[label] = {status: response.status(), retryAfter: response.headers()["retry-after"] ?? null};
  return evidence.api[label];
}

async function settled(page, timeout) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    try {
      const state = await page.evaluate(() => ({ready: document.readyState === "complete", loading: Boolean(document.querySelector("[data-library-loading]")),
        status: performance.getEntriesByType("navigation")[0]?.responseStatus ?? 0, navigations: Number(sessionStorage.getItem("kinosail-loading-navigations") || 0)}));
      if (state.ready && !state.loading) return state;
    } catch { }
    await page.waitForTimeout(250);
  }
  throw new Error("Library loading state did not settle");
}

async function observe(browser, name, viewport) {
  const context = await browser.newContext({baseURL, viewport: {width: viewport.width, height: viewport.height}, reducedMotion: viewport.reducedMotion, extraHTTPHeaders: {Authorization: `Bearer ${token}`}, serviceWorkers: "block"});
  await context.addInitScript(() => {
    sessionStorage.setItem("kinosail-loading-navigations", String(Number(sessionStorage.getItem("kinosail-loading-navigations") || 0) + 1));
    window.__layoutShift = 0;
    new PerformanceObserver(list => { for (const entry of list.getEntries()) if (!entry.hadRecentInput) window.__layoutShift += entry.value; }).observe({type: "layout-shift", buffered: true});
  });
  const page = await context.newPage();
  const record = evidence.viewports[name] = {viewport};
  const response = await page.goto(path, {waitUntil: "load"});
  record.loading = {status: response.status(), retryAfter: response.headers()["retry-after"] ?? null, cacheControl: response.headers()["cache-control"] ?? null};
  check(response.status() === 503 && record.loading.retryAfter === "5", `${name}: loading response ${response.status()} Retry-After ${record.loading.retryAfter}`);
  const region = page.locator("[data-library-loading]");
  check(await region.count() === 1, `${name}: loading region missing`);
  record.loading.heading = await page.getByRole("heading", {name: "Scanning your Library"}).textContent().catch(() => null);
  record.loading.statusText = (await page.getByRole("status").filter({hasText: "reading your media folders"}).textContent().catch(() => "")).trim();
  check(record.loading.heading === "Scanning your Library" && record.loading.statusText.length > 0, `${name}: loading heading or status text missing`);
  const placeholder = app === "player" ? ".library-loading-art" : "[data-library-loading]";
  record.loading.animation = await page.locator(app === "player" ? ".library-loading-art" : "[data-library-loading]").first().evaluate(element => getComputedStyle(element).animationName);
  if (app === "player") check(record.loading.animation === (viewport.reducedMotion === "reduce" ? "none" : "skeleton-shimmer"), `${name}: animation ${record.loading.animation}`);
  record.loading.boxes = {placeholder: await box(page, placeholder), heading: await box(page, "[data-library-loading] h2"), firstCard: app === "player" ? await box(page, ".library-loading-shelf .card") : null};
  await page.waitForTimeout(2500);
  record.loading.layoutShift = await page.evaluate(() => window.__layoutShift);
  record.loading.boxesAfterWait = {placeholder: await box(page, placeholder), heading: await box(page, "[data-library-loading] h2")};
  check(record.loading.layoutShift === 0 && JSON.stringify(record.loading.boxes.heading) === JSON.stringify(record.loading.boxesAfterWait.heading), `${name}: loading state moved (CLS ${record.loading.layoutShift})`);
  check(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), `${name}: loading page scrolls horizontally`);
  await page.screenshot({path: join(run, `${phase}-${name}-pending.png`), fullPage: false});
  if (name === "desktop") await api(context, "pending");
  const started = Date.now();
  const state = await settled(page, 90_000);
  record.settled = {...state, afterMilliseconds: Date.now() - started};
  await page.waitForTimeout(500);
  await page.screenshot({path: join(run, `${phase}-${name}-${phase === "loading" ? "loaded" : "failed"}.png`), fullPage: false});
  if (phase === "loading") {
    check(state.status === 200 && state.navigations >= 2, `${name}: loaded status ${state.status} navigations ${state.navigations}`);
    const loaded = app === "player" ? page.locator(".home-feature, .home-shelf, .destination-browser").first() : page.locator("#subtitle-content .subtitle-file, #subtitle-content [data-subtitle-item], #subtitle-content article").first();
    check(await loaded.isVisible().catch(() => false), `${name}: loaded Library content is not visible`);
    const optionalBox = async selector => await page.locator(selector).count() ? box(page, selector) : null;
    record.settled.boxes = {feature: app === "player" ? await optionalBox(".home-feature > img") : null, firstCard: app === "player" ? await optionalBox(".home-shelf .card") : null};
    check(await page.getByText("Arrival").first().isVisible().catch(() => false), `${name}: scanned title is not visible`);
    if (name === "desktop") {
      const loadedAPI = await api(context, "loaded");
      check(loadedAPI.status === 200 && loadedAPI.retryAfter === null, `api loaded ${loadedAPI.status} Retry-After ${loadedAPI.retryAfter}`);
    }
  } else {
    const text = await page.locator("main").innerText();
    record.settled.text = text.slice(0, 400);
    const html = await page.content();
    record.settled.mediaPathVisible = html.includes(process.env.KINOSAIL_MEDIA_DIR) || /input\/output error/i.test(html);
    const failed = app === "player" ? state.status === 503 && /library is unavailable/.test(text) : /Coverage is unavailable\./.test(text);
    check(failed && !record.settled.mediaPathVisible, `${name}: failed scan did not show the generic failure without the media path (status ${state.status})`);
  }
  if (name === "desktop" && evidence.api.pending) check(evidence.api.pending.status === 503 && evidence.api.pending.retryAfter === "5", `api pending ${evidence.api.pending.status} Retry-After ${evidence.api.pending.retryAfter}`);
  await context.close();
}

const browser = await chromium.launch();
try {
  const names = phase === "loading" ? Object.keys(viewports) : ["desktop"];
  await Promise.all(names.map(name => observe(browser, name, viewports[name])));
} catch (error) {
  failures.push(String(error?.message ?? error));
} finally {
  await browser.close();
  evidence.failures = failures;
  evidence.result = failures.length === 0 ? "passed" : "failed";
  await writeFile(join(run, `browser-${phase}.json`), JSON.stringify(evidence, null, 2) + "\n");
}
process.stdout.write(`${JSON.stringify({app, phase, result: evidence.result, failures})}\n`);
process.exit(failures.length === 0 ? 0 : 1);
