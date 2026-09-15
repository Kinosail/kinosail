import { expect, test } from "@playwright/test";
import { uiElementAttachment, uiElementInventory } from "../../../scripts/testing/ui-element-inventory";
import AxeBuilder from "@axe-core/playwright";
import { readFile } from "node:fs/promises";
import { join } from "node:path";

const states = ["api-key-created", "media-share-items", "mfa-setup", "passkey-account", "passkey-prompt", "supporter-certificate", "mfa-required", "oidc-mfa", "viewing-import-preview", "viewing-import-result", "mcp-approval", "state-contracts"];
const viewports = [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 800 }];

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
			const html = (state === "supporter-certificate" ? `<!doctype html><html lang="en"><head><title>Supporter certificate</title></head><body><main><h1 style="position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%)">Supporter certificate</h1>${source}</main></body></html>` : source)
				.replace("<html", '<html data-theme="dark"')
				.replace(/<link[^>]+app\.css[^>]*>/, `<style>${css}</style>`)
				.replace(/<script[^>]*src=[^>]*><\/script>/g, "");
			await page.setContent(html, { waitUntil: "domcontentloaded" });
			await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
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
					outside: [...document.querySelectorAll("main *")].filter(visible).filter((element) => {
						const box = element.getBoundingClientRect();
						return box.left < -1 || box.right > viewportWidth + 1;
					}).slice(0, 8).map((element) => {
						const box = element.getBoundingClientRect();
						return { element: `${element.tagName.toLowerCase()}.${element.className}`, left: box.left, right: box.right, width: box.width };
					}),
					tinyControls: controls.filter((element) => {
						const box = element.getBoundingClientRect();
						return box.width < 24 || box.height < 24;
					}).map((element) => element.textContent?.trim().slice(0, 50)),
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
				if (!stage || !video || video.width < stage.width * .9 || video.height < Math.min(180, stage.width * .5)) failures.push({ state, viewport: viewport.width, sharedMediaStage: { stage, video } });
			}
			if (state === "mcp-approval") {
				const approval = await page.locator(".mcp-approval-form").boundingBox();
				const fieldset = await page.locator(".mcp-approval-form fieldset").boundingBox();
				const labelHeights = await page.locator(".mcp-approval-form label").evaluateAll((labels) => labels.map((label) => label.getBoundingClientRect().height));
				expect(approval?.width, `${viewport.width}px approval width`).toBeGreaterThan(280);
				expect(fieldset?.width, `${viewport.width}px approval fieldset width`).toBeGreaterThan(260);
				expect(labelHeights.length, `${viewport.width}px approval labels`).toBeGreaterThan(0);
				expect(labelHeights.every((height) => height >= 44), `${viewport.width}px approval label height`).toBeTruthy();
			}
			if (state === "api-key-created") {
				await expect(page.locator("#api-key-secret")).toBeVisible();
				await expect(page.getByRole("button", { name: "Copy API key" })).toBeVisible();
				await expect(page.locator(".copy-status")).toHaveAttribute("aria-live", "polite");
			}
			if (accessibility.length || contract.overflow || contract.outside.length || contract.tinyControls.length || contract.clippedControls.length || contract.mixedGlyphs || contract.invalidIcons || contract.blurredSurfaces || contract.shadowedSurfaces || contract.oversizedGrant) {
				failures.push({ state, viewport: viewport.width, accessibility, contract });
			}
			await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-${state}.png`), fullPage: true });
			await testInfo.attach(`${viewport.width}-${state}-elements.json`, await uiElementAttachment(testInfo.outputPath(`${viewport.width}-${state}-elements.json`), await page.evaluate(uiElementInventory)));
		}
		expect(failures).toEqual([]);
	});
}
