import { expect, test, type Page } from "@playwright/test";

const owner = "Dashboard Owner";
const password = "correct horse battery staple";
const delayMs = 180;

test.use({ serviceWorkers: "block" });

async function authenticate(page: Page) {
  await page.goto("/");
  if (page.url().endsWith("/setup")) {
    await page.getByLabel("Name", { exact: true }).fill(owner);
    await page.getByLabel("Password", { exact: true }).fill(password);
    await page.getByLabel("Confirm password").fill(password);
    await page.getByRole("button", { name: "Create Owner & continue" }).click();
  } else if (page.url().endsWith("/login")) {
    await page.getByLabel("Name", { exact: true }).fill(owner);
    await page.getByLabel("Password", { exact: true }).fill(password);
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
  }
  await expect(page).toHaveURL(/\?passkey=offer$/);
  const offer = page.getByRole("button", { name: "Not now" });
  await expect(offer).toBeVisible();
  await offer.click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("heading", { name: "Home", exact: true })).toBeVisible();
}

test("loads identity, catalog, and a maximum-size board without an API waterfall", { tag: "@chromium" }, async ({ page }) => {
  await authenticate(page);
  const apps = Array.from({ length: 200 }, (_, index) => ({
    id: `app-${index}`,
    name: `Application ${index}`,
    url: `http://127.0.0.1:${40000 + index}`,
    healthUrl: `http://127.0.0.1:${40000 + index}`,
    checkEnabled: false,
    description: "Household application",
    category: "Household",
    icon: "app",
    accent: "slate",
    favorite: false,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    health: { state: "unchecked" },
  }));
  const payloads = {
    "/api/v1/me": { owner: { id: "owner", name: owner }, csrf: "test-csrf" },
    "/api/v1/catalog": { apps: [], aliases: {} },
    "/api/v1/board": {
      title: "Home",
      version: 1,
      apps,
      recentlyRemoved: [],
      summary: { total: apps.length, reachable: 0, slow: 0, degraded: 0, unavailable: 0, unchecked: 0, disabled: apps.length },
      audit: [],
      updatedAt: "2026-01-01T00:00:00Z",
    },
  };
  const starts = new Map<string, number>();
  const moduleStarts = new Map<string, number>();
  await page.route("**/api/v1/board/check", async route => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ board: payloads["/api/v1/board"], check: { requested: 0, completed: 0, canceled: false } }) });
  });
  await page.route(/\/static\/.*\.js$/, async route => {
    const path = new URL(route.request().url()).pathname;
    if (!moduleStarts.has(path)) moduleStarts.set(path, performance.now());
    await new Promise(resolve => setTimeout(resolve, delayMs));
    await route.continue();
  });
  for (const [path, body] of Object.entries(payloads)) {
    await page.route(`**${path}`, async route => {
      if (!starts.has(path)) starts.set(path, performance.now());
      await new Promise(resolve => setTimeout(resolve, delayMs));
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
    });
  }
  await page.addInitScript(() => {
    const metrics = { lcp: 0, cls: 0, tbt: 0 };
    (window as Window & { dashboardPerformance: typeof metrics }).dashboardPerformance = metrics;
    new PerformanceObserver(list => {
      const entries = list.getEntries();
      metrics.lcp = entries[entries.length - 1]?.startTime || metrics.lcp;
    }).observe({ type: "largest-contentful-paint", buffered: true });
    new PerformanceObserver(list => {
      for (const entry of list.getEntries()) if (!(entry as PerformanceEntry & { hadRecentInput?: boolean }).hadRecentInput) metrics.cls += (entry as PerformanceEntry & { value?: number }).value || 0;
    }).observe({ type: "layout-shift", buffered: true });
    new PerformanceObserver(list => {
      for (const entry of list.getEntries()) metrics.tbt += Math.max(0, entry.duration - 50);
    }).observe({ type: "longtask", buffered: true });
  });
  const started = performance.now();
  await page.reload();
  await page.waitForFunction(expected => document.querySelectorAll(".app-tile").length === expected, apps.length);
  const elapsedMs = Math.round(performance.now() - started);
  const requestSpreadMs = Math.round(Math.max(...starts.values()) - Math.min(...starts.values()));
  const moduleSpreadMs = Math.round(Math.max(...moduleStarts.values()) - Math.min(...moduleStarts.values()));
  const render = await page.evaluate(async () => {
    const grid = document.querySelector("#app-grid")!;
    let mutationRecords = 0;
    const observer = new MutationObserver(records => { mutationRecords += records.length; });
    observer.observe(grid, { childList: true });
    const started = performance.now();
    const search = document.querySelector<HTMLInputElement>("#board-search")!;
    search.value = "Application";
    search.dispatchEvent(new Event("input", { bubbles: true }));
    await new Promise(requestAnimationFrame);
    observer.disconnect();
    return { elapsedMs: Math.round(performance.now() - started), mutationRecords };
  });
  await page.evaluate(() => {
    const probe = { mutationRecords: 0, observer: null as MutationObserver | null };
    probe.observer = new MutationObserver(records => { probe.mutationRecords += records.length; });
    probe.observer.observe(document.querySelector("#app-grid")!, { childList: true });
    (window as Window & { gridProbe: typeof probe }).gridProbe = probe;
  });
  const check = page.getByRole("button", { name: "Check now" });
  await check.click();
  await expect(check).toBeEnabled();
  const refreshMutationRecords = await page.evaluate(() => {
    const probe = (window as Window & { gridProbe: { mutationRecords: number, observer: MutationObserver } }).gridProbe;
    probe.observer.disconnect();
    return probe.mutationRecords;
  });
  await page.waitForTimeout(250);
  const vitals = await page.evaluate(() => {
    const navigation = performance.getEntriesByType("navigation")[0] as PerformanceNavigationTiming;
    const paints = Object.fromEntries(performance.getEntriesByType("paint").map(entry => [entry.name, Math.round(entry.startTime)]));
    return {
      ...((window as Window & { dashboardPerformance: { lcp: number, cls: number, tbt: number } }).dashboardPerformance),
      fcp: paints["first-contentful-paint"] || 0,
      domContentLoaded: Math.round(navigation.domContentLoadedEventEnd),
      load: Math.round(navigation.loadEventEnd),
    };
  });
  console.log(JSON.stringify({ elapsedMs, requestSpreadMs, moduleSpreadMs, render, refreshMutationRecords, vitals }));
  expect([...starts.keys()].sort()).toEqual(Object.keys(payloads).sort());
  expect(moduleStarts.size).toBeGreaterThanOrEqual(6);
  expect(requestSpreadMs).toBeLessThan(90);
  expect(moduleSpreadMs).toBeLessThan(90);
  expect(render.mutationRecords).toBeLessThanOrEqual(2);
  expect(refreshMutationRecords).toBe(0);
  expect(vitals.lcp).toBeLessThan(2500);
  expect(vitals.cls).toBeLessThan(0.1);
  expect(vitals.tbt).toBeLessThan(200);
});
