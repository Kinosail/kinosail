import {test} from "node:test";
import assert from "node:assert/strict";
import {spawnSync} from "node:child_process";
import {mkdtempSync, readFileSync, rmSync} from "node:fs";
import {tmpdir} from "node:os";
import {join} from "node:path";

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
