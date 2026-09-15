import { test } from "@playwright/test";
import { registerLayoutEntryTests } from "./layout-audit-entry-tests";
import { registerLayoutMediaTests } from "./layout-audit-media-tests";
import { registerLayoutSettingsTests } from "./layout-audit-settings-tests";
import { registerLayoutWorkflowTests } from "./layout-audit-workflow-tests";

test.beforeEach(async ({ page }) => page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false })));

registerLayoutEntryTests();
registerLayoutMediaTests();
registerLayoutSettingsTests();
registerLayoutWorkflowTests();
