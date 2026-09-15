import { expect, test } from "@playwright/test";
import { uiElementAttachment, uiElementInventory } from "../../../scripts/testing/ui-element-inventory";
import AxeBuilder from "@axe-core/playwright";
import { readFile } from "node:fs/promises";
import { join } from "node:path";

const states = ["api-key-created", "media-share-items", "mfa-setup", "passkey-account", "passkey-prompt", "mfa-required", "oidc-mfa", "viewing-import-preview", "viewing-import-result", "mcp-approval", "supporter-populated-active", "supporter-populated-archived", "state-contracts"];
const viewports = [
	{ width: 1440, height: 900 },
	{ width: 1024, height: 768 },
	{ width: 720, height: 450 },
	{ width: 390, height: 844 },
	{ width: 320, height: 800 },
];

for (const viewport of viewports) {
	test(`conditional and transient UI states fit the ${viewport.width}px viewport`, async ({ page, request }, testInfo) => {
		const directory = process.env.KINOSAIL_UI_FIXTURE_DIR;
		test.skip(!directory, "requires exact production-template fixtures");
		test.setTimeout(120_000);
		const stylesheet = await request.get("/static/app.css");
		expect(stylesheet.ok()).toBeTruthy();
		const css = await stylesheet.text();
		const failures: object[] = [];
		await page.setViewportSize(viewport);
		for (const state of states) {
			const source = await readFile(join(directory!, `${state}.html`), "utf8");
			const html = source.replace(/<link[^>]+app\.css[^>]*>/, `<style>${css}</style>`).replace(/<script[^>]*src=[^>]*><\/script>/g, "");
			await page.setContent(html, { waitUntil: "domcontentloaded" });
			await expect(page.locator("main")).toBeVisible();
			const accessibility = (await new AxeBuilder({ page }).analyze()).violations.map(({ id }) => id);
			const contract = await page.evaluate(() => {
				const viewportWidth = document.documentElement.clientWidth;
				const visible = (element: Element) => {
					const style = getComputedStyle(element);
					const box = element.getBoundingClientRect();
					return style.visibility !== "hidden" && style.display !== "none" && box.width > 0 && box.height > 0;
				};
				const controls = [...document.querySelectorAll("button,input:not([type=hidden]):not([type=checkbox]):not([type=radio]),select,textarea,.back")].filter(visible);
				const ordinary = [...document.querySelectorAll(".settings-shell>section,.settings-flow>section")].filter(visible);
				const grantCard = document.querySelector("main.grant-card:not(.account-card)");
				return {
					overflow: document.documentElement.scrollWidth - viewportWidth,
					outside: [...document.querySelectorAll("main *")]
						.filter(visible)
						.filter((element) => {
							const box = element.getBoundingClientRect();
							return box.left < -1 || box.right > viewportWidth + 1;
						})
						.slice(0, 8)
						.map((element) => element.textContent?.trim().slice(0, 50)),
					tinyControls: controls
						.filter((element) => {
							const box = element.getBoundingClientRect();
							return box.width < 24 || box.height < 24;
						})
						.map((element) => element.textContent?.trim().slice(0, 50)),
					clippedControls: controls.filter((element) => element.scrollWidth > element.clientWidth + 1 || element.scrollHeight > element.clientHeight + 1).map((element) => element.textContent?.trim().slice(0, 50)),
					mixedGlyphs: /[▶★＋↻♫◉▤◆≡✓◇▥←]/.test(document.body.textContent ?? ""),
					invalidIcons: [...document.querySelectorAll('[class^="i-"]')].filter((element) => {
						const style = getComputedStyle(element);
						return element.getAttribute("aria-hidden") !== "true" || !visible(element) || style.maskImage === "none" || (CSS.supports("-webkit-mask-image", "none") && style.getPropertyValue("-webkit-mask-image") === "none");
					}).length,
					blurredSurfaces: ordinary.filter((element) => getComputedStyle(element).backdropFilter !== "none").length,
					shadowedSurfaces: ordinary.filter((element) => getComputedStyle(element).boxShadow !== "none").length,
					oversizedGrant: viewportWidth >= 700 && grantCard ? grantCard.getBoundingClientRect().width > 600 : false,
				};
			});
			if (state === "media-share-items") {
				const stage = await page.locator(".media-stage").boundingBox();
				const video = await page.locator(".media-stage video").boundingBox();
				if (!stage || !video || video.width < stage.width * 0.9 || video.height < Math.min(180, stage.width * 0.5))
					failures.push({
						state,
						viewport: viewport.width,
						sharedMediaStage: { stage, video },
					});
			}
			if (state === "supporter-populated-active") {
				await expect(page.locator(".family-living-standard .badge-level.current")).toContainText("Yours");
				await expect(page.getByLabel("Subscriber service marks").locator("span")).toHaveCount(6);
				await expect(page.getByText("Complete Fleet · Living")).toBeVisible();
			}
			if (state === "supporter-populated-archived") {
				await expect(page.locator(".family-living-standard .badge-level.current")).toContainText("Archived");
				await expect(page.locator(".owned-grant.is-archived")).toContainText("Active through");
				await expect(page.getByLabel("Subscriber service marks").locator("span")).toHaveCount(4);
			}
			if (accessibility.length || contract.overflow || contract.outside.length || contract.tinyControls.length || contract.clippedControls.length || contract.mixedGlyphs || contract.invalidIcons || contract.blurredSurfaces || contract.shadowedSurfaces || contract.oversizedGrant) {
				failures.push({
					state,
					viewport: viewport.width,
					accessibility,
					contract,
				});
			}
			await page.screenshot({
				path: testInfo.outputPath(`${viewport.width}-${state}.png`),
				fullPage: true,
			});
			await testInfo.attach(`${viewport.width}-${state}-elements.json`, await uiElementAttachment(testInfo.outputPath(`${viewport.width}-${state}-elements.json`), await page.evaluate(uiElementInventory)));
		}
		expect(failures).toEqual([]);
	});
}
