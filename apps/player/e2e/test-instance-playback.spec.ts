import { expect, test } from "@playwright/test";
import { configureTestInstance, login } from "./test-instance-helpers";

configureTestInstance();

test("adaptive playback starts on Auto and keeps semantic quality choices", async ({ page }) => {
  await login(page);
  await page.getByRole("link", { name: "Movies", exact: true }).click();
  const watch = await page.locator('a.card[href^="/watch/"]').first().getAttribute("href");
  expect(watch).toBeTruthy();
  await page.goto(`${watch}?compatible=1`);

  const quality = page.getByLabel("Stream quality");
  await expect(quality.locator("option")).toHaveText(["Auto", "360p", "Original"], { timeout: 30_000 });
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await expect(quality).toBeVisible({ timeout: 30_000 });
  await quality.selectOption("level:0");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("kinosail.stream-quality"))).toBe("360p");
  await quality.selectOption("auto");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("kinosail.stream-quality"))).toBe("auto");
  await expect(page.getByRole("status").filter({ hasText: /Auto/ })).toBeVisible();
});

test("Owner can inspect, update, and remove a skip marker", async ({ page }, testInfo) => {
  await login(page);
  await page.getByRole("link", { name: "Movies", exact: true }).click();
  const watch = await page.locator('a.card[href^="/watch/"]').first().getAttribute("href");
  expect(watch).toBeTruthy();
  const id = watch!.split("/").pop();
  expect(id).toBeTruthy();
  await page.goto(watch!);
  await page.getByText("Manage media", { exact: true }).click();
  await page.getByText("Edit skip markers", { exact: true }).click();

  await page.getByText("Add skip marker", { exact: true }).click();
  const saveForms = page.locator(`form[action="/markers/${id}"]`);
  const add = saveForms.last();
  await add.getByLabel("Type").selectOption("recap");
  await add.getByLabel("Start in seconds").fill("1");
  await add.getByLabel("End in seconds").fill("2");
  await add.getByRole("button", { name: "Save marker", exact: true }).click();

  for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 720 }]) {
    await page.setViewportSize(viewport);
    await page.goto(watch!);
    await page.getByText("Manage media", { exact: true }).click();
    await page.getByText("Edit skip markers", { exact: true }).click();
    await expect(page.getByText("Add skip marker", { exact: true })).toBeVisible();
    await expect(page.locator(`form[action="/markers/${id}"]`).last()).toBeHidden();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
    await page.screenshot({ path: testInfo.outputPath(`marker-editor-${viewport.width}.png`), fullPage: true });
  }

  const existing = page.locator(`form[action="/markers/${id}"]:has(option[value="recap"]:checked)`).first();
  await expect(existing.getByLabel("Start in seconds")).toHaveValue("1");
  await expect(existing.getByLabel("End in seconds")).toHaveValue("2");
  await existing.getByLabel("Start in seconds").fill("1.5");
  await existing.getByLabel("End in seconds").fill("2.5");
  await existing.getByRole("button", { name: "Save marker", exact: true }).click();

  await page.getByText("Manage media", { exact: true }).click();
  await page.getByText("Edit skip markers", { exact: true }).click();
  await expect(page.locator(`form[action="/markers/${id}"]:has(option[value="recap"]:checked)`).first().getByLabel("Start in seconds")).toHaveValue("1.5");
  await page.getByRole("button", { name: "Remove Recap marker", exact: true }).click();
  await page.getByText("Manage media", { exact: true }).click();
  await page.getByText("Edit skip markers", { exact: true }).click();
  await expect(page.getByRole("button", { name: "Remove Recap marker", exact: true })).toHaveCount(0);
});
