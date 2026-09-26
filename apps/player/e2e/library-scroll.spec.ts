import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";

const script = await readFile(new URL("../../../packages/webassets/static/pwa.js", import.meta.url), "utf8");
const origin = "http://localhost:38127";
function libraryPage(next: boolean) {
  return `<!doctype html><html><body><main id="main"><section id="library"><div data-library-group="movies"><div class="grid"><a class="card" href="/item/${next ? "beta" : "arrival"}">${next ? "Beta" : "Arrival"}</a></div></div></section>${next ? "" : '<div style="height:2200px"></div><nav data-library-pagination><a href="/?offset=1" data-library-next>Load more</a></nav>'}<p data-library-status role="status"></p></main><script src="/static/pwa.js"></script></body></html>`;
}

test("web libraries append on scroll and retry in place without a load button", async ({ page }) => {
  let requests = 0;
  let failNext = true;
  await page.route(`${origin}/**`, route => {
    const url = new URL(route.request().url());
    if (url.pathname === "/static/pwa.js") return route.fulfill({ contentType: "text/javascript", body: script });
    if (url.pathname === "/service-worker.js") return route.fulfill({ status: 404 });
    if (url.searchParams.get("offset") === "1") {
      requests++;
      if (failNext) return route.fulfill({ status: 503 });
    }
    return route.fulfill({ contentType: "text/html", body: libraryPage(url.searchParams.has("offset")) });
  });
  await page.goto(origin);
  await expect(page.locator("[data-library-next]")).toBeHidden();
  await page.evaluate(() => scrollTo(0, document.body.scrollHeight));
  await expect(page.locator("[data-library-next]")).toHaveText("Retry loading");
  expect(requests).toBe(1);
  failNext = false;
  await page.getByRole("link", { name: "Retry loading" }).click();
  await expect(page.locator("#library .card")).toHaveCount(2);
  await expect(page.locator("[data-library-next]")).toHaveCount(0);
  await expect(page.locator("[data-library-status]")).toHaveText("All titles are loaded.");
  await expect(page).toHaveURL(origin + "/");
  expect(requests).toBe(2);
});
