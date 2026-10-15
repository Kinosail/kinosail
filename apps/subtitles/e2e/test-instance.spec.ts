import { test } from "@playwright/test";
import { registerTestInstanceSupporterTests } from "./test-instance-supporter-tests";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
test.beforeEach(async ({ page }) => page.addInitScript(() => {
  if ("PublicKeyCredential" in window) Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false });
}));
registerTestInstanceSupporterTests();
