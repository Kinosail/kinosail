import { expect, test } from "@playwright/test";
import { downloadsSource } from "./static-sources";

test("offline removal deletes version-one local data before server removal without an active worker", async ({ page }) => {
	await page.addInitScript(() => {
		const serviceWorker = Object.assign(new EventTarget(), {
			controller: null,
			getRegistration: async () => undefined,
			register: async () => { (window as Window & { registrationCalls: number }).registrationCalls++; throw new Error("offline"); },
		});
		Object.assign(window, { registrationCalls: 0 });
		Object.defineProperty(navigator, "serviceWorker", { configurable: true, value: serviceWorker });
	});
	await page.route("https://kinosail.test/", (route) => route.fulfill({
		contentType: "text/html",
		body: '<body data-viewer-profile="profile"><section id="downloads" data-offline-removal-error="Lokale Offline-Daten konnten nicht entfernt werden."><form method="post" action="/offline-downloads/aaaaaaaaaaaaaaaa/remove"><button type="submit">Remove</button><span hidden data-download-remove-status></span></form></section></body>',
	}));
	await page.route("https://kinosail.test/offline-downloads/aaaaaaaaaaaaaaaa/remove", (route) => route.fulfill({ status: 204 }));
	await page.goto("https://kinosail.test/");
	await page.evaluate(() => new Promise<void>((resolve, reject) => {
		const request = indexedDB.open("kinosail-offline-v1", 1);
		request.onupgradeneeded = () => {
			request.result.createObjectStore("jobs", { keyPath: "id" });
			request.result.createObjectStore("chunks", { keyPath: "id" });
		};
		request.onerror = () => reject(request.error);
		request.onsuccess = () => {
			const database = request.result;
			const transaction = database.transaction(["jobs", "chunks"], "readwrite");
			transaction.objectStore("jobs").put({ id: "aaaaaaaaaaaaaaaa", storage: "indexeddb" });
			transaction.objectStore("jobs").put({ id: "cccccccccccccccc", storage: "indexeddb" });
			transaction.objectStore("chunks").put({ id: "aaaaaaaaaaaaaaaa:0", jobID: "aaaaaaaaaaaaaaaa", offset: 0 });
			transaction.objectStore("chunks").put({ id: "aaaaaaaaaaaaaaaa:8", jobID: "aaaaaaaaaaaaaaaa", offset: 8 });
			transaction.objectStore("chunks").put({ id: "cccccccccccccccc:0", jobID: "cccccccccccccccc", offset: 0 });
			transaction.oncomplete = () => { database.close(); resolve(); };
			transaction.onerror = () => reject(transaction.error);
		};
	}));
	await page.addScriptTag({ content: downloadsSource.replaceAll("5000", "20") });
	const form = page.locator('form[action="/offline-downloads/aaaaaaaaaaaaaaaa/remove"]');
	await expect(form).toHaveAttribute("data-bound", "true");
	const removeRequest = page.waitForRequest((request) => new URL(request.url()).pathname === "/offline-downloads/aaaaaaaaaaaaaaaa/remove");
	await form.getByRole("button", { name: "Remove" }).click();
	const request = await removeRequest;
	expect(request.method()).toBe("POST");
	await request.response();
	expect(await page.evaluate(() => (window as Window & { registrationCalls: number }).registrationCalls)).toBe(1);
	expect(await page.evaluate(() => new Promise<{ chunks: IDBValidKey[]; jobs: IDBValidKey[]; version: number }>((resolve, reject) => {
		const request = indexedDB.open("kinosail-offline-v1");
		request.onerror = () => reject(request.error);
		request.onsuccess = () => {
			const database = request.result;
			const transaction = database.transaction(["jobs", "chunks"], "readonly");
			const jobs = transaction.objectStore("jobs").getAllKeys();
			const chunks = transaction.objectStore("chunks").getAllKeys();
			transaction.oncomplete = () => { const version = database.version; database.close(); resolve({ chunks: chunks.result, jobs: jobs.result, version }); };
			transaction.onerror = () => reject(transaction.error);
		};
	}))).toEqual({ chunks: ["cccccccccccccccc:0"], jobs: ["cccccccccccccccc"], version: 1 });
});

