import { EventEmitter } from "node:events";
import { expect, test } from "@playwright/test";
import { openPaginationLibrary } from "./library-pagination-fixture";

const origin = process.env.KINOSAIL_LIBRARY_BROWSER_URL;
test.skip(!origin, "requires TestLibraryPaginationBrowserJourney disposable Go Server");
test.use({ serviceWorkers: "block" });

const showTitles = Array.from({ length: 30 }, (_, position) => `Pagination Show ${String(position + 1).padStart(2, "0")}`);
const movieTitles = Array.from({ length: 6 }, (_, position) => `Pagination Movie ${String(position + 1).padStart(2, "0")}`);

async function loadAll(page: import("@playwright/test").Page) {
	await expect.poll(async () => {
		if (await page.locator("[data-library-next]").count()) await page.locator("[data-library-status]").scrollIntoViewIfNeeded();
		return page.locator("[data-library-status]").textContent();
	}, { timeout: 20_000 }).toBe("All titles are loaded.");
}

for (const width of [390, 1440]) {
	test(`all Shows remain reachable across real Server pages at ${width}px`, async ({ page }, info) => {
		await page.setViewportSize({ width, height: 844 });
		const response = await page.request.get(`${origin}/api/v1/library?view=shows&limit=4`);
		expect(response.status()).toBe(200);
		const api = await response.json();
		expect(api.total).toBe(30);
		expect(api.items).toHaveLength(4);
		await openPaginationLibrary(page, `${origin}/?view=shows&limit=4`, info);
		const bundle = await page.locator('script[src^="/static/main.kinosail.bundle.js"]').getAttribute("src");
		expect(new URL(bundle!, origin).searchParams.get("v")).not.toBe("34-htmx4");
		await loadAll(page);
		expect((await page.locator("#library .show-details h2").allTextContents()).sort()).toEqual(showTitles);
		const links = await page.locator("#library .show-details").evaluateAll(elements => elements.map(element => element.getAttribute("href")));
		expect(new Set(links).size).toBe(30);
		for (const link of links) expect(link).toMatch(/^\/show\/[a-f0-9]{16}$/);
		expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
		await page.screenshot({ path: info.outputPath(`shows-loaded-${width}.png`), fullPage: true });
	});
}

test("real Server mixed pages keep both Show and Movie cards", async ({ page }, info) => {
	await openPaginationLibrary(page, `${origin}/?q=Pagination&limit=4`, info);
	await loadAll(page);
	expect((await page.locator('[data-library-group="shows"] .card h2').allTextContents()).sort()).toEqual(showTitles);
	expect((await page.locator('[data-library-group="movies"] .card h2').allTextContents()).sort()).toEqual(movieTitles);
	await expect(page.locator("#library .card")).toHaveCount(36);
	await page.screenshot({ path: info.outputPath("mixed-loaded.png"), fullPage: true });
});

test("Library remains usable while a decorative image is pending", async ({page}, info) => {
  test.setTimeout(12_000);
  let release: () => void = () => {};
  const gate = new Promise<void>(resolve => {release = resolve;});
  let held = 0;
  await page.route("**/static/cinema-backdrop.jpg*", async route => {
    const response = await route.fetch();
    expect(response.status()).toBe(200);
    held++;
    await gate;
    await route.fulfill({response});
  });
  await page.addInitScript(() => document.addEventListener("DOMContentLoaded", () => {
    const image = document.createElement("img");
    image.alt = ""; image.src = "/static/cinema-backdrop.jpg?pagination-test";
    document.body.append(image);
  }, {once:true}));
  try {
    await openPaginationLibrary(page, origin + "/?view=shows&limit=4", info);
    await expect.poll(() => held).toBeGreaterThan(0);
    await loadAll(page);
    expect((await page.locator("#library .show-details h2").allTextContents()).sort()).toEqual(showTitles);
    expect(await page.evaluate(() => document.readyState)).not.toBe("complete");
    await info.attach("pending-image-library-result", {contentType:"application/json", body:JSON.stringify({held, titles:showTitles.length, fixture:"real Server image response gated after HTTP200; all real catalog pages loaded"})});
  } finally {release();}
});

test("Navigation failure diagnostics remain bounded with a stalled renderer", async ({}, info) => {
  const original = new Error("Synthetic navigation timeout");
  const page = new EventEmitter() as EventEmitter & {goto:()=>Promise<never>; evaluate:()=>Promise<never>; url:()=>string};
  page.goto = async () => {throw original;};
  page.evaluate = () => new Promise(()=>{});
  page.url = () => "http://localhost:39060/login?token=synthetic-private";
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    await expect(Promise.race([
      openPaginationLibrary(page as unknown as import("@playwright/test").Page, "http://localhost:39060/", info),
      new Promise((_, reject)=>{timer=setTimeout(()=>reject(new Error("Unbounded diagnostic")),1100);}),
    ])).rejects.toBe(original);
    expect(page.listenerCount("request")).toBe(0);
    expect(page.listenerCount("requestfailed")).toBe(0);
  } finally {clearTimeout(timer);}
});
