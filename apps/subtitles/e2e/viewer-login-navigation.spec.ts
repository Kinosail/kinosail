import {expect, test, type Page} from "@playwright/test";
import {createServer} from "node:http";
import {readFile} from "node:fs/promises";
import {login, loginViewer} from "./test-instance-helpers";

const theme = await readFile(new URL("../../../packages/webassets/static/theme.js", import.meta.url), "utf8");

for (const account of ["Owner", "Viewer"] as const) test(`${account} login preserves initialized native form evidence before Firefox recovery`, {tag: "@smoke"}, async ({browser}, info) => {
  let gets = 0, posts = 0, calls = 0;
  const server = createServer((request, response) => {
    const path = new URL(request.url!, "http://fixture").pathname;
    if (path === "/theme.js") { response.writeHead(200, {"Content-Type": "text/javascript"}); response.end(theme); }
    else if (path === "/login" && request.method === "GET") {
      gets++;
      response.writeHead(200, {"Content-Type": "text/html"});
      response.end('<!doctype html><head><script defer src="/theme.js"></script></head><body><form method="post"><label>Name<input name="name"></label><label>Password<input type="password" name="password"></label><label>6-digit code<input name="code"></label><button>Sign in</button></form></body>');
    } else if (path === "/login" && request.method === "POST") {
      posts++;
      response.writeHead(200, {"Content-Type": "text/html"});
      response.end('<meta http-equiv="refresh" content="0;url=/">');
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
    await page.waitForFunction(() => document.readyState === "complete");
    if (calls === 1 && browser.browserType().name() === "firefox") {
      const error = new Error("Controlled navigation bookkeeping failure after a real initialized login");
      error.name = "TimeoutError";
      throw error;
    }
    return result;
  };
  try {
    if (account === "Owner") await login(page);
    else await loginViewer(page, "Synthetic Viewer", "synthetic-password");
    await expect(page.getByRole("heading", {name: "Fixture home"})).toBeVisible();
    expect(gets).toBe(browser.browserType().name() === "firefox" ? 2 : 1);
    expect(calls).toBe(gets);
    expect(posts).toBe(1);
    await info.attach("account-login-navigation", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
      account, gets, posts, calls, browser: browser.browserType().name(), version: browser.version(),
      data: "Synthetic native HTTP form, actual production theme, one injected automation timeout", outcome: "passed"}), contentType: "application/json"});
  } finally {
    await context.close();
    server.closeAllConnections();
    await new Promise<void>(resolve => server.close(() => resolve()));
  }
});