test("offline removal blocks server submission and shows localized status when local deletion fails", async ({ page }) => {
	await page.addInitScript(() => {
		const serviceWorker = Object.assign(new EventTarget(), {
			controller: null,
			getRegistration: async () => undefined,
			register: async () => { throw new Error("offline"); },
		});
		Object.defineProperty(navigator, "serviceWorker", { configurable: true, value: serviceWorker });
	});
	await page.route("https://kinosail.test/", (route) => route.fulfill({
		contentType: "text/html",
		body: '<body data-viewer-profile="profile"><section id="downloads" data-offline-removal-error="Lokale Offline-Daten konnten nicht entfernt werden."><form method="post" action="/offline-downloads/aaaaaaaaaaaaaaaa/remove"><button type="submit">Remove</button><span hidden data-download-remove-status></span></form></section></body>',
	}));
	let removeRequests = 0;
	await page.route("https://kinosail.test/offline-downloads/aaaaaaaaaaaaaaaa/remove", (route) => { removeRequests++; return route.fulfill({ status: 204 }); });
	await page.goto("https://kinosail.test/");
	await page.evaluate(() => new Promise<void>((resolve, reject) => {
		const request = indexedDB.open("kinosail-offline-v1", 1);
		request.onupgradeneeded = () => request.result.createObjectStore("jobs", { keyPath: "id" });
		request.onerror = () => reject(request.error);
		request.onsuccess = () => {
			const database = request.result;
			const transaction = database.transaction("jobs", "readwrite");
			transaction.objectStore("jobs").put({ id: "aaaaaaaaaaaaaaaa", storage: "indexeddb" });
			transaction.oncomplete = () => { database.close(); resolve(); };
			transaction.onerror = () => reject(transaction.error);
		};
	}));
	await page.addScriptTag({ content: downloadsSource.replaceAll("5000", "20") });
	const form = page.locator('form[action="/offline-downloads/aaaaaaaaaaaaaaaa/remove"]');
	await expect(form).toHaveAttribute("data-bound", "true");
	await form.getByRole("button", { name: "Remove" }).click();
	await expect(form.locator("[data-download-remove-status]")).toHaveText("Lokale Offline-Daten konnten nicht entfernt werden.");
	await expect(form.locator("[data-download-remove-status]")).toBeVisible();
	expect(removeRequests).toBe(0);
	expect(await page.evaluate(() => new Promise<{ jobs: IDBValidKey[]; version: number }>((resolve, reject) => {
		const request = indexedDB.open("kinosail-offline-v1");
		request.onerror = () => reject(request.error);
		request.onsuccess = () => {
			const database = request.result;
			const transaction = database.transaction("jobs", "readonly");
			const jobs = transaction.objectStore("jobs").getAllKeys();
			transaction.oncomplete = () => { const version = database.version; database.close(); resolve({ jobs: jobs.result, version }); };
			transaction.onerror = () => reject(transaction.error);
		};
	}))).toEqual({ jobs: ["aaaaaaaaaaaaaaaa"], version: 1 });
});

test("offline downloads use an exact active worker when registration fetches fail", async ({ page }) => {
	await page.addInitScript(() => {
		const worker = Object.assign(new EventTarget(), { scriptURL: "https://kinosail.test/service-worker.js?v=43", state: "activated" });
		const registration = Object.assign(new EventTarget(), { active: worker, installing: null, waiting: null });
		const serviceWorker = Object.assign(new EventTarget(), {
			controller: worker,
			getRegistration: async () => registration,
			register: async () => { (window as Window & { registrationCalls: number }).registrationCalls++; throw new Error("offline"); },
		});
		Object.assign(window, { registrationCalls: 0 });
		Object.defineProperty(navigator, "serviceWorker", { configurable: true, value: serviceWorker });
	});
	await page.route("https://kinosail.test/", (route) => route.fulfill({ contentType: "text/html", body: '<body data-viewer-profile="profile"></body>' }));
	await page.goto("https://kinosail.test/");
	await page.evaluate(() => new Promise<void>((resolve, reject) => {
		const request = indexedDB.open("kinosail-offline-v1", 3);
		request.onupgradeneeded = () => {
			const jobs = request.result.createObjectStore("jobs", { keyPath: "id" });
			const chunks = request.result.createObjectStore("chunks", { keyPath: "id" });
			chunks.createIndex("jobID", "jobID");
			chunks.createIndex("jobRange", ["jobID", "offset"]);
			jobs.put({ id: "job", itemID: "item", profileID: "profile", state: "ready", readyOffline: true, sha256: "a".repeat(64) });
		};
		request.onsuccess = () => { request.result.close(); resolve(); };
		request.onerror = () => reject(request.error);
	}));
	await page.addScriptTag({ content: downloadsSource.replaceAll("5000", "20") });
	await expect.poll(() => page.evaluate(() => (window as Window & { KinosailOfflineMedia: { source: (id: string) => Promise<string> } }).KinosailOfflineMedia.source("item"))).toBe("/offline-media/profile/job");
	expect(await page.evaluate(() => (window as Window & { registrationCalls: number }).registrationCalls)).toBe(0);
});

