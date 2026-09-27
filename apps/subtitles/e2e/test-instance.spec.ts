import { expect, test } from "@playwright/test";
import { createViewer, login, loginViewer, newViewerPage, removeViewer } from "./test-instance-helpers";
import { registerTestInstanceLibraryTests } from "./test-instance-library-tests";
import { registerTestInstancePlaybackTests } from "./test-instance-playback-tests";
import { registerTestInstanceShellTests } from "./test-instance-shell-tests";
import { registerTestInstanceSupporterTests } from "./test-instance-supporter-tests";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
test.beforeEach(async ({ page }) => page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false })));
test.beforeEach(async ({ page }) => login(page));

registerTestInstanceShellTests();
registerTestInstanceSupporterTests();
registerTestInstanceLibraryTests();
registerTestInstancePlaybackTests();

test("Viewer MFA enrollment gives account-neutral instructions", async ({ browser, page }, testInfo) => {
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
