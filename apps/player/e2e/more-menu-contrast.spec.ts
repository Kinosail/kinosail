import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";

const appCSS = readFileSync(new URL("../../../packages/webassets/static/player-app.css", import.meta.url), "utf8");

test("compact More menu keeps a solid reading surface", async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await page.emulateMedia({ colorScheme: "dark" });
	await page.setContent(`<style>${appCSS}</style>
		<main class="library-page">
			<header class="app-header">
				<nav aria-label="Main navigation">
					<details class="nav-more" open>
						<summary>More</summary>
						<div class="nav-more-menu">
							<section class="nav-more-section nav-more-library">
								<span class="nav-more-heading">Library</span>
								<a href="#">Music</a>
								<a href="#">Audiobooks</a>
							</section>
							<section class="nav-more-section nav-more-actions">
								<span class="nav-more-heading">Actions</span>
								<a href="#">Settings</a>
							</section>
						</div>
					</details>
				</nav>
			</header>
		</main>`);

	const background = () => page.locator(".nav-more-menu").evaluate((element) => getComputedStyle(element).backgroundColor);
	await expect.poll(background).toBe("rgb(13, 17, 19)");

	await page.locator("html").evaluate((element) => element.setAttribute("data-theme", "light"));
	await expect.poll(background).toBe("rgb(255, 255, 255)");
});
