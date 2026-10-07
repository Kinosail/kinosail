import {test} from "node:test";
import assert from "node:assert/strict";
import {spawnSync} from "node:child_process";
import {mkdtempSync, readFileSync, rmSync} from "node:fs";
import {tmpdir} from "node:os";
import {join} from "node:path";
import * as failureModule from "./layout-stability-failure.mjs";

test("flow failures cannot substitute an earlier closed measurement page", async () => {
  let staleReads = 0;
  const stale = {snapshot:async()=>{staleReads++; return {path:"/",identity:"owned",errorCategory:"none"};}};
  const current = {path:"/watch",identity:"owned",errorCategory:"timeout"};
  assert.equal(await failureModule.layoutFailureNavigation("measure-flows", {navigation:current}, stale, {}), current);
  assert.equal(await failureModule.layoutFailureNavigation("measure-flows", {}, stale, {}), undefined);
  assert.equal(staleReads, 0);
  assert.equal((await failureModule.layoutFailureNavigation("measure-layout", {}, stale, {})).path, "/");
  assert.equal(staleReads, 1);
});

test("failure locations retain only owned code coordinates", async () => {
  const {layoutFailureLocations} = await import("./layout-stability-failure.mjs");
  const stack = "TypeError: private token\n" +
    "    at inspect (file:///home/runner/work/kinosail/kinosail/scripts/testing/layout-stability-local.mjs:144:28)\n" +
    "    at async /Users/fixture/scripts/testing/layout-stability-routing.mjs:12:6\n" +
    "    at https://private.example/scripts/testing/layout-stability-local.mjs:3:4\n" +
    "    at /private/unknown.mjs:4:5";
  assert.deepEqual(layoutFailureLocations({stack}), [
    {file: "layout-stability-local.mjs", line: 144, column: 28},
    {file: "layout-stability-routing.mjs", line: 12, column: 6},
  ]);
  assert.doesNotMatch(JSON.stringify(layoutFailureLocations({stack})), /private|token|runner|Users|https/);
});

test("failure locations reject missing, malformed and oversized stacks", async () => {
  const {layoutFailureLocations} = await import("./layout-stability-failure.mjs");
  for (const stack of [undefined, null, 1, {}, "x".repeat(8193)]) assert.deepEqual(layoutFailureLocations({stack}), []);
  for (const coordinates of ["0:1", "1:0", "100001:1", "1:-1", "1:2?credential=private"]) {
    assert.deepEqual(layoutFailureLocations({stack: `Error\n    at /owned/scripts/testing/layout-stability-local.mjs:${coordinates}`}), []);
  }
});

test("failure locations bound cardinality and ignore the message line", async () => {
  const {layoutFailureLocations} = await import("./layout-stability-failure.mjs");
  const stack = "    at /private/scripts/testing/layout-stability-local.mjs:1:1\n" +
    Array.from({length: 25}, (_, n) => `    at /owned/scripts/testing/layout-stability-local.mjs:${n + 2}:1`).join("\n");
  assert.deepEqual(layoutFailureLocations({stack}), [2,3,4].map(line => ({file:"layout-stability-local.mjs",line,column:1})));
});

// Failure collection can race with another rejection, throw, or stall.
// Cleanup can also fail; every case must still leave a failed child exit.
for (const scenario of ["single", "second-rejection", "collection-failure", "collection-stall", "close-failure"]) test(scenario, () => {
  const dir = mkdtempSync(join(tmpdir(), "layout-failure-contract-"));
  const output = join(dir, "failure.json");
  const module = new URL("./layout-stability-failure.mjs", import.meta.url).href;
  const script = `
    import {installLayoutFailureReporter} from ${JSON.stringify(module)};
    import {writeFile} from "node:fs/promises";
    const scenario = ${JSON.stringify(scenario)};
    installLayoutFailureReporter(async error => {
      if (scenario === "collection-failure") throw new Error("private collection detail");
      if (scenario === "collection-stall") await new Promise(() => {});
      await new Promise(resolve => setTimeout(resolve, 50));
      return {result: "failed", errorClass: error.name};
    }, value => writeFile(${JSON.stringify(output)}, JSON.stringify(value)), async () => {
      if (scenario === "close-failure") throw new Error("private cleanup detail");
    });
    setTimeout(() => { throw new Error("private first detail"); }, 0);
    if (scenario === "second-rejection") setTimeout(() => Promise.reject(new Error("private second detail")), 10);
  `;
  try {
    const child = spawnSync(process.execPath, ["--input-type=module", "-e", script], {encoding: "utf8", timeout: 3500});
    assert.equal(child.error, undefined);
    assert.equal(child.status, 1);
    const report = JSON.parse(readFileSync(output, "utf8"));
    assert.equal(report.result, "failed");
    assert.equal(report.errorClass, "Error");
    assert.doesNotMatch(child.stdout + child.stderr + JSON.stringify(report), /private .* detail/);
  } finally { rmSync(dir, {recursive: true, force: true}); }
});
