import { readFile } from "node:fs/promises";

export async function readStaticSource(paths: string[], base = import.meta.url): Promise<string> {
	return (await Promise.all(paths.map((path) => readFile(new URL(path, base), "utf8")))).join("");
}

export const playerSource = await readStaticSource([
	"../internal/server/static/playback-capabilities.js",
	"../../../packages/webassets/static/player-core.js",
	"../internal/server/static/player-subtitles.js",
	"../internal/server/static/player.js",
	"../internal/server/static/player-streaming-adaptive.js",
	"../internal/server/static/player-streaming-recovery.js",
	"../internal/server/static/player-streaming-lifecycle.js",
	"../../../packages/webassets/static/player-preview.js",
	"../../../packages/webassets/static/player-controls.js",
	"../../../packages/webassets/static/player-status.js",
	"../../../packages/webassets/static/player-presentation.js",
	"../../../packages/webassets/static/player-devices.js",
	"../../../packages/webassets/static/player-progress.js",
	"../../../packages/webassets/static/player-audio-queue.js",
	"../../../packages/webassets/static/player-progress-navigation.js",
]);

export const serviceWorkerSource = await readStaticSource([
	"../../../packages/webassets/static/offline-runtime.js",
	"../../../packages/webassets/static/offline-profile.js",
	"../../../packages/webassets/static/shell-assets.js",
	"../../../packages/webassets/static/offline-shell.js",
	"../internal/server/static/service-worker.js",
]);
