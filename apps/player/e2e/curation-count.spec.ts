import { expect, test } from "@playwright/test";
import { configureTestInstance, login } from "./test-instance-helpers";

configureTestInstance();
test.use({ serviceWorkers: "block" });

for (const kind of ["playlist", "Collection"]) {
	test(`${kind} detail counts match zero, one, and multiple titles`, async ({ page }, testInfo) => {
		await login(page);
		const name = `QA count ${kind} ${testInfo.project.name} ${Date.now()}`;
		const path = `/${kind.toLowerCase()}/${encodeURIComponent(name)}`;
		await page.goto(`/?view=${kind.toLowerCase()}s`);
		await page.getByLabel(`New ${kind} name`).fill(name);
		await page.getByRole("button", { name: "Create", exact: true }).click();
		const count = page.locator("main.collection-detail > p").filter({ hasText: /^\d+ items?$/ });
		try {
			await expect(count).toHaveText("0 items");
			await page.getByRole("textbox", { name: "Find something", exact: true }).fill("Example");
			await page.getByRole("button", { name: "Find", exact: true }).click();
			await page.locator(".collection-results").getByRole("button", { name: "Add to", exact: true }).first().click();
			await expect(count).toHaveText("1 item");
			await count.scrollIntoViewIfNeeded();
			await page.screenshot({ path: testInfo.outputPath("single-curation.png") });
			await page.locator(".collection-results").getByRole("button", { name: "Add to", exact: true }).first().click();
			await expect(count).toHaveText("2 items");
			await page.locator(".collection-items").getByRole("button", { name: "Remove", exact: true }).first().click();
			await expect(count).toHaveText("1 item");
			await page.locator(".collection-items").getByRole("button", { name: "Remove", exact: true }).click();
			await expect(count).toHaveText("0 items");
			await page.screenshot({ path: testInfo.outputPath("empty-curation.png") });
		} finally {
			await page.goto(path);
			await page.getByText(`Delete ${name}?`, { exact: true }).click();
			await page.getByRole("button", { name: `Delete ${kind}`, exact: true }).click();
		}
	});
}
