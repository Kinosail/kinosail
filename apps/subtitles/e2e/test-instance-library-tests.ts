import AxeBuilder from "@axe-core/playwright";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";
import { createViewer, firstPlayable, login, loginViewer, newViewerPage, openLibrarySection, removeViewer, type OfflineClient } from "./test-instance-helpers";

export function registerTestInstanceLibraryTests() {
test("public test instance exercises every media section and local TMDB metadata", async ({ page }) => {
  await login(page);
	for (const group of ["Watch & view", "Listen & read", "Organize"]) await expect(page.getByRole("heading", { name: group })).toBeVisible();

  const sections = [
    ["Movies", "Example Movie"],
    ["Shows", "Example Show"],
    ["Music", "Example Album"],
    ["Audiobooks", "Example Audiobook"],
    ["Books", "Example PDF Book"],
    ["Photos", "Example Photo One"],
  ] as const;
  for (const [section, title] of sections) {
    await openLibrarySection(page, section);
    await expect(page.getByRole("heading", { name: title }).first()).toBeVisible();
  }

  await page.getByRole("link", { name: "Movies", exact: true }).click();
  await page.getByRole("link", { name: /Example Movie/ }).click();
  await expect(page.locator("video")).toBeVisible();
  await expect(page.getByRole("paragraph").filter({ hasText: "Example metadata from the local generated TMDB fixture." })).toBeVisible();

  await page.getByRole("link", { name: "Library" }).click();
  await openLibrarySection(page, "Music");
  await page.getByRole("link", { name: /Example Album/ }).click();
  await page.getByRole("link", { name: "Example Track One" }).click();
  await expect(page.locator("audio")).toBeVisible();

  await page.getByRole("link", { name: "Library" }).click();
  await openLibrarySection(page, "Books");
  await page.getByRole("link", { name: "Example PDF Book" }).click();
  await page.getByRole("link", { name: /Read now/ }).click();
  const reader = page.locator("iframe.book-reader");
  await expect(reader).toBeVisible();
  const document = await page.request.get((await reader.getAttribute("src"))!);
  expect(document.headers()["x-frame-options"]).toBe("SAMEORIGIN");
  expect(document.headers()["content-security-policy"]).toContain("frame-ancestors 'self'");

  await page.goto("/?view=books");
  await page.getByRole("link", { name: "Example EPUB Book" }).click();
  await page.getByRole("link", { name: /Read now/ }).click();
  await expect(page.locator("iframe.book-reader")).toBeVisible();
  await page.goto("/?view=books");
  await page.getByRole("link", { name: "Example Comic" }).click();
  await page.getByRole("link", { name: /Read now/ }).click();
  await expect(page.locator(".reader-pages img")).toHaveCount(2);
  const library = await page.request.get("/api/v1/library");
  expect(library.ok()).toBeTruthy();
  expect(await library.text()).toContain("Example metadata from the local generated TMDB fixture.");
});

test("media artwork keeps its intended ratio in the populated library", async ({ page }) => {
  await page.setViewportSize({ width: 992, height: 964 });
  await login(page);
	await page.goto("/settings");
	await page.getByRole("link", { name: "System", exact: true }).click();
	await page.locator('form[action="/settings/tasks/metadata"] button').click();
	await page.goto("/?view=shows");
	await page.getByRole("link", { name: "Example Show", exact: true }).click();
	const episode = await page.locator('a.episode[href^="/watch/"]').first().getAttribute("href");
	expect(episode).toBeTruthy();
	const show = new URL(page.url()).pathname;
	await page.goto("/");
	const recent = page.locator(".home-shelf").filter({ hasText: "Recently added" });
	await expect(recent.locator(`a[href="${show}"]`)).toContainText("2 episodes stacked");
	await expect(recent.locator(`a[href="${show}"] .recent-stack-count`)).toHaveText("2");
  await page.goto("/?view=movies");
  const poster = page.locator('a.card[href^="/watch/"] .poster').first();
  const box = await poster.boundingBox();
  expect(box).not.toBeNull();
  expect(box!.height / box!.width).toBeCloseTo(1.5, 2);
  await expect(page.getByRole("heading", { name: /Example Movie/ })).toBeInViewport();
	await page.goto("/?view=photos");
	const photo = await page.locator(".poster.photo").first().boundingBox();
	expect(photo).not.toBeNull();
	expect(photo!.height / photo!.width).toBeCloseTo(.75, 2);
});

test("beta UI surfaces stay reachable and expose only working controls", async ({ page }, testInfo) => {
  const errors: string[] = [];
  let offline = false;
  await page.setViewportSize({ width: 390, height: 844 });
  await login(page);
  page.on("console", (message) => {
    if (!offline && message.type() === "error") errors.push(`${page.url()}: ${message.text()}`);
  });
  page.on("pageerror", (error) => { if (!offline) errors.push(`${page.url()}: ${error.message}`); });

  const owner = page.getByRole("link", { name: "Owner", exact: true });
  if (!await owner.isVisible()) await page.locator(".nav-more > summary").click();
  await expect(owner).toBeVisible();
  await expect(owner).toHaveAttribute("href", "/account");
  await page.getByRole("link", { name: "Movies", exact: true }).click();
  const watch = await page.locator('a.card[href^="/watch/"]').filter({ hasText: "Example Movie" }).first().getAttribute("href");
  await page.getByRole("link", { name: "Shows", exact: true }).click();
  const show = await page.locator('a.show-details[href^="/show/"]').first().getAttribute("href");
  await openLibrarySection(page, "Music");
  const album = await page.locator('a.card[href^="/album/"]').first().getAttribute("href");
  await openLibrarySection(page, "Books");
  const book = await page.locator('a.card[href^="/book/"]').first().getAttribute("href");
  expect(watch && show && album && book).toBeTruthy();

  await openLibrarySection(page, "Playlists");
  const smartName = `Beta ${testInfo.project.name} ${Date.now()}`;
  await page.getByText("New smart playlist", { exact: true }).click();
  await page.getByLabel("Smart playlist name").fill(smartName);
  await page.getByLabel("Match text").fill("Example");
  await page.getByLabel("Media").selectOption("video");
  await page.getByRole("button", { name: "Create smart playlist", exact: true }).click();
  await expect(page.getByRole("heading", { name: smartName, exact: true })).toBeVisible();
  await page.goto("/");
  const collectionName = `Beta Collection ${testInfo.project.name} ${Date.now()}`;
  await openLibrarySection(page, "Collections");
  await page.getByLabel("New Collection name").fill(collectionName);
  await page.getByLabel("New Collection name").press("Enter");
  await expect(page.getByRole("heading", { name: collectionName, exact: true })).toBeVisible();
  const routes = [
    "/", "/?view=list", "/?view=movies", "/?view=shows", "/?view=collections", "/?view=playlists", "/?view=music", "/?view=audiobooks", "/?view=books", "/?view=photos",
    "/?view=history", "/?view=unwatched", "/quick-connect", "/offline-downloads", "/settings", "/settings/configuration",
    "/settings/backups", "/settings/agent-connections", "/account", watch!, show!, album!, book!, `/collection/${encodeURIComponent(collectionName)}`, `/playlist/${encodeURIComponent(smartName)}`,
  ];
  for (const route of routes) {
    const response = await page.goto(route);
    expect(response?.ok(), route).toBeTruthy();
    await expect(page.locator("main")).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), route).toBeTruthy();
  }

  await page.goto(watch!);
  await expect(page.getByRole("button", { name: new RegExp(`(?:Add to|Remove from) ${smartName}`) })).toHaveCount(0);
  await page.getByText("Playback & downloads", { exact: true }).click();
  await page.getByRole("button", { name: "Prepare 720p offline", exact: true }).click();
  await expect(page.getByText("720p · Ready offline", { exact: true })).toBeVisible({ timeout: 30_000 });
  await expect(page.getByRole("link", { name: "Save file", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Download to this device", exact: true }).click();
  await expect(page.getByText("Ready offline on this device", { exact: true })).toBeVisible({ timeout: 30_000 });
  await page.goto("/offline");
  await page.getByRole("link", { name: /Example Movie · 720p/ }).click();
  await expect(page.locator("video")).toHaveAttribute("src", /\/offline-media\//);
  offline = true;
  await page.context().setOffline(true);
  try {
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.locator("video")).toHaveAttribute("src", /\/offline-media\//, { timeout: 15_000 });
  } finally {
    await page.context().setOffline(false);
  }
  expect(errors.filter((error) => !error.includes("blob:http://") && !error.includes("Applying inline style violates") && !(testInfo.project.name === "firefox" && error.includes("NS_BINDING_ABORTED")))).toEqual([]);
});
}
