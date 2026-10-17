import {expect, test, type Page, type Request} from "@playwright/test";

const origin = process.env.KINOSAIL_HISTORY_BROWSER_URL;
const phase = process.env.KINOSAIL_HISTORY_PHASE;
test.skip(!origin, "requires TestLibraryHistoryBrowserJourney disposable Go Server");
test.use({serviceWorkers: "block"});

const newestFirst = Array.from({length: 9}, (_, position) => `History Movie ${String(9 - position).padStart(2, "0")}`);
const sizes = [{name: "phone", width: 390, height: 844}, {name: "desktop", width: 1440, height: 900}, {name: "tv", width: 1920, height: 1080}];
const continuation = (request: Request) => request.headers()["x-kinosail-library-page"] === "1";

async function observeLayoutShift(page: Page) {
  await page.addInitScript(() => {
    const record = window as unknown as {historyLayoutShifts: object[]};
    record.historyLayoutShifts = [];
    new PerformanceObserver(list => {
      for (const entry of list.getEntries() as unknown as {value: number, hadRecentInput: boolean, sources: {node?: Node | null}[]}[]) {
        if (entry.hadRecentInput) continue;
        const library = document.querySelector("#library");
        for (const source of entry.sources) {
          const node = source.node;
          const below = Boolean(library && node && !library.contains(node) && library.compareDocumentPosition(node) & Node.DOCUMENT_POSITION_FOLLOWING);
          if (!below) record.historyLayoutShifts.push({value: entry.value, node: node instanceof Element ? `${node.tagName}.${node.className}` : String(node?.nodeName)});
        }
      }
    }).observe({type: "layout-shift", buffered: true});
  });
}

async function catalogAnchors(page: Page) {
  await page.evaluate(() => document.fonts.ready);
  return page.evaluate(() => [document.querySelector('select[name="sort"]'), document.querySelector("#library .card")].map(element => {
    const rect = element!.getBoundingClientRect();
    return [Math.round(rect.left), Math.round(rect.top + scrollY), Math.round(rect.width), Math.round(rect.height)];
  }));
}

