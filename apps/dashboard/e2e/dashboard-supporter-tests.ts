import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { expectNoOverflow, login, owner, password, screenshot } from "./dashboard-helpers";

export function registerDashboardSupporterTests() {
test.describe("Supporter Badge Case", () => {
test.use({ serviceWorkers: "block" });
test("Badge Case renders every ownership state and the joined Fleet Command", async ({ page }, testInfo) => {
  await page.goto("/");
  if (page.url().endsWith("/setup")) {
    await page.getByLabel("Name", { exact: true }).fill(owner);
    await page.getByLabel("Password", { exact: true }).fill(password);
    await page.getByLabel("Confirm password").fill(password);
    await page.getByRole("button", { name: "Create Owner & continue" }).click();
    await expect(page).toHaveURL(/\?passkey=offer$/);
    await page.getByRole("button", { name: "Not now" }).click();
    await expect(page).toHaveURL(/\/$/);
  } else {
    await login(page);
  }
  const base = { appId: "kino-dashboard", activationAvailable: true, supportUrl: "https://support.example", patronOrder: null, livingStandard: null };
  const living = { family: "living", tier: "admiral", name: "Admiral", supporterId: "A1B2C3D4E5", supportedSince: "2025-01-01T00:00:00Z", expiresAt: "2026-10-01T00:00:00Z", rank: 8, active: true, expired: false, founding: true };
  const patron = { family: "patron", tier: "lighthouse", name: "Lighthouse", supporterId: "A1B2C3D4E5", supportedSince: "2025-01-01T00:00:00Z", rank: 6, active: true, expired: false, founding: true };
  const states = [
    ["empty", { ...base, badgeCase: { livingLevel: 0, patronLevel: 0, masterworkLevel: 0, unlocked: 0, total: 20, masterworkName: "Fleet Command", masterworkEarned: false, masterworkActive: false } }],
    ["living", { ...base, livingStandard: living, badgeCase: { livingLevel: 8, patronLevel: 0, masterworkLevel: 0, unlocked: 8, total: 20, masterworkName: "Fleet Command", masterworkEarned: false, masterworkActive: false } }],
    ["patron", { ...base, patronOrder: patron, badgeCase: { livingLevel: 0, patronLevel: 6, masterworkLevel: 0, unlocked: 6, total: 20, masterworkName: "Fleet Command", masterworkEarned: false, masterworkActive: false } }],
    ["both-active", { ...base, livingStandard: living, patronOrder: patron, badgeCase: { livingLevel: 8, patronLevel: 6, masterworkLevel: 6, unlocked: 14, total: 20, masterworkName: "Fleet Command", masterworkEarned: true, masterworkActive: true } }],
    ["both-dormant", { ...base, livingStandard: { ...living, active: false, expired: true }, patronOrder: patron, badgeCase: { livingLevel: 8, patronLevel: 6, masterworkLevel: 6, unlocked: 14, total: 20, masterworkName: "Fleet Command", masterworkEarned: true, masterworkActive: false } }],
  ] as const;
  const mockStatus = async (status: object) => {
    await page.unroute("**/api/v1/supporter");
    await page.route("**/api/v1/supporter", (route) => route.fulfill({ status: 200, contentType: "application/json", headers: { "Cache-Control": "no-store" }, body: JSON.stringify(status) }));
  };
  for (const [name, next] of states) {
    await mockStatus(next);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`/supporter?state=${name}`);
    await expect(page.locator(".case-badge")).toHaveCount(20);
    await expect(page.locator(".case-badge.is-collected")).toHaveCount(next.badgeCase.unlocked);
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
    await expectNoOverflow(page);
    await screenshot(page, testInfo, `supporter-${name}-1440.png`);
  }
  await page.setViewportSize({ width: 320, height: 700 });
  await mockStatus(states[3][1]);
  await page.goto("/supporter?state=both-active-mobile");
  await expect(page.getByRole("heading", { name: "Operator Bridge" })).toBeVisible();
  await expectNoOverflow(page);
  await screenshot(page, testInfo, "supporter-both-active-320.png");
});
});
}
