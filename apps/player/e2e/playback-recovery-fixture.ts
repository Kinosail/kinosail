import { expect, type Page, type TestInfo } from "@playwright/test";

export async function injectUnknownFailure(page: Page, info: TestInfo, label: string) {
	const video = page.locator("video");
	const ready = await video.evaluate(async (element: HTMLVideoElement) => {
		element.muted = true;
		await element.play();
		element.pause();
		return { currentTime: element.currentTime, duration: element.duration,
			readyState: element.readyState, seeking: element.seeking, paused: element.paused,
			errorCode: element.error?.code ?? 0 };
	});
	await info.attach(`recovery-${label}-prerequisite`, { contentType: "application/json", body: JSON.stringify(ready) });
	expect(ready).toMatchObject({ seeking: false, paused: true, errorCode: 0 });
	expect(ready.readyState).toBeGreaterThanOrEqual(3);
	await video.evaluate((video) => {
		video.pause();
		video.dataset.compatibilityMode = "transcode";
		video.dataset.compatibilityLabel = "Transcoding video";
		video.dataset.compatibilityDescription = "This device cannot decode the original video.";
		Object.defineProperty(video, "error", {
			configurable: true,
			value: { code: 0 },
		});
		video.dispatchEvent(new Event("error"));
	});
}