async function expectSortMenu(page: Page) {
  const sort = page.locator('select[name="sort"]').last();
  await expect(sort).toHaveValue("title");
  await expect(sort.locator("option:checked")).toHaveText("Recently played");
  await expect(sort.locator("option")).toHaveText(["Recently played", "Newest", "Year"]);
  await expect(page.locator("[data-title-jump]")).toHaveCount(0);
  await page.evaluate(() => document.fonts.ready);
  const fit = await sort.evaluate((select: HTMLSelectElement) => {
    const style = getComputedStyle(select);
    const canvas = document.createElement("canvas").getContext("2d")!;
    canvas.font = `${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
    const rem = parseFloat(getComputedStyle(document.documentElement).fontSize);
    const box = select.getBoundingClientRect();
    const textEnd = parseFloat(style.paddingLeft) + canvas.measureText(select.selectedOptions[0].text).width;
    const count = select.form!.querySelector(":scope > span")!.getBoundingClientRect();
    const label = select.closest("label")!.getBoundingClientRect();
    return {textEnd, chevronStart: box.width - 1.4 * rem, countEnd: count.width ? count.right : -Infinity, labelStart: label.left};
  });
  expect(fit.textEnd, JSON.stringify(fit)).toBeLessThanOrEqual(fit.chevronStart - 4);
  expect(fit.countEnd, JSON.stringify(fit)).toBeLessThanOrEqual(fit.labelStart);
}

async function cardTitles(page: Page) {
  return page.locator("#library .card h2").allTextContents();
}

test.describe("empty history", () => {
  test.skip(phase !== "empty", "runs before the journey saves playback progress");
  for (const size of sizes) {
    test(`shows the empty state without pending or jump controls on ${size.name}`, async ({page}, info) => {
      await page.setViewportSize(size);
      const response = await page.goto(`${origin}/?view=history&limit=4`);
      expect(response?.status()).toBe(200);
      await expect(page.getByRole("heading", {name: "Nothing played yet."})).toBeVisible();
      await expect(page.getByText("Start a title and its progress will appear here.")).toBeVisible();
      await expect(page.locator("#library .card")).toHaveCount(0);
      await expect(page.locator("[data-library-pagination]")).toHaveCount(0);
      await expectSortMenu(page);
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(size.width);
      const api = await (await page.request.get(`${origin}/api/v1/library?view=history&sort=title&limit=24`)).json();
      expect(api).toMatchObject({items: [], total: 0, sort: "title", letters: null});
      await page.screenshot({path: info.outputPath(`history-empty-${size.name}.png`), fullPage: true});
    });
  }
});

test.describe("loaded history", () => {
  test.skip(phase !== "loaded", "runs after the journey saves playback progress");
  test("versioned library history pages list the newest playback first", async ({request}) => {
    const first = await (await request.get(`${origin}/api/v1/library?view=history&sort=title&offset=0&limit=4`)).json();
    expect(first.items.map((item: {title: string}) => item.title)).toEqual(newestFirst.slice(0, 4));
    expect(first).toMatchObject({total: 9, sort: "title", letters: null});
    const second = await (await request.get(`${origin}/api/v1/library?view=history&sort=title&offset=4&limit=4`)).json();
    expect(second.items.map((item: {title: string}) => item.title)).toEqual(newestFirst.slice(4, 8));
    const direct = await (await request.get(`${origin}/api/v1/history?limit=9`)).json();
    expect(direct.items.map((item: {title: string}) => item.title)).toEqual(newestFirst);
  });

  for (const size of sizes) {
    test(`loads every history page newest first with stable layout and focus on ${size.name}`, async ({page}, info) => {
      await page.setViewportSize(size);
      await observeLayoutShift(page);
      let release!: () => void;
      const held = new Promise<void>(resolve => {release = resolve;});
      let pending = 0;
      await page.route(`${origin}/?**`, async route => {
        if (!continuation(route.request())) return route.continue();
        pending++;
        if (pending === 1) await held;
        if (pending === 2) return route.fulfill({status: 503, body: "Unavailable"});
        return route.continue();
      });
      const response = await page.goto(`${origin}/?view=history&limit=4`);
      expect(response?.status()).toBe(200);
      expect(await cardTitles(page)).toEqual(newestFirst.slice(0, 4));
      await expectSortMenu(page);
      const anchors = await catalogAnchors(page);

      const navigation = page.locator("[data-library-pagination]");
      const status = page.locator("[data-library-status]");
      await status.scrollIntoViewIfNeeded();
      await expect(navigation).toHaveAttribute("aria-busy", "true");
      await expect(status).toHaveText("Loading more titles…");
      await page.screenshot({path: info.outputPath(`history-pending-${size.name}.png`)});
      release();
      await expect(navigation).not.toHaveAttribute("aria-busy", "true");
      await expect.poll(() => cardTitles(page)).toEqual(newestFirst.slice(0, 8));

      await status.scrollIntoViewIfNeeded();
      const retry = page.getByRole("link", {name: "Retry loading"});
      await expect(retry).toBeVisible();
      await expect(status).toHaveAttribute("data-failure", "http");
      await expect(status).toHaveText("Could not load more titles.");
      await expect(navigation).not.toHaveAttribute("aria-busy", "true");
      expect(await cardTitles(page)).toEqual(newestFirst.slice(0, 8));
      await page.screenshot({path: info.outputPath(`history-failed-${size.name}.png`)});
      await retry.focus();
      await expect(retry).toBeFocused();
      await page.keyboard.press("Enter");
      await expect(status).toHaveText("All titles are loaded.");
      expect(await cardTitles(page)).toEqual(newestFirst);
      await expect(status).not.toHaveAttribute("data-failure");
      await expect(page.locator("[data-title-jump]")).toHaveCount(0);
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(size.width);
      expect(await page.evaluate(() => (window as unknown as {historyLayoutShifts: object[]}).historyLayoutShifts)).toEqual([]);
      await page.evaluate(() => scrollTo(0, 0));
      await page.waitForFunction(() => scrollY === 0);
      expect(await catalogAnchors(page)).toEqual(anchors);
      await page.screenshot({path: info.outputPath(`history-loaded-${size.name}.png`), fullPage: true});

      const cards = page.locator("#library a.card");
      await cards.first().focus();
      await expect(cards.first()).toBeFocused();
      await page.keyboard.press("Tab");
      await expect(cards.nth(1)).toBeFocused();
      const ring = await cards.nth(1).evaluate(element => {
        const outlined = (target: Element | null) => {
          if (!target) return false;
          const style = getComputedStyle(target);
          return style.outlineStyle !== "none" && parseFloat(style.outlineWidth) >= 2 && style.outlineColor !== "rgba(0, 0, 0, 0)";
        };
        return outlined(element) || outlined(element.querySelector(".poster"));
      });
      expect(ring).toBe(true);
      const destination = await cards.nth(1).getAttribute("href");
      expect(destination).toMatch(/^\/watch\/[a-f0-9]{16}$/);
      await page.keyboard.press("Enter");
      await expect(page).toHaveURL(`${origin}${destination}`);
      await info.attach("history-browser-journey", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION, browser: info.project.name,
        size, order: newestFirst, continuationRequests: pending, result: "passed"}), contentType: "application/json"});
      await page.unrouteAll({behavior: "wait"});
    });
  }
});
