import type { Cookie, Page } from "@playwright/test";

const sessions = new Map<string, Cookie[]>();

function instanceOrigin(): string {
  const url = process.env.KINOSAIL_E2E_URL;
  return url && process.env.KINOSAIL_TEST_TOTP_SECRET ? new URL(url).origin : "";
}

async function targetsInstance(page: Page, origin: string): Promise<boolean> {
  const response = await page.request.get("/healthz", { maxRedirects: 0, failOnStatusCode: false });
  return new URL(response.url()).origin === origin;
}

export async function reuseSignedInSession(page: Page, account: string, signIn: () => Promise<void>) {
  const origin = instanceOrigin();
  const key = `${origin} ${account}`;
  const cookies = origin ? sessions.get(key) : undefined;
  if (cookies && await targetsInstance(page, origin)) {
    await page.context().addCookies(cookies);
    const response = await page.goto("/");
    if (response?.ok() && new URL(page.url()).pathname !== "/login") return;
    sessions.delete(key);
    await page.context().clearCookies();
  }
  await signIn();
  if (origin && new URL(page.url()).origin === origin) sessions.set(key, await page.context().cookies(origin));
}
