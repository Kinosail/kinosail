import { expect, test } from "@playwright/test";
import { downloadsSource, serviceWorkerSource } from "./static-sources";

test("service worker selects offline chunks through the target job index", async () => {
	expect(serviceWorkerSource).toContain('indexedDB.open(offlineDatabase, 4)');
	expect(serviceWorkerSource).toContain('chunks.createIndex("jobRange", ["jobID", "offset"])');
	expect(serviceWorkerSource).toContain('request = index.get([jobID, offset])');
	expect(serviceWorkerSource).not.toContain('store.getAll())).filter((chunk) => chunk.jobID === job.id)');
});

test("service worker streams verified IndexedDB chunks with backpressure", async ({ page }) => {
	await page.route("https://kinosail.test/", (route) => route.fulfill({ contentType: "text/html", body: "<!doctype html><title>Offline stream contract</title>" }));
	await page.goto("https://kinosail.test/");
	const contract = await page.evaluate(async (source) => {
		type WorkerEvent = { request: Request; respondWith: (response: Promise<Response | undefined>) => void };
		await new Promise<void>((resolve, reject) => {
			const request = indexedDB.deleteDatabase("kinosail-offline-v1");
			request.onsuccess = () => resolve();
			request.onerror = () => reject(request.error);
		});
		const content = new TextEncoder().encode("target");
		const chunks = [content.slice(0, 2), content.slice(2, 4), content.slice(4)];
		const sha256 = async (data: Uint8Array) => [...new Uint8Array(await crypto.subtle.digest("SHA-256", data))].map((byte) => byte.toString(16).padStart(2, "0")).join("");
		const hashes = await Promise.all(chunks.map(sha256));
		await new Promise<void>((resolve, reject) => {
			const request = indexedDB.open("kinosail-offline-v1", 4);
			request.onupgradeneeded = () => {
				request.result.createObjectStore("identity");
				request.result.createObjectStore("jobs", { keyPath: "id" });
				const store = request.result.createObjectStore("chunks", { keyPath: "id" });
				store.createIndex("jobID", "jobID");
				store.createIndex("jobRange", ["jobID", "offset"]);
			};
			request.onerror = () => reject(request.error);
			request.onsuccess = () => {
				const database = request.result;
				const transaction = database.transaction(["jobs", "chunks", "identity"], "readwrite");
				transaction.objectStore("identity").put({profile: "profile", revision: 1}, "active-profile");
				transaction.objectStore("jobs").put({ id: "aaaaaaaaaaaaaaaa", integrityVersion: 2, profileID: "profile", state: "ready", readyOffline: true, size: content.byteLength, storage: "indexeddb", sha256: "a".repeat(64), extension: ".mp4" });
				chunks.forEach((data, index) => transaction.objectStore("chunks").put({ id: `aaaaaaaaaaaaaaaa:${index * 2}`, jobID: "aaaaaaaaaaaaaaaa", offset: index * 2, length: data.byteLength, sha256: hashes[index], data: data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength) }));
				transaction.oncomplete = () => { database.close(); resolve(); };
				transaction.onerror = () => reject(transaction.error);
			};
		});

		const reads: (number | "all")[] = [];
		const originalGet = IDBIndex.prototype.get;
		const originalGetAll = IDBIndex.prototype.getAll;
		IDBIndex.prototype.get = function(query: IDBValidKey | IDBKeyRange) {
			if (this.name === "jobRange" && Array.isArray(query)) reads.push(Number(query[1]));
			return originalGet.call(this, query);
		};
		IDBIndex.prototype.getAll = function(...args: Parameters<IDBIndex["getAll"]>) {
			if (this.name === "jobRange") reads.push("all");
			return originalGetAll.apply(this, args);
		};
		try {
			const handlers = new Map<string, (event: WorkerEvent) => void>();
			const scope = { addEventListener: (name: string, handler: (event: WorkerEvent) => void) => handlers.set(name, handler), location: { origin: "https://kinosail.test" } };
			new Function("self", "caches", "fetch", "indexedDB", source.replace("const chunkSize = 8 * 1024 * 1024;", "const chunkSize = 2;"))(scope, {}, async () => { throw new TypeError("offline"); }, indexedDB);
			const requestMedia = async () => {
				let responsePromise: Promise<Response | undefined> | undefined;
				handlers.get("fetch")?.({ request: new Request("https://kinosail.test/offline-media/profile/aaaaaaaaaaaaaaaa"), respondWith: (value) => { responsePromise = value; } });
				return responsePromise;
			};
			const changeMiddleChunk = async (change: "corrupt" | "delete") => new Promise<void>((resolve, reject) => {
				const request = indexedDB.open("kinosail-offline-v1", 4);
				request.onerror = () => reject(request.error);
				request.onsuccess = () => {
					const database = request.result;
					const transaction = database.transaction("chunks", "readwrite");
					const store = transaction.objectStore("chunks");
					if (change === "delete") store.delete("aaaaaaaaaaaaaaaa:2");
					else store.put({ id: "aaaaaaaaaaaaaaaa:2", jobID: "aaaaaaaaaaaaaaaa", offset: 2, length: 2, sha256: hashes[1], data: new TextEncoder().encode("zz").buffer });
					transaction.oncomplete = () => { database.close(); resolve(); };
					transaction.onerror = () => reject(transaction.error);
				};
			});
			const response = await requestMedia();
			await new Promise((resolve) => setTimeout(resolve, 25));
			const readsBeforeConsumption = [...reads];
			const reader = response?.body?.getReader();
			const parts: Uint8Array[] = [];
			if (reader) for (;;) {
				const part = await reader.read();
				if (part.done) break;
				parts.push(part.value);
			}
			const body = new Uint8Array(parts.reduce((size, part) => size + part.byteLength, 0));
			let offset = 0;
			for (const part of parts) { body.set(part, offset); offset += part.byteLength; }
			const successfulReads = [...reads];
			await changeMiddleChunk("corrupt");
			reads.length = 0;
			let corruptionRejected = false;
			try { await (await requestMedia())?.arrayBuffer(); } catch { corruptionRejected = true; }
			const corruptReads = [...reads];
			await changeMiddleChunk("delete");
			reads.length = 0;
			let gapRejected = false;
			try { await (await requestMedia())?.arrayBuffer(); } catch { gapRejected = true; }
			const gapReads = [...reads];
			reads.length = 0;
			const cancelled = await requestMedia();
			await cancelled?.body?.cancel();
			const cancelReads = [...reads];
			const databaseClosed = await new Promise<boolean>((resolve, reject) => {
				const request = indexedDB.deleteDatabase("kinosail-offline-v1");
				request.onblocked = () => resolve(false);
				request.onerror = () => reject(request.error);
				request.onsuccess = () => resolve(true);
			});
			return { body: new TextDecoder().decode(body), cancelReads, contentLength: response?.headers.get("Content-Length"), corruptReads, corruptionRejected, databaseClosed, gapReads, gapRejected, reads: successfulReads, readsBeforeConsumption, status: response?.status };
		} finally {
			IDBIndex.prototype.get = originalGet;
			IDBIndex.prototype.getAll = originalGetAll;
		}
	}, serviceWorkerSource);

	expect(contract).toEqual({ body: "target", cancelReads: [0], contentLength: "6", corruptReads: [0, 2], corruptionRejected: true, databaseClosed: true, gapReads: [0, 2], gapRejected: true, reads: [0, 2, 4], readsBeforeConsumption: [0], status: 200 });
});

