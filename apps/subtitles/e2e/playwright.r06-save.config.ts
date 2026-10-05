import { defineConfig } from "@playwright/test";
import { isAbsolute } from "node:path";

const output = process.env.R06_SAVE_PRIVATE_OUTPUT;
if (!output || !isAbsolute(output)) throw new Error("fixed-private-output-boundary");

export default defineConfig({
  testDir: ".",
  testMatch: "subtitle-save-recovery.journey.ts",
  timeout: 70000,
  expect: { timeout: 5000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  forbidOnly: true,
  reporter: [["./subtitle-save-recovery-reporter.ts"]],
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
