import { expect, test, type Page, type TestInfo } from "@playwright/test";
import { assetReceipt, observe, origin, record, settle, snapshot, type Snapshot } from "./browse-return-helpers";

// Source-prepared supplemental coverage: the original nine-case selector does
// not register this file. Root must explicitly admit it with the real Go fixture.
test.skip(!origin, "requires disposable Go Server browse-return fixture");
test.use({ serviceWorkers: "block", video: "off" });
test.setTimeout(25_000);
test.beforeEach(async ({ page }) => observe(page));

async function home(page: Page, info: TestInfo) {
  const response = await page.goto(`${origin}/`);
  expect(response?.status(), "Home prerequisite: real Go root document").toBe(200);
  await expect(page.locator("#main")).toBeVisible();
  await expect(page.locator("#library"), "Home uses shelves, not a Library grid").toHaveCount(0);
  await assetReceipt(page, info);
}

async function selectResume(page: Page, info: TestInfo) {
  const api = await page.request.get(`${origin}/api/v1/library?view=movies&q=Return+Movie+25&limit=4`);
  expect(api.status(), "Home prerequisite: public fictional catalog").toBe(200);
  const data = await api.json();
  expect(data).toMatchObject({ total: 1, items: [{ title: "Return Movie 25" }] });
  const id = data.items[0].id;
  expect(id, "Home prerequisite: canonical fictional identity").toMatch(/^[a-f0-9]{16}$/);
  // Only the authorized disposable Server receives this progress write. No
  // Remove action or production account/media is involved, and playback decode
  // is outside this focus/scroll contract.
  const progress = await page.request.post(`${origin}/progress/${id}`, { form: { seconds: "3" } });
  expect(progress.status(), "Home prerequisite: public Continue watching seed").toBe(204);
  await home(page, info);
  const href = `/watch/${id}`;
  const link = page.locator(`.continue-shelf a.resume-link[href="${href}"]`);
  await expect(link).toHaveCount(1);
  await link.scrollIntoViewIfNeeded();
  await link.focus();
  await expect(link).toBeFocused();
  await settle(page, href);
  const before = await record(page, info, "home-before", href);
  expect(before.state.profile, "Home prerequisite: actual Viewer Profile").toBe("local-owner");
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(`${origin}${href}`);
  await expect(page.locator("body.player-page"), "Home prerequisite: real Go Player").toBeVisible();
  await record(page, info, "home-player", href);
  return { href, before };
}

async function assertHomeReturn(page: Page, before: Snapshot, href: string, selector: string) {
  await expect(page).toHaveURL(`${origin}/`);
  await expect(page.locator("#main")).toBeVisible();
  await expect(page.locator("#library")).toHaveCount(0);
  await expect(page.locator(selector), "Q14 Home acceptance: original action regains focus").toBeFocused({ timeout: 4000 });
  await settle(page, href);
  const state = await snapshot(page, href);
  expect({ path: state.path, values: state.values, profile: state.profile }).toEqual({ path: before.path, values: before.values, profile: before.profile });
  expect(Math.abs(state.scroll.x - before.scroll.x), "Q14 Home acceptance: settled horizontal position").toBeLessThanOrEqual(2);
  expect(Math.abs(state.scroll.y - before.scroll.y), "Q14 Home acceptance: settled vertical position").toBeLessThanOrEqual(2);
}

async function assertColdBoundary(page: Page, info: TestInfo, before: Awaited<ReturnType<typeof record>>, href: string) {
  await expect.poll(async () => (await snapshot(page, href)).document.shows.at(-1)?.persisted, { timeout: 1500, message: "Home cold prerequisite: completed non-persisted pageshow" }).toBe(false);
  const returned = await record(page, info, "home-cold-boundary", href);
  expect(returned.state.document.document, "Home cold prerequisite: fresh document").not.toBe(before.state.document.document);
  expect(returned.state.navigation).toContain("back_forward");
  expect(returned.peer.slice(before.peer.length).some(request => !request.continuation && !request.history), "Home cold prerequisite: real root document GET").toBe(true);
}

test("visible Player Back restores the Home Continue watching action without a Library grid", async ({ page }, info) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const { href, before } = await selectResume(page, info);
  try {
    const back = page.locator("a.back");
    await expect(back).toHaveAttribute("href", "/");
    await back.focus();
    await expect(back).toBeFocused();
    await page.keyboard.press("Enter");
    await assertHomeReturn(page, before.state, href, `.continue-shelf a.resume-link[href="${href}"]`);
  } finally {
    await record(page, info, "home-returned", href);
  }
});

test("cold native Back restores the Home Continue watching action without a Library grid", async ({ page }, info) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const { href, before } = await selectResume(page, info);
  try {
    await page.goBack({ waitUntil: "domcontentloaded" });
    await assertColdBoundary(page, info, before, href);
    await assertHomeReturn(page, before.state, href, `.continue-shelf a.resume-link[href="${href}"]`);
  } finally {
    await record(page, info, "home-returned", href);
  }
});

test("cold native Back restores the scrolled Home Movies destination without a Library grid", async ({ page }, info) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await home(page, info);
  const href = "/?view=movies";
  const link = page.locator(`.destination-card[href="${href}"]`);
  await expect(link).toHaveCount(1);
  await link.scrollIntoViewIfNeeded();
  await link.focus();
  await expect(link).toBeFocused();
  await settle(page, href);
  const before = await record(page, info, "home-before", href);
  expect(before.state.profile).toBe("local-owner");
  expect(before.state.scroll.y, "Home prerequisite: a scrolled destination").toBeGreaterThan(100);
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(`${origin}${href}`);
  await expect(page.locator("#library"), "destination prerequisite: real Go Movies grid").toBeVisible();
  try {
    await page.goBack({ waitUntil: "domcontentloaded" });
    await assertColdBoundary(page, info, before, href);
    await assertHomeReturn(page, before.state, href, `.destination-card[href="${href}"]`);
    const finalPeer = await (await page.request.get(`${origin}/__browse-return`)).json();
    expect(finalPeer.slice(before.peer.length).some((request: { continuation: boolean }) => request.continuation), "Home restoration must not fetch Library continuation").toBe(false);
  } finally {
    await record(page, info, "home-returned", href);
  }
});
