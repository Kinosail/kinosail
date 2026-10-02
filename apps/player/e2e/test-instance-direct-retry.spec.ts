import {test} from "@playwright/test";
import {configureTestInstance, firstPlayable, login} from "./test-instance-helpers";
import {directRetryViewports, verifyDirectRetry} from "./direct-retry-journey";

configureTestInstance();
test.skip(({browserName}) => browserName !== "chromium", "Native direct retry is currently verified in Chromium only.");
test.use({video: "off"});

for (const viewport of directRetryViewports) {
  test.describe(`direct retry at ${viewport.width}×${viewport.height}`, () => {
    test.use({viewport, hasTouch: viewport.width === 390});
    test("@smoke direct-only playback retries one failed media request", async ({page, browserName}, info) => {
      await verifyDirectRetry(page, info, browserName, async () => {
        await login(page);
        return firstPlayable(page);
      }, viewport);
    });
  });
}
