import {recordHomeAssistantEvidence} from "../../player/e2e/home-assistant-document-evidence";
import {expect, test} from "@playwright/test";
import {createHash} from "node:crypto";
import {readFileSync} from "node:fs";
import {fileURLToPath} from "node:url";
import path from "node:path";
import {browserScriptBundles} from "../../../scripts/quality/browser-script-bundles.mjs";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public Server");
test("subtitles serves the exact composed Home Assistant document asset", {tag: "@smoke"}, async ({page}, info) => {
  const root = fileURLToPath(new URL("../../../", import.meta.url));
  const bundle = browserScriptBundles(root).find(bundle => bundle.name === "subtitles.playerJS");
  expect(bundle).toBeTruthy();
  expect(bundle!.files).toContain("packages/webassets/static/player-home-assistant-identity.js");
  expect(bundle!.files.indexOf("packages/webassets/static/player-home-assistant-identity.js")).toBeLessThan(
    bundle!.files.indexOf("packages/webassets/static/player-progress.js"));
  const expected = createHash("sha256").update(Buffer.concat(bundle!.files.map(file => readFileSync(path.join(root, file))))).digest("hex");
  const response = await page.request.get("/static/player.js");
  expect(response.status()).toBe(200);
  const served = createHash("sha256").update(await response.body()).digest("hex");
  expect(served).toBe(expected);
  await recordHomeAssistantEvidence(info, "current-served-document-asset", {consumer: "subtitles", sha256: served,
    exactComposedSource: true, helperBeforeProgress: true});
});
