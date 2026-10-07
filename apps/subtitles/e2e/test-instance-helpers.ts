import { createHmac } from "node:crypto";
import { expect, test, type Browser, type Page, type TestInfo } from "@playwright/test";
import { navigationDiagnostics } from "../../../scripts/testing/navigation-diagnostics.mjs";

function totp(): string {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  const bits = [...(process.env.KINOSAIL_TEST_TOTP_SECRET ?? "")].map((character) => alphabet.indexOf(character).toString(2).padStart(5, "0")).join("");
  const secret = Buffer.from(bits.match(/.{8}/g)?.map((byte) => Number.parseInt(byte, 2)) ?? []);
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30_000)));
  const digest = createHmac("sha1", secret).update(counter).digest();
  const offset = digest[19] & 15;
  return (((digest.readUInt32BE(offset) & 0x7fffffff) % 1_000_000).toString().padStart(6, "0"));
}

export async function login(page: Page, info: TestInfo = test.info()) {
  const navigation = navigationDiagnostics(page, info.project.use.baseURL);
  const recorded = new Set<string>();
  try {
    let setupTimer: ReturnType<typeof setTimeout> | undefined;
    try {
      await Promise.race([navigation.observeDocument((record: {kind: string}) => {
        if (recorded.has(record.kind)) return;
        recorded.add(record.kind);
        void info.attach("login-document-" + record.kind, {contentType: "application/json", body: JSON.stringify(record)}).catch(() => {});
      }), new Promise(resolve => {setupTimer = setTimeout(resolve, 500);})]);
    } catch { /* Observation setup cannot prevent the original navigation. */ }
    finally {clearTimeout(setupTimer);}
    navigation.markNavigation("/login");
    await page.goto("/login", { waitUntil: "commit" });
  } catch (error) {
    let timer: ReturnType<typeof setTimeout> | undefined;
    try {
      const body = JSON.stringify(await navigation.snapshot(error));
      await Promise.race([info.attach("login-navigation-failure", {contentType: "application/json", body}),
        new Promise(resolve => {timer = setTimeout(resolve, 500);})]);
    } catch { /* Diagnostics cannot replace the original navigation failure. */ }
    finally {clearTimeout(timer);}
    throw error;
  } finally {navigation.stop();}
  await expect(page.getByLabel("Name")).toBeVisible();
  await page.getByLabel("Name").fill(process.env.KINOSAIL_E2E_OWNER_NAME ?? "Owner");
  await page.getByLabel("Password", { exact: true }).fill(process.env.KINOSAIL_E2E_OWNER_PASSWORD ?? "test-instance-password");
  await page.getByLabel("6-digit code").fill(totp());
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page.waitForURL((url) => url.pathname !== "/login");
  if (new URL(page.url()).pathname === "/account") await page.getByRole("link", { name: "Not now" }).click();
}

export async function loginViewer(page: Page, name: string, password: string) {
  await page.goto("/login", { waitUntil: "domcontentloaded" });
  await expect(page.getByLabel("Name")).toBeVisible();
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
  const context = await browser.newContext({ baseURL, ignoreHTTPSErrors: false });
  await context.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false }));
  return context.newPage();
}
