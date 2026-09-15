import { expect, test } from "@playwright/test";
import { createHmac } from "node:crypto";

test.skip(!process.env.KINOSAIL_TEST_INSTANCE, "requires the populated test instance");

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

test("a show card keeps Details and plays the next episode with one click", async ({ page }, testInfo) => {
	await page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false }));
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password").fill("test-instance-password");
	await page.getByLabel("Authentication or recovery code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await page.getByRole("link", { name: "Shows", exact: true }).click();
	for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		const cardActions = await page.locator(".show-card").evaluateAll((cards) => cards.map((card) => {
			const cardBox = card.getBoundingClientRect();
			const action = card.querySelector(".show-play")?.getBoundingClientRect();
			return { cardTop: Math.round(cardBox.top), actionTop: action ? Math.round(action.top) : null };
		}));
		for (const row of new Set(cardActions.map(({ cardTop }) => cardTop))) {
			const actions = cardActions.filter(({ cardTop }) => cardTop === row).map(({ actionTop }) => actionTop);
			expect(new Set(actions).size, `show actions align at ${viewport.width}px row ${row}`).toBe(1);
		}
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-shows.png`), fullPage: true });
	}
	const card = page.locator(".show-card").filter({ hasText: "Example Show" });
	await expect(card.getByRole("link", { name: "Example Show", exact: true })).toHaveAttribute("href", /^\/show\//);
	await expect(card.getByRole("link", { name: /^(?:Resume|Play next|Play again)/ })).toHaveAttribute("href", /^\/watch\//);
});

test("collection and playlist actions share a baseline when titles wrap", async ({ page }) => {
	await page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false }));
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password").fill("test-instance-password");
	await page.getByLabel("Authentication or recovery code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await expect(page.locator('meta[name="kinosail-csrf"]')).toHaveAttribute("content", /.+/);

	const suffix = `${Date.now()}`;
	const collection = `Alignment collection ${suffix}`;
	const playlist = `Alignment playlist ${suffix}`;
	const ids = await page.evaluate(async () => {
		const response = await fetch("/api/v1/library?view=all");
		if (!response.ok) throw new Error(`library failed: ${response.status}`);
		const catalog = await response.json() as { items?: Array<{ id?: string; kind?: string }> };
		return (catalog.items ?? []).filter((item) => item.kind === "video" && item.id).slice(0, 2).map((item) => item.id!);
	});
	expect(ids.length).toBe(2);
	const setup = await page.evaluate(async ({ collection, playlist, ids }) => {
		const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')!.content;
		const json = (path: string, method: string, body: object) => fetch(path, { method, headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf }, body: JSON.stringify(body) });
		const collectionResponse = await json("/api/v1/collections", "POST", { name: collection });
		const playlistResponse = await json("/api/v1/playlists", "POST", { name: playlist, ids });
		const itemResponses = await Promise.all(ids.map((id) => json(`/api/v1/collections/${encodeURIComponent(collection)}/items/${id}`, "PUT", { included: true })));
		return { collection: collectionResponse.status, playlist: playlistResponse.status, items: itemResponses.map((response) => response.status) };
	}, { collection, playlist, ids });
	expect(setup).toEqual({ collection: 201, playlist: 201, items: [200, 200] });

	for (const route of [`/collection/${encodeURIComponent(collection)}`, `/playlist/${encodeURIComponent(playlist)}`]) {
		await page.goto(route);
		for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
			await page.setViewportSize(viewport);
			const cardActions = await page.locator(".collection-items > .card").evaluateAll((cards) => cards.map((card) => {
				const cardBox = card.getBoundingClientRect();
				const action = card.querySelector(":scope > form button, :scope > .playlist-order")?.getBoundingClientRect();
				return { cardTop: Math.round(cardBox.top), actionTop: action ? Math.round(action.top) : null };
			}));
			for (const row of new Set(cardActions.map(({ cardTop }) => cardTop))) {
				const actions = cardActions.filter(({ cardTop }) => cardTop === row).map(({ actionTop }) => actionTop);
				expect(new Set(actions).size, `${route} actions align at ${viewport.width}px row ${row}`).toBe(1);
			}
		}
	}
});
