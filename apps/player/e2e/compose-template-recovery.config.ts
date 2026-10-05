import { defineConfig } from "@playwright/test";

const suite = process.env.KINOSAIL_Q47_SUITE ?? "primary";
if (!["primary", "recovery", "supersession", "contracts"].includes(suite)) {
  throw new Error("Q47 configuration: invalid suite");
}

export default defineConfig({
  testDir: ".",
  testMatch: "compose-template-recovery.journey.ts",
  outputDir: process.env.KINOSAIL_E2E_OUTPUT_DIR ?? "test-results-q47",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 35_000,
  globalTimeout: 70_000,
  forbidOnly: true,
  failOnFlakyTests: true,
  reporter: [["./compose-template-proof-reporter.ts"]],
  use: {
    baseURL: "http://127.0.0.1:41847",
    browserName: "chromium",
    headless: true,
    ignoreHTTPSErrors: false,
    trace: "off",
    screenshot: "off",
    video: "off",
  },
  projects: [{ name: "chromium" }],
});
