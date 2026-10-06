import { test, expect } from "@playwright/test";
import { runSaveCase, ASSERTION_IDS } from "./subtitle-save-recovery-helpers";

const CASES = [
  { id: "save-headers-phone", title: "R06 Save headers held after completed write - phone", mode: "headers", width: 390 },
  { id: "save-headers-desktop", title: "R06 Save headers held after completed write - desktop", mode: "headers", width: 1440 },
  { id: "save-body-phone", title: "R06 Save body held after completed write - phone", mode: "body", width: 390 },
  { id: "save-body-desktop", title: "R06 Save body held after completed write - desktop", mode: "body", width: 1440 },
] as const;

// Collection is unconditional. Root selects one exact two-case suite with --grep.
// Failures never skip the later navigation exercise: the helper records fixed
// observations, releases the old response in the same document, then navigates.
for (const entry of CASES) {
  test(entry.title, async ({ page }, testInfo) => {
    const result = await runSaveCase(page, testInfo, entry);
    await testInfo.attach("r06-save-browser-v1", {
      body: Buffer.from(JSON.stringify(result)),
      contentType: "application/json",
    });
    expect.soft(result.eligible, "eligible-real-save-history-transport").toBe(true);
    for (const id of ASSERTION_IDS) {
      expect.soft(result.assertions[id].attempted, id + "-attempted").toBe(true);
      expect.soft(result.assertions[id].passed, id).toBe(true);
    }
    expect.soft(result.fixtureStopped, "owned-fixture-stopped").toBe(true);
  });
}