test("service worker streams verified OPFS chunks before later verification", async ({ page }) => {
	await page.route("https://kinosail.test/", (route) => route.fulfill({ contentType: "text/html", body: "<!doctype html><title>OPFS stream contract</title>" }));
	await page.goto("https://kinosail.test/");
	const contract = await page.evaluate(async (source) => {
		type WorkerEvent = { request: Request; respondWith: (response: Promise<Response | undefined>) => void };
		await new Promise<void>((resolve, reject) => {
			const request = indexedDB.deleteDatabase("kinosail-offline-v1");
			request.onsuccess = () => resolve();
			request.onerror = () => reject(request.error);
		});
		const content = new TextEncoder().encode("target");
		const chunks = [content.slice(0, 2), content.slice(2, 4), content.slice(4)];
		const sha256 = async (data: Uint8Array) => [...new Uint8Array(await crypto.subtle.digest("SHA-256", data))].map((byte) => byte.toString(16).padStart(2, "0")).join("");
		const hashes = await Promise.all(chunks.map(sha256));
		await new Promise<void>((resolve, reject) => {
			const request = indexedDB.open("kinosail-offline-v1", 4);
			request.onupgradeneeded = () => {
				request.result.createObjectStore("identity");
				request.result.createObjectStore("jobs", { keyPath: "id" });
				const store = request.result.createObjectStore("chunks", { keyPath: "id" });
				store.createIndex("jobID", "jobID");
				store.createIndex("jobRange", ["jobID", "offset"]);
			};
			request.onerror = () => reject(request.error);
			request.onsuccess = () => {
				const database = request.result;
				const transaction = database.transaction(["jobs", "chunks", "identity"], "readwrite");
				transaction.objectStore("identity").put({profile: "profile", revision: 1}, "active-profile");
				transaction.objectStore("jobs").put({ id: "bbbbbbbbbbbbbbbb", integrityVersion: 2, profileID: "profile", state: "ready", readyOffline: true, size: content.byteLength, storage: "opfs", sha256: "a".repeat(64), extension: ".mp4" });
				chunks.forEach((data, index) => transaction.objectStore("chunks").put({ id: `bbbbbbbbbbbbbbbb:${index * 2}`, jobID: "bbbbbbbbbbbbbbbb", offset: index * 2, length: data.byteLength, sha256: hashes[index] }));
				transaction.oncomplete = () => { database.close(); resolve(); };
				transaction.onerror = () => reject(transaction.error);
			};
		});

		const fileReads: number[] = [];
		let releaseLater = () => {};
		const later = new Promise<void>((resolve) => { releaseLater = resolve; });
		let laterReleased = false;
		const fallback = setTimeout(() => { laterReleased = true; releaseLater(); }, 500);
		const file = {
			size: content.byteLength,
			slice: (start: number, end: number) => ({ arrayBuffer: async () => {
				fileReads.push(start);
				if (start === 2) await later;
				const data = content.slice(start, end);
				return data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength);
			} }),
		};
		const mockNavigator = { storage: { getDirectory: async () => ({ getFileHandle: async () => ({ getFile: async () => file }) }) } };
		const handlers = new Map<string, (event: WorkerEvent) => void>();
		const scope = { addEventListener: (name: string, handler: (event: WorkerEvent) => void) => handlers.set(name, handler), location: { origin: "https://kinosail.test" } };
		new Function("self", "caches", "fetch", "indexedDB", "navigator", source.replace("const chunkSize = 8 * 1024 * 1024;", "const chunkSize = 2;"))(scope, {}, async () => { throw new TypeError("offline"); }, indexedDB, mockNavigator);
		let responsePromise: Promise<Response | undefined> | undefined;
		handlers.get("fetch")?.({ request: new Request("https://kinosail.test/offline-media/profile/bbbbbbbbbbbbbbbb"), respondWith: (value) => { responsePromise = value; } });
		const response = await responsePromise;
		const responseBeforeLater = !laterReleased;
		const reader = response?.body?.getReader();
		const first = await reader?.read();
		const readsAfterFirst = [...fileReads];
		const secondPending = reader?.read();
		const secondBlocked = await Promise.race([secondPending?.then(() => false), new Promise<true>((resolve) => setTimeout(() => resolve(true), 25))]);
		clearTimeout(fallback);
		releaseLater();
		const second = await secondPending;
		const third = await reader?.read();
		const done = await reader?.read();
		const body = new Uint8Array([...(first?.value || []), ...(second?.value || []), ...(third?.value || [])]);
		return { body: new TextDecoder().decode(body), contentLength: response?.headers.get("Content-Length"), done: done?.done, fileReads, readsAfterFirst, responseBeforeLater, secondBlocked, status: response?.status };
	}, serviceWorkerSource);

	expect(contract).toEqual({ body: "target", contentLength: "6", done: true, fileReads: [0, 2, 4], readsAfterFirst: [0], responseBeforeLater: true, secondBlocked: true, status: 200 });
});

