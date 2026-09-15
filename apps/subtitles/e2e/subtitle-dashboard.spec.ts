import { test } from "@playwright/test";
import { login } from "./subtitle-dashboard-helpers";
import { registerSubtitleLanguageTests } from "./subtitle-dashboard-language-tests";
import { registerSubtitleLayoutTests } from "./subtitle-dashboard-layout-tests";
import { registerSubtitleProviderTests } from "./subtitle-dashboard-provider-tests";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated Kinosail Subtitles test instance");
test.describe.configure({ mode: "serial", timeout: 120_000 });
test.beforeEach(async ({ page }) => login(page));

registerSubtitleLanguageTests();
registerSubtitleProviderTests();
registerSubtitleLayoutTests();
