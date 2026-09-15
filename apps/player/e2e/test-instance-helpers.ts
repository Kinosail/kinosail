import { createHmac } from "node:crypto";
import { test, type Browser, type Page } from "@playwright/test";

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
  await page.goto("/login");
  await page.getByLabel("Name").fill(process.env.KINOSAIL_E2E_OWNER_NAME ?? "Owner");
  await page.getByLabel("Password", { exact: true }).fill(process.env.KINOSAIL_E2E_OWNER_PASSWORD ?? "test-instance-password");
  await page.getByLabel("Authentication or recovery code").fill(totp());
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
}

export async function loginViewer(page: Page, name: string, password: string) {
  await page.goto("/login");
  await page.getByLabel("Name").fill(name);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
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
  const context = await browser.newContext({ baseURL, ignoreHTTPSErrors: true });
  await context.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false }));
  return context.newPage();
}

export async function firstPlayable(page: Page): Promise<string> {
  for (const path of ["/?view=movies", "/"]) {
    await page.goto(path);
    const cards = page.locator('a.card[href^="/watch/"], a.card[href^="/item/"]');
    const example = cards.filter({ hasText: "Example Movie" });
    const watch = await (await example.count() ? example.first() : cards.first()).getAttribute("href");
    if (watch) return watch.replace(/^\/item\//, "/watch/");
  }
  throw new Error("the Library does not contain playable video");
}

export async function openLibrarySection(page: import("@playwright/test").Page, section: string) {
  const navigation = page.getByRole("navigation", { name: "Main navigation" });
  const link = navigation.getByRole("link", { name: section, exact: true });
  if (!await link.isVisible()) await navigation.getByText("More", { exact: true }).click();
  await link.click();
}
