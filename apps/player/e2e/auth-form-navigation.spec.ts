import {expect, test, type Page} from "@playwright/test";
import {createServer} from "node:http";
import {readFile} from "node:fs/promises";
import {createHash} from "node:crypto";
import {gotoAuthForm} from "../../../scripts/testing/auth-form-navigation";

const theme = await readFile(new URL("../../../packages/webassets/static/theme.js", import.meta.url), "utf8");
test.use({serviceWorkers: "block"});
const scenarios = ["ready", "missing-script", "missing-name", "missing-password", "disabled-submit", "disabled-fieldset", "override-action", "override-method", "override-target", "form-target", "wrong-action", "pending-script", "normal-delayed-script", "http-error", "redirect", "slow-response", "other-error", "repeated-timeout"] as const;

for (const path of ["/login", "/login?next=/", "/setup"] as const) for (const scenario of scenarios) {
  if (scenario === "normal-delayed-script" && path !== "/setup") continue;
  test(`${path} navigation recovery preserves the first document's ${scenario} evidence`, {tag: "@smoke"}, async ({browser}, info) => {
    const formPath = new URL(path, "http://fixture").pathname;
    const submitLabel = formPath === "/setup" ? "Create Owner & continue" : "Sign in";
    let gets = 0, posts = 0, calls = 0, settled = false, release!: () => void;
    const barrier = new Promise<void>(resolve => release = resolve);
    const server = createServer(async (request, response) => {
      const path = new URL(request.url!, "http://fixture").pathname;
      if (path === "/theme.js") {
        if (scenario === "pending-script" || scenario === "normal-delayed-script") await barrier;
        response.writeHead(scenario === "missing-script" ? 404 : 200, {"Content-Type": "text/javascript"});
        response.end(scenario === "missing-script" ? "" : theme);
      } else if (path === formPath && request.method === "POST") {
        posts++;
        response.writeHead(200, {"Content-Type": "text/html"});
        response.end('<meta http-equiv="refresh" content="0;url=/">');
      } else if (path === formPath) {
        gets++;
        if (scenario === "slow-response") await new Promise(resolve => setTimeout(resolve, 2100));
        if (scenario === "redirect") { response.writeHead(302, {Location: "/different"}); response.end(); return; }
        const name = scenario === "missing-name" ? "" : '<label>Name<input name="name"></label>';
        const password = scenario === "missing-password" ? "" : '<label>Password<input type="password" name="password"></label>';
        const override = scenario === "override-action" ? 'formaction="/different"' : scenario === "override-method" ? 'formmethod="get"' : scenario === "override-target" ? 'formtarget="_blank"' : "";
        response.writeHead(scenario === "http-error" ? 500 : 200, {"Content-Type": "text/html"});
        response.end(`<!doctype html><head><script defer src="/theme.js"></script></head><body><h1>${formPath === "/setup" ? "Set up your Server." : "Kinosail"}</h1><form method="post" ${scenario === "wrong-action" ? 'action="/different"' : ""} ${scenario === "form-target" ? 'target="_blank"' : ""}>${scenario === "disabled-fieldset" ? "<fieldset disabled>" : ""}${name}${password}<button ${scenario === "disabled-submit" ? "disabled" : ""} ${override}>${formPath === "/setup" ? "Create Owner &amp; continue" : "Sign in"}</button>${scenario === "disabled-fieldset" ? "</fieldset>" : ""}</form></body>`);
      } else { response.writeHead(200, {"Content-Type": "text/html"}); response.end("<h1>Fixture home</h1>"); }
    });
    await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
    const origin = `http://127.0.0.1:${(server.address() as {port: number}).port}`;
    const context = await browser.newContext({baseURL: origin, serviceWorkers: "block"});
    const page = await context.newPage();
    const original = page.goto.bind(page);
    page.goto = async (...args: Parameters<Page["goto"]>) => {
      calls++;
      const result = await original(...args);
      if (scenario === "normal-delayed-script") return result;
      if (scenario !== "pending-script") await page.waitForFunction(() => document.readyState === "complete");
      if (calls === 1 || scenario === "repeated-timeout") {
        const error = new Error("Controlled navigation bookkeeping failure after a real HTTP document");
        error.name = scenario === "other-error" ? "Error" : "TimeoutError";
        throw error;
      }
      return result;
    };
    try {
      const eligible = scenario === "normal-delayed-script" || browser.browserType().name() === "firefox" && scenario === "ready";
      const attempt = gotoAuthForm(page, path, info).then(response => ({ok: true, status: response?.status()}), error => ({ok: false, error: String(error)})).finally(() => {settled = true;});
      if (scenario === "normal-delayed-script") {
        await expect(page.getByRole("heading", {name: "Set up your Server.", exact: true})).toBeVisible();
        expect(settled).toBe(false);
        expect(await page.getByRole("button", {name: "Show secret", exact: true}).count()).toBe(0);
        expect(posts).toBe(0);
        release();
      }
      const result = await attempt;
      expect(result.ok).toBe(eligible);
      expect(gets).toBe(browser.browserType().name() === "firefox" && (scenario === "ready" || scenario === "repeated-timeout") ? 2 : 1);
      expect(calls).toBe(gets);
      if (eligible) {
        expect(result).toEqual({ok: true, status: 200});
        await expect(page).toHaveURL(origin + path);
        if (formPath === "/setup") expect(await page.getByRole("heading", {name: "Set up your Server.", exact: true}).isVisible()).toBe(true);
        await page.getByLabel("Name", {exact: true}).fill("Synthetic Owner");
        await page.getByLabel("Password", {exact: true}).fill("synthetic-password");
        await page.getByRole("button", {name: submitLabel, exact: true}).click();
        await expect(page.getByRole("heading", {name: "Fixture home"})).toBeVisible();
        expect(posts).toBe(1);
      } else {
        expect(posts).toBe(0);
        if (scenario !== "redirect" && scenario !== "missing-password") expect(await page.locator('input[name="password"]').inputValue()).toBe("");
        if (scenario === "redirect") expect(await page.locator("input").count()).toBe(0);
        if (scenario !== "missing-name" && scenario !== "redirect") expect(await page.locator('input[name="name"]').inputValue()).toBe("");
      }
      await info.attach("login-navigation-control", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
        path, scenario, calls, gets, posts, result, browser: browser.browserType().name(), version: browser.version(),
        themeSHA256: createHash("sha256").update(theme).digest("hex"),
        data: "Synthetic native HTTP form with actual production theme; injected automation timeout", outcome: "passed"}), contentType: "application/json"});
    } finally {
      release();
      await context.close();
      server.closeAllConnections();
      await new Promise<void>(resolve => server.close(() => resolve()));
    }
  });
}
