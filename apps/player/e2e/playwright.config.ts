import { defineConfig, devices } from "@playwright/test";

const channel = process.env.PLAYWRIGHT_CHANNEL ?? (!process.env.CI && process.platform === "darwin" ? "chrome" : undefined);
const fullMatrix = process.env.KINOSAIL_BROWSER_MATRIX === "full";
const configuredWorkers = Number(process.env.KINOSAIL_BROWSER_WORKERS ?? 0);
const selectedProject = process.env.KINOSAIL_BROWSER_PROJECT;
const supportedProjects = ["chromium", "firefox", "webkit"];
if (selectedProject && !supportedProjects.includes(selectedProject)) throw new Error(`Unsupported browser project: ${selectedProject}`);
const extendedProjects = fullMatrix || selectedProject === "firefox" || selectedProject === "webkit";
const projects = [
	{ name: "chromium", use: { ...devices["Desktop Chrome"], ...(channel ? { channel } : {}), launchOptions: { args: ["--allow-insecure-localhost", "--ignore-certificate-errors"] } } },
	...(extendedProjects ? [
    { name: "firefox", use: { ...devices["Desktop Firefox"] } },
    { name: "webkit", use: { ...devices["Desktop Safari"] } },
  ] : []),
];

export default defineConfig({
  testDir: ".",
  outputDir: process.env.KINOSAIL_E2E_OUTPUT_DIR ?? "test-results",
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: true,
  workers: configuredWorkers || (fullMatrix ? 1 : undefined),
  forbidOnly: Boolean(process.env.CI),
  failOnFlakyTests: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: process.env.KINOSAIL_E2E_URL ?? "https://127.0.0.1:38127",
    ignoreHTTPSErrors: true,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: process.env.KINOSAIL_E2E_VIDEO === "off" ? "off" : "retain-on-failure",
  },
  projects: projects.filter(({ name }) => !selectedProject || name === selectedProject),
});
