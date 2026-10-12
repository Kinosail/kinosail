import {expect, test, type Page} from "@playwright/test";
import {readFile} from "node:fs/promises";
import {createHash} from "node:crypto";
import {createServer} from "node:http";
import {login} from "./test-instance-helpers";

const theme = await readFile(new URL("../../../packages/webassets/static/theme.js", import.meta.url), "utf8");
for (const scriptState of ["delayed", "missing"]) {
  test(`login waits for initialized password controls when its script is ${scriptState}`, {tag: ["@smoke", "@routed-fault"]}, async ({browser}, info) => {
    let release!: () => void, requested = false, posts = 0;
    const barrier = new Promise<void>(resolve => release = resolve);
    // Serve the complete redirect chain: intercepted redirects can reach the
    // unrelated configured Server before Playwright routes the next document.
    const server = createServer(async (request, response) => {
      const url = new URL(request.url!, "http://fixture");
      if (url.pathname === "/theme-readiness.js") {
        requested = true;
        if (scriptState === "delayed") { await barrier; response.writeHead(200, {"Content-Type": "text/javascript"}); response.end(theme); }
        else { response.writeHead(404); response.end("Missing script"); }
      } else if (url.pathname === "/login" && request.method === "POST") {
        posts++;
        response.writeHead(303, {Location: "/"}); response.end();
      } else if (url.pathname === "/login") {
        response.writeHead(200, {"Content-Type": "text/html"});
        response.end('<!doctype html><head><script defer src="/theme-readiness.js"></script></head><body><form action="/login" method="post"><label>Name<input name="name"></label><label>Password<input name="password" type="password"></label><label>Authentication or recovery code<input name="factor"></label><button>Sign in</button></form></body>');
      } else { response.writeHead(200, {"Content-Type": "text/html"}); response.end("<h1>Fixture home</h1>"); }
    });
    await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
    const origin = `http://127.0.0.1:${(server.address() as {port: number}).port}`;
    let page: Page | undefined;
    let attempt: Promise<{ok: boolean; error: string}> | undefined;
    try {
      page = await browser.newPage({baseURL: origin, serviceWorkers: "block"});
      // Observe missing-script failures immediately, before checking controls.
      attempt = login(page).then(() => ({ok: true, error: ""}), error => ({ok: false, error: String(error)}));
      await expect.poll(() => requested).toBe(true);
      if (scriptState === "delayed") {
        await expect(page.getByLabel("Name", {exact: true})).toBeVisible();
        await expect(page.getByRole("button", {name: "Show secret", exact: true})).toHaveCount(0);
        await expect(page.getByLabel("Name", {exact: true})).toHaveValue("");
        await expect(page.getByLabel("Password", {exact: true})).toHaveValue("");
        expect(posts).toBe(0);
        release();
        expect(await attempt).toEqual({ok: true, error: ""});
        await expect(page.getByRole("heading", {name: "Fixture home"})).toBeVisible();
        expect(posts).toBe(1);
      } else {
        const result = await attempt;
        await info.attach("missing-script-side-effects", {body: JSON.stringify({posts, result}), contentType: "application/json"});
        expect(result.ok).toBe(false);
        await expect(page.getByLabel("Name", {exact: true})).toHaveValue("");
        await expect(page.getByLabel("Password", {exact: true})).toHaveValue("");
        expect(posts).toBe(0);
      }
      await info.attach("native-login-script-readiness", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
        scriptState, posts, browser: info.project.name, themeSHA256: createHash("sha256").update(theme).digest("hex"),
        data: "Disposable HTTP peer with native form, real redirects, and production theme; delayed/missing script", result: "passed"}), contentType: "application/json"});
    } finally {
      release(); await attempt;
      try { await page?.close(); } finally {
        server.closeAllConnections();
        await new Promise<void>(resolve => server.close(() => resolve()));
      }
    }
  });
}
