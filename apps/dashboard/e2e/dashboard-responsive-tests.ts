import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { expectNoOverflow, login, screenshot } from "./dashboard-helpers";

export function registerDashboardResponsiveTests() {
  for (const viewport of [
    { width: 1440, height: 900, columns: 2 },
    { width: 1024, height: 768, columns: 2 },
    { width: 720, height: 450, columns: 2 },
    { width: 390, height: 844, columns: 1 },
    { width: 320, height: 700, columns: 1 },
  ]) {
    test(`keeps one populated board usable at ${viewport.width}px`, async ({ page }, testInfo) => {
      await page.setViewportSize(viewport);
      await login(page);
      await expect(page.locator(".app-tile")).toHaveCount(7);
      await expectNoOverflow(page);
      const columns = await page.locator("#app-grid").evaluate(element => getComputedStyle(element).gridTemplateColumns.split(" ").length);
      expect(columns).toBe(viewport.columns);
      const firstRowTops = await page.locator(".app-tile").evaluateAll((tiles, count) => tiles.slice(0, count).map(tile => Math.round(tile.getBoundingClientRect().top)), viewport.columns);
      expect(new Set(firstRowTops).size).toBe(1);
      const headingSize = await page.locator("#board-title").evaluate(element => Number.parseFloat(getComputedStyle(element).fontSize));
      const rem = await page.evaluate(() => Number.parseFloat(getComputedStyle(document.documentElement).fontSize));
      expect(headingSize).toBeCloseTo(viewport.width <= 760 ? 2.75 * rem : Math.min(4.5 * rem, Math.max(2.5 * rem, viewport.width * 0.045)), 0);
      if (viewport.width <= 390) {
        const filterHeight = await page.locator("#category-filters").evaluate(element => ({ client: element.clientHeight, scroll: element.scrollHeight }));
        expect(filterHeight.scroll).toBeLessThanOrEqual(filterHeight.client + 1);
        const details = await page.locator(".health-detail").allTextContents();
        expect(details.every(detail => !/^\s*[·•]/u.test(detail))).toBeTruthy();
        const targets = page.locator(".brand, #command-button, #settings-button, #check-all-button, #edit-button, #add-button, .app-link");
        for (let index = 0; index < await targets.count(); index++) {
          const box = await targets.nth(index).boundingBox();
          const minimum = Math.min(box?.width ?? 0, box?.height ?? 0);
          expect(Math.round(minimum * 100) / 100).toBeGreaterThanOrEqual(44);
        }
        if (viewport.width === 320) {
          const editBox = await page.locator("#edit-button").boundingBox();
          const addBox = await page.locator("#add-button").boundingBox();
          expect(editBox?.height).toBe(addBox?.height);
        }
      }
      if (viewport.width <= 390) {
        const firstTile = await page.locator(".app-tile").first().boundingBox();
        expect(firstTile?.y).toBeLessThan(500);
      }
      await screenshot(page, testInfo, `populated-${viewport.width}.png`);
    });
  }

  test("keeps modern choices visible, keyboard operable, and touch sized", async ({ page }, testInfo) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await login(page);
    await page.getByRole("button", { name: "Add application" }).click();
    const accents = page.getByRole("group", { name: "Accent" });
    await expect(accents.getByRole("radio")).toHaveCount(7);
    await expect(accents.getByRole("radio", { name: "Slate" })).toBeChecked();
    await accents.getByRole("radio", { name: "Slate" }).focus();
    await page.keyboard.press("ArrowLeft");
    await expect(accents.getByRole("radio", { name: "Violet" })).toBeChecked();
    for (const option of await accents.getByRole("radio").all()) {
      const box = await option.boundingBox();
      expect(box?.height).toBeGreaterThanOrEqual(44);
    }
    expect((await new AxeBuilder({ page }).include("#app-dialog").analyze()).violations).toEqual([]);
    await screenshot(page, testInfo, "modern-accents-390.png");
    await page.getByRole("button", { name: "Close" }).click();

    await page.getByRole("button", { name: "Open board settings" }).click();
    const viewMode = page.getByRole("group", { name: "View mode" });
    await expect(viewMode.getByRole("radio", { name: /Household/ })).toBeChecked();
    await viewMode.getByRole("radio", { name: /Operations/ }).check();
    await expect(page.locator("body")).toHaveAttribute("data-view-mode", "operations");
    for (const option of await viewMode.getByRole("radio").all()) {
      const box = await option.boundingBox();
      expect(box?.height).toBeGreaterThanOrEqual(44);
    }
    expect((await new AxeBuilder({ page }).include("#settings-dialog").analyze()).violations).toEqual([]);
    await screenshot(page, testInfo, "modern-choices-390.png");
    await page.setViewportSize({ width: 320, height: 700 });
    await expectNoOverflow(page);
    await screenshot(page, testInfo, "modern-choices-320.png");
    await viewMode.getByRole("radio", { name: /Household/ }).check();
  });

  test("passes accessibility, reduced-motion, and forced-color checks", async ({ page }, testInfo) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await login(page);
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.getByRole("button", { name: "Add application" }).click();
    const motion = await page.locator("#app-dialog").evaluate(element => {
      const style = getComputedStyle(element);
      return { animation: style.animationName, transition: style.transitionDuration };
    });
    expect(motion.animation).toBe("none");
    await screenshot(page, testInfo, "390-reduced-motion-dialog.png");
    await page.keyboard.press("Escape");

    await page.emulateMedia({ reducedMotion: "reduce", forcedColors: "active" });
    expect((await new AxeBuilder({ page }).disableRules(["color-contrast"]).analyze()).violations).toEqual([]);
    await page.reload();
    await page.locator(".skip-link").focus();
    const forced = await page.locator(".skip-link").evaluate(element => {
      const button = getComputedStyle(document.querySelector("#add-button")!);
      const systemProbe = document.createElement("span");
      systemProbe.style.color = "ButtonText";
      document.body.append(systemProbe);
      const buttonText = getComputedStyle(systemProbe).color;
      systemProbe.remove();
      return {
        focused: document.activeElement === element,
        active: matchMedia("(forced-colors: active)").matches,
        borderColor: button.borderTopColor,
        borderStyle: button.borderTopStyle,
        borderWidth: Number.parseFloat(button.borderTopWidth),
        buttonText,
      };
    });
    expect(forced.active).toBeTruthy();
    expect(forced.focused).toBeTruthy();
    expect(forced.borderStyle).toBe("solid");
    expect(forced.borderWidth).toBeGreaterThanOrEqual(1);
    expect(forced.borderColor).toBe(forced.buttonText);
    await expectNoOverflow(page);
    await screenshot(page, testInfo, "390-forced-colors-board.png");
  });
}
