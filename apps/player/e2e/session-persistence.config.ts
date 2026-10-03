import {defineConfig, devices} from "@playwright/test";

// Local synthetic authentication checks never accept a bad HTTPS certificate.
export default defineConfig({
  testDir: process.env.KINOSAIL_SESSION_TEST_DIR ?? ".", timeout: 60_000, workers: 1, reporter: "list",
  use: {baseURL: process.env.KINOSAIL_E2E_URL ?? "http://localhost:38127", ignoreHTTPSErrors: false, trace: "off", screenshot: "off", video: "off"},
  projects: [{name: "chromium", use: {...devices["Desktop Chrome"], channel: "chrome"}},
    {name: "webkit", use: {...devices["Desktop Safari"]}}],
});
