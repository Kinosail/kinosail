import {expect, test} from "@playwright/test";
import {createServer} from "node:http";
import {readFile, writeFile} from "node:fs/promises";
import {createHash} from "node:crypto";
import {spawn} from "node:child_process";
import {fileURLToPath} from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const theme = await readFile(new URL("../../../packages/webassets/static/theme.js", import.meta.url), "utf8");
for (const scenario of ["initial-error", "replacement-error", "replacement-redirect"] as const) {
  test(`layout CLI rejects ${scenario} before credentials`, {tag: "@smoke"}, async ({browserName}, info) => {
    test.skip(scenario !== "initial-error" && browserName !== "firefox", "Only Firefox admits the controlled initial timeout");
    test.setTimeout(90_000);
    let gets = 0, posts = 0;
    const server = createServer((request, response) => {
      const url = new URL(request.url!, "http://fixture");
      if (request.method === "POST") {
        posts++;
        response.writeHead(303, {Location: "/"}); response.end();
      } else if (url.pathname === "/theme.js") {
        response.writeHead(200, {"Content-Type": "text/javascript"}); response.end(theme);
      } else if (url.pathname === "/login") {
        gets++;
        if (scenario === "replacement-redirect" && gets === 2) {
          response.writeHead(302, {Location: "/login?unexpected=1"}); response.end(); return;
        }
        const status = scenario === "initial-error" || scenario === "replacement-error" && gets === 2 ? 500 : 200;
        response.writeHead(status, {"Content-Type": "text/html"});
        response.end('<!doctype html><head><script defer src="/theme.js"></script></head><body><form method="post" action="/login"><label>Name<input name="name"></label><label>Password<input name="password" type="password"></label><button>Sign in</button></form></body>');
      } else {
        response.writeHead(200, {"Content-Type": "application/json"}); response.end('{"items":[]}');
      }
    });
    await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
    const origin = `http://127.0.0.1:${(server.address() as {port: number}).port}`;
    const run = info.outputPath(".");
    const wrapper = info.outputPath("controlled-navigation.mjs");
    await writeFile(wrapper, `import {createRequire} from "node:module";
const require=createRequire(${JSON.stringify(root + "apps/player/e2e/package.json")});
const engine=require("@playwright/test")[${JSON.stringify(browserName)}], launch=engine.launch.bind(engine);
engine.launch=async (...args)=>{const browser=await launch(...args), createContext=browser.newContext.bind(browser);
browser.newContext=async (...args)=>{const context=await createContext(...args), createPage=context.newPage.bind(context);
context.newPage=async (...args)=>{const page=await createPage(...args), navigate=page.goto.bind(page);let calls=0;
page.goto=async (...args)=>{const response=await navigate(...args);
if(args[0]==="/login"&&++calls===1&&${scenario !== "initial-error"}){
await page.waitForFunction(()=>document.readyState==="complete");const error=new Error("Controlled initial navigation timeout");error.name="TimeoutError";throw error;}
return response;};return page;};return context;};return browser;};
await import(${JSON.stringify(root + "scripts/testing/layout-stability-local.mjs")});
`);
    let output = "";
    const child = spawn(process.execPath, [wrapper], {cwd: root, env: {...process.env,
      KINOSAIL_E2E_URL: origin, KINOSAIL_LAYOUT_RUN: run, KINOSAIL_LAYOUT_APP: "subtitles",
      KINOSAIL_LAYOUT_BROWSER: browserName, KINOSAIL_LAYOUT_TOTP: "JBSWY3DPEHPK3PXP"}});
    child.stdout.on("data", data => output += data);
    child.stderr.on("data", data => output += data);
    const timer = setTimeout(() => child.kill("SIGKILL"), 60_000);
    try {
      const code = await new Promise<number | null>((resolve, reject) => {child.once("exit", resolve); child.once("error", reject);});
      const failure = JSON.parse(await readFile(info.outputPath("failure.json"), "utf8"));
      const receipt = {revision: process.env.KINOSAIL_TEST_REVISION,
        command: `pnpm --dir apps/player/e2e exec playwright test --project=${browserName} --retries=0 layout-auth-navigation.spec.ts`,
        environment: `${process.platform} ${process.arch}; native ${browserName}; disposable HTTP peer`, result: "not_completed",
        scenario, browserName, gets, posts, code, failure,
        cliSHA256: createHash("sha256").update(await readFile(root + "scripts/testing/layout-stability-local.mjs")).digest("hex"),
        helperSHA256: createHash("sha256").update(await readFile(root + "scripts/testing/auth-form-navigation.ts")).digest("hex"),
        testSHA256: createHash("sha256").update(await readFile(new URL("./layout-auth-navigation.spec.ts", import.meta.url))).digest("hex"),
        data: "Synthetic HTTP forms and actual native browser; controlled original automation timeout"};
      await writeFile(info.outputPath("layout-login-rejection.json"), JSON.stringify(receipt));
      expect(code, output).toBe(1);
      expect(failure.stage).toBe("login-page");
      expect(failure.completedCases).toBe(0);
      expect(posts).toBe(0);
      expect(failure.loginResponses).toEqual([]);
      expect(gets).toBe(scenario === "replacement-redirect" ? 3 : scenario === "replacement-error" ? 2 : 1);
      if (scenario !== "initial-error") {
        const recovery = JSON.parse(await readFile(info.outputPath("firefox-auth-form-navigation-recovery.json"), "utf8"));
        expect(recovery.originalDocument.ready).toBe(true);
        await info.attach("original-navigation-trace", {path: info.outputPath("login-original-navigation-trace.zip"), contentType: "application/zip"});
      }
      receipt.result = "passed";
      await writeFile(info.outputPath("layout-login-rejection.json"), JSON.stringify(receipt));
      await info.attach("layout-login-rejection", {path: info.outputPath("layout-login-rejection.json"), contentType: "application/json"});
    } finally {
      clearTimeout(timer); child.kill("SIGKILL");
      server.closeAllConnections();
      await new Promise<void>(resolve => server.close(() => resolve()));
    }
  });
}
