import {expect, test} from "@playwright/test";
import {openHappyPathSetup} from "./happy-path-setup";

test("Setup navigation remains usable while a real decorative image is pending", async ({page}, info) => {
  let release!: () => void, fetched!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  const observed = new Promise<void>(resolve => { fetched = resolve; });
  await page.route("**/static/cinema-backdrop.jpg?setup-navigation-test", async route => {
    const response = await route.fetch();
    expect(response.status()).toBe(200);
    fetched(); await held;
    await route.fulfill({response});
  });
  await page.addInitScript(() => addEventListener("DOMContentLoaded", () => {
    const image = document.createElement("img");
    image.alt = ""; image.src = "/static/cinema-backdrop.jpg?setup-navigation-test";
    document.body.append(image);
  }, {once: true}));
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    await Promise.race([openHappyPathSetup(page, info), new Promise((_, reject) => {
      timer = setTimeout(() => reject(new Error("Usable setup must not await the decorative image")), 12000);
    })]);
    await observed;
    await expect(page.getByLabel("Name", {exact: true})).toBeVisible();
    expect(await page.evaluate(() => document.readyState)).toBe("interactive");
  } finally { clearTimeout(timer); release(); }
});
