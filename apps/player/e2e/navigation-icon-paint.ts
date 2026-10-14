import { expect, type Page, type TestInfo } from "@playwright/test";

// An accessible name and a CSS mask can both exist while the icon paints nothing.
// Compare the real viewport with a negative control that hides only those glyphs.
export async function expectNavigationIconPaint(page: Page, info: TestInfo, name: string) {
	const selector = ".header-compact-menu > summary, .mobile-navigation > a:visible, .mobile-navigation .nav-more > summary";
	const controls = page.locator(selector);
	await expect(controls).toHaveCount(6);
	await page.evaluate(async () => {
		await document.fonts.ready;
		await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
	});
	const boxes = await controls.evaluateAll(elements => elements.map(element => {
		const box = element.getBoundingClientRect();
		return { name: element.textContent!.trim(), x: box.x, y: box.y, width: box.width, height: box.height };
	}));
	const hiddenStyle = ".header-compact-menu > summary::before, .mobile-navigation > a::before, .mobile-navigation .nav-more > summary::before { visibility: hidden !important; }";
	// Inline screenshot styles are rejected by the real Server's CSP. Use CSSOM
	// for this temporary negative control, and verify it actually hides each glyph.
	const sheet = await page.evaluateHandle(rule => {
		const sheet = new CSSStyleSheet();
		sheet.replaceSync(rule);
		return sheet;
	}, hiddenStyle);
	const capture = async (hidden: boolean, suffix: string) => {
		await sheet.evaluate((sheet, hidden) => {
			document.adoptedStyleSheets = document.adoptedStyleSheets.filter(candidate => candidate !== sheet);
			if (hidden) document.adoptedStyleSheets.push(sheet);
		}, hidden);
		expect(await controls.evaluateAll(elements => elements.map(element => getComputedStyle(element, "::before").visibility)))
			.toEqual(Array(6).fill(hidden ? "hidden" : "visible"));
		return page.screenshot({ path: info.outputPath(`${name}-icons${suffix}.png`), animations: "disabled", scale: "css" });
	};
	let hiddenBefore: Buffer, painted: Buffer, hiddenAfter: Buffer;
	let primaryFailed = false;
	try {
		hiddenBefore = await capture(true, "-hidden-before");
		painted = await capture(false, "");
		hiddenAfter = await capture(true, "-hidden-after");
	} catch (error) {
		primaryFailed = true;
		throw error;
	} finally {
		let cleanupError: unknown;
		await sheet.evaluate(sheet => { document.adoptedStyleSheets = document.adoptedStyleSheets.filter(candidate => candidate !== sheet); })
			.catch(error => { cleanupError = error; });
		await sheet.dispose().catch(error => { cleanupError ??= error; });
		if (cleanupError && !primaryFailed) throw cleanupError;
	}
	const pixels = await page.evaluate(async ({ original, firstControl, secondControl, boxes }) => {
		const decode = async (png: string) => {
			const image = new Image();
			image.src = `data:image/png;base64,${png}`;
			await image.decode();
			const canvas = document.createElement("canvas");
			canvas.width = image.width; canvas.height = image.height;
			const context = canvas.getContext("2d")!;
			context.drawImage(image, 0, 0);
			return context;
		};
		const [before, after, repeated] = await Promise.all([decode(original), decode(firstControl), decode(secondControl)]);
		const luminance = (data: Uint8ClampedArray, index: number) => [0, 1, 2].reduce((total, channel) => {
			const value = data[index + channel] / 255;
			return total + (value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4) * [.2126, .7152, .0722][channel];
		}, 0);
		return boxes.map(box => {
			const width = Math.floor(box.width), height = Math.floor(box.height);
			const a = before.getImageData(Math.floor(box.x), Math.floor(box.y), width, height).data;
			const b = after.getImageData(Math.floor(box.x), Math.floor(box.y), width, height).data;
			const c = repeated.getImageData(Math.floor(box.x), Math.floor(box.y), width, height).data;
			let contrastingPixels = 0, controlDifference = 0;
			for (let i = 0; i < a.length; i += 4) {
				const [light, dark] = [luminance(a, i), luminance(b, i)].sort((x, y) => y - x);
				if ((light + .05) / (dark + .05) >= 3) contrastingPixels++;
				const [controlLight, controlDark] = [luminance(b, i), luminance(c, i)].sort((x, y) => y - x);
				if ((controlLight + .05) / (controlDark + .05) >= 3) controlDifference++;
			}
			return { ...box, contrastingPixels, controlDifference };
		});
	}, { original: painted.toString("base64"), firstControl: hiddenBefore.toString("base64"), secondControl: hiddenAfter.toString("base64"), boxes });
	await info.attach(`${name}-icon-paint.json`, { body: JSON.stringify({ revision: process.env.KINOSAIL_TEST_REVISION, browser: info.project.name, data: "Real populated Server navigation; hidden-pseudo-element negative control", pixels }), contentType: "application/json" });
	for (const icon of pixels) {
		expect(icon.controlDifference, `${name} ${icon.name} hidden control stability`).toBe(0);
		expect.soft(icon.contrastingPixels, `${name} ${icon.name} painted icon at 3:1 contrast`).toBeGreaterThanOrEqual(8);
	}
}
