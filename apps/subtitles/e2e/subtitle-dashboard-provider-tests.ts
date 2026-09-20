import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { compactViewports, expectNoHorizontalOverflow, expectSkipLinkOffscreen, initiallyOccludedTargets, occludedTargets, setSubtitleLanguages, supportedViewports } from "./subtitle-dashboard-helpers";

export function registerSubtitleProviderTests() {
test("Twenty long language choices remain usable at the preference limit", async ({ page }, testInfo) => {
  const languages = ["es-419", "pt-MZ", "zh-Hant", "sr-Cyrl", "yue", "ckb", "mni", "cnr", "sat", "syr", "tet", "tok", "azb", "ast", "ext", "prs", "fil", "aa", "en", "de"];
  await page.goto("/settings#language");
  expect(await setSubtitleLanguages(page, languages)).toBe(200);
  try {
    await page.reload();
    const section = page.locator("#language");
    const list = section.getByRole("list", { name: "Preferred subtitle languages" });
    await expect(list.getByRole("listitem")).toHaveCount(20);
    await expect(list.getByText("Spanish (Latin America)", { exact: true })).toBeVisible();
    await expect(list.getByText("Portuguese (Mozambique)", { exact: true })).toBeVisible();
    await expect(list.getByText("Chinese (Traditional)", { exact: true })).toBeVisible();
    await expect(list).toContainText("SubDL, OpenSubtitles, SubSource");
    await expect(list).toContainText("No configured provider");
    await expect(section.getByLabel("Add a language")).toBeDisabled();
    await expect(section.getByRole("button", { name: "Add language" })).toBeDisabled();
    await expect(section.getByText("You have selected 20 languages. Remove one before adding another.", { exact: true })).toBeVisible();

    const movePortuguese = list.getByRole("button", { name: "Move Portuguese (Mozambique) earlier" });
    await movePortuguese.focus();
    await expect(movePortuguese).toBeFocused();
    await Promise.all([
      page.waitForURL((url) => url.pathname === "/settings" && url.hash === "#language"),
      page.keyboard.press("Enter"),
    ]);
    await expect(list.locator("li strong").first()).toHaveText("Portuguese (Mozambique)");
    const removePortuguese = list.getByRole("button", { name: "Remove Portuguese (Mozambique)" });
    await removePortuguese.focus();
    await expect(removePortuguese).toBeFocused();

    for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
      await page.setViewportSize(viewport);
      await section.scrollIntoViewIfNeeded();
      const geometry = await section.evaluate((element) => {
        const sectionBox = element.getBoundingClientRect();
        const rows = [...element.querySelectorAll<HTMLElement>(".subtitle-language-list li")];
        const controls = [...element.querySelectorAll<HTMLElement>("button, select")];
        return {
          pageOverflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
          rowOverflow: rows.some((row) => row.scrollWidth > row.clientWidth + 1),
          outsideSection: rows.some((row) => {
            const box = row.getBoundingClientRect();
            return box.left < sectionBox.left - 1 || box.right > sectionBox.right + 1;
          }),
          smallTouchTarget: controls.some((control) => {
            const box = control.getBoundingClientRect();
            return box.width < 44 || box.height < 44;
          }),
        };
      });
      expect(geometry).toEqual({ pageOverflow: 0, rowOverflow: false, outsideSection: false, smallTouchTarget: false });
      expect((await new AxeBuilder({ page }).include("#language").analyze()).violations).toEqual([]);
      await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-twenty-subtitle-languages.png`), fullPage: true });
    }

    await page.setViewportSize({ width: 390, height: 844 });
    await page.emulateMedia({ reducedMotion: "reduce", forcedColors: "active" });
    expect((await new AxeBuilder({ page }).include("#language").analyze()).violations).toEqual([]);
    await page.screenshot({ path: testInfo.outputPath("twenty-subtitle-languages-forced-colors.png"), fullPage: true });
  } finally {
    await page.emulateMedia({ reducedMotion: "no-preference", forcedColors: "none" });
    await page.goto("/settings#language");
    expect(await setSubtitleLanguages(page, ["en"])).toBe(200);
  }
});

test("Local-only languages do not advertise unavailable item actions", async ({ page }, testInfo) => {
  let addedAfar = false;
  let removedEnglish = false;
  try {
    await page.setViewportSize({ width: 320, height: 800 });
    await page.goto("/settings#language");
    const languageSection = page.locator("#language");
    await languageSection.getByLabel("Add a language").selectOption("aa");
    await Promise.all([
      page.waitForURL((url) => url.pathname === "/settings" && url.hash === "#language"),
      languageSection.getByRole("button", { name: "Add language" }).click(),
    ]);
    addedAfar = true;
    await page.goto("/settings#language");
    await Promise.all([
      page.waitForURL((url) => url.pathname === "/settings" && url.hash === "#language"),
      page.getByRole("button", { name: "Remove English" }).click(),
    ]);
    removedEnglish = true;

    await page.goto("/?view=wanted");
    const file = page.locator(".subtitle-file").first();
    await file.locator(":scope > summary").click();
    await expect(page.getByRole("button", { name: "Find aa" })).toHaveCount(0);
    await expect(file.getByRole("link", { name: "Add a local subtitle" })).toBeVisible();
    expect((await new AxeBuilder({ page }).include("main").analyze()).violations).toEqual([]);
    await page.screenshot({ path: testInfo.outputPath("320-local-only-language.png"), fullPage: true });
  } finally {
    await page.goto("/settings#language");
    if (removedEnglish) {
      const languageSection = page.locator("#language");
      await languageSection.getByLabel("Add a language").selectOption("en");
      await Promise.all([
        page.waitForURL((url) => url.pathname === "/settings" && url.hash === "#language"),
        languageSection.getByRole("button", { name: "Add language" }).click(),
      ]);
    }
    if (addedAfar) {
      await page.goto("/settings#language");
      const removeAfar = page.getByRole("button", { name: "Remove Afar" });
      if (await removeAfar.isEnabled().catch(() => false)) {
        await Promise.all([
          page.waitForURL((url) => url.pathname === "/settings" && url.hash === "#language"),
          removeAfar.click(),
        ]);
      }
    }
  }
});

test("Trusted HTTPS guidance keeps the paired names readable", async ({ page }, testInfo) => {
  await page.goto("/settings#trusted-https");
  const trusted = page.locator("#trusted-https");
  await expect(trusted.locator('form[action="/settings/trusted-https"]')).toBeVisible();
  await expect(trusted).toContainText("no router port forwarding is needed");
  await expect(trusted).toContainText("default 38128");
  await expect(trusted).toContainText("does not make Kinosail Subtitles available away from home");
  await expect(trusted).toContainText("a DuckDNS or deSEC account");
  const explanation = trusted.getByText("Kinosail points the hostname at the private LAN address.");
  await expect(explanation).not.toBeVisible();
  await trusted.getByText("How trusted HTTPS works").click();
  await expect(explanation).toBeVisible();
  await expect(trusted).toContainText("renews the certificate automatically");
  for (const viewport of supportedViewports) {
    await page.setViewportSize(viewport);
    await expectNoHorizontalOverflow(page);
    await expect(page.getByText("myhome-subtitles.duckdns.org", { exact: true })).toBeVisible();
    expect((await new AxeBuilder({ page }).include("#trusted-https").analyze()).violations).toEqual([]);
    await expectSkipLinkOffscreen(page);
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-trusted-https.png`), fullPage: true });
  }
});

