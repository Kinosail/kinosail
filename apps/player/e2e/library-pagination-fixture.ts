import {expect, type Page, type TestInfo} from "@playwright/test";
import {navigationDiagnostics} from "../../../scripts/testing/navigation-diagnostics.mjs";

export async function openPaginationLibrary(page: Page, url: string, info?: TestInfo) {
  const navigation = navigationDiagnostics(page);
  try {
    await page.goto(url, {waitUntil:"domcontentloaded"});
    await expect(page.locator("#library")).toBeVisible();
  } catch (error) {
    await info?.attach("library-navigation-failure", {body:JSON.stringify(await navigation.snapshot()), contentType:"application/json"});
    throw error;
  } finally {navigation.stop();}
}
