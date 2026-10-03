import {expect, test} from "@playwright/test";
import {writeFile} from "node:fs/promises";
import {execFileSync} from "node:child_process";
import {login} from "./test-instance-helpers";

// Authentication bodies and cookie values must stay out of traces/screenshots.
test.use({serviceWorkers:"block", trace:"off", screenshot:"off", video:"off"});
test("closing a signed-in tab resumes its saved login URL @smoke", async ({page, context}, info) => {
  test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the isolated populated Server");
  await page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", {value:async()=>false}));
  await login(page);
  const origin = new URL(page.url()).origin;
  await context.addInitScript(() => localStorage.setItem("kinosail-passkey", "1"));
  await page.close();
  const reopened = await context.newPage();
  let begins = 0;
  reopened.on("request", request => { if (new URL(request.url()).pathname === "/api/v1/passkeys/login/begin") begins++; });
  await reopened.goto(`${origin}/login?next=%2Faccount`);
  await expect(reopened).toHaveURL(origin+"/account");
  await expect(reopened.locator("[data-passkey-login]")).toHaveCount(0);
  const status = await reopened.evaluate(async () => (await fetch("/api/v1/me")).status);
  expect(status).toBe(200);
  expect(begins).toBe(0);
  await reopened.goto(`${origin}/login?stepup=1&next=%2Faccount`);
  await expect(reopened.locator("[data-passkey-login]")).toBeVisible();
  expect(new URL(reopened.url()).pathname).toBe("/login");
  const receipt = info.outputPath("session-persistence.json");
  await writeFile(receipt, JSON.stringify({revision:execFileSync("git", ["rev-parse", "HEAD"], {encoding:"utf8"}).trim(), result:"passed",
    command:"playwright test test-instance-session-persistence.spec.ts --project=chromium",
    data:"isolated populated Server Owner fixture; same browser context, closed tab and new tab",
    browser:info.project.name, meStatus:status, ordinaryReturnPasskeyBegins:0, explicitStepUpRetained:true}, null, 2));
  await info.attach("session-persistence", {path:receipt, contentType:"application/json"});
});
