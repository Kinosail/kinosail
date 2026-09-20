import { readFile } from "node:fs/promises";

export async function readStaticSource(paths: string[], base = import.meta.url): Promise<string> {
	return (await Promise.all(paths.map((path) => readFile(new URL(path, base), "utf8")))).join("");
}

export const downloadsSource = await readStaticSource([
	"../../../packages/webassets/static/offline-identity.js",
	"../../../packages/webassets/static/offline-runtime.js",
	"../../../packages/webassets/static/downloads.js",
	"../../../packages/webassets/static/downloads-integrity.js",
	"../../../packages/webassets/static/downloads-storage.js",
	"../../../packages/webassets/static/downloads-transfer.js",
	"../../../packages/webassets/static/downloads-progress.js",
	"../../../packages/webassets/static/downloads-ui.js",
]);

export const playerSource = await readStaticSource([
	"../../../packages/webassets/static/player-core.js",
	"../internal/server/static/player-subtitles.js",
	"../internal/server/static/player.js",
	"../internal/server/static/player-streaming-adaptive.js",
	"../internal/server/static/player-streaming-recovery.js",
	"../internal/server/static/player-streaming-offline.js",
	"../../../packages/webassets/static/player-controls.js",
	"../../../packages/webassets/static/player-presentation.js",
	"../../../packages/webassets/static/player-devices.js",
	"../../../packages/webassets/static/player-progress.js",
]);

export const serviceWorkerSource = await readStaticSource([
	"../../../packages/webassets/static/offline-runtime.js",
	"../../../packages/webassets/static/offline-media.js",
	"../internal/server/static/service-worker.js",
]);
