import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { readFile, readdir } from "node:fs/promises";
import { join, relative } from "node:path";
import { expect, request as playwrightRequest, test, type APIRequestContext, type Page } from "@playwright/test";
import { login } from "./subtitle-dashboard-helpers";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires an isolated populated Subtitles instance");
test.setTimeout(180_000);

type Route = { method: string; pattern: string; path: string };

async function removedRoutes(): Promise<Route[]> {
  const source = await readFile(new URL("../internal/server/testdata/removed-routes.txt", import.meta.url), "utf8");
  return source.split("\n").map((line) => line.trim()).filter(Boolean).map((line) => {
    const [method, pattern] = line.split(" ");
    const path = pattern.replace(/\{[^}]+\}/g, (wildcard) => wildcard.endsWith("...}") ? "removed-probe/x" : "removed-probe");
    return { method, pattern, path };
  });
}

async function mediaChecksum(root: string): Promise<string> {
  const hash = createHash("sha256");
  async function walk(directory: string): Promise<void> {
    const entries = (await readdir(directory, { withFileTypes: true })).sort((left, right) => left.name.localeCompare(right.name));
    for (const entry of entries) {
      const path = join(directory, entry.name);
      if (entry.isDirectory()) await walk(path);
      else if (entry.isFile()) hash.update(`${relative(root, path)}\0`).update(createHash("sha256").update(await readFile(path)).digest()).update("\n");
    }
  }
  await walk(root);
  return hash.digest("hex");
}

async function ownerState(page: Page): Promise<string> {
  const library = await page.request.get("/api/v1/subtitle-library?view=library");
  const settings = await page.request.get("/api/v1/settings");
  expect(library.status()).toBe(200);
  expect(settings.status()).toBe(200);
  return createHash("sha256").update(await library.text()).update("\0").update(await settings.text()).digest("hex");
}

async function probe(context: APIRequestContext, route: Route, csrf: string): Promise<number> {
  const response = await context.fetch(route.path, {
    method: route.method,
    headers: { "Content-Type": "application/json", Accept: "application/json", ...(csrf ? { "X-Kinosail-CSRF": csrf } : {}) },
    data: route.method === "GET" || route.method === "HEAD" ? undefined : "{}",
    maxRedirects: 0,
  });
  return response.status();
}

test("removed Player routes return 404 without changing Subtitles state @smoke", async ({ page, baseURL }, info) => {
  const media = process.env.KINOSAIL_E2E_MEDIA_DIR ?? (process.env.KINOSAIL_TEST_ROOT ? join(process.env.KINOSAIL_TEST_ROOT, "media") : undefined);
  expect(media, "KINOSAIL_E2E_MEDIA_DIR or KINOSAIL_TEST_ROOT names the mounted media tree").toBeTruthy();
  const routes = await removedRoutes();
  expect(routes.length).toBeGreaterThan(0);
  await login(page);
  await page.goto("/settings");
  const csrf = await page.locator('meta[name="kinosail-csrf"]').getAttribute("content") ?? "";
  expect(csrf).not.toBe("");
  const beforeState = await ownerState(page);
  const beforeMedia = await mediaChecksum(media!);
  const anonymous = await playwrightRequest.newContext({ baseURL, ignoreHTTPSErrors: true });
  const results: Array<{ route: string; owner: number; anonymous: number }> = [];
  try {
    for (const route of routes) {
      results.push({ route: `${route.method} ${route.pattern}`, owner: await probe(page.request, route, csrf), anonymous: await probe(anonymous, route, "") });
    }
  } finally {
    await anonymous.dispose();
  }
  const afterState = await ownerState(page);
  const afterMedia = await mediaChecksum(media!);
  await info.attach("removed-routes", { contentType: "application/json", body: JSON.stringify({
    revision: execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim(),
    sourceState: execFileSync("git", ["status", "--porcelain", "--untracked-files=no"], { encoding: "utf8" }).trim() ? "working tree" : "committed",
    command: "playwright test removed-routes.spec.ts", data: "Isolated Owner and generated CC0 library",
    environment: { platform: process.platform, browser: info.project.name },
    state: { before: beforeState, after: afterState }, media: { before: beforeMedia, after: afterMedia }, results,
  }, null, 2) });
  expect(results.filter((result) => result.owner !== 404 || result.anonymous !== 404)).toEqual([]);
  expect(afterState).toBe(beforeState);
  expect(afterMedia).toBe(beforeMedia);
});
