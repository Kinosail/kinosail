import { readFile } from "node:fs/promises";
import { expect, test, type Page } from "@playwright/test";

const script = await readFile(new URL("../internal/server/static/subtitle-status.js", import.meta.url), "utf8");
const origin = "http://localhost:38128";
const escape = (value: string) => value.replaceAll("&", "&amp;").replaceAll('"', "&quot;").replaceAll("<", "&lt;");
function fixture(url: URL) {
  return `<!doctype html><html lang="en"><head><title>Library</title><meta name="kinosail-csrf" content="fixture"></head><body class="subtitle-dashboard"><main id="main" data-view="library" data-error-label="Could not load" data-retry-label="Reload view" data-action-error="Could not confirm" data-action-done="Completed"><div id="subtitle-feedback" role="status" hidden></div><div id="subtitle-content" data-total="2" data-ready="0" data-wanted="0" data-pending="2" data-unavailable="0"><div id="subtitle-update" hidden>Library updated <a href="?view=library" data-subtitle-refresh>Refresh</a></div><h1 id="subtitle-list-title" tabindex="-1">Page ${url.searchParams.get("page") === "2" ? "2" : "1"}</h1><form data-subtitle-search><input type="hidden" name="view" value="library"><label>Search<input type="search" id="subtitle-search" name="q" value="${escape(url.searchParams.get("q") || "")}"></label><button>Search</button></form><details class="subtitle-file" id="file-one"><summary>Arrival</summary>File history<form data-subtitle-action data-api="/api/v1/subtitle-library/0000000000000001/fetch"><button>Find subtitles</button></form></details><a href="?view=library&page=2" data-subtitle-page>Next</a></div></main><script src="/static/subtitle-status.js"></script></body></html>`;
}
async function libraryPage(page: Page, pending: unknown = 0) {
  const requests = { pages: 0, checks: 0, writes: 0, failPage: false, failWrite: false };
  await page.clock.install();
  await page.route(`${origin}/**`, async route => {
    const url = new URL(route.request().url());
    if (url.pathname === "/static/subtitle-status.js") return route.fulfill({ contentType: "text/javascript", body: script });
    if (route.request().method() === "POST") {
      requests.writes++;
      return route.fulfill({ status: requests.failWrite ? 500 : 204 });
    }
    if (url.pathname === "/api/v1/subtitle-library") {
      requests.checks++;
      return route.fulfill({ json: { total: 2, ready: 2, wanted: 0, pending, unavailable: 0 } });
    }
    requests.pages++;
    return route.fulfill({ status: requests.failPage ? 503 : 200, contentType: "text/html", body: fixture(url) });
  });
  await page.goto(`${origin}/?view=library`);
  return requests;
}

test("background completion offers refresh without closing file details", async ({ page }) => {
  const requests = await libraryPage(page);
  await page.locator("summary").click();
  await page.clock.fastForward(15000);
  await expect(page.getByText("Library updated")).toBeVisible();
  expect(requests.pages).toBe(1);
  await expect(page.locator("details")).toHaveAttribute("open", "");
  await page.getByRole("link", { name: "Refresh", exact: true }).click();
  await expect.poll(() => requests.pages).toBe(2);
  await expect(page.locator("details")).toHaveAttribute("open", "");
});

test("live search retains the query and paging supports browser back", async ({ page }) => {
  await libraryPage(page);
  await page.getByLabel("Search", { exact: true }).fill("Arrival");
  await page.clock.fastForward(350);
  await expect(page).toHaveURL(/q=Arrival/);
  await expect(page.getByLabel("Search", { exact: true })).toBeFocused();
  await page.getByRole("link", { name: "Next", exact: true }).click();
  await expect(page.getByRole("heading")).toHaveText("Page 2");
  await page.goBack();
  await expect(page.getByLabel("Search", { exact: true })).toHaveValue("Arrival");
});

test("failed navigation preserves the current view and exposes a retry", async ({ page }) => {
  const requests = await libraryPage(page);
  requests.failPage = true;
  await page.getByRole("link", { name: "Next", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Could not load");
  await expect(page.getByRole("heading")).toHaveText("Page 1");
  requests.failPage = false;
  await page.getByRole("link", { name: "Reload view" }).click();
  await expect(page.getByRole("heading")).toHaveText("Page 2");
});

test("actions keep details open and a failed write is never retried automatically", async ({ page }) => {
  const requests = await libraryPage(page);
  await page.locator("summary").click();
  await page.getByRole("button", { name: "Find subtitles" }).click();
  await expect(page.getByRole("status")).toHaveText("Completed");
  await expect(page.locator("details")).toHaveAttribute("open", "");
  requests.failWrite = true;
  await page.getByRole("button", { name: "Find subtitles" }).click();
  await expect(page.getByRole("status")).toContainText("Could not confirm");
  await page.getByRole("link", { name: "Reload view" }).click();
  await page.clock.fastForward(15000);
  expect(requests.writes).toBe(2);
});

for (const pending of [null, "0", -1, 0.5]) {
  test(`malformed coverage ${JSON.stringify(pending)} cannot trigger an update`, async ({ page }) => {
    const requests = await libraryPage(page, pending);
    await page.clock.fastForward(15000);
    await expect.poll(() => requests.checks).toBe(1);
    expect(requests.pages).toBe(1);
    await expect(page.locator("#subtitle-update")).toBeHidden();
  });
}
