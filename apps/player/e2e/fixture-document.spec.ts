import { expect, test } from "@playwright/test";
import { fixtureDocument } from "../../../scripts/testing/fixture-document";

test("exported fixtures remove external scripts through inert parsing", async ({ page }) => {
  const requests: string[] = [];
  await page.route("https://fixture.test/**", async route => {
    requests.push(route.request().url());
    await route.abort();
  });
  const source = `<!doctype html><html><head><link rel="stylesheet" href="/static/app.css?v=1"></head><body><main>Fixture</main>
    <SCRIPT SRC="https://fixture.test/upper.js"></SCRIPT >
    <script src='https://fixture.test/single.js'>ignored</script>
    <script data-note=">" src=https://fixture.test/unquoted.js></script>
    <script>document.body.dataset.inline = "preserved";</script></body></html>`;
  const html = await page.evaluate(fixtureDocument, { source, css: "main { color: rgb(1, 2, 3); }", dark: true });
  await page.setContent(html);
  await expect(page.locator("script[src]")).toHaveCount(0);
  await expect(page.locator("body")).toHaveAttribute("data-inline", "preserved");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.locator("main")).toHaveCSS("color", "rgb(1, 2, 3)");
  expect(requests).toEqual([]);
});
