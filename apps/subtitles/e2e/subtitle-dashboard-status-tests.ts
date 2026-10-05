import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { compactViewports, initiallyOccludedTargets, occludedTargets } from "./subtitle-dashboard-helpers";

export function registerSubtitleStatusLayoutTests() {
  test("Dashboard feedback preserves content through pending, failure, and empty results", async ({ page }, testInfo) => {
    for (const viewport of [{width: 568, height: 320}, {width: 720, height: 450}, {width: 390, height: 844}, {width: 1440, height: 900}]) {
      await page.setViewportSize(viewport);
      await page.goto("/", {waitUntil: "domcontentloaded"});
      await expect(page.locator(".subtitle-overview")).toBeVisible();
      await expect(page.locator("#subtitle-loading")).toBeHidden();
      const geometry = () => page.evaluate(() => Object.fromEntries([".subtitle-feedback-slot", ".subtitle-page-heading", ".subtitle-overview", ".subtitle-overview-actions", ".app-header nav", "#subtitle-content"].map(selector => [selector, document.querySelector(selector)!.getBoundingClientRect().toJSON()])));
      const before = await geometry();
      await testInfo.attach(viewport.width + "-initial-geometry", {contentType: "application/json", body: JSON.stringify({viewport, before})});
      let release: () => void = () => {};
      const gate = new Promise<void>(resolve => {release = resolve;});
      await page.route("**/?*", async route => {
        if (route.request().resourceType() !== "fetch") return route.continue();
        await gate;
        await route.abort("failed");
      });
      await page.getByRole("link", {name: "Library", exact: true}).click();
      await expect(page.locator("#main")).toHaveAttribute("aria-busy", "true");
      await expect(page.locator("#subtitle-feedback")).toHaveText("Loading library…");
      const pending = await geometry();
      expect(pending["#subtitle-content"]).toEqual(before["#subtitle-content"]);
      await page.screenshot({path: testInfo.outputPath(viewport.width + "-pending.png")});
      release();
      await expect(page.locator("#main")).not.toHaveAttribute("aria-busy", "true");
      await expect(page.getByRole("link", {name: "Reload view", exact: true})).toBeVisible();
      const failed = await geometry();
      expect(failed["#subtitle-content"]).toEqual(before["#subtitle-content"]);
      expect((await new AxeBuilder({page}).include("main").analyze()).violations).toEqual([]);
      await page.screenshot({path: testInfo.outputPath(viewport.width + "-failed.png")});
      await page.unroute("**/?*");
      await page.getByRole("link", {name: "Reload view", exact: true}).click();
      await expect(page.locator("#subtitle-feedback")).toBeHidden();
      const search = page.getByRole("searchbox", {name: "Search subtitle library"});
      await search.fill("No matching synthetic subtitle title");
      await expect(page.getByRole("heading", {name: "No files match.", exact: true})).toBeVisible();
      await expect(page.locator("#subtitle-feedback")).toBeHidden();
      await expect(page.locator("#subtitle-loading")).toBeHidden();
      expect((await new AxeBuilder({page}).include("main").analyze()).violations).toEqual([]);
      await page.screenshot({path: testInfo.outputPath(viewport.width + "-empty.png")});
      await testInfo.attach(viewport.width + "-geometry", {contentType: "application/json", body: JSON.stringify({viewport, before, pending, failed})});
    }
  });
test("Compact navigation does not cover the current subtitle task", async ({ page }, testInfo) => {
  const failures: string[] = [];
  for (const viewport of compactViewports) {
    await page.setViewportSize(viewport);
    await page.goto("/");
    if (viewport.height <= 600) {
      failures.push(...(await initiallyOccludedTargets(page, [".subtitle-overview-copy h2", ".subtitle-coverage-stat > p", ".subtitle-overview-actions :is(button, a.button)"], [".app-header nav"])).map((failure) => `${viewport.width}px dashboard: ${failure}`));
    }
    failures.push(...(await occludedTargets(page, [".subtitle-coverage-stat > p", ".subtitle-overview-copy h2", ".subtitle-overview-actions :is(button, a.button)", ".subtitle-system > summary"], [".app-header nav"])).map((failure) => `${viewport.width}px dashboard: ${failure}`));

    await testInfo.attach(`${viewport.width}-dashboard-scroll`, {contentType: "application/json", body: JSON.stringify(await page.evaluate(() => ({scrollTop: document.scrollingElement!.scrollTop, scrollHeight: document.scrollingElement!.scrollHeight, clientHeight: document.scrollingElement!.clientHeight, mainPadding: getComputedStyle(document.querySelector("main")!).paddingBottom, bodyOverflow: getComputedStyle(document.body).overflowY, summary: document.querySelector(".subtitle-system>summary")?.getBoundingClientRect().toJSON()})))});
    await page.goto("/settings#provider");
    failures.push(...(await occludedTargets(page, ["#provider h2", "#provider input[name=apiKey]", "#provider form[action='/settings/subtitles/subsource'] button"], [".app-header nav", ".search", ".settings-nav"])).map((failure) => `${viewport.width}px provider settings: ${failure}`));
    const skip = await page.locator(".skip").boundingBox();
    if (!skip || skip.y + skip.height > 0) failures.push(`${viewport.width}px settings: skip link is visible`);

    await page.goto("/settings#trusted-https");
    failures.push(...(await occludedTargets(page, ["#trusted-https input[name=provider]", "#trusted-https input[name=domain]", "#trusted-https form[action='/settings/trusted-https'] button"], [".app-header nav", ".search", ".settings-nav"])).map((failure) => `${viewport.width}px trusted HTTPS: ${failure}`));
  }
  expect(failures).toEqual([]);
});


  test("Shared subtitle shell keeps final page controls above the phone dock", async ({page}, testInfo) => {
    for (const viewport of [{width:320, height:800}, {width:390, height:844}, {width:568, height:320}]) {
      await page.setViewportSize(viewport);
      for (const path of ["/settings", "/settings/backups", "/offline-downloads"]) {
        await page.goto(path, {waitUntil:"domcontentloaded"});
        const last = page.locator("main :is(button, a[href], input:not([type=hidden]), select, summary):visible").last();
        await expect(last).toBeVisible();
        await last.evaluate(node => node.setAttribute("data-e2e-final-control", ""));
        await last.focus();
        await expect(last).toBeFocused();
        expect(await occludedTargets(page, ["[data-e2e-final-control]"], [".app-header", ".app-header nav"])).toEqual([]);
        await page.screenshot({path:testInfo.outputPath(viewport.width + "-" + path.replaceAll("/", "-") + "-last-control.png")});
      }
    }
  });
}
