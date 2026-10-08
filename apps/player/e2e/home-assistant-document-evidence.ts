import type {TestInfo} from "@playwright/test";

// Explicit test receipts contain only aggregate outcomes and public source hashes.
// Claims, cookies, response bodies, personal targets and fixture passwords stay private.
export async function recordHomeAssistantEvidence(info: TestInfo, label: string, evidence: Record<string, unknown>) {
  const body = JSON.stringify(evidence);
  await info.attach(label, {body, contentType: "application/json"});
  process.stdout.write("R18_RUNTIME_RECEIPT " + JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
    test: info.title, project: info.project.name, retry: info.retry, label, evidence}) + "\n");
}
