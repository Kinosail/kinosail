import type { Cookie, Page } from "@playwright/test";

const sessions = new Map<string, Cookie[]>();

export async function reuseSignedInSession(page: Page, account: string, signIn: () => Promise<void>) {
  const cookies = sessions.get(account);
  if (cookies) {
    await page.context().addCookies(cookies);
    const response = await page.goto("/");
    if (response?.ok() && new URL(page.url()).pathname !== "/login") return;
    sessions.delete(account);
    await page.context().clearCookies();
  }
  await signIn();
  sessions.set(account, await page.context().cookies());
}
