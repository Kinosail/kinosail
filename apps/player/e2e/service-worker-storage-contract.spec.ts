type WorkerMessage = {origin: string; source: {type: string; url: string}; data: {type: string; profile: string; revision: number}; waitUntil: () => void};
import { expect, test } from "@playwright/test";
import { serviceWorkerSource } from "./static-sources";

test("uncached private artwork is delivered before cache maintenance finishes", async ({ page }) => {
	await page.route("https://kinosail.test/", route => route.fulfill({contentType: "text/html", body: "<!doctype html><title>Artwork cache timing</title>"}));
	await page.goto("https://kinosail.test/");
	const result = await page.evaluate(async source => {
		type WorkerEvent = {request: Request; respondWith: (value: Promise<Response>) => void; waitUntil: (value: Promise<void>) => void};
		const handlers = new Map<string, (event: WorkerEvent | WorkerMessage) => void>();
		let releaseWrite!: () => void;
		const slowWrite = new Promise<void>(resolve => { releaseWrite = resolve; });
		let writes = 0;
		const cache = {
			match: async () => undefined,
			put: async () => { writes++; await slowWrite; },
			keys: async () => [],
			delete: async () => true,
		};
		const scope = {addEventListener: (name: string, handler: (event: WorkerEvent | WorkerMessage) => void) => handlers.set(name, handler), location: {origin: "https://kinosail.test"}};
		new Function("self", "caches", "fetch", "indexedDB", source + "\nviewerProfile = 'viewer';")(
			scope, {open: async () => cache}, async () => new Response("image", {headers: {"Content-Type": "image/png"}}), indexedDB);
		let response: Promise<Response> | undefined;
		const maintenance: Promise<void>[] = [];
		handlers.get("fetch")?.({
			request: new Request("https://kinosail.test/art/movie"),
			respondWith: (value: Promise<Response>) => { response = value; },
			waitUntil: (value: Promise<void>) => { maintenance.push(value); },
		});
		const first = await Promise.race([response!.then(() => "image"), new Promise<string>(resolve => setTimeout(() => resolve("cache"), 1_000))]);
		releaseWrite();
		await Promise.all(maintenance);
		return {first, writes, body: await (await response!).text()};
	}, serviceWorkerSource);
	expect(result).toEqual({first: "image", writes: 1, body: "image"});
});
