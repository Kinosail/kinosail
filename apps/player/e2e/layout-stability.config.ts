import {defineConfig, devices} from "@playwright/test";

// Geometry fixtures use loopback HTTP. No insecure TLS options.
export default defineConfig({
  testDir: ".", workers: 1, timeout: 30000,
  use: {baseURL: "http://localhost:31808", ignoreHTTPSErrors: false, trace: "retain-on-failure"},
  outputDir: process.env.KINOSAIL_E2E_OUTPUT_DIR || "../../../.verification/layout-fixtures",
  projects: [
    {name: "chromium", use: {...devices["Desktop Chrome"], ...(process.platform === "darwin" ? {channel: "chrome"} : {})}},
    ...(process.env.KINOSAIL_BROWSER_MATRIX === "full" ? [
      {name: "webkit", use: {...devices["Desktop Safari"]}},
      {name: "firefox", use: {...devices["Desktop Firefox"]}},
    ] : []),
  ],
});
