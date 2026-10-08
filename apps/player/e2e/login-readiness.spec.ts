import {expect, test} from "@playwright/test";
import {readFile} from "node:fs/promises";
import {createHash} from "node:crypto";
import {login} from "./test-instance-helpers";

const theme = await readFile(new URL("../../../packages/webassets/static/theme.js", import.meta.url), "utf8");
test.use({serviceWorkers: "block"});

for (const scriptState of ["delayed", "missing"]) {
  test(`login waits for initialized password controls when its script is ${scriptState}`, {tag: ["@smoke", "@routed-fault"]}, async ({page}, info) => {
    let release!: () => void, requested = false, posts = 0;
    const barrier = new Promise<void>(resolve => release = resolve);
    await page.route("**/*", async route => {
      const url = new URL(route.request().url());
      if (url.pathname === "/theme-readiness.js") {
        requested = true;
        if (scriptState === "delayed") { await barrier; await route.fulfill({contentType: "text/javascript", body: theme}); }
        else await route.fulfill({status: 404, body: "Missing script"});
      } else if (url.pathname === "/login" && route.request().method() === "POST") {
        posts++;
        await route.fulfill({contentType: "text/html", body: '<meta http-equiv="refresh" content="0;url=/">'});
      } else if (url.pathname === "/login") {
        await route.fulfill({contentType: "text/html", body: '<!doctype html><head><script defer src="/theme-readiness.js"></script></head><body><form action="/login" method="post"><label>Name<input name="name"></label><label>Password<input name="password" type="password"></label><label>Authentication or recovery code<input name="factor"></label><button>Sign in</button></form></body>'});
      } else await route.fulfill({contentType: "text/html", body: "<h1>Fixture home</h1>"});
    });
    // Attach the rejection handler immediately so a missing-script failure is observed.
    const attempt = login(page).then(() => ({ok: true, error: ""}), error => ({ok: false, error: String(error)}));
    try {
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
        data: "Synthetic native form and actual production theme; delayed/missing response", result: "passed"}), contentType: "application/json"});
    } finally { release(); await attempt; if (!page.isClosed()) await page.unrouteAll({behavior: "ignoreErrors"}); }
  });
}
