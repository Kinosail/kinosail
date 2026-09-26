import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { access, unlink, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { compactViewports, expectNoHorizontalOverflow, expectSkipLinkOffscreen, initiallyOccludedTargets, occludedTargets, setSubtitleLanguages, supportedViewports } from "./subtitle-dashboard-helpers";

export function registerSubtitleLanguageTests() {
test("Owner deletes other languages and English forced subtitles from a populated library", async ({ page }, testInfo) => {
  const root = process.env.KINOSAIL_TEST_ROOT;
  if (!root) throw new Error("populated test media root is unavailable");
  const media = join(root, "media", "Movies");
  const spanish = join(media, "Example Movie.es.srt");
  const forced = join(media, "Example Movie.en.forced.srt");
  const rescan = () => page.evaluate(async () => {
    const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')?.content ?? "";
    return (await fetch("/scan", { method: "POST", headers: { "X-Kinosail-CSRF": csrf } })).status;
  });
  await writeFile(spanish, "1\n00:00:01,000 --> 00:00:02,000\nHola\n");
  await writeFile(forced, "1\n00:00:01,000 --> 00:00:02,000\nSigns\n");
  try {
    expect(await rescan()).toBe(200);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/settings#cleanup");
    await page.getByLabel("Forced subtitles in that language").selectOption("delete");
    await page.getByRole("button", { name: "Preview files to delete" }).click();
    await expect(page.getByRole("heading", { name: "2 subtitle files to delete" })).toBeVisible();
    await expect(page.getByText("Example Movie.es.srt")).toBeVisible();
    await expect(page.getByText("Example Movie.en.forced.srt")).toBeVisible();
    await expectNoHorizontalOverflow(page);
    await page.screenshot({ path: testInfo.outputPath("390-populated-subtitle-cleanup-preview.png"), fullPage: true });
    await page.getByRole("button", { name: "Delete 2 subtitle files" }).click();
    await expect(page.getByRole("heading", { name: "Subtitle cleanup complete" })).toBeVisible();
    await expect(page.getByText("en is now your only preferred language. Deleted 2 subtitle files.")).toBeVisible();
    await expect(access(spanish)).rejects.toMatchObject({ code: "ENOENT" });
    await expect(access(forced)).rejects.toMatchObject({ code: "ENOENT" });
    await access(join(media, "Example Movie.vtt"));
  } finally {
    await Promise.all([unlink(spanish).catch(() => {}), unlink(forced).catch(() => {})]);
    await rescan().catch(() => {});
  }
});

test("Owner previews language cleanup and forced subtitle choice", async ({ page }, testInfo) => {
  for (const viewport of [{ width: 390, height: 844 }, { width: 1440, height: 900 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/settings#cleanup");
    const cleanup = page.locator("#cleanup");
    await expect(cleanup.getByRole("heading", { name: "Delete subtitle languages" })).toBeVisible();
    await expect(cleanup.getByLabel("Keep language")).toHaveValue("en");
    await cleanup.getByLabel("Forced subtitles in that language").selectOption("delete");
    await expectNoHorizontalOverflow(page);
    expect((await new AxeBuilder({ page }).include("#cleanup").analyze()).violations).toEqual([]);
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-subtitle-cleanup-setting.png`), fullPage: true });
    await cleanup.getByRole("button", { name: "Preview files to delete" }).click();
    await expect(page.getByRole("heading", { name: "Subtitle cleanup" })).toBeVisible();
    await expect(page.getByText("Delete forced en subtitles.")).toBeVisible();
    await expect(page.getByRole("button", { name: "Keep only en" })).toBeVisible();
    await expectNoHorizontalOverflow(page);
    expect((await new AxeBuilder({ page }).include("main").analyze()).violations).toEqual([]);
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-subtitle-cleanup-preview.png`), fullPage: true });
  }
});

test("Owner sees real coverage, wanted files, and a focused setup path", { tag: "@smoke" }, async ({ page }) => {
  await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
  await expect(page.getByRole("region", { name: "Subtitle coverage" })).toContainText(/\d+%/);
  await expect(page.getByRole("meter", { name: "Subtitle coverage" })).toHaveAttribute("aria-valuetext", /\d+ of \d+ files ready/);
  await expect(page.getByRole("button", { name: /Find and improve subtitles/ })).toBeVisible();
  await expect(page.locator(".subtitle-system-state")).toContainText("Ready");
  await page.locator(".subtitle-system > summary").click();
  await expect(page.getByText("Local embedded text extraction is available", { exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Server readiness" })).toBeVisible();
  await expect(page.getByText("Media library readable", { exact: true })).toBeVisible();
  await expect(page.getByText("Last successful subtitle write", { exact: true })).toBeVisible();
  await expect(page.getByRole("region", { name: "Subtitle sources" })).toBeVisible();
  await expect(page.getByRole("region", { name: "Subtitle sources" }).getByRole("link", { name: /Manage providers/ })).toHaveAttribute("href", "/settings#provider");
  await page.getByRole("link", { name: "Wanted", exact: true }).click();
  await expect(page).toHaveURL(/view=wanted/);
  await expect(page.getByRole("heading", { name: "Wanted", exact: true })).toBeVisible();
  await expect(page.locator(".subtitle-file-list").getByText("Wanted", { exact: true }).first()).toBeVisible();
  await page.getByRole("banner").getByRole("link", { name: "Settings", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Subtitle settings" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Preferred languages" })).toBeVisible();
  await expect(page.getByRole("group", { name: "Preferred subtitle role" }).locator('input[value="standard"]')).toBeChecked();
  await expect(page.getByRole("button", { name: "Test provider credentials" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Configure SubDL" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "SubSource" })).toBeVisible();
  await expect(page.getByLabel(/Use SubSource only for personal household use/)).toBeVisible();
  await expect(page.getByRole("link", { name: "Configure OpenSubtitles" })).toBeVisible();
  await expect(page.getByText("Media Libraries", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "Access", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Trusted HTTPS" })).toBeVisible();
  await expect(page.getByText("myhome.duckdns.org", { exact: true })).toBeVisible();
  await expect(page.getByText("myhome-subtitles.duckdns.org", { exact: true })).toBeVisible();
});

test("Owner manages an ordered preferred-language list at every supported width", async ({ page, browserName }, testInfo) => {
  await page.goto("/settings#language");
  const languageSection = page.locator("#language");
  const languageList = languageSection.getByRole("list", { name: "Preferred subtitle languages" });
  const addLanguage = languageSection.getByLabel("Add a language");
  const addButton = languageSection.getByRole("button", { name: "Add language" });

  await expect(languageList.getByRole("listitem")).toHaveCount(1);
  await expect(languageList.getByText("English", { exact: true })).toBeVisible();
  const availableTags = await addLanguage.locator("option").evaluateAll((options) => options.map((option) => (option as HTMLOptionElement).value));
  expect(availableTags.length).toBeGreaterThan(187);
  expect(availableTags.indexOf("es")).toBeLessThan(availableTags.indexOf("aa"));
  expect(availableTags.indexOf("es-419")).toBeLessThan(availableTags.indexOf("aa"));

  await page.setViewportSize({ width: 320, height: 800 });
  await page.evaluate(async () => { await document.fonts.ready; });
  await page.waitForTimeout(250);
  const initialGeometry = await languageSection.evaluate((section) => {
    const navigationBox = document.querySelector<HTMLElement>(".app-header nav")?.getBoundingClientRect();
    const controls = [...section.querySelectorAll<HTMLElement>("button, select")];
    const overlaps = (left: DOMRect, right: DOMRect) => left.bottom > right.top && left.top < right.bottom && left.right > right.left && left.left < right.right;
    return {
      smallTouchTarget: controls.some((control) => {
        const box = control.getBoundingClientRect();
        return box.width < 44 || box.height < 44;
      }),
      navigationControlOverlap: navigationBox ? controls.some((control) => overlaps(control.getBoundingClientRect(), navigationBox)) : false,
    };
  });
  expect(initialGeometry).toEqual({ smallTouchTarget: false, navigationControlOverlap: false });

  await addLanguage.focus();
  await expect(addLanguage).toBeFocused();
  await addLanguage.selectOption("es");
  await page.keyboard.press(browserName === "webkit" && process.platform === "darwin" ? "Alt+Tab" : "Tab");
  await expect(addButton).toBeFocused();
  await Promise.all([
    page.waitForURL((url) => url.pathname === "/settings" && url.hash === "#language"),
    page.keyboard.press("Enter"),
  ]);

  const spanish = languageList.getByRole("listitem").filter({ has: page.getByText("Spanish", { exact: true }) });
  try {
    await expect(spanish).toContainText("SubDL, OpenSubtitles, SubSource");
    const moveSpanishEarlier = spanish.getByRole("button", { name: "Move Spanish earlier" });
    await moveSpanishEarlier.focus();
    await expect(moveSpanishEarlier).toBeFocused();
    await Promise.all([
      page.waitForURL((url) => url.pathname === "/settings" && url.hash === "#language"),
      page.keyboard.press("Enter"),
    ]);
    await expect(languageList.locator("li strong")).toHaveText(["Spanish", "English"]);
    await expect(spanish).toContainText("Primary");

    for (const viewport of supportedViewports) {
      await page.setViewportSize(viewport);
      await languageSection.scrollIntoViewIfNeeded();
      const geometry = await languageSection.evaluate((section) => {
        const sectionBox = section.getBoundingClientRect();
        const rows = [...section.querySelectorAll<HTMLElement>(".subtitle-language-list li")];
        return {
          pageOverflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
          rowOverflow: rows.some((row) => row.scrollWidth > row.clientWidth + 1),
          controlsOutside: rows.some((row) => {
            const rowBox = row.getBoundingClientRect();
            return [...row.querySelectorAll<HTMLElement>("button")].some((button) => {
              const buttonBox = button.getBoundingClientRect();
              return buttonBox.left < rowBox.left - 1 || buttonBox.right > rowBox.right + 1;
            });
          }),
          outsideSection: rows.some((row) => {
            const rowBox = row.getBoundingClientRect();
            return rowBox.left < sectionBox.left - 1 || rowBox.right > sectionBox.right + 1;
          }),
          smallTouchTarget: [...section.querySelectorAll<HTMLElement>(".subtitle-language-list button, .subtitle-language-add button, .subtitle-language-add select")].some((control) => {
            const controlBox = control.getBoundingClientRect();
            return controlBox.width < 44 || controlBox.height < 44;
          }),
        };
      });
      expect(geometry).toEqual({ pageOverflow: 0, rowOverflow: false, controlsOutside: false, outsideSection: false, smallTouchTarget: false });
      // Short landscape screens must make each control reachable by scrolling.
      expect(await occludedTargets(page, ["#language button", "#language select"], [".app-header", ".app-header nav"])).toEqual([]);
      expect((await new AxeBuilder({ page }).include("#language").analyze()).violations).toEqual([]);
      await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-subtitle-languages.png`), fullPage: true });
    }

    await page.setViewportSize({ width: 390, height: 844 });
    await page.emulateMedia({ reducedMotion: "reduce", forcedColors: "active" });
    await expect(spanish.getByRole("button", { name: "Remove Spanish" })).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("subtitle-languages-forced-colors.png"), fullPage: true });
  } finally {
    await page.emulateMedia({ reducedMotion: "no-preference", forcedColors: "none" });
    await page.goto("/settings#language");
    const removeSpanish = page.getByRole("button", { name: "Remove Spanish" });
    if (await removeSpanish.isVisible().catch(() => false)) {
      await removeSpanish.focus();
      await expect(removeSpanish).toBeFocused();
      await Promise.all([
        page.waitForURL((url) => url.pathname === "/settings" && url.hash === "#language"),
        page.keyboard.press("Enter"),
      ]);
    }
  }

  await expect(languageList.getByRole("listitem")).toHaveCount(1);
  await expect(languageList.getByText("English", { exact: true })).toBeVisible();
  await expect(languageList).toContainText("Primary");
});
}
