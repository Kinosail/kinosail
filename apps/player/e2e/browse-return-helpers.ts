import { createHash } from "node:crypto";
import { expect, type Page, type TestInfo } from "@playwright/test";

export const origin = process.env.KINOSAIL_BROWSE_RETURN_URL;
export const moviePath = "/?view=movies&q=Return+Movie&sort=title&offset=4&limit=4";
export const movieTitles = Array.from({ length: 28 }, (_, position) => `Return Movie ${String(position + 5).padStart(2, "0")}`);
export const showTitles = Array.from({ length: 12 }, (_, position) => `Return Show ${String(position + 1).padStart(2, "0")}`);

type Observation = {
  document: string; shows: { persisted: boolean }[]; histories: number;
};
declare global {
  interface Window { browseReturnObservation: Observation }
}

export async function observe(page: Page) {
  await page.addInitScript(() => {
    const observation = { document: crypto.randomUUID(), shows: [] as { persisted: boolean }[], histories: 0 };
    window.browseReturnObservation = observation;
    window.addEventListener("pageshow", event => observation.shows.push({ persisted: event.persisted }));
    document.addEventListener("htmx:before:history:restore", () => observation.histories++);
  });
}

export async function loadAll(page: Page, expected: string[]) {
  await expect.poll(async () => {
    if (await page.locator("[data-library-next]").count()) await page.locator("[data-library-status]").scrollIntoViewIfNeeded();
    return page.locator("[data-library-status]").textContent();
  }, { timeout: 10_000, message: "fixture prerequisite: complete real pagination" }).toBe("All titles are loaded.");
  expect(await page.locator("#library .card h2").allTextContents(), "fixture prerequisite: independent title order").toEqual(expected);
  const hrefs = await page.locator("#library .card").evaluateAll(cards => cards.map(card => card.matches("a") ? card.getAttribute("href") : card.querySelector("a.show-details")?.getAttribute("href")));
  expect(new Set(hrefs).size, "fixture prerequisite: distinct canonical cards").toBe(expected.length);
  for (const href of hrefs) expect(href).toMatch(/^\/(watch|show)\/[a-f0-9]{16}$/);
}

export async function snapshot(page: Page, selected: string) {
  return page.evaluate(href => {
    const selectedLink = [...document.querySelectorAll<HTMLAnchorElement>("a[href]")].find(link => link.getAttribute("href") === href);
    const rect = selectedLink?.getBoundingClientRect();
    return {
      path: location.pathname, values: Object.fromEntries(new URLSearchParams(location.search)),
      profile: document.querySelector<HTMLElement>("[data-nav-profile]")?.dataset.navProfile || document.body.dataset.viewerProfile,
      document: window.browseReturnObservation,
      navigation: performance.getEntriesByType("navigation").map(entry => (entry as PerformanceNavigationTiming).type),
      titles: [...document.querySelectorAll("#library .card h2")].map(element => element.textContent),
      hrefs: [...document.querySelectorAll("#library .card")].map(card => card.matches("a") ? card.getAttribute("href") : card.querySelector("a.show-details")?.getAttribute("href")),
      focused: document.activeElement?.closest("a")?.getAttribute("href") || null,
      scroll: { x: scrollX, y: scrollY }, selected: selectedLink ? { href, top: rect!.top, bottom: rect!.bottom } : null,
    };
  }, selected);
}
export type Snapshot = Awaited<ReturnType<typeof snapshot>>;

export async function record(page: Page, info: TestInfo, stage: string, selected: string) {
  const state = await snapshot(page, selected);
  const peer = await (await page.request.get(`${origin}/__browse-return`)).json();
  await info.attach(`${stage}-state`, { body: JSON.stringify({ state, peer }, null, 2), contentType: "application/json" });
  await page.screenshot({ path: info.outputPath(`${stage}.png`) });
  return { state, peer: peer as { values: Record<string, string>; continuation: boolean; history: boolean; htmx: boolean }[] };
}

