import {expect, test, type Page} from "@playwright/test";
import {readFile} from "node:fs/promises";
import {createHash} from "node:crypto";
import {createServer} from "node:http";
import {firstPlayable, login} from "./test-instance-helpers";
import {login as layoutLogin} from "./layout-audit-helpers";

const theme = await readFile(new URL("../../../packages/webassets/static/theme.js", import.meta.url), "utf8");
for (const scriptState of ["delayed", "missing"]) {
  test(`login waits for initialized password controls when its script is ${scriptState}`, {tag: ["@smoke", "@routed-fault"]}, async ({browser}, info) => {
    let release!: () => void, requested = false, posts = 0;
    const barrier = new Promise<void>(resolve => release = resolve);
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

for (const helper of [{name: "populated", run: login}, {name: "layout", run: layoutLogin}]) for (const homeState of ["delayed", "blocked"] as const) {
  test(`${helper.name === "layout" ? "layout " : ""}login finishes the Not now destination before Library when Home is ${homeState}`, {tag: "@smoke"}, async ({browser}, info) => {
    let release!: () => void, homeRequested = false, posts = 0, homeGets = 0, libraryGets = 0;
    const barrier = new Promise<void>(resolve => release = resolve);
    const server = createServer(async (request, response) => {
      const url = new URL(request.url!, "http://fixture");
      if (url.pathname === "/theme.js") {
        response.writeHead(200, {"Content-Type": "text/javascript"}); response.end(theme);
      } else if (url.pathname === "/login" && request.method === "POST") {
        posts++; response.writeHead(303, {Location: "/account"}); response.end();
      } else if (url.pathname === "/login") {
        response.writeHead(200, {"Content-Type": "text/html"});
        response.end('<!doctype html><head><script defer src="/theme.js"></script></head><body><form action="/login" method="post"><label>Name<input name="name"></label><label>Password<input name="password" type="password"></label><label>Authentication or recovery code<input name="factor"></label><button>Sign in</button></form></body>');
      } else if (url.pathname === "/account") {
        response.writeHead(200, {"Content-Type": "text/html"}); response.end('<a href="/">Not now</a>');
      } else if (url.pathname === "/home-ready.js") {
        homeRequested = true; await barrier;
        response.writeHead(200, {"Content-Type": "text/javascript"}); response.end("window.fixtureHomeLoaded = true;");
      } else if (url.pathname === "/" && url.searchParams.get("view") === "movies") {
        libraryGets++; response.writeHead(200, {"Content-Type": "text/html"});
        response.end('<a class="card" href="/watch/fictional">Example Movie</a>');
      } else if (url.pathname === "/") {
        homeGets++; response.writeHead(200, {"Content-Type": "text/html"});
        response.end('<!doctype html><head><script defer src="/home-ready.js"></script></head><body><h1>Fixture home</h1></body>');
      } else { response.writeHead(404); response.end(); }
    });
    await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
    const origin = `http://127.0.0.1:${(server.address() as {port: number}).port}`;
    let page: Page | undefined, loginReturned = false;
    let home: {state: string; initialized: boolean} | undefined;
    let attempt: Promise<{ok: boolean; errorName: string; watch?: string}> | undefined;
    try {
      page = await browser.newPage({baseURL: origin, serviceWorkers: "block"});
      page.setDefaultNavigationTimeout(3000);
      const destination = page;
      attempt = helper.run(destination).then(async () => {
        loginReturned = true;
        home = await destination.evaluate(() => ({state: document.readyState,
          initialized: (window as Window & {fixtureHomeLoaded?: boolean}).fixtureHomeLoaded === true}));
        return {ok: true, errorName: "", watch: await firstPlayable(destination)};
      }).catch(error => ({ok: false, errorName: error instanceof Error ? error.name : "unknown"}));
      await expect.poll(() => homeRequested).toBe(true);
      await expect(page.getByRole("heading", {name: "Fixture home"})).toBeVisible();
      expect(await page.evaluate(() => document.readyState)).not.toBe("complete");
      expect(loginReturned).toBe(false);
      expect(libraryGets).toBe(0);
      if (homeState === "delayed") {
        release();
        expect(await attempt).toEqual({ok: true, errorName: "", watch: "/watch/fictional"});
        expect(home).toEqual({state: "complete", initialized: true});
        expect(libraryGets).toBe(1);
      } else {
        expect(await attempt).toEqual({ok: false, errorName: "TimeoutError"});
        expect(loginReturned).toBe(false);
        expect(home).toBeUndefined();
        expect(libraryGets).toBe(0);
      }
      expect(posts).toBe(1); expect(homeGets).toBe(1);
      await info.attach("post-login-destination-readiness", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
        loginHelper: helper.name, homeState, posts, homeGets, libraryGets, home, browser: info.project.name, version: browser.version(),
        themeSHA256: createHash("sha256").update(theme).digest("hex"),
        boundary: "Isolated real HTTP login and redirect chain; held Home resource; no populated Server or WebKit root-cause claim", result: "passed"}), contentType: "application/json"});
    } finally {
      try {
        await info.attach("post-login-destination-observations", {body: JSON.stringify({
          loginHelper: helper.name, homeState, homeRequested, loginReturned, posts, homeGets, libraryGets, home,
          browser: info.project.name, version: browser.version(), phase: "before fixture cleanup",
          boundary: "Isolated HTTP fixture observations; no credentials, URLs, or raw errors",
        }), contentType: "application/json"});
      } finally {
        release();
        try { await attempt; } finally {
          try { await page?.close(); } finally {
            server.closeAllConnections();
            await new Promise<void>(resolve => server.close(() => resolve()));
          }
        }
      }
    }
  });
}
