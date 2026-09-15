import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

const nativeURL = process.env.KINOSAIL_NATIVE_URL;
test.skip(!nativeURL, "KINOSAIL_NATIVE_URL is required for native client QA.");

async function connect(page: Page) {
  await page.goto(nativeURL!);
  await page.getByLabel("Kinosail Server URL").fill(nativeURL!);
  const started = Date.now();
  await page.getByRole("button", { name: "Connect" }).click();
  await expect(page.getByText("Continue watching")).toBeVisible();
  return Date.now() - started;
}

async function expectSoundLayout(page: Page) {
  const layout = await page.evaluate(() => ({
    overflow: document.documentElement.scrollWidth - innerWidth,
    tiny: [...document.querySelectorAll('[role="button"], input')]
      .map((element) => element.getBoundingClientRect())
      .filter((box) => box.width > 0 && box.height > 0 && box.height < 44)
      .length,
  }));
  expect(layout).toEqual({ overflow: 0, tiny: 0 });
}

async function expectFullyVisible(page: Page, name: string) {
  const visible = await page
    .getByRole("button", { name })
    .evaluate((button) => {
      const box = button.getBoundingClientRect();
      return box.top >= 0 && box.bottom <= innerHeight;
    });
  expect(visible).toBe(true);
}

test("connects, browses details, and attempts direct playback", async ({
  page,
}, testInfo) => {
  const mediaRequests: string[] = [];
  page.on("request", (request) => {
    const path = new URL(request.url()).pathname;
    if (path.startsWith("/api/v1/items/")) mediaRequests.push(path);
  });
  const connectMilliseconds = await connect(page);
  expect(connectMilliseconds).toBeLessThan(1500);
  await expect(page).toHaveTitle("Kinosail Player");
  await expect(
    page.getByRole("navigation", { name: "Media navigation" }),
  ).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await expectSoundLayout(page);
  await page.screenshot({
    path: testInfo.outputPath("home-1440.png"),
    fullPage: true,
  });

  await page.getByRole("tab", { name: "Movies", exact: true }).click();
  await page.getByRole("button", { name: "Arrival, 2016" }).first().click();
  await expect(page.getByRole("heading", { name: "Arrival" })).toBeVisible();
  await expect(page.getByText("2016 · PG-13 · MP4")).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("detail-1440.png"),
    fullPage: true,
  });
  const mediaResponse = page.waitForResponse((response) =>
    response.url().includes("/media/arrival"),
  );
  await page.getByRole("button", { name: "Resume" }).click();
  await expect(page.getByText("Playback stopped")).toBeVisible();
  await expect(
    page.getByText(
      /^(Playback could not continue on this device\.|Your Server denied access to this title\. Check your Viewer permissions or reconnect\.)$/,
    ),
  ).toBeVisible();
  expect((await mediaResponse).status()).toBe(401);
  expect([
    ...new Set(mediaRequests.filter((path) => path.endsWith("/playback"))),
  ]).toEqual(["/api/v1/items/arrival/playback"]);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

for (const viewport of [
  { name: "tablet", width: 1024, height: 768 },
  { name: "portrait-tablet", width: 834, height: 1194 },
  { name: "compact", width: 390, height: 844 },
  { name: "reflow", width: 320, height: 720 },
]) {
  test(`${viewport.name} layout stays usable`, async ({ page }, testInfo) => {
    await page.setViewportSize(viewport);
    await page.goto(nativeURL!);
    await expectFullyVisible(page, "Connect");
    await page.screenshot({
      path: testInfo.outputPath(`${viewport.name}-setup.png`),
      fullPage: true,
    });
    await connect(page);
    await expectSoundLayout(page);
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
    await page.screenshot({
      path: testInfo.outputPath(`${viewport.name}-home.png`),
      fullPage: true,
    });
  });
}

test("compact authorization stays clear while approval is pending", async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.route("**/api/v1/quick-connect/token", async (route) => {
    await route.fulfill({
      body: JSON.stringify({ status: "pending" }),
      contentType: "application/json",
      status: 202,
    });
  });
  await page.goto(nativeURL!);
  await page.getByLabel("Kinosail Server URL").fill(nativeURL!);
  await page.getByRole("button", { name: "Connect" }).click();
  await expect(page.getByText("381 204")).toBeVisible();
  await expect(
    page.getByText("Approve from a signed-in Player app or browser."),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Use another server" })
    .scrollIntoViewIfNeeded();
  await expectFullyVisible(page, "Use another server");
  await expectSoundLayout(page);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.screenshot({
    path: testInfo.outputPath("compact-authorization.png"),
    fullPage: true,
  });
});

test("compact connection keeps its busy label and blocks repeated submits", async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  let release!: () => void;
  let requests = 0;
  const pending = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route("**/api/v1/quick-connect", async (route) => {
    requests += 1;
    await pending;
    await route.continue();
  });
  await page.goto(nativeURL!);
  const input = page.getByLabel("Kinosail Server URL");
  await input.fill(nativeURL!);
  await page.getByRole("button", { name: "Connect" }).click();
  try {
    const busy = page.getByRole("button", { name: "Connecting…" });
    await expect(busy).toBeVisible();
    await expect(busy).toBeDisabled();
    await expect(busy).toHaveAttribute("aria-busy", "true");
    await expect(input).not.toBeEditable();
    await input.press("Enter");
    await expect.poll(() => requests).toBe(1);
    await expectSoundLayout(page);
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
    await page.screenshot({
      path: testInfo.outputPath("compact-connecting.png"),
      fullPage: true,
    });
  } finally {
    release();
  }
  await expect(page.getByText("Continue watching")).toBeVisible();
  expect(requests).toBe(1);
});

