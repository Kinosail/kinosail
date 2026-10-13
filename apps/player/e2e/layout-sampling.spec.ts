import {expect, test} from "@playwright/test";
import {createServer} from "node:http";
import {readFile, writeFile} from "node:fs/promises";
import {createHash} from "node:crypto";
import {spawn} from "node:child_process";
import {fileURLToPath} from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const geometryCSS = "body{margin:0}main{padding:16px}h1{font-size:24px}.card{height:80px;background:#ddd}";

// A populated Server cannot deterministically hold native first paint beyond
// the recorder's old cutoff. This isolated CLI contract uses a real critical
// stylesheet response and native browser clocks; it does not prove product UI.
for (const delay of [0, 8500]) {
  test(`layout CLI samples fresh frames after ${delay ? "slow" : "normal"} first paint`, {tag: "@smoke"}, async ({browserName}, info) => {
    test.setTimeout(90_000);
    let posts = 0, documents = 0, styles = 0;
    const styledDocuments = new Set<string>();
    const timers = new Set<ReturnType<typeof setTimeout>>();
    const server = createServer((request, response) => {
      const url = new URL(request.url!, "http://fixture"), path = url.pathname;
      if (request.method === "POST" && path === "/login") {
        posts++;
        response.writeHead(303, {Location: "/"}); response.end();
      } else if (path === "/login") {
        response.writeHead(200, {"Content-Type": "text/html"});
        response.end('<!doctype html><form method="post" action="/login"><label>Name<input name="name"></label><label>Password<input name="password" type="password"></label><button>Sign in</button></form>');
      } else if (path.startsWith("/api/")) {
        response.writeHead(200, {"Content-Type": "application/json"});
        response.end('{"items":[{"id":"example","title":"Layout Example"}]}');
      } else if (path === "/critical.css") {
        styles++;
        styledDocuments.add(url.searchParams.get("document")!);
        const timer = setTimeout(() => {
          timers.delete(timer);
          response.writeHead(200, {"Content-Type": "text/css"});
          response.end(geometryCSS + "body{visibility:visible}");
        }, delay);
        timers.add(timer);
      } else if (path === "/") {
        documents++;
        response.writeHead(200, {"Content-Type": "text/html"});
        // The first document completes authentication. Measurement documents
        // load the slow stylesheet through the actual layout CLI's route.
        response.end(`<!doctype html><head><style>${geometryCSS}${documents > 1 ? "body{visibility:hidden}" : ""}</style>${documents > 1 ? `<link rel="stylesheet" href="/critical.css?document=${documents}">` : ""}</head><body><main><h1>Layout example</h1><div class="card">Stable content</div></main></body>`);
      } else {
        response.writeHead(204); response.end();
      }
    });
    await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
    const origin = `http://127.0.0.1:${(server.address() as {port: number}).port}`;
    const child = spawn(process.execPath, ["scripts/testing/layout-stability-local.mjs"], {cwd: root, env: {...process.env,
      KINOSAIL_E2E_URL: origin, KINOSAIL_LAYOUT_RUN: info.outputPath("."), KINOSAIL_LAYOUT_APP: "subtitles",
      KINOSAIL_LAYOUT_BROWSER: browserName, KINOSAIL_LAYOUT_TOTP: "JBSWY3DPEHPK3PXP",
      KINOSAIL_LAYOUT_PATHS: "/", KINOSAIL_LAYOUT_QUICK: "1", KINOSAIL_LAYOUT_VARIANTS: "",
      KINOSAIL_LAYOUT_FLOWS: "", KINOSAIL_LAYOUT_ENFORCE: "1"}});
    let output = "";
    child.stdout.on("data", data => output += data);
    child.stderr.on("data", data => output += data);
    const timer = setTimeout(() => child.kill("SIGKILL"), 75_000);
    try {
      const code = await new Promise<number | null>((resolve, reject) => {child.once("exit", resolve); child.once("error", reject);});
      await writeFile(info.outputPath("layout-cli.log"), output);
      const measurements = JSON.parse(await readFile(info.outputPath("measurements.json"), "utf8"));
      const frames = measurements.reports.map((report: {baselineAfter: number; initialFrameTime: number; frames: {time: number}[]}) => ({
        baseline: report.baselineAfter, initial: report.initialFrameTime, settled: report.frames.at(-1)?.time,
      }));
      const receipt = {revision: process.env.KINOSAIL_TEST_REVISION, browserName, delay, code, posts, styles, styledDocuments: [...styledDocuments], frames,
        command: `pnpm --dir apps/player/e2e exec playwright test --project=${browserName} --retries=0 layout-sampling.spec.ts`,
        environment: `${process.platform} ${process.arch}; native browser; disposable HTTP peer`,
        cliSHA256: createHash("sha256").update(await readFile(root + "scripts/testing/layout-stability-local.mjs")).digest("hex"),
        testSHA256: createHash("sha256").update(await readFile(new URL("./layout-sampling.spec.ts", import.meta.url))).digest("hex"),
        boundary: "Isolated layout-recorder contract with real delayed CSS; no mocked clock or product E2E claim"};
      await writeFile(info.outputPath("layout-sampling-receipt.json"), JSON.stringify(receipt, null, 2));
      await info.attach("layout-sampling-receipt", {body: JSON.stringify(receipt), contentType: "application/json"});
      expect(code, output).toBe(0);
      expect(posts).toBe(1);
      expect(styledDocuments.size).toBe(2);
      expect(frames).toHaveLength(2);
      for (const frame of frames) {
        if (delay) expect(frame.baseline).toBeGreaterThan(8000);
        expect(frame.initial).toBeGreaterThanOrEqual(frame.baseline);
        expect(frame.settled).toBeGreaterThan(frame.initial);
      }
    } finally {
      clearTimeout(timer); child.kill("SIGKILL");
      timers.forEach(timer => clearTimeout(timer));
      server.closeAllConnections();
      await new Promise<void>(resolve => server.close(() => resolve()));
    }
  });
}
