import {execFileSync} from "node:child_process";
import type {TestInfo} from "@playwright/test";

const sourceRevision = execFileSync("git", ["rev-parse", "HEAD"], {encoding: "utf-8"}).trim();
if (!/^[0-9a-f]{40}$/.test(sourceRevision)) throw new Error("invalid source revision");

// Explicit test receipts contain only aggregate outcomes and public source hashes.
// Claims, cookies, response bodies, personal targets and fixture passwords stay private.
export async function recordHomeAssistantEvidence(info: TestInfo, label: string, evidence: Record<string, unknown>) {
  const body = JSON.stringify(evidence);
  process.stdout.write("R18_RUNTIME_RECEIPT " + JSON.stringify({revision: sourceRevision,
    test: info.title, project: info.project.name, retry: info.retry, label, evidence}) + "\n");
  await info.attach(label, {body, contentType: "application/json"});
}

type FixtureCleanup = {operation: string; action: () => Promise<unknown>};

export async function cleanupHomeAssistantFixture(info: TestInfo, primary: unknown, actions: FixtureCleanup[]) {
  let failure: unknown;
  const failures: Array<{operation: string; failureClass: string}> = [];
  for (const {operation, action} of actions) {
    try {await action();} catch (error) {
      failure ||= error;
      failures.push({operation, failureClass: error instanceof Error && error.name === "TimeoutError" ? "timeout" : "operation-failed"});
    }
  }
  if (failures.length) {
    try {await recordHomeAssistantEvidence(info, "actual-fixture-cleanup", {primaryFailurePreserved: !!primary, failures});}
    catch (error) {failure ||= error;}
  }
  if (!primary && failure) throw failure;
}

