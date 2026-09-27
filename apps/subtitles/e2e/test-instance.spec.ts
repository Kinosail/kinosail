import { expect, test } from "@playwright/test";
import { createViewer, login, loginViewer, newViewerPage, removeViewer } from "./test-instance-helpers";
import { registerTestInstanceSupporterTests } from "./test-instance-supporter-tests";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
test.beforeEach(async ({ page }) => page.addInitScript(() => {
  if ("PublicKeyCredential" in window) Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false });
}));
registerTestInstanceSupporterTests();

test("Viewer MFA enrollment gives account-neutral instructions @smoke", async ({ browser, page }, testInfo) => {
  await login(page);
  await page.goto("/settings");
  expect((await (await page.context().request.get("/api/v1/settings")).json()).requireMfa).toBe(true);
  const name = `QA Viewer ${Date.now()}`;
  const password = "qa-viewer-password";
  const id = await createViewer(page, name, password);
  const viewer = await newViewerPage(browser, new URL(page.url()).origin);
  try {
    await loginViewer(viewer, name, password);
    await expect(viewer).toHaveURL(/\/account\?mfa=required$/);
    await expect(viewer.getByRole("heading", { name: "Extra sign-in protection is required" })).toBeVisible();
    await viewer.screenshot({ path: testInfo.outputPath("viewer-mfa-required.png") });
    await expect(viewer.getByText("Add a passkey or authenticator app to continue.", { exact: true })).toBeVisible();
    await expect(viewer.getByText("Owner account")).toHaveCount(0);
  } finally {
    await viewer.context().close();
    await removeViewer(page, id);
  }
});

test("Owner authorizes a waiting device with Quick Connect", async ({ page, request }) => {
  await login(page);
  const started = await request.post("/api/v1/quick-connect", { data: { device: "E2E subtitles client" } });
  expect(started.status()).toBe(201);
  const pending = await started.json() as { code: string; secret: string };
  await page.goto("/quick-connect");
  await page.getByLabel("Code").fill(pending.code);
  await page.getByRole("button", { name: "Authorize device" }).click();
  await expect(page).toHaveURL("/");
  const completed = await request.post("/api/v1/quick-connect/token", { data: { secret: pending.secret } });
  expect(completed.status()).toBe(201);
  expect((await completed.json()).token).toEqual(expect.any(String));
});
