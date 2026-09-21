import { test } from "@playwright/test";
import { completeHappyPath } from "./happy-path-complete";
import { startHappyPath } from "./happy-path-setup";

// This journey persists Owner/MFA state, so replaying it cannot retry the same setup.
test.describe.configure({ retries: 0 });

test("Owner can set up, create a passkey, find, play, resume, curate, install, and sign back in", async ({ page }, testInfo) => {
  // Full onboarding, playback, accessibility, and offline checks share this budget.
  test.setTimeout(90_000);
  const state = await startHappyPath(page, testInfo);
  await completeHappyPath(page, testInfo, state);
});
