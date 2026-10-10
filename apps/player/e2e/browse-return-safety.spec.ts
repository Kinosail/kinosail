import type {JSONObject} from "../../../scripts/testing/json-value";
import { expect, test } from "@playwright/test";
import { observe, origin, selectMovie } from "./browse-return-helpers";

// Additional real-Server controls. The original nine-case hosted selector is
// unchanged; root must explicitly admit this spec in a later fixture run.
test.skip(!origin, "requires disposable Go Server browse-return fixture");
test.use({ serviceWorkers: "block", video: "off" });
test.setTimeout(25_000);
test.beforeEach(async ({ page }) => observe(page));
const key = "kinosail:browse-return:v1";

const corruptions: [string, JSONObject][] = [
  ["external origin", { url: "https://untrusted.invalid/?view=movies" }],
  ["protocol-relative origin", { url: "//untrusted.invalid/?view=movies" }],
  ["non-browse route", { url: "/settings" }],
  ["duplicate query", { url: "/?view=movies&view=shows" }],
  ["unknown query", { url: "/?token=fictional" }],
  ["oversized query", { url: `/?q=${"x".repeat(513)}` }],
  ["excessive extent", { extent: 2001 }],
  ["different profile", { profile: "fictional-other-profile" }],
  ["different destination", { destination: "/watch/0000000000000000" }],
];
for (const [name, patch] of corruptions) {
  test(`saved return rejects ${name} before navigation or continuation`, async ({ page }, info) => {
    const { href } = await selectMovie(page, info);
    const peerBefore = await (await page.request.get(`${origin}/__browse-return`)).json();
    await page.evaluate(({ key, patch }) => {
      const state = JSON.parse(sessionStorage.getItem(key) || "null");
      if (!state) throw new Error("public departure did not record return state");
      sessionStorage.setItem(key, JSON.stringify({ ...state, ...patch }));
    }, { key, patch });
    await page.reload();
    await expect(page).toHaveURL(`${origin}${href}`);
    await expect(page.locator("a.back")).toHaveAttribute("href", "/");
    await expect(page.locator("a.back")).toHaveText(/Library/);
    const peerAfter = await (await page.request.get(`${origin}/__browse-return`)).json();
    expect(peerAfter, "rejected state does not issue a browse request").toEqual(peerBefore);
    await info.attach("safe-rejection", { body: JSON.stringify({ name, href, noBrowseRequests: true }), contentType: "application/json" });
  });
}

test("a direct Player opened in another tab does not inherit browse return state", async ({ page, context }, info) => {
  const { href } = await selectMovie(page, info);
  const another = await context.newPage();
  await another.goto(`${origin}${href}`);
  await expect(another.locator("a.back")).toHaveAttribute("href", "/");
  await expect(another.locator("a.back")).toHaveText(/Library/);
  expect(await another.evaluate(key => sessionStorage.getItem(key), key)).toBeNull();
  await another.close();
});
