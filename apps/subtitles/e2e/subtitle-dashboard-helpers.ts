import { expect } from "@playwright/test";
import { createHmac } from "node:crypto";

export const supportedViewports = [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 568, height: 320 }, { width: 390, height: 844 }, { width: 320, height: 800 }];
export const compactViewports = supportedViewports.slice(1);

function totp(): string {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  const bits = [...(process.env.KINOSAIL_TEST_TOTP_SECRET ?? "")].map((character) => alphabet.indexOf(character).toString(2).padStart(5, "0")).join("");
  const secret = Buffer.from(bits.match(/.{8}/g)?.map((byte) => Number.parseInt(byte, 2)) ?? []);
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30_000)));
  const digest = createHmac("sha1", secret).update(counter).digest();
  const offset = digest[19] & 15;
  return ((digest.readUInt32BE(offset) & 0x7fffffff) % 1_000_000).toString().padStart(6, "0");
}

export async function login(page: import("@playwright/test").Page) {
  await page.goto("/login?next=/");
  await page.getByLabel("Name").fill("Owner");
  await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
  await page.getByLabel("6-digit code").fill(totp());
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page.waitForURL((url) => url.pathname !== "/login");
  if (new URL(page.url()).pathname === "/account") {
    await page.getByRole("link", { name: "Not now" }).click();
  }
  await expect(page).toHaveURL("/");
}

export async function setSubtitleLanguages(page: import("@playwright/test").Page, languages: string[]): Promise<number> {
  return page.evaluate(async (selected) => {
    const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')?.content ?? "";
    const response = await fetch("/api/v1/settings/subtitles", {
      method: "PUT",
      headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf },
      body: JSON.stringify({ languages: selected }),
    });
    return response.status;
  }, languages);
}

type Rect = { x: number; y: number; width: number; height: number };

function overlaps(left: Rect, right: Rect): boolean {
  return left.x < right.x + right.width && left.x + left.width > right.x && left.y < right.y + right.height && left.y + left.height > right.y;
}

function freeVerticalBand(height: number, target: Rect, occluders: Rect[]): { top: number; bottom: number } {
  const intervals = occluders
    .filter((box) => target.x < box.x + box.width && target.x + target.width > box.x)
    .map((box) => ({ top: Math.max(0, box.y), bottom: Math.min(height, box.y + box.height) }))
    .filter((interval) => interval.top < interval.bottom)
    .sort((left, right) => left.top - right.top);
  const bands: { top: number; bottom: number }[] = [];
  let top = 0;
  for (const interval of intervals) {
    if (interval.top > top) bands.push({ top, bottom: interval.top });
    top = Math.max(top, interval.bottom);
  }
  if (top < height) bands.push({ top, bottom: height });
  return bands.sort((left, right) => (right.bottom - right.top) - (left.bottom - left.top))[0] ?? { top: 0, bottom: height };
}

