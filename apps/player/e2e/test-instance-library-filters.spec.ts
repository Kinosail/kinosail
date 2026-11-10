import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { configureTestInstance, login } from "./test-instance-helpers";

configureTestInstance();
test.use({ serviceWorkers: "block" });

type Facet = { value: string; count: number };

test("library filters narrow movies, explain an empty result, and clear", { tag: "@smoke" }, async ({ page }, info) => {
  await login(page);
  const cards = page.locator('#library a.card[href^="/watch/"], #library .card > a[href^="/watch/"]');
  const titles = () => page.locator("#library .card h2").allTextContents();
  const toolbar = page.locator("form.browse-toolbar");
  const apply = () => Promise.all([page.waitForURL(url => url.searchParams.get("view") === "movies"), toolbar.getByRole("button", { name: "Apply", exact: true }).click()]);
  const fits = () => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth);

  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/?view=movies");
  const all = await titles();
  expect(all.length).toBeGreaterThan(1);
  const target = all[0];
  const id = (await cards.first().getAttribute("href"))!.split("/").pop()!;
  const listing = await (await page.request.get("/api/v1/library?view=movies")).json();
  const facets = listing.facets as { genres: Facet[]; decades: Facet[]; ratings: Facet[] };
  expect(listing.filters).toEqual({ genre: "", decade: "", rating: "", watched: "" });
  const mark = async (watched: boolean) => {
    const response = await page.request.put(`/api/v1/items/${id}/watched`, { headers: { "X-Kinosail-CSRF": await page.locator('meta[name="kinosail-csrf"]').getAttribute("content") || "", Origin: new URL(page.url()).origin }, data: { watched } });
    expect(response.status()).toBe(200);
  };

  try {
    await mark(true);
    for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
      await page.setViewportSize(viewport);
      await page.goto("/?view=movies");
      await expect(toolbar.getByLabel("Watched")).toBeVisible();
      await expect(toolbar.getByLabel("Genre")).toHaveCount(facets.genres.length ? 1 : 0);
      await expect(toolbar.getByLabel("Decade")).toHaveCount(facets.decades.length ? 1 : 0);
      await expect(toolbar.getByLabel("Rating")).toHaveCount(facets.ratings.length ? 1 : 0);
      await expect(toolbar.getByRole("link", { name: "Clear filters" })).toHaveCount(0);
      expect(await fits()).toBe(true);
      expect((await new AxeBuilder({ page }).include("form.browse-toolbar").analyze()).violations).toEqual([]);

      await toolbar.getByLabel("Watched").selectOption("watched");
      await apply();
      expect(new URL(page.url()).searchParams.get("watched")).toBe("watched");
      expect(await titles()).toEqual([target]);
      await expect(toolbar.locator("strong")).toHaveText("1");
      await expect(toolbar.getByLabel("Watched")).toHaveValue("watched");
      await expect(toolbar.getByRole("link", { name: "Clear filters" })).toBeVisible();
      expect(await fits()).toBe(true);
      expect((await new AxeBuilder({ page }).include("form.browse-toolbar").analyze()).violations).toEqual([]);
      await toolbar.screenshot({ path: info.outputPath(`${viewport.width}-filters.png`) });

      await toolbar.getByLabel("Watched").selectOption("unwatched");
      await apply();
      expect(await titles()).toEqual(all.filter(title => title !== target));

      await page.goto("/?view=movies&watched=watched&genre=No+Such+Genre");
      expect(await titles()).toEqual([]);
      const empty = page.locator("#library .empty");
      await expect(empty.getByRole("heading")).toHaveText("No titles match these filters.");
      expect(await fits()).toBe(true);
      await page.screenshot({ path: info.outputPath(`${viewport.width}-filters-empty.png`) });
      await empty.getByRole("link", { name: "Clear filters" }).click();
      await page.waitForURL(url => !url.searchParams.has("genre") && !url.searchParams.has("watched"));
      expect(await titles()).toEqual(all);
    }

    for (const [name, values] of [["decade", facets.decades], ["rating", facets.ratings], ["genre", facets.genres]] as const) {
      if (!values.length) continue;
      const narrowed = await (await page.request.get(`/api/v1/library?view=movies&${name}=${encodeURIComponent(values[0].value)}`)).json();
      expect(narrowed.total).toBe(values[0].count);
      expect(narrowed.filters[name]).toBe(values[0].value);
      expect(narrowed.facets).toEqual(facets);
    }
    const watched = await (await page.request.get("/api/v1/library?view=movies&watched=watched")).json();
    expect(watched.items.map((item: { title: string }) => item.title)).toEqual([target]);
    for (const query of ["view=movies&decade=1995", "view=movies&watched=yes", "view=history&watched=watched", "view=movies&genre=a&genre=b"]) {
      expect((await page.request.get(`/api/v1/library?${query}`)).status(), query).toBe(400);
      expect((await page.request.get(`/?${query}`)).status(), query).toBe(400);
    }
    await info.attach("library-filters", { body: JSON.stringify({ revision: process.env.KINOSAIL_TEST_REVISION, browser: info.project.name, all, target, facets,
      data: "Real populated Server Movies library; real form submissions, watched state and API reads", result: "passed" }), contentType: "application/json" });
  } finally {
    await page.goto("/?view=movies");
    await mark(false);
  }
});