test("compact details and playback recovery stay usable", async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 320, height: 720 });
  await connect(page);
  await page.getByRole("tab", { name: "Movies", exact: true }).click();
  await page.getByRole("button", { name: "Arrival, 2016" }).first().click();
  await expect(page.getByRole("heading", { name: "Arrival" })).toBeVisible();
  await expectFullyVisible(page, "Resume");
  await expectSoundLayout(page);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.screenshot({
    path: testInfo.outputPath("compact-detail.png"),
    fullPage: true,
  });

  await page.getByRole("button", { name: "Resume" }).click();
  await expect(page.getByText("Playback stopped")).toBeVisible();
  await expect(page.getByRole("button", { name: "Back" })).toHaveCount(0);
  await expectFullyVisible(page, "Return to details");
  await expectSoundLayout(page);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.screenshot({
    path: testInfo.outputPath("compact-playback-error.png"),
    fullPage: true,
  });
});

test("compact saved-server failure offers safe recovery", async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 320, height: 720 });
  await page.route("**/api/v1/library**", (route) =>
    route.abort("connectionfailed"),
  );
  await page.goto(nativeURL!);
  await page.getByLabel("Kinosail Server URL").fill(nativeURL!);
  await page.getByRole("button", { name: "Connect" }).click();
  await expect(
    page.getByText(
      "Could not reach Kinosail Server. Check the address and network, then try again.",
    ),
  ).toBeVisible();
  await expectFullyVisible(page, "Try again");
  await expectFullyVisible(page, "Use another server");
  await expectSoundLayout(page);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.screenshot({
    path: testInfo.outputPath("compact-saved-server-recovery.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "Use another server" }).click();
  await expect(page.getByLabel("Kinosail Server URL")).toBeVisible();
});

test("supports keyboard, reduced motion, forced colors, and saved themes", async ({
  page,
}, testInfo) => {
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  await connect(page);
  await expect(page.getByRole("button", { name: "Theme: Dark" })).toBeVisible();
  await page.keyboard.press("Tab");
  await expect(page.locator(":focus")).toBeVisible();
  await page.getByRole("button", { name: "Theme: Dark" }).click();
  await expect(
    page.getByRole("button", { name: "Theme: Light" }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Theme: Light" }),
  ).toBeVisible();
  await page.emulateMedia({ forcedColors: "active", reducedMotion: "reduce" });
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await expectSoundLayout(page);
  await page.screenshot({
    path: testInfo.outputPath("forced-colors-home.png"),
    fullPage: true,
  });
});

test("keeps long titles and missing artwork usable on compact screens", async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await connect(page);
  const recent = page.getByText("Recently added");
  await recent.evaluate((heading) =>
    heading.scrollIntoView({ block: "start" }),
  );
  await expect(
    page
      .getByRole("list", { name: "Recently added shelf" })
      .getByRole("button", { name: "Moon, 2009" }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("compact-missing-art.png"),
    fullPage: true,
  });
  await page
    .getByRole("list", { name: "Recently added shelf" })
    .evaluate((shelf) => {
      shelf.scrollLeft = shelf.scrollWidth;
      shelf.dispatchEvent(new Event("scroll", { bubbles: true }));
    });
  const longTitle = page
    .getByRole("list", { name: "Recently added shelf" })
    .getByRole("button", {
      name: "Everything Everywhere All at Once, 2022",
    });
  await expect(longTitle).toBeVisible();
  await expectSoundLayout(page);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.screenshot({
    path: testInfo.outputPath("compact-long-title.png"),
    fullPage: true,
  });
});

test("playback navigation stays separate from native transport controls after rotation", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await connect(page);
  await page.getByRole("tab", { name: "Movies", exact: true }).click();
  await page.getByRole("button", { name: "Arrival, 2016" }).first().click();
  await expect(
    page.getByRole("heading", { name: "Arrival", exact: true }),
  ).toBeVisible();
  let release!: () => void;
  const pending = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route("**/media/**", async (route) => {
    await pending;
    await route.abort();
  });
  try {
    await page.getByRole("button", { name: "Resume", exact: true }).click();
    for (const viewport of [
      { width: 390, height: 844 },
      { width: 844, height: 390 },
    ]) {
      await page.setViewportSize(viewport);
      const back = page.getByRole("button", { name: "Back", exact: true });
      await expect(back).toBeVisible();
      const backBox = await back.boundingBox();
      const videoBox = await page.locator("video").boundingBox();
      expect(backBox).not.toBeNull();
      expect(videoBox).not.toBeNull();
      expect(backBox!.y).toBeGreaterThanOrEqual(videoBox!.y + videoBox!.height);
      expect(videoBox!.height).toBeGreaterThan(200);
    }
    await page.getByRole("button", { name: "Back", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "Arrival", exact: true }),
    ).toBeVisible();
  } finally {
    release();
  }
});
