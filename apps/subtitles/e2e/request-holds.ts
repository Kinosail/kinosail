import { expect, type Page } from "@playwright/test";

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
	const marker = "mainRequestHeld";
	const releaseEvent = "kinosail-release-main-request";
	await page.evaluate(({ marker, parameter, releaseEvent, value }) => {
		const open = XMLHttpRequest.prototype.open;
		const send = XMLHttpRequest.prototype.send;
		const urls = new WeakMap<XMLHttpRequest, string>();
		XMLHttpRequest.prototype.open = function (...args) {
			urls.set(this, String(args[1]));
			return Reflect.apply(open, this, args);
		};
		XMLHttpRequest.prototype.send = function (...args) {
			if (new URL(urls.get(this) ?? "", location.href).searchParams.get(parameter) === value && !document.documentElement.dataset[marker]) {
				document.documentElement.dataset[marker] = "true";
				const request = this;
				window.addEventListener(releaseEvent, () => Reflect.apply(send, request, args), { once: true });
				return;
			}
			return Reflect.apply(send, this, args);
		};
	}, { marker, parameter, releaseEvent, value });
	return hold(page, marker, releaseEvent);
}