export async function assetReceipt(page: Page, info: TestInfo) {
  const src = await page.locator('script[src^="/static/main.kinosail.bundle.js"]').getAttribute("src");
  expect(src, "fixture prerequisite: Go-served browse asset").toMatch(/^\/static\/main\.kinosail\.bundle\.js\?v=/);
  const response = await page.request.get(`${origin}${src}`);
  expect(response.status()).toBe(200);
  const body = await response.body();
  await info.attach("served-browse-asset", { body: JSON.stringify({ src, bytes: body.length, sha256: createHash("sha256").update(body).digest("hex") }), contentType: "application/json" });
}

// Observe native smooth scrolling through three equal real geometry samples.
// No CSS, scrolling implementation, storage, app state, or lifecycle is mocked.
export async function settle(page: Page, selected: string) {
  let previous = "", equal = 0;
  await expect.poll(async () => {
    const state = await snapshot(page, selected);
    const geometry = JSON.stringify({ y: state.scroll.y, selected: state.selected });
    equal = geometry === previous ? equal + 1 : 0;
    previous = geometry;
    return equal;
  }, { timeout: 2500, intervals: [50, 100, 100], message: "fixture prerequisite: native scroll settles" }).toBeGreaterThanOrEqual(2);
}

export async function selectMovie(page: Page, info: TestInfo, path = moviePath) {
  const api = await page.request.get(`${origin}/api/v1/library${path.slice(1)}`);
  expect(api.status(), "fixture prerequisite: public catalog available").toBe(200);
  const data = await api.json();
  expect(data).toMatchObject({ total: 32, offset: 4, limit: 4, view: "movies", sort: "title", query: "Return Movie" });
  expect(data.items.map((item: { title: string }) => item.title)).toEqual(movieTitles.slice(0, 4));
  const rendered = await page.goto(`${origin}${path}`);
  expect(rendered?.status(), "fixture prerequisite: real Go browse HTML").toBe(200);
  await assetReceipt(page, info);
  await loadAll(page, movieTitles);
  const link = page.locator("#library a.card").filter({ has: page.getByRole("heading", { name: "Return Movie 25", exact: true }) });
  const href = (await link.getAttribute("href"))!;
  expect(href).toMatch(/^\/watch\/[a-f0-9]{16}$/);
  await link.scrollIntoViewIfNeeded();
  await link.focus();
  await settle(page, href);
  const before = await record(page, info, "before", href);
  expect(before.state.profile, "fixture prerequisite: current real Viewer Profile").toBe("local-owner");
  expect(before.state.scroll.y, "fixture prerequisite: a later scrolled card").toBeGreaterThan(100);
  await link.click();
  await expect(page).toHaveURL(`${origin}${href}`);
  await expect(page.locator("body.player-page"), "fixture prerequisite: real Go Player").toBeVisible();
  expect(await page.locator("body").getAttribute("data-viewer-profile")).toBe("local-owner");
  await record(page, info, "player", href);
  return { href, before };
}

export async function assertReturn(page: Page, before: Snapshot, href: string, expectedTitles = movieTitles) {
  await expect.poll(async () => {
    const state = await snapshot(page, href);
    return { path: state.path, values: state.values };
  }, { timeout: 4000, message: "Q14 acceptance: return to the same public browse URL" }).toEqual({ path: before.path, values: before.values });
  await expect(page.locator("#library .card h2")).toHaveText(expectedTitles, { timeout: 5000 });
  const hrefs = await page.locator("#library .card").evaluateAll(cards => cards.map(card => card.matches("a") ? card.getAttribute("href") : card.querySelector("a.show-details")?.getAttribute("href")));
  expect(new Set(hrefs).size).toBe(expectedTitles.length);
  await expect.poll(async () => (await snapshot(page, href)).focused, { timeout: 4000, message: "Q14 acceptance: selected title action regains focus" }).toBe(href);
  await settle(page, href);
  const state = await snapshot(page, href);
  expect(state.profile).toBe(before.profile);
  expect(Math.abs(state.scroll.y - before.scroll.y), "Q14 acceptance: same settled browse position").toBeLessThanOrEqual(2);
  expect(state.selected).not.toBeNull();
}
