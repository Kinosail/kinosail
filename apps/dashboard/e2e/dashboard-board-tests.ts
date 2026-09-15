import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { addFixtureApp, login, resetBoard, screenshot } from "./dashboard-helpers";

export function registerDashboardBoardTests() {
	test("board saving exposes progress, ignores repeat submissions, and recovers after failure", async ({ page }, testInfo) => {
		await login(page);
		await page.getByRole("button", { name: "Open board settings" }).click();
		const form = page.locator("#settings-form");
		const save = form.getByRole("button", { name: "Save board", exact: true });
		let writes = 0;
		let release: () => void = () => {};
		const pending = new Promise<void>((resolve) => {
			release = resolve;
		});
		await page.route("**/api/v1/board", async (route) => {
			if (route.request().method() !== "PUT") return route.continue();
			writes++;
			await pending;
			await route.fulfill({
				status: 503,
				contentType: "application/json",
				body: JSON.stringify({ error: "Board unavailable. Try again." }),
			});
		});
		await save.click();
		try {
			await expect(form).toHaveAttribute("aria-busy", "true");
			await expect(form.getByRole("button", { name: "Saving board…", exact: true })).toBeDisabled();
			await form.evaluate((element) => {
				(element as HTMLFormElement).requestSubmit();
				(element as HTMLFormElement).requestSubmit();
			});
			await screenshot(page, testInfo, "board-saving.png");
			expect(writes).toBe(1);
		} finally {
			release();
		}
		await expect(page.locator("#settings-error")).toContainText("Try again");
		await expect(save).toBeEnabled();
		await expect(form).not.toHaveAttribute("aria-busy", "true");
		await page.unroute("**/api/v1/board");
		await save.click();
		await expect(page.locator("#settings-dialog")).not.toBeVisible();
	});

	test("previews safe external imports and supports view modes", async ({ page }) => {
		await login(page);
		await resetBoard(page);
		await page.getByRole("button", { name: "Open board settings" }).click();
		await page.locator("#external-import-file").setInputFiles({
			name: "broken.yml",
			mimeType: "text/yaml",
			buffer: Buffer.from("sections: ["),
		});
		await expect(page.locator("#settings-error")).toBeVisible();
		await page.locator("#external-import-file").setInputFiles({
			name: "conf.yml",
			mimeType: "text/yaml",
			buffer: Buffer.from("pageInfo:\n  title: Family\nsections:\n  - name: Media\n    items:\n      - title: Jellyfin\n        url: http://127.0.0.1:8096\n"),
		});
		await expect(page.locator("#settings-error")).toBeHidden();
		await expect(page.getByText("1 applications ready from dashy")).toBeVisible();
		await expect(page.getByText("Jellyfin · http://127.0.0.1:8096")).toBeVisible();
		await page.getByRole("button", { name: "Add these applications" }).click();
		await expect(page.locator("#app-grid").getByText("Jellyfin", { exact: true })).toBeVisible();

		const view = page.getByRole("button", { name: /Switch dashboard view/ });
		await view.click();
		await expect(view).toContainText("Operations");
		await view.click();
		await expect(view).toContainText("TV mode");
		await view.click();
		await expect(view).toContainText("Household");
		await resetBoard(page);
	});

	test("adds, edits, searches, reorders, removes, and restores applications", async ({ page }, testInfo) => {
		await login(page);
		await resetBoard(page);
		await page.getByRole("button", { name: "Add application" }).click();
		await expect(page.getByRole("heading", { name: "Add application" })).toBeVisible();
		expect((await new AxeBuilder({ page }).include("#app-dialog").analyze()).violations).toEqual([]);
		await page.getByLabel("Search the app dictionary").fill("subtitle");
		await expect(page.getByRole("button", { name: "Kinosail Subtitles" })).toBeVisible();
		await page.getByLabel("Search the app dictionary").fill("");
		await page.getByRole("button", { name: "Jellyfin" }).click();
		await page.getByLabel("Application address").fill("http://127.0.0.1:8096");
		await expect(page.locator("#app-dialog")).not.toContainText("30 seconds");
		await page.getByLabel("Check reachability automatically").uncheck();
		await page.locator("#app-dialog").getByRole("button", { name: "Add application", exact: true }).click();
		await expect(page.locator("#app-grid").getByText("Jellyfin", { exact: true })).toBeVisible();

		for (const [index, fixture] of [
			["Home Assistant", "ocean"],
			["Paperless", "amber"],
			["Grafana", "coral"],
			["Immich", "sky"],
			["Sonarr", "violet"],
			["AdGuard Home", "slate"],
		].entries()) {
			await addFixtureApp(page, fixture[0], 8100 + index, fixture[1]);
		}
		await page.reload();
		await expect(page.locator(".app-tile")).toHaveCount(7);

		await page.getByRole("button", { name: "Edit board" }).click();
		await expect(page.getByRole("complementary", { name: "Board editing status" })).toBeVisible();
		const crowdedTiles = await page.locator(".app-tile").evaluateAll((tiles) =>
			tiles.flatMap((tile) => {
				const tileBox = tile.getBoundingClientRect();
				const handle = tile.querySelector(".edit-handle")?.getBoundingClientRect();
				const name = tile.querySelector(".edit-name")?.getBoundingClientRect();
				const actionsTop = tile.querySelector(".move-actions")?.getBoundingClientRect().top;
				if (!handle || !name || actionsTop === undefined || (tileBox.height >= 146 && handle.bottom + 4 <= name.top && name.bottom + 4 <= actionsTop)) return [];
				return [
					{
						height: tileBox.height,
						handleBottom: handle.bottom,
						nameTop: name.top,
						nameBottom: name.bottom,
						actionsTop,
					},
				];
			}),
		);
		expect(crowdedTiles).toEqual([]);
		const jellyfin = page.locator(".app-tile", { hasText: "Jellyfin" });
		await jellyfin.getByRole("button", { name: "Edit" }).click();
		await page.getByLabel("Description").fill("Movies and shows for the living room");
		await page.getByLabel("Mark as a favorite").check();
		await page.getByRole("button", { name: "Save changes" }).click();
		await expect(jellyfin).toContainText("Movies and shows for the living room");

		const before = await page.locator(".app-name").allTextContents();
		const beforeIndex = before.indexOf("Jellyfin");
		await jellyfin.getByRole("button", { name: "Move Jellyfin later" }).click();
		await expect.poll(async () => (await page.locator(".app-name").allTextContents())[beforeIndex + 1]).toBe("Jellyfin");
		const after = await page.locator(".app-name").allTextContents();
		expect(after).not.toEqual(before);
		await page.getByRole("button", { name: "Done" }).click();

		await page.getByRole("button", { name: "Favorites" }).click();
		await expect(page.locator(".app-tile")).toHaveCount(1);
		await expect(page.locator(".app-tile").getByText("Jellyfin", { exact: true })).toBeVisible();
		await page.getByRole("button", { name: "All apps" }).click();
		await expect(page.locator(".app-tile")).toHaveCount(7);

		await page.getByPlaceholder("Find an application").fill("movies");
		await expect(page.locator(".app-tile")).toHaveCount(1);
		await expect(page.getByText("1 of 7")).toBeVisible();
		await page.getByPlaceholder("Find an application").fill("no such household app");
		await expect(page.getByRole("heading", { name: "No matching applications" })).toBeVisible();
		await page.getByRole("button", { name: "Clear search" }).click();

		await page.getByRole("button", { name: "Edit board" }).click();
		await jellyfin.getByRole("button", { name: "Edit" }).click();
		await page.getByRole("button", { name: "Remove from Dashboard" }).click();
		await page.getByRole("button", { name: "Confirm removal" }).click();
		await expect(page.locator(".app-tile", { hasText: "Jellyfin" })).toHaveCount(0);
		await page.getByRole("button", { name: "Open board settings" }).click();
		const removed = page.locator(".removed-row", { hasText: "Jellyfin" });
		await removed.getByRole("button", { name: "Restore" }).click();
		await expect(page.locator(".app-tile", { hasText: "Jellyfin" })).toBeVisible();
		await screenshot(page, testInfo, "populated-edit-board.png");
	});

	test("restores canonical order and focus after a reorder conflict", async ({ page }) => {
		await login(page);
		await page.getByRole("button", { name: "Edit board" }).click();
		const before = await page.locator(".app-name").allTextContents();
		let conflict = true;
		await page.route("**/api/v1/apps/order", async (route) => {
			if (!conflict) return route.continue();
			conflict = false;
			await route.fulfill({
				status: 409,
				contentType: "application/json",
				body: JSON.stringify({
					error: "Board version changed. Refresh and try again.",
				}),
			});
		});
		const paperless = page.locator(".app-tile", { hasText: "Paperless" });
		const moveLater = paperless.getByRole("button", {
			name: "Move Paperless later",
		});
		const refreshed = page.waitForResponse((response) => response.url().endsWith("/api/v1/board") && response.request().method() === "GET");
		await moveLater.click();
		await refreshed;
		await expect.poll(() => page.locator(".app-name").allTextContents()).toEqual(before);
		await expect(paperless.getByRole("button", { name: "Move Paperless later" })).toBeFocused();
		await page.getByRole("button", { name: "Retry" }).click();
		await expect.poll(async () => (await page.locator(".app-name").allTextContents()).indexOf("Paperless")).toBe(before.indexOf("Paperless") + 1);

		const secondName = (await page.locator(".app-name").allTextContents())[1];
		const secondTile = page.locator(".app-tile", { hasText: secondName });
		await secondTile.getByRole("button", { name: `Move ${secondName} earlier` }).click();
		await expect(secondTile.getByRole("button", { name: `Move ${secondName} later` })).toBeFocused();
	});

	test("does not repeat a saved move when its board refresh fails", async ({ page }) => {
		await login(page);
		await page.getByRole("button", { name: "Edit board" }).click();
		const names = await page.locator(".app-name").allTextContents();
		const movedName = names[2];
		let failedBoardReads = 0;
		let orderWrites = 0;
		await page.route("**/api/v1/apps/order", async (route) => {
			orderWrites++;
			const response = await route.fetch();
			failedBoardReads = 2;
			await route.fulfill({ response });
		});
		await page.route("**/api/v1/board", async (route) => {
			if (!failedBoardReads) return route.continue();
			failedBoardReads--;
			await route.abort("failed");
		});
		await page
			.locator(".app-tile", { hasText: movedName })
			.getByRole("button", { name: `Move ${movedName} later` })
			.click();
		await page.getByRole("button", { name: "Refresh" }).click();
		await page.getByRole("button", { name: "Retry" }).click();
		await expect.poll(async () => (await page.locator(".app-name").allTextContents()).indexOf(movedName)).toBe(3);
		expect(orderWrites).toBe(1);
	});

	test("recovers a saved drag without repeating its write", async ({ page }, testInfo) => {
		await login(page);
		await page.getByRole("button", { name: "Edit board" }).click();
		const names = await page.locator(".app-name").allTextContents();
		const movedName = names[1];
		let failBoardRead = false;
		let orderWrites = 0;
		await page.route("**/api/v1/apps/order", async (route) => {
			orderWrites++;
			const response = await route.fetch();
			failBoardRead = true;
			await route.fulfill({ response });
		});
		await page.route("**/api/v1/board", async (route) => {
			if (!failBoardRead) return route.continue();
			failBoardRead = false;
			await route.abort("failed");
		});
		const handle = page.locator(".app-tile", { hasText: movedName }).getByRole("button", { name: `Drag ${movedName} to reorder` });
		const targetIndex = 3;
		const target = page.locator(".app-tile").nth(targetIndex);
		await handle.evaluate(element => element.scrollIntoView({ block: "center" }));
		const handleBox = await handle.boundingBox();
		const targetBox = await target.boundingBox();
		if (!handleBox || !targetBox) throw new Error("Drag controls were not rendered");
		await page.mouse.move(handleBox.x + handleBox.width / 2, handleBox.y + handleBox.height / 2);
		await page.mouse.down();
		await expect(handle.locator("xpath=ancestor::article")).toHaveAttribute("data-dragging", "true");
		await page.mouse.move(targetBox.x + targetBox.width / 2, targetBox.y + targetBox.height / 2 + 20);
		await expect.poll(async () => (await page.locator(".app-name").allTextContents()).indexOf(movedName)).toBe(targetIndex);
		await page.mouse.up();
		await expect.poll(() => orderWrites).toBe(1);
		await page.getByRole("button", { name: "Refresh" }).click();
		await expect.poll(async () => (await page.locator(".app-name").allTextContents()).indexOf(movedName)).toBe(targetIndex);
		await expect(page.locator('[data-dragging="true"]')).toHaveCount(0);
		expect(orderWrites).toBe(1);
		await screenshot(page, testInfo, "recovered-drag-order.png");
	});
}
