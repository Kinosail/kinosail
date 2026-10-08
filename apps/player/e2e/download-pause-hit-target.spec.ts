import { expect, test, type Locator, type TestInfo } from "@playwright/test";
import { attachDownloadEnvironment, downloadChunk, downloadHash, downloadPeer, downloadServer, inspectDownload, openDownloadPage } from "./download-pause-fixture";

test.skip(!downloadServer, "requires actual Go-rendered Downloads and navigation");
test.use({serviceWorkers: "allow"});
test.beforeEach(async ({browser}, info) => attachDownloadEnvironment(browser, info));

async function hitTarget(target: Locator, name: string, info: TestInfo) {
  const receipt = await target.evaluate((element) => {
    const box = element.getBoundingClientRect(), x = box.x + box.width / 2, y = box.y + box.height / 2;
    const hit = document.elementFromPoint(x, y);
    return {box: {x: box.x, y: box.y, width: box.width, height: box.height}, viewport: {width: innerWidth, height: innerHeight}, scroll: {x: scrollX, y: scrollY}, centerInViewport: x >= 0 && x < innerWidth && y >= 0 && y < innerHeight, receivesPointer: hit === element || element.contains(hit), hitTag: hit?.tagName, hitClass: typeof hit?.className === "string" ? hit.className : "", focused: document.activeElement === element};
  });
  await info.attach(name, {body: JSON.stringify(receipt), contentType: "application/json"});
  return receipt;
}

async function centerAndFocus(target: Locator) {
  await target.evaluate((element) => element.scrollIntoView({block: "center"}));
  await target.focus();
  // The shipped html uses smooth scrolling; inspect the settled public geometry.
  let previous = "", stable = 0;
  await expect.poll(async () => {
    const position = await target.evaluate((element) => {
      const box = element.getBoundingClientRect();
      return {x: box.x, y: box.y, width: box.width, height: box.height, scrollX, scrollY};
    });
    const current = JSON.stringify(position);
    stable = current === previous ? stable + 1 : 0;
    previous = current;
    return stable;
  }, {timeout: 2_000, intervals: [50, 75, 100]}).toBeGreaterThanOrEqual(3);
}

test("actual Go phone: fictional download controls and ready Play link remain reachable after explicit scrolling", async ({page, context}, info) => {
  test.setTimeout(45_000);
  await page.setViewportSize({width: 390, height: 844});
  const peer = await downloadPeer();
  try {
    const served = await openDownloadPage(page, peer.origin, "indexeddb", info);
    await info.attach("served-hit-target-bundle", {body: JSON.stringify(served), contentType: "application/json"});
    await page.locator("[data-download-device]").click();
    await expect.poll(async () => (await inspectDownload(page, served.jobID)).job?.bytes).toBe(downloadChunk);
    const pause = page.getByRole("button", {name: "Pause download", exact: true});
    await centerAndFocus(pause);
    expect(await hitTarget(pause, "phone-pause-hit-target", info)).toMatchObject({centerInViewport: true, receivesPointer: true, focused: true});
    await pause.click();
    const resume = page.getByRole("button", {name: "Resume on this device", exact: true});
    await expect(resume).toBeEnabled();
    await expect.poll(async () => (await peer.stats()).closed).toBe(1);
    await centerAndFocus(resume);
    expect(await hitTarget(resume, "phone-resume-hit-target", info)).toMatchObject({centerInViewport: true, receivesPointer: true, focused: true});
    await resume.click();
    await expect(page.locator("[data-download-device-status]")).toHaveText("Saved and verified. Play to check compatibility.");
    expect((await inspectDownload(page, served.jobID)).fileHash).toBe(downloadHash);
    const play = page.locator("[data-download-play]");
    await expect(play).toBeVisible();
    await hitTarget(play, "phone-play-before-explicit-scroll", info);
    await centerAndFocus(play);
    expect(await hitTarget(play, "phone-play-pointer-hit-target", info)).toMatchObject({centerInViewport: true, receivesPointer: true, focused: true});
    await page.screenshot({path: info.outputPath("phone-ready-play-pointer.png"), fullPage: false});
    await play.click();
    await expect(page).toHaveURL(`${peer.origin}/offline?job=${served.jobID}`);
    await expect(page.locator("[data-offline-library] h2")).toHaveText("Fictional Range Fixture");
    await info.attach("phone-play-pointer-navigation", {body: JSON.stringify({pathname: new URL(page.url()).pathname, selectedJob: new URL(page.url()).searchParams.get("job"), decodingClaim: false}), contentType: "application/json"});
    // Reload the public Server page; this does not claim actual BFCache admission.
    await page.goto(`${peer.origin}/offline-downloads`);
    const again = page.locator("[data-download-play]");
    await expect(again).toBeVisible();
    await centerAndFocus(again);
    expect(await hitTarget(again, "phone-play-keyboard-hit-target", info)).toMatchObject({centerInViewport: true, receivesPointer: true, focused: true});
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(`${peer.origin}/offline?job=${served.jobID}`);
    await expect(page.locator("[data-offline-library] h2")).toHaveText("Fictional Range Fixture");
    expect((await peer.stats()).ranges).toEqual([0, downloadChunk, downloadChunk, downloadChunk * 2]);
    expect((await peer.stats()).removals).toBe(0);
    await info.attach("phone-play-keyboard-navigation", {body: JSON.stringify({pathname: new URL(page.url()).pathname, selectedJob: new URL(page.url()).searchParams.get("job"), peer: await peer.stats(), decodingClaim: false}), contentType: "application/json"});
  } finally { await context.close(); await peer.close(); }
});
