import { expect, test, type Page } from "@playwright/test";

// The populated fixture shares one IP limit: 30 credential attempts per minute.
// Pace ordinary successful logins, not limiter or rejected-credential journeys.
export async function submitFixtureLogin(page: Page, code?: () => string) {
  const info = test.info();
  const pacingMs = process.env.KINOSAIL_TEST_INSTANCE === "1" ? 2100 : 0;
  if (pacingMs) {
    expect(info.config.workers, "use one worker per populated test instance").toBe(1);
    await new Promise(resolve => setTimeout(resolve, pacingMs));
  }
  // Generate rotating codes after pacing so the wait cannot age a captured code.
  if (code) await page.getByLabel("Authentication or recovery code").fill(code());
  const origin = new URL(page.url()).origin;
  const [response] = await Promise.all([
    page.waitForResponse(response => response.request().method() === "POST" &&
      new URL(response.url()).origin === origin && new URL(response.url()).pathname === "/login"),
    page.getByRole("button", { name: "Sign in", exact: true }).click(),
  ]);
  await info.attach("fixture-login-response", { contentType: "application/json", body: JSON.stringify({
    route: "/login", status: response.status(), submitActions: 1, pacingMs,
  }) });
  expect(response.status(), "fixture sign-in must succeed without a retry").toBe(303);
}
