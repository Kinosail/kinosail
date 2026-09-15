import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { createHmac } from "node:crypto";

const viewports = [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 800 }];

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

async function login(page: Page) {
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password").fill("test-instance-password");
	if (process.env.KINOSAIL_TEST_TOTP_SECRET) await page.getByLabel("Authentication code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
}

test("season reel keeps episode choice cinematic, scannable, and responsive", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	test.setTimeout(180_000);
	await page.emulateMedia({ colorScheme: "dark", reducedMotion: "reduce" });
	await page.setViewportSize(viewports[0]);
	await login(page);
	await page.goto("/?view=shows");
	await page.getByRole("link", { name: "Example Show", exact: true }).click();
	const rows = page.locator("[data-episode-row]");
	const first = await rows.first().getAttribute("href");
	expect(first).toMatch(/^\/watch\/[a-f0-9]+$/);
	await expect(rows).toHaveCount(2);
	await expect(rows.first()).toHaveClass(/is-next/);
	const preview = page.locator("[data-preview-link]");
	await expect(preview).toHaveAttribute("href", first!);
	await expect(preview.locator("[data-preview-image]")).toBeVisible();

	await rows.nth(1).focus();
	await expect(preview.locator("[data-preview-title]")).toHaveText("Example Episode Two");
	await rows.first().focus();
	await expect(preview.locator("[data-preview-title]")).toHaveText("Example Episode One");
	await expect(preview).toHaveAttribute("href", first!);

	for (const viewport of viewports) {
		await page.setViewportSize(viewport);
		const contract = await page.evaluate(() => {
			const reel = document.querySelector<HTMLElement>("[data-season-reel]")!;
			const previewElement = document.querySelector<HTMLElement>("[data-preview-link]")!;
			const row = document.querySelector<HTMLElement>("[data-episode-row]")!;
			return {
				overflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
				columns: getComputedStyle(reel).gridTemplateColumns.split(" ").length,
				preview: getComputedStyle(previewElement).display,
				rowRadius: getComputedStyle(row).borderTopLeftRadius,
				rowHeight: row.getBoundingClientRect().height,
				stillRight: row.querySelector<HTMLElement>(".episode-still")!.getBoundingClientRect().right,
				copyLeft: row.querySelector<HTMLElement>(".episode-copy")!.getBoundingClientRect().left,
			};
		});
		expect(contract.overflow, `${viewport.width}px overflow`).toBe(0);
		expect(contract.rowRadius, `${viewport.width}px row radius`).toBe("0px");
		expect(contract.rowHeight, `${viewport.width}px row height`).toBeGreaterThanOrEqual(64);
		expect(contract.copyLeft, `${viewport.width}px episode copy placement`).toBeGreaterThanOrEqual(contract.stillRight - 1);
		if (viewport.width > 900) {
			expect(contract.columns, `${viewport.width}px columns`).toBe(2);
			expect(contract.preview, `${viewport.width}px preview`).not.toBe("none");
		} else if (viewport.width > 700) {
			expect(contract.columns, `${viewport.width}px columns`).toBe(1);
			expect(contract.preview, `${viewport.width}px preview`).not.toBe("none");
		} else {
			expect(contract.preview, `${viewport.width}px preview`).toBe("none");
		}
		expect((await new AxeBuilder({ page }).analyze()).violations, `${viewport.width}px accessibility`).toEqual([]);
		await page.screenshot({ path: testInfo.outputPath(`show-ledger-${viewport.width}.png`), fullPage: true });
	}
});
