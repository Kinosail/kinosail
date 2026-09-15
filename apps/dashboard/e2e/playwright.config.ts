import { defineConfig, devices } from "@playwright/test";

const listenAddress = process.env.KINOSAIL_DASHBOARD_E2E_LISTEN ?? "127.0.0.1:38491";
const listenPort = listenAddress.slice(listenAddress.lastIndexOf(":") + 1);
const baseURL = process.env.KINOSAIL_DASHBOARD_E2E_URL ?? `http://localhost:${listenPort}`;

export default defineConfig({
  testDir: ".",
  outputDir: "test-results",
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    // CDP WebAuthn and deterministic browser timing are Chromium-only contracts.
    { name: "firefox", grepInvert: /@chromium/, use: { ...devices["Desktop Firefox"] } },
    { name: "mobile-webkit", grepInvert: /@chromium/, use: { ...devices["iPhone 13"] } },
  ],
  webServer: process.env.KINOSAIL_DASHBOARD_E2E_URL ? undefined : {
    command: `KINOSAIL_DASHBOARD_E2E_LISTEN=${listenAddress} exec ./scripts/start-test-server.sh`,
    cwd: ".",
    url: `${baseURL}/healthz`,
    timeout: 120_000,
    reuseExistingServer: false,
    gracefulShutdown: { signal: "SIGTERM", timeout: 5_000 },
  },
});
