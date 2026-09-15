import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const nativeURL = process.env.KINOSAIL_NATIVE_URL;
test.skip(!nativeURL, "KINOSAIL_NATIVE_URL is required for native client QA.");

for (const viewport of [
  { width: 320, height: 720 },
  { width: 390, height: 844 },
]) {
  test(`approval recovery fits ${viewport.width}px without requesting another code`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize(viewport);
    let polls = 0;
    await page.route("**/api/v1/quick-connect/token", async (route) => {
      polls += 1;
      if (polls === 1)
        await route.fulfill({
          status: 503,
          body: "{}",
          contentType: "application/json",
        });
      else await route.continue();
    });
    await page.goto(nativeURL!);
    await page.getByLabel("Kinosail Server URL").fill(nativeURL!);
    await page.getByRole("button", { name: "Connect" }).click();
    await expect(page.getByText("Approval check stopped.")).toBeVisible();
    await expect(page.getByText("381 204")).toBeVisible();
    await expect(
      page.getByText("Kinosail Server could not complete the request."),
    ).toBeInViewport({ ratio: 1 });
    const retry = page.getByRole("button", { name: "Retry approval" });
    await retry.scrollIntoViewIfNeeded();
    await expect(retry).toBeInViewport();
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
    await page.screenshot({
      path: testInfo.outputPath("compact-approval-retry.png"),
      fullPage: true,
    });
    await retry.click();
    await expect(page.getByText("Continue watching")).toBeVisible();
    expect(polls).toBe(2);
  });
}

test("compact empty library can refresh after media is added", async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  let empty = true;
  await page.route(/\/api\/v1\/library\?/, async (route) => {
    if (empty)
      await route.fulfill({
        status: 200,
        body: JSON.stringify({ items: [] }),
        contentType: "application/json",
      });
    else await route.continue();
  });
  await page.goto(nativeURL!);
  await page.getByLabel("Kinosail Server URL").fill(nativeURL!);
  await page.getByRole("button", { name: "Connect" }).click();
  await expect(
    page.getByText("Your library is ready for its first title."),
  ).toBeVisible();
  const refresh = page.getByRole("button", { name: "Refresh library" });
  await expect(refresh).toBeInViewport();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.screenshot({
    path: testInfo.outputPath("compact-empty-library.png"),
    fullPage: true,
  });
  empty = false;
  await refresh.click();
  await expect(page.getByText("Continue watching")).toBeVisible();
});