test("offline downloads reject zero-byte manifests before storage or file requests", async ({ page }) => {
	await page.addInitScript(() => {
		const worker = Object.assign(new EventTarget(), { scriptURL: "https://kinosail.test/service-worker.js?v=43", state: "activated" });
		const registration = Object.assign(new EventTarget(), { active: worker, installing: null, waiting: null });
		const serviceWorker = Object.assign(new EventTarget(), { controller: worker, getRegistration: async () => registration, register: async () => registration });
		Object.defineProperty(navigator, "serviceWorker", { configurable: true, value: serviceWorker });
		Object.defineProperty(navigator.storage, "persist", { configurable: true, value: async () => true });
	});
	await page.route("https://kinosail.test/", (route) => route.fulfill({ contentType: "text/html", body: '<body data-viewer-profile="profile"><article data-download-job="aaaaaaaaaaaaaaaa"><button data-download-device data-job-id="aaaaaaaaaaaaaaaa" data-item-id="bbbbbbbbbbbbbbbb" data-title="Movie" data-quality="720p">Download</button><span data-download-device-status></span></article></body>' }));
	let fileRequests = 0;
	await page.route("https://kinosail.test/api/v1/downloads/aaaaaaaaaaaaaaaa", (route) => route.fulfill({ contentType: "application/json", body: JSON.stringify({ id: "aaaaaaaaaaaaaaaa", itemId: "bbbbbbbbbbbbbbbb", profileId: "profile", title: "Movie", quality: "720p", state: "ready", readyOffline: true, extension: ".mp4", sha256: "a".repeat(64), size: 0 }) }));
	await page.route("https://kinosail.test/api/v1/downloads/aaaaaaaaaaaaaaaa/file", (route) => { fileRequests++; return route.abort(); });
	await page.goto("https://kinosail.test/");
	await page.evaluate(() => new Promise<void>((resolve, reject) => {
		const request = indexedDB.open("kinosail-offline-v1", 3);
		request.onupgradeneeded = () => {
			request.result.createObjectStore("jobs", { keyPath: "id" });
			const chunks = request.result.createObjectStore("chunks", { keyPath: "id" });
			chunks.createIndex("jobID", "jobID");
			chunks.createIndex("jobRange", ["jobID", "offset"]);
		};
		request.onsuccess = () => { request.result.close(); resolve(); };
		request.onerror = () => reject(request.error);
	}));
	await page.addScriptTag({ content: downloadsSource.replaceAll("5000", "20") });
	const button = page.locator("[data-download-device]");
	await expect(button).toHaveAttribute("data-bound", "true");
	await page.evaluate(() => (window as Window & { KinosailOfflineMedia: { source: (id: string) => Promise<string> } }).KinosailOfflineMedia.source("missing"));
	await button.click();
	await expect(page.locator("[data-download-device-status]")).toHaveText("The download could not be verified");
	expect(fileRequests).toBe(0);
	expect(await page.evaluate(() => new Promise<{ chunks: IDBValidKey[]; jobs: IDBValidKey[]; version: number }>((resolve, reject) => {
		const request = indexedDB.open("kinosail-offline-v1");
		request.onsuccess = () => {
			const database = request.result;
			const transaction = database.transaction(["jobs", "chunks"], "readonly");
			const jobs = transaction.objectStore("jobs").getAllKeys();
			const chunks = transaction.objectStore("chunks").getAllKeys();
			transaction.oncomplete = () => { const version = database.version; database.close(); resolve({ chunks: chunks.result, jobs: jobs.result, version }); };
			transaction.onerror = () => reject(transaction.error);
		};
		request.onerror = () => reject(request.error);
	}))).toEqual({ chunks: [], jobs: [], version: 3 });
});
