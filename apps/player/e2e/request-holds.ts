import { expect, type Page } from "@playwright/test";

type RequestHold = {
	release(): Promise<void>;
	waitUntilStarted(): Promise<void>;
};

function hold(page: Page, marker: string, releaseEvent: string): RequestHold {
	const attribute = marker.replace(/[A-Z]/g, (character) => `-${character.toLowerCase()}`);
	return {
		release: async () => {await page.evaluate((event) => window.dispatchEvent(new Event(event)), releaseEvent);},
		waitUntilStarted: () => expect.poll(() => page.locator(`html[data-${attribute}]`).count()).toBe(1),
	};
}

export async function holdNextLibraryPage(page: Page, offset: string): Promise<RequestHold> {
	const marker = "libraryLoadHeld";
	const releaseEvent = "kinosail-release-library-load";
	await page.addInitScript(({ marker, offset, releaseEvent }) => {
		const fetch = window.fetch;
		window.fetch = async (input, init) => {
			const url = new URL(input instanceof Request ? input.url : String(input), location.href);
			const headers = new Headers(init?.headers ?? (input instanceof Request ? input.headers : undefined));
			if (headers.get("X-Kinosail-Library-Page") === "1" && url.searchParams.get("offset") === offset && !document.documentElement.dataset[marker]) {
				document.documentElement.dataset[marker] = "true";
				await new Promise<void>((resolve) => window.addEventListener(releaseEvent, () => resolve(), { once: true }));
			}
			return fetch(input, init);
		};
	}, { marker, offset, releaseEvent });
	return hold(page, marker, releaseEvent);
}

export async function holdNextMainRequest(page: Page, parameter: string, value: string): Promise<RequestHold> {
	const marker = "mainRequestHeld";
	const releaseEvent = "kinosail-release-main-request";
	await page.evaluate(({parameter, value, marker, releaseEvent}) => {
		delete document.documentElement.dataset[marker];
		const fetch = window.fetch;
		const prototype = XMLHttpRequest.prototype;
		const open = prototype.open, send = prototype.send, abort = prototype.abort;
		const targets = new WeakMap<XMLHttpRequest, {method: string; url: URL}>();
		const cancelled = new WeakSet<XMLHttpRequest>();
		let started = false;
		let release!: () => void;
		const gate = new Promise<void>(resolve => {release = resolve;});
		const matches = (method: string, url: URL) => method.toUpperCase() === "GET"
			&& url.origin === location.origin && url.pathname === "/" && url.searchParams.get(parameter) === value;
		const wait = async (method: string, url: URL) => {
			if (started || !matches(method, url)) return;
			started = true;
			document.documentElement.dataset[marker] = "true";
			await gate;
		};
		const heldFetch: typeof window.fetch = async (input, init) => {
			const url = new URL(input instanceof Request ? input.url : String(input), location.href);
			await wait(init?.method ?? (input instanceof Request ? input.method : "GET"), url);
			return fetch(input, init);
		};
		const heldOpen: typeof open = function (this: XMLHttpRequest, method: string, url: string | URL,
			async: boolean = true, username?: string | null, password?: string | null) {
			targets.set(this, {method, url: new URL(String(url), location.href)});
			cancelled.delete(this);
			return open.call(this, method, url, async, username, password);
		};
		const heldSend: typeof send = function (this: XMLHttpRequest, body) {
			const target = targets.get(this);
			if (!target || started || !matches(target.method, target.url)) return send.call(this, body);
			void wait(target.method, target.url).then(() => {
				if (!cancelled.has(this)) send.call(this, body);
			});
		};
		const heldAbort: typeof abort = function (this: XMLHttpRequest) {cancelled.add(this); return abort.call(this);};
		window.fetch = heldFetch;
		prototype.open = heldOpen;
		prototype.send = heldSend;
		prototype.abort = heldAbort;
		// Gate before the browser transport enters a service worker. Context routes
		// do not reliably intercept an already controlled document's requests.
		window.addEventListener(releaseEvent, () => {
			release();
			if (window.fetch === heldFetch) window.fetch = fetch;
			if (prototype.open === heldOpen) prototype.open = open;
			if (prototype.send === heldSend) prototype.send = send;
			if (prototype.abort === heldAbort) prototype.abort = abort;
		}, {once: true});
	}, {parameter, value, marker, releaseEvent});
	return hold(page, marker, releaseEvent);
}
