import { readFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { expect, test, type Page, type TestInfo } from "@playwright/test";

const fixtureRoot = process.env.KINOSAIL_SUBTITLE_ACTION_FIXTURE_DIR;
const revision = process.env.KINOSAIL_TEST_REVISION ?? execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim();
const origin = "http://subtitle-actions.test";
test.skip(!fixtureRoot, "requires real public Save/Restore control artifacts");

async function actionFixture(page: Page, testInfo: TestInfo, action: "save" | "restore", surface: "inspector" | "dashboard", completed: boolean, stalledRead?: "status" | "draft") {
  const dir = `${fixtureRoot}/${action}`;
  const source = await readFile(`${dir}/${surface}.html`, "utf8");
  const historyPage = await readFile(`${dir}/history.html`, "utf8");
  const before = JSON.parse(await readFile(`${dir}/before.json`, "utf8"));
  const after = JSON.parse(await readFile(`${dir}/after.json`, "utf8"));
  const history = JSON.parse(await readFile(`${dir}/${completed ? "history" : "before-history"}.json`, "utf8"));
  const preview = action === "save" ? JSON.parse(await readFile(`${dir}/preview.json`, "utf8")) : null;
  const release: Array<() => void> = [];
  const requests = { preparations: [] as unknown[], mutations: [] as Array<{ path: string; input: unknown }>, statuses: 0, drafts: 0, inspections: 0, histories: 0, pages: 0 };
  const operation = { id: "a".repeat(64), action: action === "save" ? "apply" : "restore", item: before.id, state: "prepared", statusURL: `/api/v1/subtitle-operations/${"a".repeat(64)}` };
  const scripts = { inspector: await readFile(`${dir}/subtitle-inspector.js`), dashboard: await readFile(`${dir}/subtitle-status.js`) };
  await testInfo.attach("verification-context", { contentType: "application/json", body: JSON.stringify({ revision, action, surface, completed, width: page.viewportSize()?.width, command: "playwright test subtitle-action-recovery.spec.ts --workers=1", browserVersion: page.context().browser()?.version(), boundary: "isolated browser transport using actual public Go HTML/assets/Save/Restore reads; prepared/running/completed receipts are simulated at the transport boundary; POST completion is represented by captured reads, not executed in this browser fixture; media aborted", sourceSHA256: createHash("sha256").update(scripts[surface]).digest("hex"), beforeFingerprint: before.fingerprint, afterFingerprint: after.fingerprint }) });
  await page.clock.install();
  await page.route(`${origin}/**`, async route => {
    const request = route.request(), url = new URL(request.url()), path = url.pathname;
    if (path === "/api/v1/subtitle-operations") {
      expect(request.method()).toBe("POST");
      expect(request.postDataJSON()).toEqual({ action: operation.action, item: operation.item });
      requests.preparations.push(request.postDataJSON());
      return route.fulfill({ status: 201, json: operation });
    }
    if (path === operation.statusURL) {
      expect(request.method()).toBe("GET");
      requests.statuses++;
      if (stalledRead === "status") {
        await new Promise<void>(resolve => release.push(resolve));
        return route.abort().catch(() => {});
      }
      return route.fulfill({ json: completed ? { ...operation, state: "completed", outcome: "success", status: action === "save" ? 200 : 204 } : { ...operation, state: "running" } });
    }
    if (path.endsWith("/preview")) {
      expect(request.method()).toBe("POST");
      return route.fulfill({ json: preview });
    }
    if (request.method() === "POST") {
      expect(request.headers()["x-kinosail-operation"]).toBe(operation.id);
      requests.mutations.push({ path, input: request.postDataJSON() });
      await new Promise<void>(resolve => release.push(resolve));
      return route.abort().catch(() => {});
    }
    if (path.endsWith("/inspect")) {
      requests.inspections++;
      return route.fulfill({ json: completed && requests.mutations.length ? after : before });
    }
    if (path.endsWith("/draft")) {
      requests.drafts++;
      if (stalledRead === "draft") {
        await new Promise<void>(resolve => release.push(resolve));
        return route.abort().catch(() => {});
      }
      return route.fulfill({ json: { state: "none", message: "No draft", words: [] } });
    }
    if (path === "/api/v1/subtitle-library") {
      if (url.searchParams.get("view") === "history") { requests.histories++; return route.fulfill({ json: history }); }
      return route.fulfill({ json: { total: 1, ready: 1, wanted: 0, pending: 0, unavailable: 0 } });
    }
    if (path.startsWith("/media/")) return route.abort();
    const types: Record<string, string> = { "app.css": "text/css", "subtitle-inspector.css": "text/css", "subtitle-inspector.js": "text/javascript", "subtitle-status.js": "text/javascript", "theme.js": "text/javascript", "icon.svg": "image/svg+xml", "manrope.woff2": "font/woff2" };
    const name = path.split("/").pop() || "";
    if (types[name]) return route.fulfill({ contentType: types[name], body: await readFile(`${dir}/${name}`) });
    expect(request.method(), "recovery and navigation may only read").toBe("GET");
    requests.pages++;
    return route.fulfill({ contentType: "text/html", body: surface === "dashboard" && url.searchParams.get("view") === "history" ? historyPage : source });
  });
  await page.goto(`${origin}/${surface === "inspector" ? "subtitles/inspect/fixture" : "?view=library"}`);
  return {
    before, after, requests,
    async finish() {
      await testInfo.attach("action-requests", { contentType: "application/json", body: JSON.stringify(requests) });
      release.forEach(resolve => resolve());
    },
  };
}

async function beginInspectorAction(page: Page, action: "save" | "restore") {
  await expect(page.locator('#subtitle-edit-form [name="language"]')).toBeEnabled();
  if (action === "save") {
    await page.getByText("Timing anchors and cleanup", { exact: true }).click();
    await page.locator('[name="offset"]').fill("0.5");
    await page.getByRole("button", { name: "Preview changes", exact: true }).click();
    await expect(page.locator("#apply-subtitle")).toBeEnabled();
    await page.locator("#apply-subtitle").click();
  } else await page.locator("#restore-subtitle").click();
  await expect(page.locator('#subtitle-edit-form [name="language"]')).toBeDisabled();
}

for (const width of [390, 1440]) {
  for (const action of ["save", "restore"] as const) {
    test(`lost completed ${action} response recovers without replay at ${width}px`, { tag: "@smoke" }, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 900 });
      const fixture = await actionFixture(page, testInfo, action, "inspector", true);
      try {
        await beginInspectorAction(page, action);
        await expect.poll(() => fixture.requests.mutations.length).toBe(1);
        await page.clock.fastForward(45_000);
        await expect(page.locator('#subtitle-edit-form [name="language"]'), "a lost response must not lock the editor indefinitely").toBeEnabled({ timeout: 1500 });
        await expect.poll(() => fixture.requests.inspections, { timeout: 1500 }).toBeGreaterThan(1);
        await expect.poll(() => fixture.requests.histories, { timeout: 1500 }).toBeGreaterThan(0);
        await expect(page.locator("#inspector-status")).toContainText(action === "save" ? /saved|completed|installed/i : /restored|completed/i);
        await expect(page.locator("#apply-subtitle")).toBeDisabled();
        await page.clock.fastForward(60_000);
        expect(fixture.requests.mutations, "the completed mutation must never be retried automatically").toHaveLength(1);
        expect(fixture.requests.preparations).toHaveLength(1);
        expect(fixture.requests.statuses).toBeGreaterThan(0);
        await page.screenshot({ path: testInfo.outputPath(`${action}-reconciled-${width}.png`), fullPage: true });
      } finally { await fixture.finish(); }
    });
  }

  test(`dashboard restore response deadline preserves safe navigation at ${width}px`, { tag: "@smoke" }, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 });
    const fixture = await actionFixture(page, testInfo, "restore", "dashboard", false);
    try {
      await page.locator(".subtitle-file > summary").click();
      const form = page.locator('[data-subtitle-action][data-api$="/restore"]');
      await form.getByRole("button", { name: "Restore previous subtitle", exact: true }).click();
      await expect(form).toHaveAttribute("aria-busy", "true");
      await expect.poll(() => fixture.requests.mutations.length).toBe(1);
      await page.clock.fastForward(45_000);
      await expect(form, "an uncertain write must stop occupying the global pending state").not.toHaveAttribute("aria-busy", "true", { timeout: 1500 });
      await expect(page.locator("#subtitle-feedback")).toContainText(/unknown|could not confirm|still|pending/i);
      await page.getByRole("link", { name: "History", exact: true }).click();
      await expect.poll(() => fixture.requests.pages).toBeGreaterThan(1);
      await expect(page.locator("#main")).toHaveAttribute("data-view", "history");
      await page.clock.fastForward(60_000);
      expect(fixture.requests.mutations, "read-only navigation must not replay Restore").toHaveLength(1);
      expect(fixture.requests.preparations).toHaveLength(1);
      await page.screenshot({ path: testInfo.outputPath(`restore-unknown-dashboard-${width}.png`), fullPage: true });
    } finally { await fixture.finish(); }
  });
}

