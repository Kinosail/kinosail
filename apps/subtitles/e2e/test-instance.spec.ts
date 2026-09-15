import { test } from "@playwright/test";
import { login } from "./test-instance-helpers";
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
