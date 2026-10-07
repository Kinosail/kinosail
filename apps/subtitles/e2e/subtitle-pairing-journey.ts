import AxeBuilder from "@axe-core/playwright";
import { expect, type Page, type TestInfo } from "@playwright/test";

export async function reviewSubtitlePairing(page: Page, testInfo: TestInfo) {
  await expect(page.locator("#inspector-status")).toHaveText("Current subtitle loaded. Preview a change before saving.");
  await page.locator('input[name="automaticSync"]').uncheck();
  await page.locator('#subtitle-edit-form details').first().locator("summary").click();
  await page.locator('input[name="offset"]').fill("0.75");
  await page.getByRole("button", { name: "Preview changes", exact: true }).click();
  await expect(page.locator("#inspector-status")).toContainText("Preview ready");
  const rows = page.locator(".subtitle-cue-row");
  await page.screenshot({ path: testInfo.outputPath("actual-cleanup.png"), fullPage: true });
  await expect(rows).toHaveCount(3);
  await expect(rows.nth(0)).toContainText("Removed during cleanup");
  await expect(rows.nth(0).locator("div").first()).toContainText("Downloaded from www.example.com");
  await expect(rows.nth(1)).toContainText("Merged");
  expect(await rows.nth(1).locator("div").first().locator("p").allTextContents()).toEqual(["Hello there", "Hello there"]);
  expect(await rows.nth(1).locator("div").nth(1).locator("p").allTextContents()).toEqual(["Hello there"]);
  await expect(rows.nth(1)).toContainText("Current · cues 2, 3");
  await expect(rows.nth(1)).toContainText("Proposed · cue 1");
  expect(await rows.nth(2).locator("p").allTextContents()).toEqual(["A later line", "A later line"]);
  await rows.nth(1).locator("div").first().getByRole("button", { name: "Seek Current cue 3: 0:04.000 to 0:05.000", exact: true }).click();
  await expect(page.locator('input[name="preview-track"][value="current"]')).toBeChecked();
  await expect.poll(() => page.locator("video").evaluate(video => video.currentTime)).toBe(3);
  await expect(page.locator("video")).toBeFocused();
  await rows.nth(2).locator("div").nth(1).getByRole("button", { name: "Seek Proposed cue 2: 0:07.750 to 0:08.750", exact: true }).click();
  await expect(page.locator('input[name="preview-track"][value="proposed"]')).toBeChecked();
  await expect.poll(() => page.locator("video").evaluate(video => video.currentTime)).toBe(6.75);
  await expect(page.locator("video")).toBeFocused();
  const flagged = page.locator("#show-flagged");
  await flagged.evaluate(input => {
    const events: {type: string; checked: boolean; trusted: boolean}[] = [];
    const record = (event: Event) => {
      events.push({type: event.type, checked: (input as HTMLInputElement).checked, trusted: event.isTrusted});
      if (events.length > 8) events.shift();
    };
    for (const type of ["pointerdown", "click", "input", "change"]) input.addEventListener(type, record);
    Object.assign(input, {pairingObservation: {events, record}});
  });
  try {await flagged.check();}
  finally {
    const observation = await flagged.evaluate(input => {
      const target = input as HTMLInputElement & {pairingObservation: {
        events: {type: string; checked: boolean; trusted: boolean}[]; record: EventListener;
      }};
      for (const type of ["pointerdown", "click", "input", "change"]) target.removeEventListener(type, target.pairingObservation.record);
      const box = input.getBoundingClientRect();
      const result = {checked: target.checked, events: target.pairingObservation.events,
        box: {x: box.x, y: box.y, width: box.width, height: box.height},
        viewport: {width: innerWidth, height: innerHeight}};
      delete (target as Partial<typeof target>).pairingObservation;
      return result;
    });
    await testInfo.attach("flagged-checkbox-observation", {body: JSON.stringify(observation), contentType: "application/json"});
  }
  await expect(rows).toHaveCount(1);
  await expect(rows.first()).toContainText("Downloaded from www.example.com");
  await page.locator("#show-flagged").uncheck();
  await expect(rows).toHaveCount(3);
  await page.screenshot({ path: testInfo.outputPath("paired-cleanup.png"), fullPage: true });
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
  expect((await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21aa"]).analyze()).violations).toEqual([]);

  await page.locator('input[name="offset"]').fill("0");
  await page.locator('input[name="file"]').setInputFiles({ name: "translation.srt", mimeType: "application/x-subrip", buffer: Buffer.from("1\n00:00:03,000 --> 00:00:05,000\nBonjour <i>ami</i>\n") });
  await page.getByRole("button", { name: "Preview changes", exact: true }).click();
  await expect(rows).toHaveCount(5);
  await expect(rows.nth(0)).toContainText("Correspondence not verified");
  await expect(rows.nth(4).locator("p").last()).toHaveText("Bonjour <i>ami</i>");
  await expect(rows.locator("i")).toHaveCount(0);
  await expect(rows.nth(4)).toContainText("Correspondence not verified");
  await page.screenshot({ path: testInfo.outputPath("unpaired-import.png"), fullPage: true });
}