for (const mode of ["registration failure", "no controller"] as const) test(`offline downloads reject ${mode} before storage changes`, async ({ page }) => {
	await page.addInitScript((failureMode) => {
		const worker = Object.assign(new EventTarget(), { scriptURL: "https://kinosail.test/service-worker.js?v=52", state: "activated" });
		const registration = Object.assign(new EventTarget(), { active: worker, installing: null, waiting: null });
		const serviceWorker = Object.assign(new EventTarget(), {
			controller: null,
			getRegistration: async () => undefined,
			register: async () => {
				if (failureMode === "registration failure") throw new Error("registration failed");
				return registration;
			},
		});
		Object.defineProperty(navigator, "serviceWorker", { configurable: true, value: serviceWorker });
	}, mode);
	await page.route("https://kinosail.test/", (route) => route.fulfill({ contentType: "text/html", body: '<body data-viewer-profile="profile"><article data-download-job="aaaaaaaaaaaaaaaa"><button data-download-device data-job-id="aaaaaaaaaaaaaaaa" data-item-id="bbbbbbbbbbbbbbbb" data-title="Movie" data-quality="720p">Download</button><span data-download-device-status></span></article></body>' }));
	let fileRequests = 0;
	await page.route("https://kinosail.test/api/v1/downloads/aaaaaaaaaaaaaaaa", (route) => route.fulfill({ contentType: "application/json", body: JSON.stringify({ id: "aaaaaaaaaaaaaaaa", itemId: "bbbbbbbbbbbbbbbb", profileId: "profile", title: "Movie", quality: "720p", state: "ready", readyOffline: true, extension: ".mp4", sha256: "a".repeat(64), size: 1 }) }));
	await page.route("https://kinosail.test/api/v1/downloads/aaaaaaaaaaaaaaaa/file", (route) => { fileRequests++; return route.abort(); });
	await page.goto("https://kinosail.test/");
	await page.addScriptTag({ content: downloadsSource.replaceAll("5000", "20") });
	const button = page.locator("[data-download-device]");
	await expect(button).toHaveAttribute("data-bound", "true");
	await button.click();
	await expect(page.locator("[data-download-device-status]")).toHaveText("Offline playback is not ready in this browser. Reconnect to the Server and reload Kinosail.");
	expect(fileRequests).toBe(0);
	expect(await page.evaluate(() => (window as Window & { KinosailOfflineMedia: { source: (id: string) => Promise<string> } }).KinosailOfflineMedia.source("item"))).toBe("");
	expect(await page.evaluate(() => new Promise<number>((resolve, reject) => {
		const request = indexedDB.open("kinosail-offline-v1", 1);
		request.onupgradeneeded = () => { request.result.createObjectStore("identity");
				request.result.createObjectStore("jobs", { keyPath: "id" }); request.result.createObjectStore("chunks", { keyPath: "id" }); };
		request.onsuccess = () => { const version = request.result.version; request.result.close(); resolve(version); };
		request.onerror = () => reject(request.error);
	}))).toBe(1);
});
