import { expect, test } from "@playwright/test";
import { serviceWorkerSource } from "./static-sources";

const currentCacheName = serviceWorkerSource.match(/const cacheName = "([^"]+)"/)![1];

test("service worker validates a fresh shell and removes retired browser downloads", async ({ page }) => {
	await page.route("https://kinosail.test/", (route) => route.fulfill({ contentType: "text/html", body: "<!doctype html><title>Service worker contract</title>" }));
	await page.goto("https://kinosail.test/");
	const contract = await page.evaluate(async ({source, currentCacheName}) => {
		type WorkerEvent = { request?: Request; respondWith?: (response: Promise<Response | undefined>) => void; waitUntil?: (work: Promise<void>) => void };
		await new Promise<void>((resolve, reject) => {
			const request = indexedDB.deleteDatabase("kinosail-offline-v1");
			request.onsuccess = () => resolve();
			request.onerror = () => reject(request.error);
		});
		const targetData = new TextEncoder().encode("target");
		const decoyData = new TextEncoder().encode("decoy!");
		const sha256 = async (data: Uint8Array) => [...new Uint8Array(await crypto.subtle.digest("SHA-256", data))].map((byte) => byte.toString(16).padStart(2, "0")).join("");
		const targetHash = await sha256(targetData);
		const decoyHash = await sha256(decoyData);
		const targetChunks = [targetData.slice(0, 2), targetData.slice(2, 4), targetData.slice(4)];
		const targetChunkHashes = await Promise.all(targetChunks.map(sha256));
		await new Promise<void>((resolve, reject) => {
			const request = indexedDB.open("kinosail-offline-v1", 1);
			request.onupgradeneeded = () => {
				request.result.createObjectStore("identity");
				request.result.createObjectStore("jobs", { keyPath: "id" });
				request.result.createObjectStore("chunks", { keyPath: "id" });
			};
			request.onerror = () => reject(request.error);
			request.onsuccess = () => {
				const database = request.result;
				const transaction = database.transaction(["jobs", "chunks", "identity"], "readwrite");
				transaction.objectStore("identity").put({profile: "profile", revision: 1}, "active-profile");
				const jobs = transaction.objectStore("jobs");
				const chunks = transaction.objectStore("chunks");
				jobs.put({ id: "aaaaaaaaaaaaaaaa", integrityVersion: 2, profileID: "profile", state: "ready", readyOffline: true, size: targetData.byteLength, storage: "indexeddb", sha256: targetHash, extension: ".mp4" });
				jobs.put({ id: "bbbbbbbbbbbbbbbb", integrityVersion: 2, profileID: "profile", state: "ready", readyOffline: true, size: decoyData.byteLength, storage: "indexeddb", sha256: decoyHash, extension: ".mp4" });
				targetChunks.forEach((data, index) => chunks.put({ id: `aaaaaaaaaaaaaaaa:${index * 2}`, jobID: "aaaaaaaaaaaaaaaa", offset: index * 2, length: data.byteLength, sha256: targetChunkHashes[index], data: data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength) }));
				chunks.put({ id: "bbbbbbbbbbbbbbbb:0", jobID: "bbbbbbbbbbbbbbbb", offset: 0, length: decoyData.byteLength, sha256: decoyHash, data: decoyData.buffer.slice(0) });
				transaction.oncomplete = () => { database.close(); resolve(); };
				transaction.onerror = () => reject(transaction.error);
			};
		});
		const handlers = new Map<string, (event: WorkerEvent) => void>();
		const records = { claims: 0, deleted: [] as string[], fetches: [] as { path: string; cache?: RequestCache; credentials?: RequestCredentials }[], puts: [] as string[], skips: 0 };
		const stores = new Map<string, Map<string, Response>>();
		const seed = (name: string, path?: string, body?: string) => {
			const store = new Map<string, Response>();
			if (path && body) store.set(path, new Response(body, { status: 200 }));
			stores.set(name, store);
		};
		seed("kinosail-shell-v13", "/offline", "old-offline");
		seed("kinosail-offline-pages-v1");
		seed("unrelated-cache", "/static/app.css", "unrelated-css");
		stores.get("unrelated-cache")!.set("/offline", new Response("unrelated-offline", { status: 200 }));
		const cacheFor = (name: string) => {
			if (!stores.has(name)) stores.set(name, new Map());
			const store = stores.get(name)!;
			return {
				match: async (path: string) => store.get(path)?.clone(),
				put: async (path: string, response: Response) => { records.puts.push(path); store.set(path, response.clone()); },
			};
		};
		const cacheStorage = {
			delete: async (name: string) => { records.deleted.push(name); return stores.delete(name); },
			keys: async () => [...stores.keys()],
			match: async (path: string) => {
				for (const store of stores.values()) if (store.has(path)) return store.get(path)?.clone();
			},
			open: async (name: string) => cacheFor(name),
		};
		const types: Record<string, string> = {
			"/offline": "text/html", "/static/app.css": "text/css", "/static/manrope.woff2": "font/woff2", "/static/main.kinosail.bundle.js": "text/javascript", "/static/theme.js": "text/javascript", "/static/pwa.js": "text/javascript", "/static/icon.svg": "image/svg+xml", "/static/icon-192.png": "image/png", "/static/icon-512.png": "image/png", "/static/icon-maskable-512.png": "image/png", "/static/apple-touch-icon.png": "image/png", "/static/cinema-backdrop.jpg": "image/jpeg",
		};
		let failure: "" | "mime" | "partial" | "redirect" | "origin" | "path" | "status" = "status";
		let offline = false;
		let offlineShell = "fresh:/offline";
		const network = async (request: string | Request, options?: RequestInit) => {
			const path = typeof request === "string" ? request : new URL(request.url).pathname;
			records.fetches.push({ path, cache: options?.cache, credentials: options?.credentials });
			if (offline) throw new TypeError("offline");
			const failed = path === "/static/theme.js";
			const response = new Response(path === "/offline" ? offlineShell : `fresh:${path}`, { status: failed && failure === "status" ? 503 : failed && failure === "partial" ? 206 : 200, headers: { "Content-Type": failed && failure === "mime" ? "text/html" : types[path] } });
			Object.defineProperties(response, {
				redirected: { value: failed && failure === "redirect" },
				url: { value: `https://${failed && failure === "origin" ? "other.test" : "kinosail.test"}${failed && ["redirect", "path"].includes(failure) ? "/login" : path}` },
			});
			return response;
		};
		const scope = {
			addEventListener: (name: string, handler: (event: WorkerEvent) => void) => handlers.set(name, handler),
			clients: { claim: async () => { records.claims++; } },
			location: { origin: "https://kinosail.test" },
			skipWaiting: async () => { records.skips++; },
		};
		new Function("self", "caches", "fetch", source)(scope, cacheStorage, network);
		const runLifecycle = async (name: "install" | "activate") => {
			let work: Promise<void> | undefined;
			handlers.get(name)?.({ waitUntil: (value) => { work = value; } });
			await work;
		};
		const failedInstalls: Record<string, { deleted: string[]; oldOffline?: string; rejected: boolean; skips: number }> = {};
		for (const mode of ["status", "partial", "mime", "redirect", "origin", "path"] as const) {
			failure = mode;
			stores.delete(currentCacheName);
			records.deleted = [];
			let rejected = false;
			try { await runLifecycle("install"); } catch { rejected = true; }
			failedInstalls[mode] = { deleted: [...records.deleted], oldOffline: await (await cacheFor("kinosail-shell-v13").match("/offline"))?.text(), rejected, skips: records.skips };
		}

		failure = "";
		stores.delete(currentCacheName);
		records.fetches = [];
		records.puts = [];
		await runLifecycle("install");
		const installedPaths = [...stores.get(currentCacheName)!.keys()].sort();
		const freshFetches = records.fetches.every((entry) => entry.cache === "reload" && entry.credentials === (entry.path === "/offline" ? "same-origin" : "omit"));
		await runLifecycle("activate");
		let mediaHandled = false;
		handlers.get("fetch")?.({ request: new Request("https://kinosail.test/offline-media/profile/aaaaaaaaaaaaaaaa"), respondWith: () => { mediaHandled = true; } });
		const remaining = await new Promise<{ jobs: number; chunks: number }>((resolve, reject) => {
			const request = indexedDB.open("kinosail-offline-v1");
			request.onerror = () => reject(request.error);
			request.onsuccess = () => {
				const database = request.result;
				const transaction = database.transaction(["jobs", "chunks"], "readonly");
				const jobs = transaction.objectStore("jobs").count();
				const chunks = transaction.objectStore("chunks").count();
				transaction.oncomplete = () => { database.close(); resolve({ jobs: jobs.result, chunks: chunks.result }); };
				transaction.onerror = () => reject(transaction.error);
			};
		});
		offlineShell = "localized:/offline";
		const refreshNavigation = new Request("https://kinosail.test/?language=updated");
		Object.defineProperty(refreshNavigation, "mode", { value: "navigate" });
		let refreshResponse: Promise<Response | undefined> | undefined;
		let refreshWork: Promise<void> | undefined;
		handlers.get("fetch")?.({ request: refreshNavigation, respondWith: (value) => { refreshResponse = value; }, waitUntil: (value) => { refreshWork = value; } });
		await refreshResponse;
		await refreshWork;

		offline = true;
		let styleResponse: Promise<Response | undefined> | undefined;
		handlers.get("fetch")?.({ request: new Request("https://kinosail.test/static/app.css?v=81"), respondWith: (value) => { styleResponse = value; } });
		const cachedStyle = await styleResponse;
		let fontResponse: Promise<Response | undefined> | undefined;
		handlers.get("fetch")?.({ request: new Request("https://kinosail.test/static/manrope.woff2?v=1"), respondWith: (value) => { fontResponse = value; } });
		const cachedFont = await fontResponse;
		const navigation = new Request("https://kinosail.test/?offline-check=1");
		Object.defineProperty(navigation, "mode", { value: "navigate" });
		let navigationResponse: Promise<Response | undefined> | undefined;
		let navigationWork: Promise<void> | undefined;
		handlers.get("fetch")?.({ request: navigation, respondWith: (value) => { navigationResponse = value; }, waitUntil: (value) => { navigationWork = value; } });
		const cachedNavigation = await navigationResponse;
		await navigationWork;
		return {
			cacheNames: [...stores.keys()].sort(),
			cachedNavigation: await cachedNavigation?.text(),
			cachedStyle: await cachedStyle?.text(),
			cachedFont: await cachedFont?.text(),
			claims: records.claims,
			failedInstalls,
			freshFetches,
			installedPaths,
			mediaHandled,
			remaining,
			retired: records.deleted.sort(),
			skips: records.skips,
		};
	}, {source: serviceWorkerSource, currentCacheName});

	for (const mode of ["status", "partial", "mime", "redirect", "origin", "path"]) expect(contract.failedInstalls[mode]).toEqual({ deleted: [], oldOffline: "old-offline", rejected: true, skips: 0 });
	expect(contract.freshFetches).toBeTruthy();
	expect(contract.installedPaths).toEqual([
		"/offline", "/static/app.css", "/static/apple-touch-icon.png", "/static/cinema-backdrop.jpg", "/static/icon-192.png", "/static/icon-512.png", "/static/icon-maskable-512.png", "/static/icon.svg", "/static/main.kinosail.bundle.js", "/static/manrope.woff2", "/static/pwa.js", "/static/theme.js",
	]);
	expect(contract.retired).toEqual(["kinosail-offline-pages-v1", "kinosail-shell-v13"]);
	expect(contract.cacheNames).toEqual([currentCacheName, "unrelated-cache"]);
	expect(contract.cachedNavigation).toBe("localized:/offline");
	expect(contract.cachedStyle).toBe("fresh:/static/app.css");
	expect(contract.cachedFont).toBe("fresh:/static/manrope.woff2");
	expect(contract.remaining).toEqual({ jobs: 0, chunks: 0 });
	expect(contract.mediaHandled).toBe(false);
	expect(contract.skips).toBe(1);
	expect(contract.claims).toBe(1);
});