export async function occludedTargets(page: import("@playwright/test").Page, targets: string[], occluders: string[]): Promise<string[]> {
  const viewport = page.viewportSize();
  if (!viewport) return ["viewport is unavailable"];
  await page.evaluate(() => {
    document.documentElement.style.scrollBehavior = "auto";
  });
  const failures: string[] = [];
  for (const selector of targets) {
    const locators = await page.locator(selector).all();
    if (locators.length === 0) failures.push(`${selector} is missing`);
    for (const locator of locators) {
      if (!await locator.isVisible()) {
        failures.push(`${selector} is hidden`);
        continue;
      }
      await locator.evaluate((element) => element.scrollIntoView({ block: "center", inline: "nearest" }));
      await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => resolve())));
      for (let attempt = 0; attempt < 5; attempt += 1) {
        const box = await locator.boundingBox();
        if (!box) break;
        const occluderBoxes = (await Promise.all(occluders.map(async (occluder) => Promise.all((await page.locator(occluder).all()).map((element) => element.boundingBox()))))).flat().filter((candidate): candidate is Rect => candidate !== null);
        const band = freeVerticalBand(viewport.height, box, occluderBoxes);
        if (box.height > band.bottom - band.top) break;
        const desiredTop = band.top + (band.bottom - band.top - box.height) / 2;
        if (Math.abs(box.y - desiredTop) <= 1) break;
        await locator.evaluate((element, top) => {
          const scroller = document.scrollingElement;
          if (scroller) scroller.scrollTop += element.getBoundingClientRect().top - top;
        }, desiredTop);
        await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
      }
      const box = await locator.boundingBox();
      if (!box || box.y + box.height <= 0 || box.y >= viewport.height) {
        failures.push(`${selector} is outside the viewport after scrolling`);
        continue;
      }
      const boxes = (await Promise.all(occluders.map(async (occluder) => Promise.all((await page.locator(occluder).all()).map(async (element) => ({ selector: occluder, box: await element.boundingBox() })))))).flat();
      for (const occluder of boxes) {
        if (occluder.box && overlaps(box, occluder.box)) failures.push(`${selector} at ${Math.round(box.y)}-${Math.round(box.y + box.height)} under ${occluder.selector} at ${Math.round(occluder.box.y)}-${Math.round(occluder.box.y + occluder.box.height)}`);
      }
    }
  }
  return failures;
}

export async function initiallyOccludedTargets(page: import("@playwright/test").Page, targets: string[], occluders: string[]): Promise<string[]> {
  const viewport = page.viewportSize();
  if (!viewport) return ["viewport is unavailable"];
  const failures: string[] = [];
  const occluderBoxes = (await Promise.all(occluders.map(async (selector) => Promise.all((await page.locator(selector).all()).map(async (element) => ({ selector, box: await element.boundingBox() })))))).flat();
  for (const selector of targets) {
    const locators = await page.locator(selector).all();
    if (locators.length === 0) failures.push(`${selector} is missing`);
    for (const locator of locators) {
      const box = await locator.boundingBox();
      if (!await locator.isVisible() || !box) {
        failures.push(`${selector} is hidden`);
        continue;
      }
      if (box.y < 0 || box.y + box.height > viewport.height) failures.push(`${selector} is outside the initial viewport`);
      for (const occluder of occluderBoxes) {
        if (occluder.box && overlaps(box, occluder.box)) failures.push(`${selector} at ${Math.round(box.y)}-${Math.round(box.y + box.height)} initially overlaps ${occluder.selector} at ${Math.round(occluder.box.y)}-${Math.round(occluder.box.y + occluder.box.height)}`);
      }
    }
  }
  return failures;
}

export async function expectSkipLinkOffscreen(page: import("@playwright/test").Page) {
  await page.evaluate(() => {
    (document.activeElement as HTMLElement | null)?.blur();
    document.documentElement.style.scrollBehavior = "auto";
    if (document.scrollingElement) document.scrollingElement.scrollTop = 0;
  });
  await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
  const skip = await page.locator(".skip").boundingBox();
  expect(skip === null || skip.y + skip.height <= 0).toBe(true);
}

export async function expectNoHorizontalOverflow(page: import("@playwright/test").Page) {
  const dimensions = await page.locator("html").evaluate((element) => {
    const viewportWidth = window.innerWidth;
    const offenders = [...document.body.querySelectorAll("*")].flatMap((candidate) => {
      const box = candidate.getBoundingClientRect();
      const style = getComputedStyle(candidate);
      return box.right > viewportWidth + 1 || box.left < -1 || (candidate.scrollWidth > candidate.clientWidth + 1 && style.overflowX === "visible")
        ? [`${candidate.tagName.toLowerCase()}.${candidate.className}:${box.left.toFixed(1)}-${box.right.toFixed(1)}/${candidate.clientWidth}-${candidate.scrollWidth}`]
        : [];
    });
    return { viewportWidth, contentWidth: element.scrollWidth, offenders: offenders.slice(0, 5) };
  });
  expect(dimensions.contentWidth, JSON.stringify(dimensions)).toBeLessThanOrEqual(dimensions.viewportWidth);
}
