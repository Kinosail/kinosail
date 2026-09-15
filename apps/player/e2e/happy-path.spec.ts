import { test } from "@playwright/test";
import { completeHappyPath } from "./happy-path-complete";
import { startHappyPath } from "./happy-path-setup";

test("Owner can set up, create a passkey, find, play, resume, curate, install, and sign back in", async ({ page }, testInfo) => {
  const state = await startHappyPath(page, testInfo);
  await completeHappyPath(page, testInfo, state);
});
