import { createHmac } from "node:crypto";
import { expect, test, type Browser, type Page } from "@playwright/test";
import {gotoAuthForm} from "../../../scripts/testing/auth-form-navigation";

export { downloadsSource } from "./static-sources";

export type OfflineClient = {
  KinosailOfflineMedia: { remove: (jobID: string) => Promise<boolean>; source: (itemID: string) => Promise<string> };
  streamingOfflineDigest: () => { close: () => void; hex: () => Promise<string>; update: (data: ArrayBuffer) => Promise<void> };
};


export function configureTestInstance() {
  test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
  test.beforeEach(async ({ page }) => page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false })));
}

export function totp(): string {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  const bits = [...(process.env.KINOSAIL_TEST_TOTP_SECRET ?? "")].map((character) => alphabet.indexOf(character).toString(2).padStart(5, "0")).join("");
  const secret = Buffer.from(bits.match(/.{8}/g)?.map((byte) => Number.parseInt(byte, 2)) ?? []);
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30_000)));
  const digest = createHmac("sha1", secret).update(counter).digest();
  const offset = digest[19] & 15;
  return (((digest.readUInt32BE(offset) & 0x7fffffff) % 1_000_000).toString().padStart(6, "0"));
}

export async function login(page: import("@playwright/test").Page) {
  const response = await gotoAuthForm(page, "/login", test.info());
  expect(response?.status()).toBe(200);
  expect(response!.request().redirectedFrom()).toBeNull();
  const loginURL = new URL(response!.url());
  expect(loginURL.pathname + loginURL.search + loginURL.hash).toBe("/login");
  await expect(page).toHaveURL(response!.url());
  const password = page.getByLabel("Password", {exact: true});
  await expect(page.getByLabel("Name")).toBeEditable();
  await expect(password).toBeEditable();
  await expect(page.locator(".password-control").filter({has: password}).getByRole("button", {name: "Show secret", exact: true})).toBeVisible();
  await expect(page.getByRole("button", {name: "Sign in", exact: true})).toBeEnabled();
  await page.getByLabel("Name").fill(process.env.KINOSAIL_E2E_OWNER_NAME ?? "Owner");
  await page.getByLabel("Password", { exact: true }).fill(process.env.KINOSAIL_E2E_OWNER_PASSWORD ?? "test-instance-password");
  await page.getByLabel("Authentication or recovery code").fill(totp());
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page.waitForURL(url => url.pathname !== "/login");
  if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
}

export async function loginViewer(page: Page, name: string, password: string) {
  await page.goto("/login");
  await page.getByLabel("Name").fill(name);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page.waitForURL(url => url.pathname !== "/login");
}

export async function saveSubtitleChoices(page: Page, choice: "on" | "off") {
  const form = page.locator('form[action="/settings/subtitles/picker"]');
  await form.getByLabel("Playback subtitle choices").selectOption(choice);
  const previousDocument = await page.evaluate(() => performance.timeOrigin);
  const action = new URL("/settings/subtitles/picker", page.url()).href, destination = new URL("/settings", page.url()).href;
  const [response, redirected] = await Promise.all([
    page.waitForResponse(response => response.request().method() === "POST" &&
      response.url() === action, {timeout: 10_000}),
    page.waitForResponse(response => response.url() === destination && response.request().method() === "GET" &&
      response.request().redirectedFrom()?.url() === action && response.request().redirectedFrom()?.method() === "POST", {timeout: 10_000}),
    form.getByRole("button", {name: "Save subtitle choices", exact: true}).click({noWaitAfter: true}),
  ]);
  expect(response.status()).toBe(303);
  expect(response.headers().location).toBe("/settings#playback");
  expect(redirected.status()).toBe(200);
  expect(redirected.request().redirectedFrom()).toBe(response.request());
  await page.waitForFunction(previous => performance.timeOrigin !== previous, previousDocument, {timeout: 10_000});
  const refreshed = await page.reload({waitUntil: "load"});
  expect(refreshed?.status()).toBe(200);
  await expect(page).toHaveURL(destination + "#playback");
  await expect(form.getByLabel("Playback subtitle choices")).toHaveValue(choice);
  await test.info().attach("subtitle-choice-save", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
    choice, status: response.status(), verified: "fresh settings GET after real POST acknowledgement"}), contentType: "application/json"});
}

export async function createViewer(page: Page, name: string, password: string): Promise<string> {
  await page.goto("/settings");
  return page.evaluate(async ({ name, password }) => {
    const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')?.content || "";
    const response = await fetch("/api/v1/profiles", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf },
      body: JSON.stringify({ name, password, rating: "all", libraries: ["all"] }),
    });
    if (!response.ok) throw new Error(`create Viewer failed: ${response.status}`);
    return (await response.json()).id;
  }, { name, password });
}

export async function removeViewer(page: Page, id: string) {
  await page.goto("/settings");
  await page.evaluate(async (profileID) => {
    const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')?.content || "";
    const response = await fetch(`/api/v1/profiles/${encodeURIComponent(profileID)}`, { method: "DELETE", headers: { "X-Kinosail-CSRF": csrf } });
    if (!response.ok) throw new Error(`remove Viewer failed: ${response.status}`);
  }, id);
}

export async function newViewerPage(browser: Browser, baseURL: string): Promise<Page> {
  const context = await browser.newContext({ baseURL, ignoreHTTPSErrors: false });
  await context.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false }));
  return context.newPage();
}

export async function firstPlayable(page: Page): Promise<string> {
  for (const path of ["/?view=movies", "/"]) {
    await page.goto(path);
    const cards = page.locator('a.card[href^="/watch/"]');
    const example = cards.filter({ hasText: "Example Movie" });
    const watch = await (await example.count() ? example.first() : cards.first()).getAttribute("href");
    if (watch) return watch;
  }
  throw new Error("the Library does not contain playable video");
}

export async function openLibrarySection(page: import("@playwright/test").Page, section: string) {
  const navigation = page.getByRole("navigation", { name: "Main navigation" });
  const link = navigation.getByRole("link", { name: section === "Shows" ? /^(TV )?Shows$/ : section, exact: true });
  if (!await link.isVisible()) await navigation.getByText("More", { exact: true }).click();
  await link.click();
}
