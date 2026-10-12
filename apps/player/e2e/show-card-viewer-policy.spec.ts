import { expect, test } from "@playwright/test";
import { configureTestInstance, createViewer, login, newViewerPage } from "./test-instance-helpers";
import { login as loginWithCredentials } from "./layout-audit-helpers";

configureTestInstance();

test("Show card actions respect each Viewer's episode and Library restrictions", { tag: "@smoke" }, async ({ page, browser }, testInfo) => {
  await login(page);
  await page.goto("/?view=shows");
  const origin = new URL(page.url()).origin;
  const csrf = await page.locator('meta[name="kinosail-csrf"]').getAttribute("content");
  expect(csrf).toBeTruthy();
  const headers = { Origin: origin, "X-Kinosail-CSRF": csrf! };
  await page.locator(".show-card").filter({ hasText: "Example Show" }).getByRole("link", { name: "Example Show", exact: true }).click();
  const showPath = new URL(page.url()).pathname;
  const episodes = await page.locator("[data-episode-row]").evaluateAll(rows => rows.map(row => row.getAttribute("href")!));
  expect(episodes).toHaveLength(2);
  const previous: Array<{ id: string; rating: string }> = [];
  for (const watch of episodes) {
    expect(watch).toMatch(/^\/watch\/[a-f0-9]+$/);
    const id = watch.split("/").at(-1)!;
    const response = await page.request.get(`/api/v1/items/${id}`);
    expect(response.status()).toBe(200);
    previous.push({ id, rating: (await response.json()).item.rating ?? "" });
  }
  const name = `Show policy ${testInfo.project.name} ${Date.now()}`;
  const password = "synthetic-viewer-password";
  let viewerID: string | undefined;
  let viewer: Awaited<ReturnType<typeof newViewerPage>> | undefined;
  const rate = async (id: string, rating: string) => page.request.post("/api/v1/metadata/bulk", { headers, data: { ids: [id], rating } });
  try {
    for (const [index, item] of previous.entries()) expect((await rate(item.id, index === 0 ? "PG" : "R")).status()).toBe(200);
    viewerID = await createViewer(page, name, password);
    const policy = async (rating: string, libraries: string[]) => page.request.put(`/api/v1/profiles/${viewerID}`, { headers, data: { rating, libraries } });
    expect((await policy("family", ["Shows"])).status()).toBe(200);
    viewer = await newViewerPage(browser, origin);
    await loginWithCredentials(viewer, name, password);
    await viewer.goto(showPath);
    await expect(viewer.locator("[data-episode-row]")).toHaveCount(1);
    await expect(viewer.locator("[data-episode-row]")).toHaveAttribute("href", episodes[0]);
    await viewer.goto("/?view=shows");
    const token = await viewer.locator('meta[name="kinosail-csrf"]').getAttribute("content");
    expect(token).toBeTruthy();
    expect((await viewer.request.put(`/api/v1/items/${previous[0].id}/progress`, { headers: { Origin: origin, "X-Kinosail-CSRF": token! }, data: { seconds: 0, watched: true } })).status()).toBe(200);
    await viewer.reload();
    const card = viewer.locator(".show-card").filter({ hasText: "Example Show" });
    await expect(card.getByRole("link", { name: /^Play again/ })).toHaveAttribute("href", episodes[0]);
    expect((await viewer.request.get(`/api/v1/items/${previous[1].id}`)).status()).toBe(404);
    expect((await viewer.request.get(episodes[1])).status()).toBe(404);
    await card.screenshot({ path: testInfo.outputPath("family-visible-episode.png") });

    expect((await policy("all", ["Movies"])).status()).toBe(200);
    await viewer.reload();
    await expect(viewer.locator(".show-card")).toHaveCount(0);
    expect((await viewer.request.get(showPath)).status()).toBe(404);
    expect((await viewer.request.get(episodes[0])).status()).toBe(404);
    await viewer.screenshot({ path: testInfo.outputPath("excluded-Shows-library.png") });

    expect((await policy("all", ["Shows"])).status()).toBe(200);
    await viewer.reload();
    await expect(card.getByRole("link", { name: /^Play next/ })).toHaveAttribute("href", episodes[1]);
    await viewer.goto(showPath);
    await expect(viewer.locator("[data-episode-row]")).toHaveCount(2);
    await testInfo.attach("show-card-viewer-policy", { contentType: "application/json", body: JSON.stringify({ revision: process.env.KINOSAIL_TEST_REVISION,
      browser: testInfo.project.name, fixture: "Real HTTPS Server, PG episode one and R episode two, disposable Viewer", result: "Family action stays within permitted episodes; excluded Library has no action or route access; all ratings restores next episode" }) });
  } finally {
    await viewer?.context().close();
    if (viewerID) expect.soft((await page.request.delete(`/api/v1/profiles/${viewerID}`, { headers })).status()).toBe(200);
    for (const item of previous) expect.soft((await rate(item.id, item.rating)).status()).toBe(200);
  }
});
