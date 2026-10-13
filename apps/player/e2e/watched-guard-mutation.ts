import {createHash} from "node:crypto";
import {expect, type Page} from "@playwright/test";

type MutationFetches = {armed: boolean[]; overflow: boolean};

// Only the served browser script is mutated; production source stays unchanged.
export async function installWatchedGuardMutation(page: Page) {
  const receipt = {replacements: 0, originalSHA256: "", mutatedSHA256: ""};
  await page.addInitScript(() => {
    const original = window.fetch;
    const observed: MutationFetches = {armed: [], overflow: false};
    Object.assign(window, {qaWatchedMutationFetches: observed});
    window.fetch = function(input, init) {
      try {
        const url = new URL(input instanceof Request ? input.url : String(input), location.href);
        const method = init?.method ?? (input instanceof Request ? input.method : "GET");
        if (url.origin === location.origin && /^\/progress\/[a-zA-Z0-9_-]{1,128}$/.test(url.pathname) && method.toUpperCase() === "POST") {
          if (observed.armed.length < 64) observed.armed.push((window as Window & {qaWatchedGuardBypassed?: boolean}).qaWatchedGuardBypassed === true);
          else observed.overflow = true;
        }
      } catch {observed.overflow = true;}
      return Reflect.apply(original, this, [input, init]);
    };
  });
  const guard = "if (watchedDeparture || (watchedSubmission?.eventPhase === Event.NONE && !watchedSubmission.defaultPrevented)) return Promise.resolve({ok: true});";
  const replacement = "if (!window.qaWatchedGuardBypassed && (watchedDeparture || (watchedSubmission?.eventPhase === Event.NONE && !watchedSubmission.defaultPrevented))) return Promise.resolve({ok: true});";
  await page.route(url => url.pathname === "/static/player.js", async route => {
    const response = await route.fetch();
    expect(response.status()).toBe(200);
    const original = await response.text();
    expect(original.split(guard)).toHaveLength(2);
    expect(receipt.replacements).toBe(0);
    const mutated = original.replace(guard, replacement);
    receipt.replacements++;
    receipt.originalSHA256 = createHash("sha256").update(original).digest("hex");
    receipt.mutatedSHA256 = createHash("sha256").update(mutated).digest("hex");
    expect(receipt.mutatedSHA256).not.toBe(receipt.originalSHA256);
    const headers = {...response.headers()};
    for (const name of ["content-encoding", "content-length", "etag"]) delete headers[name];
    await route.fulfill({response, headers, body: mutated});
  });
  return receipt;
}

export async function verifyMutationInitiations(relay: Page, baselineCount: number, lateCount: number) {
  const observed = await relay.evaluate(() => (opener as Window & {qaWatchedMutationFetches: MutationFetches}).qaWatchedMutationFetches);
  expect(observed.overflow).toBe(false);
  expect(observed.armed).toHaveLength(baselineCount + lateCount);
  expect(observed.armed.slice(0, baselineCount).every(armed => !armed)).toBe(true);
  const late = observed.armed.slice(baselineCount);
  expect(late.length).toBeGreaterThan(0);
  expect(late.every(armed => armed)).toBe(true);
  return observed;
}
