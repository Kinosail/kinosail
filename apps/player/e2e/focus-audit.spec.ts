import { expect, test } from "@playwright/test";
import { attachResponsiveFailure } from "./responsive-failure-witness.mjs";
import { writeFile } from "node:fs/promises";
import { configureTestInstance, login } from "./test-instance-helpers";

configureTestInstance();
test.use({ serviceWorkers: "block" });

for (const width of [390, 1440]) {
	test(`populated Player pages expose visible keyboard highlights at ${width}px`, async ({ page }, testInfo) => {
		test.setTimeout(240_000);
		await page.setViewportSize({ width, height: 900 });
		await page.emulateMedia({ reducedMotion: "reduce" });
		await login(page);
		const routes = new Set(["/", ...["all", "movies", "shows", "music", "audiobooks", "books", "photos", "list", "collections", "playlists", "history", "unwatched"].map(view => `/?view=${view}`), "/?q=missing-focus-audit-title", "/settings", "/account", "/quick-connect", "/offline-downloads", "/supporter"]);
		const report: object[] = [];
		const failures: object[] = [];
		for (const route of routes) {
			const response = await page.goto(route, { waitUntil: "domcontentloaded" });
			expect(response?.ok(), route).toBeTruthy();
			if (!response?.headers()["content-type"]?.includes("text/html")) continue;
			await expect(page.locator("main")).toBeVisible();
			const media = page.locator("video,audio");
			if (/^\/(watch|album)\//.test(route) && await media.count()) {
				await expect.poll(() => media.first().evaluate((element: HTMLMediaElement) => element.readyState)).toBeGreaterThanOrEqual(2);
				// The short fixture episodes advance automatically; audit one stable document.
				await media.evaluateAll(elements => elements.forEach((element: HTMLMediaElement) => element.pause()));
			}
			// Discover detail and settings pages from production links, excluding exports and mutation endpoints.
			const discovered = await page.locator("main a[href]:not([download])").evaluateAll(links => links.map(link => link.getAttribute("href")!).filter(href => /^\/(show|album|book|item|collection|watch|photo|playlist)\/[^/?.#]+$/.test(href) || /^\/settings(?:\/[^/.?#]+)?$/.test(href) && !["/settings/metrics", "/settings/backup"].includes(href)));
			for (const href of discovered) routes.add(href);
			const controls = page.locator('a[href]:visible,button:visible:not(:disabled),input:visible:not(:disabled):not([type=hidden]),select:visible:not(:disabled),textarea:visible:not(:disabled),summary:visible,[tabindex="0"]:visible');
			let checked = 0;
			for (const control of await controls.all()) {
				if (!(await control.isEnabled())) continue;
                // Closed native details may retain descendant boxes in WebKit.
                if (!await control.evaluate(element => {
                  for (let parent = element.parentElement; parent; parent = parent.parentElement) {
                    if (parent instanceof HTMLDetailsElement && !parent.open && !parent.querySelector(":scope > summary")?.contains(element)) return false;
                  }
                  return true;
                })) continue;
				await control.focus();
				const state = await control.evaluate(element => {
					const style = getComputedStyle(element);
					const hasOutline = (target: Element, pseudo?: string) => {
						const s = getComputedStyle(target, pseudo);
						return s.outlineStyle !== "none" && parseFloat(s.outlineWidth) >= 2 && s.outlineColor !== "rgba(0, 0, 0, 0)";
					};
					const poster = element.querySelector(".poster,.curation-poster,.collection-poster");
					const choice = style.opacity === "0" ? element.nextElementSibling : null;
					return { label: element.getAttribute("aria-label") || element.textContent?.trim().slice(0, 70) || element.getAttribute("name") || element.tagName,
						skipControl: element.matches('a.skip[href="#main"]'), focused: document.activeElement === element, visible: hasOutline(element) || !!poster && hasOutline(poster) || !!choice && hasOutline(choice) };
				});
				if (!state.focused || !state.visible) {
					failures.push({ route, ...state });
					if (state.skipControl) await attachResponsiveFailure(page, testInfo, "keyboard-focus");
				}
				checked++;
			}
			report.push({ route, controls: checked });
			console.log(`${width}px ${route}: ${checked} controls`);
			const artwork = page.locator("main a:visible:has(.poster,.curation-poster,.collection-poster)").first();
			const sample = await artwork.count() ? artwork : page.locator("main a:visible,main button:visible:not(:disabled)").first();
			if (await sample.count()) await sample.focus();
			await page.screenshot({ path: testInfo.outputPath(`${report.length}-focus.png`) });
		}
		const evidence = testInfo.outputPath("focus-audit.json");
		await writeFile(evidence, JSON.stringify({ revision: process.env.KINOSAIL_TEST_REVISION ?? process.env.GITHUB_SHA, command: "pnpm exec playwright test focus-audit.spec.ts", width, browser: testInfo.project.name, environment: "isolated populated test Server, generated media and metadata", pages: report, failures, result: failures.length ? "failed" : "passed" }, null, 2));
		await testInfo.attach("focus-audit.json", { path: evidence, contentType: "application/json" });
		expect(report.length).toBeGreaterThan(25);
		expect(failures).toEqual([]);
	});
}
