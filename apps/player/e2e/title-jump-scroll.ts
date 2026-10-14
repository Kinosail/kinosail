import { expect, type Page, type TestInfo } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

async function observeScroll(page: Page) {
  await page.addInitScript(() => {
    const timeline = { events: [] as unknown[], dropped: 0, errors: 0 };
    (window as Window & { titleScrollTimeline: typeof timeline }).titleScrollTimeline = timeline;
    const kind = (target: unknown) => target === document.body ? "body"
      : target === document.documentElement ? "html"
      : target instanceof Element && target.id === "main" ? "main"
      : target instanceof Element && target.matches("[data-title-letter]") ? "title-letter"
      : target instanceof Element && target.matches("a.card") ? "card" : "other";
    const record = (operation: string, target?: unknown, options?: unknown) => {
      try {
        const settings: Record<string, unknown> = {};
        if (typeof options === "boolean") settings.alignToTop = options;
        if (options && typeof options === "object") {
          for (const key of ["behavior", "block", "inline", "preventScroll"]) {
            const value = (options as Record<string, unknown>)[key];
            if (typeof value === "boolean" || ["auto", "smooth", "instant", "start", "end", "center", "nearest"].includes(value as string)) settings[key] = value;
          }
        }
        const caller = [...(new Error().stack || "").matchAll(/(htmx\.min|main\.kinosail\.bundle)\.js(?:\?[^:\s)]*)?:(\d+):(\d+)/g)]
          .slice(0, 4).map(match => `${match[1]}.js:${match[2]}:${match[3]}`);
        timeline.events.push({ at: performance.now(), operation, target: kind(target), settings, caller,
          y: scrollY, focused: kind(document.activeElement),
          selected: Boolean(document.querySelector('[data-title-letter][aria-current="true"]')) });
        if (timeline.events.length > 200) { timeline.events.shift(); timeline.dropped++; }
      } catch { timeline.errors++; }
    };
    const intoView = Element.prototype.scrollIntoView;
    Element.prototype.scrollIntoView = function(options) {
      record("scrollIntoView", this, options);
      return intoView.call(this, options);
    };
    const focus = HTMLElement.prototype.focus;
    HTMLElement.prototype.focus = function(options) {
      record("focus", this, options);
      return focus.call(this, options);
    };
    window.addEventListener("scroll", () => record("scroll"), { passive: true });
    for (const event of ["htmx:before:request", "htmx:after:request", "htmx:before:settle", "htmx:after:settle", "htmx:after:swap", "htmx:finally:request"])
      document.addEventListener(event, () => record(event));
  });
}

async function record(page: Page, info: TestInfo, name: string) {
  await expect(page.locator('#main[aria-busy="true"]')).toHaveCount(0);
  const receipt = await page.evaluate(async () => {
    await document.fonts.ready;
    const started = performance.now();
    const samples: { at: number; y: number }[] = [];
    await new Promise<void>(resolve => {
      const sample = () => {
        const at = performance.now() - started;
        samples.push({ at, y: window.scrollY });
        if (at >= 800) resolve();
        else requestAnimationFrame(sample);
      };
      requestAnimationFrame(sample);
    });
    const rect = (element: Element) => {
      const { x, y, width, height } = element.getBoundingClientRect();
      return { x, y, width, height };
    };
    const settled = samples.filter(sample => sample.at >= 550);
    return {
      viewport: { width: innerWidth, height: innerHeight },
      documentStarted: performance.timeOrigin,
      scroll: {
        initial: samples[0].y, final: scrollY,
        min: Math.min(...samples.map(sample => sample.y)),
        max: Math.max(...samples.map(sample => sample.y)),
        settledMin: Math.min(...settled.map(sample => sample.y)),
        settledMax: Math.max(...settled.map(sample => sample.y)),
        settledFrames: settled.length, duration: samples.at(-1)!.at,
        extent: document.documentElement.scrollHeight - innerHeight,
      },
      body: rect(document.body), main: rect(document.getElementById("main")!),
      cardCount: document.querySelectorAll("#library a.card").length,
      pending: document.querySelector('#main[aria-busy="true"]') !== null,
      horizontalOverflow: document.documentElement.scrollWidth > innerWidth,
    };
  });
  await info.attach(`${name}-geometry`, { body: JSON.stringify(receipt, null, 2), contentType: "application/json" });
  await page.screenshot({ path: info.outputPath(`${name}.png`), fullPage: true });
  return receipt;
}