for (const action of ["save", "restore"] as const) {
  test(`still pending ${action} permits safe editing but blocks another write`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width: 390, height: 900 });
    const fixture = await actionFixture(page, testInfo, action, "inspector", false);
    try {
      await beginInspectorAction(page, action);
      await expect.poll(() => fixture.requests.mutations.length).toBe(1);
      await page.clock.fastForward(45_000);
      await expect(page.locator('#subtitle-edit-form [name="language"]'), "safe editing must recover even when unchanged reads cannot confirm a write outcome").toBeEnabled({ timeout: 1500 });
      await expect(page.locator("#inspector-status")).toContainText(/unknown|could not confirm|still|pending/i);
      await expect(page.locator("#apply-subtitle")).toBeDisabled();
      await expect(page.locator("#restore-subtitle")).toBeDisabled();
      if (action === "save") await expect(page.locator('[name="offset"]')).toHaveValue("0.5");
      await page.clock.fastForward(60_000);
      expect(fixture.requests.mutations).toHaveLength(1);
      expect(fixture.requests.preparations).toHaveLength(1);
    } finally { await fixture.finish(); }
  });
}

for (const action of ["save", "restore"] as const) {
  test(`stalled ${action} reconciliation read reaches its own deadline without replay`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width: 390, height: 900 });
    const fixture = await actionFixture(page, testInfo, action, "inspector", false, "status");
    try {
      await beginInspectorAction(page, action);
      await expect.poll(() => fixture.requests.mutations.length).toBe(1);
      await page.clock.fastForward(30_000);
      await expect.poll(() => fixture.requests.statuses).toBeGreaterThan(0);
      await page.clock.fastForward(15_000);
      await expect(page.locator('#subtitle-edit-form [name="language"]'), "recovery GET must have a deadline too").toBeEnabled({ timeout: 1500 });
      await expect(page.locator("#inspector-status")).toContainText(/unknown|could not confirm|still|pending/i);
      await expect(page.locator("#apply-subtitle")).toBeDisabled();
      await expect(page.locator("#restore-subtitle")).toBeDisabled();
      expect(fixture.requests.mutations).toHaveLength(1);
      expect(fixture.requests.preparations).toHaveLength(1);
    } finally { await fixture.finish(); }
  });
}

test("stalled draft GET enters bounded backoff while editing stays usable", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 900 });
  const fixture = await actionFixture(page, testInfo, "save", "inspector", false, "draft");
  try {
    await expect(page.locator('#subtitle-edit-form [name="language"]')).toBeEnabled();
    await expect.poll(() => fixture.requests.drafts).toBe(1);
    await page.clock.fastForward(15_000);
    await expect(page.locator("#draft-status"), "a stalled read must reach the existing five-second backoff").toContainText(/unavailable.*Retrying in 5 seconds/i, { timeout: 1500 });
    await page.clock.fastForward(5_000);
    await expect.poll(() => fixture.requests.drafts).toBe(2);
    await expect(page.locator('#subtitle-edit-form [name="language"]')).toBeEnabled();
    expect(fixture.requests.mutations).toHaveLength(0);
    expect(fixture.requests.preparations).toHaveLength(0);
  } finally { await fixture.finish(); }
});
