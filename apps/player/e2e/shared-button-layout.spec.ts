import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";

const stylesheet = new URL("../../../packages/webassets/static/player-app.css", import.meta.url);
const longLabel = "Deployment configuration for a household device with an unusually long name";

for (const width of [320, 390, 768, 900, 1440]) {
	for (const hasTouch of [false, true]) {
		test(`shared action buttons fit and remain operable at ${width}px, touch=${hasTouch}`, async ({ browser }, testInfo) => {
			const context = await browser.newContext({ viewport: { width, height: 900 }, hasTouch });
			try {
				const page = await context.newPage();
				await page.setContent(`<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body class="settings-page">
					<main class="settings-shell"><div class="settings-flow"><section>
						<h2>Integrations</h2><p>OpenID Connect: <strong class="status">Not configured</strong> · <a class="mode" href="#destination"><span>Deployment configuration</span></a></p>
						<p><a class="mode" href="#destination"><span>${longLabel}</span></a></p>
						<p><a class="mode" href="#destination"><span>${"Configuration".repeat(12)}</span></a></p>
						<form action="/settings/sessions/revoke"><button type="button">Sign out other devices</button></form>
						<form><button type="button">${longLabel}</button><button type="button" disabled>Managed by deployment</button><button type="button" hidden>Unavailable action</button></form>
						<p><a class="header-link" href="#destination"><span>Account</span></a> <a class="button" href="#destination">Continue</a></p>
						<div class="marker-actions"><button type="button">${longLabel}</button><button type="button">Skip credits</button></div>
						<details><summary>Manual pairing</summary><p>Pairing instructions</p></details>
						<div id="destination">Destination</div>
					</section></div></main></body></html>`);
				await page.addStyleTag({ content: await readFile(stylesheet, "utf8") });
				const controls = page.locator("button:visible, a.mode, a.button, a.header-link, summary");
				for (const control of await controls.all()) {
					await control.scrollIntoViewIfNeeded();
					const geometry = await control.evaluate((element) => {
						const box = element.getBoundingClientRect();
						const hit = document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2);
						return { width: box.width, height: box.height, left: box.left, right: box.right,
							clipped: element.scrollWidth > element.clientWidth + 1 || element.scrollHeight > element.clientHeight + 1,
							reachable: !!hit && (hit === element || element.contains(hit)) };
					});
					expect(geometry.clipped, await control.textContent()).toBe(false);
					expect(geometry.reachable, await control.textContent()).toBe(true);
					expect(geometry.left).toBeGreaterThanOrEqual(0);
					expect(geometry.right).toBeLessThanOrEqual(width);
					if (width <= 900 || hasTouch) {
						expect(geometry.height).toBeGreaterThanOrEqual(44);
						expect(geometry.width).toBeGreaterThanOrEqual(44);
					}
				}
				for (const control of await page.locator("a.mode, a.header-link").all()) {
					expect(await control.evaluate((element) => {
						const box = element.getBoundingClientRect();
						const label = element.querySelector("span")!.getBoundingClientRect();
						return Math.abs(label.y + label.height / 2 - box.y - box.height / 2);
					})).toBeLessThanOrEqual(1);
				}
				await expect(page.getByRole("button", { name: "Managed by deployment" })).toBeDisabled();
				await expect(page.getByText("Unavailable action")).toBeHidden();
				await page.getByText("Manual pairing", { exact: true }).click();
				await expect(page.getByText("Pairing instructions")).toBeVisible();
				await page.getByRole("link", { name: "Deployment configuration", exact: true }).focus();
				await page.keyboard.press("Enter");
				expect(await page.evaluate(() => location.hash)).toBe("#destination");
				await page.screenshot({ path: testInfo.outputPath("shared-buttons.png"), fullPage: true });
			} finally {
				await context.close();
			}
		});
	}
}