async function letter(page: Page, selected: boolean, compact: boolean) {
  const index = page.getByRole("navigation", { name: "Jump to title", exact: true });
  if (compact) {
    await expect(index).toBeHidden();
    await page.getByRole("button", { name: /^Jump to title/ }).click();
    await expect(page.getByRole("dialog", { name: "Jump to title", exact: true })).toBeVisible();
  }
  const navigation = compact ? page.getByRole("navigation", { name: "Choose a title letter" }) : index;
  await navigation.getByRole("link", { name: selected ? "B, 1 title, selected; activate to show all titles" : "B, 1 title", exact: true }).click();
}

export async function verifyTitleJumpScroll(page: Page, info: TestInfo) {
  await observeScroll(page);
  for (const viewport of [
    { width: 1440, height: 900, compact: false, rapid: true },
    { width: 1440, height: 900, compact: false },
    { width: 390, height: 650, compact: true },
    { width: 390, height: 844, compact: false },
    { width: 1920, height: 1080, compact: false },
  ]) {
    let primaryFailure = false;
    try {
      await page.setViewportSize(viewport);
      await page.goto("/?view=movies");
      const prefix = `title-jump-${viewport.rapid ? "rapid-" : ""}${viewport.width}x${viewport.height}`;
      const documentStarted = await page.evaluate(() => performance.timeOrigin);
      if (!viewport.rapid) await record(page, info, `${prefix}-loaded`);
      await letter(page, false, viewport.compact);
      await expect(page).toHaveURL(/letter=B/);
      await expect(page.getByRole("heading", { name: "Beta", exact: true })).toBeVisible();
      await expect(page.getByRole("heading", { name: "Arrival", exact: true })).toHaveCount(0);
      await expect(page.locator("#library a.card").first()).toBeFocused();
      if (viewport.rapid) {
        // Keep the original interaction pace. Sampling or screenshots here can
        // let an in-flight swap settle before the viewer clears the same letter.
        await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0);
      } else {
        const selected = await record(page, info, `${prefix}-selected`);
        expect.soft(selected.scroll.settledFrames, prefix).toBeGreaterThan(1);
        expect.soft([selected.scroll.settledMin, selected.scroll.settledMax], prefix).toEqual([0, 0]);
      }
      await letter(page, true, viewport.compact);
      await expect(page).not.toHaveURL(/letter=/);
      await expect(page.getByRole("heading", { name: "Arrival", exact: true })).toBeVisible();
      await expect(page.getByRole("heading", { name: "Beta", exact: true })).toBeVisible();
      const cleared = await record(page, info, `${prefix}-cleared`);
      expect.soft(cleared.documentStarted, "Letter filtering must use the same document").toBe(documentStarted);
      expect.soft(cleared.scroll.settledFrames, prefix).toBeGreaterThan(1);
      expect.soft([cleared.scroll.settledMin, cleared.scroll.settledMax], prefix).toEqual([0, 0]);
      expect.soft(cleared.horizontalOverflow, prefix).toBe(false);
      expect.soft((await new AxeBuilder({ page }).include("#main").analyze()).violations, prefix).toEqual([]);
    } catch (error) {
      primaryFailure = true;
      throw error;
    } finally {
      if (viewport.rapid) {
        const observation = await page.evaluate(() => ({
          timeline: (window as Window & { titleScrollTimeline: { events: unknown[]; dropped: number; errors: number } }).titleScrollTimeline,
          scrollBehavior: getComputedStyle(document.documentElement).scrollBehavior,
        })).catch(() => ({ unavailable: true }));
        await info.attach("title-jump-rapid-scroll-timeline", {
          contentType: "application/json", body: JSON.stringify(observation),
        }).catch(error => {
          if (!primaryFailure) throw error;
          info.annotations.push({ type: "diagnostic", description: "Rapid scroll attachment unavailable after a primary failure." });
        });
      }
    }
  }
}
