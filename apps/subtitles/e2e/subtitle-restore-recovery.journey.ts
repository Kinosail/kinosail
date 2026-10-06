import { test, expect } from "@playwright/test";
import { runRestoreCase, ASSERTION_IDS } from "./subtitle-restore-recovery-helpers";

const CASES = [
  { id: "r06-restore-headers-desktop", title: "R06 Restore held headers releases desktop editor", mode: "headers", width: 1280, height: 900 },
  { id: "r06-restore-headers-phone", title: "R06 Restore held headers releases phone editor", mode: "headers", width: 390, height: 844 },
  { id: "r06-restore-inspect-body-desktop", title: "R06 Restore held inspection body releases desktop editor", mode: "inspect-body", width: 1280, height: 900 },
  { id: "r06-restore-inspect-body-phone", title: "R06 Restore held inspection body releases phone editor", mode: "inspect-body", width: 390, height: 844 },
] as const;

// Discovery is unconditional; root selects one exact pair. Soft expectations
// preserve navigation/settlement evidence after an intended deadline failure.
for (const entry of CASES) {
  test(entry.title, async ({ page }, testInfo) => {
    const result = await runRestoreCase(page, entry);
    await testInfo.attach("r06-restore-browser-v1", {
      body: Buffer.from(JSON.stringify(result)),
      contentType: "application/json",
    });
    expect.soft(result.eligible, "eligible-real-restore-history-transport").toBe(true);
    for (const id of ASSERTION_IDS) {
      expect.soft(result.assertions[id].attempted, id + "-attempted").toBe(true);
      expect.soft(result.assertions[id].completed, id + "-completed").toBe(true);
      expect.soft(result.assertions[id].passed, id).toBe(true);
    }
    expect.soft(result.fixtureStopped, "owned-fixture-stopped").toBe(true);
  });
}
