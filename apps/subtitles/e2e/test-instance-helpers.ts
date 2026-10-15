import { reuseSignedInSession } from "../../../scripts/testing/owner-session";
import { fixtureTotp } from "../../../scripts/testing/fixture-totp";
import { expect, test, type Page } from "@playwright/test";
import {gotoAuthForm} from "../../../scripts/testing/auth-form-navigation";

function totp(): string {
  return fixtureTotp();
}

export async function login(page: Page) {
  await reuseSignedInSession(page, process.env.KINOSAIL_E2E_OWNER_NAME ?? "Owner", () => signInOwner(page));
}

async function signInOwner(page: Page) {
  await openLogin(page);
  await page.getByLabel("Name").fill(process.env.KINOSAIL_E2E_OWNER_NAME ?? "Owner");
  await page.getByLabel("Password", { exact: true }).fill(process.env.KINOSAIL_E2E_OWNER_PASSWORD ?? "test-instance-password");
  await page.getByLabel("6-digit code").fill(totp());
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page.waitForURL((url) => url.pathname !== "/login");
  if (new URL(page.url()).pathname === "/account") await page.getByRole("link", { name: "Not now" }).click();
}

async function openLogin(page: Page) {
  const response = await gotoAuthForm(page, "/login", test.info());
  expect(response?.status()).toBe(200);
  expect(response!.request().redirectedFrom()).toBeNull();
  const url = new URL(response!.url());
  expect(url.pathname + url.search + url.hash).toBe("/login");
  await expect(page).toHaveURL(response!.url());
  const password = page.getByLabel("Password", {exact: true});
  await expect(page.getByLabel("Name")).toBeEditable();
  await expect(password).toBeEditable();
  await expect(page.locator(".password-control").filter({has: password}).getByRole("button", {name: "Show secret", exact: true})).toBeVisible();
  await expect(page.getByRole("button", {name: "Sign in", exact: true})).toBeEnabled();
}
