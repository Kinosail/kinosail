import { expect, type Page, type Route } from "@playwright/test";

type RequestHold = {
	release(): Promise<void>;
	waitUntilStarted(): Promise<void>;
};

function hold(page: Page, marker: string, releaseEvent: string): RequestHold {
	const attribute = marker.replace(/[A-Z]/g, (character) => `-${character.toLowerCase()}`);
	return {
		release: () => page.evaluate((event) => window.dispatchEvent(new Event(event)), releaseEvent),
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
	const origin = new URL(page.url()).origin;
	const matches = (url: URL) => url.origin === origin && url.pathname === "/" && url.searchParams.get(parameter) === value;
	let started = false;
	let release!: () => void, finish!: () => void;
	const gate = new Promise<void>(resolve => { release = resolve; });
	const handled = new Promise<void>(resolve => { finish = resolve; });
	const handler = async (route: Route) => {
		if (started || route.request().method() !== "GET") {
			await route.fallback();
			return;
		}
		started = true;
		await gate;
		try { await route.continue(); }
		catch (error) {
			if (!route.request().failure() && !page.isClosed()) throw error;
		} finally { finish(); }
	};
	// HTMX uses XHR. The context boundary also observes network requests made
	// through a service worker without replacing either browser transport.
	await page.context().route(matches, handler);
	return {
		release: async () => {
			release();
			if (started) await handled;
			await page.context().unroute(matches, handler);
		},
		waitUntilStarted: () => expect.poll(() => started).toBe(true),
	};
}