test("Provider setup stays compact and accessible at every supported width", async ({ page }, testInfo) => {
  for (const viewport of supportedViewports) {
    await page.setViewportSize(viewport);
    await page.goto("/settings#provider");
    const providers = page.locator("#provider");
    await expect(providers.getByText("One provider is enough.")).toBeVisible();
    await expect(providers.getByRole("heading", { name: "SubSource" })).toBeVisible();
    await expect(providers.getByLabel("SubSource API key")).toBeVisible();
    await expect(providers.getByLabel(/Use SubSource only for personal household use/)).toBeVisible();
    await expectNoHorizontalOverflow(page);
    if (viewport.width <= 900) {
      const navigationBox = await page.locator(".settings-nav").boundingBox();
      const providerBox = await providers.boundingBox();
      expect(navigationBox).not.toBeNull();
      expect(providerBox).not.toBeNull();
      expect(providerBox!.y).toBeGreaterThanOrEqual(navigationBox!.y + navigationBox!.height - 2.1);
    }
    expect((await new AxeBuilder({ page }).include("#provider").analyze()).violations).toEqual([]);
    await expectSkipLinkOffscreen(page);
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-subtitle-providers.png`), fullPage: true });

    await providers.getByRole("link", { name: "Configure OpenSubtitles" }).click();
    const openSubtitles = page.locator("#integrations\\.opensubtitles");
    await expect(openSubtitles.getByLabel("OpenSubtitles API key")).toBeVisible();
    await expect(openSubtitles.getByLabel("OpenSubtitles username")).toBeVisible();
    await expect(openSubtitles.getByLabel("OpenSubtitles password")).toBeVisible();
    await expect(openSubtitles.getByRole("button", { name: "Save OpenSubtitles" })).toBeVisible();
    await expect(openSubtitles).toContainText("Save all three values together.");
    await expectNoHorizontalOverflow(page);
    expect((await new AxeBuilder({ page }).include("#integrations\\.opensubtitles").analyze()).violations).toEqual([]);
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-opensubtitles-configuration.png`), fullPage: true });
  }
});
}
