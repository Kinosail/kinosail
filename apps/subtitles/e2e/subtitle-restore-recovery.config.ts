import { defineConfig } from "@playwright/test";
import { isAbsolute } from "node:path";

const output = process.env.R06_RESTORE_PRIVATE_OUTPUT;
if (!output || !isAbsolute(output)) throw new Error("fixed-private-output-boundary");

export default defineConfig({
  testDir: ".",
  testMatch: "subtitle-restore-recovery.journey.ts",
  timeout: 110000,
  expect: { timeout: 5000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  forbidOnly: true,
  reporter: [["./subtitle-restore-proof-reporter.ts"]],
  outputDir: output,
  use: {
    browserName: "chromium",
    headless: true,
    ignoreHTTPSErrors: true,
    locale: "en-US",
    trace: "off",
    video: "off",
    screenshot: "off",
  },
});
