import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";

const script = await readFile(new URL("../internal/server/static/subtitle-inspector.js", import.meta.url), "utf8");
const cues = Array.from({ length: 85 }, (_, index) => ({ start: index * 3, end: index * 3 + 2, text: `Cue ${index + 1}`, warnings: [] }));
const words = Array.from({ length: 210 }, (_, index) => ({ start: index, text: `Word ${index + 1}`, confidence: 0.9 }));
const review = { role: "translation", source: "local", matchEvidence: "matched", warnings: [], language: "en", originalAvailable: false,
  restorable: false, current: { cues, quality: { cueCount: cues.length, fastCues: 0, overlaps: 0, longLines: 0, timing: "ok", completeness: "ok" } } };
const fixture = `<!doctype html><html><body class="subtitle-inspector" data-id="0000000000000001">
  <video id="subtitle-preview-video"></video><p id="video-help"></p><p id="inspector-status"></p>
  <input type="radio" name="preview-track" value="current" checked><input type="radio" name="preview-track" value="proposed">
  <div id="subtitle-quality"></div><ul id="subtitle-warnings"></ul><a id="export-original"></a>
  <button id="restore-subtitle"></button><button id="analyze-speech"></button><canvas id="subtitle-waveform"></canvas><p id="speech-summary"></p>
  <form id="subtitle-edit-form"><select name="language"><option value="en">English</option></select><select name="role"><option value="translation">Translation</option></select><select name="encoding"><option value="auto">Auto</option></select><input name="offset" value="0"><input name="automaticSync" type="checkbox"><input name="removeCredits" type="checkbox"><input name="mergeRepeated" type="checkbox"><input name="file" type="file"><textarea name="text"></textarea><button type="submit">Preview</button></form>
  <button id="apply-subtitle"></button><button id="edit-preview-text"></button><button id="clear-preview-text"></button><button id="add-anchor"></button><div id="subtitle-anchors"></div>
  <input id="show-flagged" type="checkbox"><div id="subtitle-cues"></div><p id="cue-page"></p>
  <button id="previous-cues">Previous</button><button id="next-cues">Next</button>
  <select id="draft-method"><option value="ocr">OCR</option></select><button id="start-draft"></button><button id="cancel-draft"></button><button id="review-draft"></button><p id="draft-status"></p>
  <details id="draft-confidence"><summary>Recognition confidence</summary><div id="draft-words"></div><p id="draft-words-status"></p><button id="more-draft-words">Show more words</button></details>
  <script src="/static/subtitle-inspector.js"></script></body></html>`;

test("long cue and draft-word reviews load as the reader scrolls", async ({ page }) => {
  await page.route("http://localhost:38128/**", route => {
    const url = new URL(route.request().url());
    if (url.pathname === "/static/subtitle-inspector.js") return route.fulfill({ contentType: "text/javascript", body: script });
    if (url.pathname.endsWith("/inspect")) return route.fulfill({ json: review });
    if (url.pathname.endsWith("/draft")) return route.fulfill({ json: { state: "ready", message: "Ready", words, id: "draft" } });
    return route.fulfill({ contentType: "text/html", body: fixture });
  });
  await page.goto("http://localhost:38128/inspect-fixture");
  await expect(page.locator(".subtitle-cue-row")).toHaveCount(40);
  for (const selector of ["#next-cues", "#previous-cues", "#more-draft-words"]) await expect(page.locator(selector)).toBeHidden();
  await expect.poll(async () => {
    await page.locator("#cue-page").scrollIntoViewIfNeeded();
    return page.locator(".subtitle-cue-row").count();
  }).toBe(85);
  await page.locator("#draft-confidence summary").click();
  await expect(page.locator("#draft-words button")).toHaveCount(100);
  await expect.poll(async () => {
    await page.locator("#draft-words-status").scrollIntoViewIfNeeded();
    return page.locator("#draft-words button").count();
  }).toBe(210);
});
