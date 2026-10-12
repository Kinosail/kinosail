import { expect, test } from "@playwright/test";
import { login } from "./test-instance-helpers";

test.skip(!process.env.KINOSAIL_TEST_INSTANCE, "requires the populated test instance");

test("a show card keeps Details and plays the next episode with one click", async ({ page }, testInfo) => {
	await page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false }));
	await login(page);
	await page.getByRole("link", { name: "Shows", exact: true }).click();
	const csrf = await page.locator('meta[name="kinosail-csrf"]').getAttribute("content");
	expect(csrf).toBeTruthy();
	const headers = { Origin: new URL(page.url()).origin, "X-Kinosail-CSRF": csrf! };
	const card = page.locator(".show-card").filter({ hasText: "Example Show" });
	await card.getByRole("link", { name: "Example Show", exact: true }).click();
	const episodes = await page.locator("[data-episode-row]").evaluateAll(rows => rows.map(row => row.getAttribute("href")!));
	expect(episodes).toHaveLength(2);
	const previous: Array<{ watch: string; seconds: number; watched: boolean }> = [];
	for (const watch of episodes) {
		expect(watch).toMatch(/^\/watch\/[a-f0-9]+$/);
		const response = await page.request.get(`/api/v1/items/${watch.split("/").at(-1)}`);
		expect(response.ok()).toBe(true);
		const { item } = await response.json();
		previous.push({ watch, seconds: item.progress.seconds ?? 0, watched: item.progress.watched ?? false });
	}
	try {
		for (const [index, watch] of episodes.entries()) {
			const response = await page.request.put(`/api/v1/items/${watch.split("/").at(-1)}/progress`, {
				headers, data: { seconds: 0, watched: index === 0 },
			});
			expect(response.ok()).toBe(true);
		}
		await page.reload();
		await expect(page.locator("[data-episode-row].is-next")).toHaveAttribute("href", episodes[1]);
		await page.locator("a.back").click();
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
		await expect(card.getByRole("link", { name: "Example Show", exact: true })).toHaveAttribute("href", /^\/show\//);
		const play = card.getByRole("link", { name: /^Play next/ });
		await expect(play).toHaveAttribute("href", episodes[1]);
		await play.click();
		await page.waitForURL(url => url.pathname === episodes[1]);
		const video = page.locator("video");
		await expect(video).toBeVisible();
		const before = await video.evaluate((media: HTMLVideoElement) => ({ seconds: media.currentTime, frames: media.getVideoPlaybackQuality().totalVideoFrames }));
		await expect.poll(() => video.evaluate((media: HTMLVideoElement, start) => !media.paused && !media.ended && !media.error
			&& media.currentTime > start.seconds + 0.2 && media.getVideoPlaybackQuality().totalVideoFrames > start.frames + 2, before), { timeout: 15_000 }).toBe(true);
		const after = await video.evaluate((media: HTMLVideoElement) => ({ seconds: media.currentTime, frames: media.getVideoPlaybackQuality().totalVideoFrames }));
		await video.screenshot({ path: testInfo.outputPath("show-card-moving-episode.png") });
		await testInfo.attach("show-card-playback.json", { contentType: "application/json", body: JSON.stringify({ revision: process.env.KINOSAIL_TEST_REVISION,
			data: "Episode one watched; episode two unwatched; real Server with synthetic media", selectedEpisode: episodes[1].split("/").at(-1), before, after,
			result: "One card click opened the next episode and advanced decoded frames and media time without another play call" }) });
		await page.goto("/?view=shows");
		for (const state of [
			{ label: "Resume", firstWatched: true, secondWatched: false, seconds: 3, selected: episodes[1] },
			{ label: "Play again", firstWatched: true, secondWatched: true, seconds: 0, selected: episodes[0] },
			{ label: "Play next", firstWatched: false, secondWatched: false, seconds: 0, selected: episodes[0] },
		]) {
			for (const [index, watch] of episodes.entries()) {
				const response = await page.request.put(`/api/v1/items/${watch.split("/").at(-1)}/progress`, { headers,
					data: { seconds: index === 1 ? state.seconds : 0, watched: index === 0 ? state.firstWatched : state.secondWatched } });
				expect(response.status()).toBe(200);
			}
			await page.reload();
			await expect(card.getByRole("link", { name: new RegExp(`^${state.label}`) })).toHaveAttribute("href", state.selected);
			await card.screenshot({ path: testInfo.outputPath(`show-card-${state.label.toLowerCase().replace(" ", "-")}.png`) });
		}
	} finally {
		await page.goto("/?view=shows");
		for (const { watch, seconds, watched } of previous) {
			const response = await page.request.put(`/api/v1/items/${watch.split("/").at(-1)}/progress`, { headers, data: { seconds, watched } });
			expect.soft(response.status()).toBe(200);
		}
	}
});

test("collection and playlist actions share a baseline when titles wrap", async ({ page }) => {
	await page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false }));
	await login(page);
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
