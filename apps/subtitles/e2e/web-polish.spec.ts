import { expect, test } from "@playwright/test";
import { login } from "./subtitle-dashboard-helpers";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires an isolated populated Subtitles instance");
test.setTimeout(120_000);
test.use({ serviceWorkers: "block" });

test("light appearance keeps the header opaque over scrolled content", async ({ page }) => {
  await page.addInitScript(() => { try { localStorage.setItem("kinosail-theme", "light"); } catch {} });
  await login(page);
  for (const width of [390, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    for (const route of ["/", "/settings"]) {
      await page.goto(route);
      const alpha = await page.locator(".app-header").first().evaluate((element) => {
        const parts = getComputedStyle(element).backgroundColor.match(/[\d.]+/g) || [];
        return parts.length > 3 ? Number(parts[3]) : 1;
      });
      expect(alpha, `${route} at ${width}px`).toBe(1);
    }
  }
});

test("enlarged phone text keeps backup history inside the window", async ({ page }) => {
  await login(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/settings/backups");
  await page.evaluate(() => document.documentElement.style.setProperty("font-size", "200%", "important"));
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0);
});

test("enlarged text keeps the Server name, filters, file titles and overview readable", async ({ page }) => {
  await login(page);
  for (const width of [390, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    for (const route of ["/", "/?view=library", "/settings"]) {
      await page.goto(route);
      await page.evaluate(() => document.documentElement.style.setProperty("font-size", "200%", "important"));
      const issues = await page.evaluate(() => {
        const found: string[] = [];
        const overflow = document.documentElement.scrollWidth - document.documentElement.clientWidth;
        if (overflow > 0) found.push(`page overflows by ${overflow}px`);
        for (const name of document.querySelectorAll(".subtitle-brand-name")) {
          if (name.getClientRects().length && name.scrollWidth > name.clientWidth + 1) found.push(`Server name is cut to ${name.clientWidth}px`);
        }
        for (const select of document.querySelectorAll<HTMLSelectElement>(".subtitle-filter select")) {
          const style = getComputedStyle(select);
          const needed = Number.parseFloat(style.fontSize) * 1.15 + Number.parseFloat(style.paddingTop) + Number.parseFloat(style.paddingBottom);
          if (select.clientHeight < needed) found.push(`${select.name} filter is ${select.clientHeight}px tall for ${Math.round(needed)}px of text`);
        }
        for (const title of document.querySelectorAll(".subtitle-file-title strong")) {
          if (title.scrollWidth > title.clientWidth + 1 && title.clientWidth / Number.parseFloat(getComputedStyle(title).fontSize) < 8) found.push(`${title.textContent} title is ${title.clientWidth}px wide`);
        }
        for (const element of document.querySelectorAll("main :is(h1,h2,h3,strong,button,.button,label), .subtitle-brand-name")) {
          if (!element.getClientRects().length) continue;
          for (const node of element.childNodes) {
            if (node.nodeType !== Node.TEXT_NODE) continue;
            for (const word of (node as Text).data.matchAll(/[\p{L}\d]{3,}/gu)) {
              const range = document.createRange();
              const tops = new Set<number>();
              for (let index = word.index!; index < word.index! + word[0].length; index++) {
                range.setStart(node, index);
                range.setEnd(node, index + 1);
                const rect = range.getBoundingClientRect();
                if (rect.width) tops.add(Math.round(rect.top));
              }
              if (tops.size > 1) found.push(`${element.tagName.toLowerCase()}.${element.className} splits ${word[0]}`);
            }
          }
        }
        return found;
      });
      expect(issues, `${route} at ${width}px`).toEqual([]);
    }
  }
});

async function inspector(page: import("@playwright/test").Page) {
  await page.goto("/?view=library");
  const href = await page.locator('main a[href^="/subtitles/inspect/"]').first().getAttribute("href");
  expect(href).toBeTruthy();
  return href!;
}

test("inspector leads back like Player and styles the file picker", async ({ page }) => {
  await login(page);
  await page.goto(await inspector(page));
  const back = page.getByRole("link", { name: "Back to library", exact: true });
  await expect(back.locator("i.i-back")).toHaveCount(1);
  const picker = await page.locator('input[type="file"]').evaluate((input) => {
    const button = getComputedStyle(input, "::file-selector-button");
    const quiet = getComputedStyle([...document.querySelectorAll("main button")].find((candidate) => candidate.textContent?.trim() === "Analyze audio")!);
    return { font: button.fontFamily, radius: button.borderTopLeftRadius, quietRadius: quiet.borderTopLeftRadius, background: button.backgroundColor, quietBackground: quiet.backgroundColor, border: button.borderTopColor, quietBorder: quiet.borderTopColor, height: Number.parseFloat(button.minHeight) || 0 };
  });
  expect(picker.font).toContain("Manrope");
  expect(picker.radius).toBe(picker.quietRadius);
  expect(picker.background).toBe(picker.quietBackground);
  expect(picker.border).toBe(picker.quietBorder);
  expect(picker.height).toBeGreaterThanOrEqual(36);
});

test("settings tabs and section dividers share one width", async ({ page }) => {
  await login(page);
  for (const width of [390, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/settings");
    const edges = await page.evaluate(() => {
      const main = document.querySelector("main.settings-shell")!;
      return [...main.querySelectorAll(":scope > .settings-nav, :scope > .settings-intro, :scope > .settings-flow > section")]
        .filter((element) => element.getBoundingClientRect().width > 0)
        .map((element) => {
          const box = element.getBoundingClientRect(), style = getComputedStyle(element);
          return { name: `${element.tagName}#${element.id}`, left: Math.round(box.left), right: Math.round(box.right), introDivider: element.matches(".settings-intro") && style.borderBottomStyle !== "none" && Number.parseFloat(style.borderBottomWidth) > 0 };
        });
    });
    expect(edges.length).toBeGreaterThan(3);
    for (const edge of edges) {
      expect(Math.abs(edge.left - edges[0].left), `${edge.name} at ${width}px`).toBeLessThanOrEqual(1);
      expect(Math.abs(edge.right - edges[0].right), `${edge.name} at ${width}px`).toBeLessThanOrEqual(1);
    }
    expect(edges.filter((edge) => edge.introDivider), `intro divider at ${width}px`).toEqual([]);
  }
});

test("cleanup preview without a valid form shows the Subtitles error page and changes nothing", async ({ page }) => {
  await login(page);
  const snapshot = async () => Promise.all(["/api/v1/settings", "/api/v1/subtitle-library"].map(async (path) => (await page.request.get(path)).text()));
  const before = await snapshot();
  for (const query of ["", "?enabled=on", "?enabled=on&language=zz-invalid&forced=keep", "?enabled=on&language=en&forced=delete"]) {
    const response = await page.goto(`/settings/subtitles/cleanup${query}`);
    expect(response!.status(), query).toBe(400);
    expect(response!.headers()["content-type"], query).toContain("text/html");
    await expect(page.locator('link[rel="stylesheet"][href^="/static/app.css"]'), query).toHaveCount(1);
    await expect(page.locator("main h2"), query).toContainText("Preview files to hide again");
    await expect(page.getByRole("link", { name: "Open Subtitle settings" }), query).toHaveAttribute("href", "/settings#cleanup");
  }
  expect(await snapshot()).toEqual(before);
});
