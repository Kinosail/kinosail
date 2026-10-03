import {defineConfig, devices} from "@playwright/test";

// The disposable runner serves supported loopback HTTP. No TLS bypass flags.
export default defineConfig({
  testDir: ".", timeout: 60_000, expect: {timeout: 10_000}, workers: 1,
  outputDir: process.env.KINOSAIL_E2E_OUTPUT_DIR, reporter: [["list"], ["json", {outputFile: process.env.KINOSAIL_E2E_REPORT}]],
  use: {baseURL: process.env.KINOSAIL_E2E_URL, ignoreHTTPSErrors: false, trace: "on", screenshot: "only-on-failure"},
  projects: [{name: "chromium", use: {...devices["Desktop Chrome"], channel: process.env.KINOSAIL_STARTUP_BROWSER_CHANNEL || "chrome"}}],
});
