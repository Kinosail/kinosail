import { readStaticSource } from "../../player/e2e/static-sources";

export const playerSource = await readStaticSource([
  "../../../packages/webassets/static/player-core.js",
  "../internal/server/static/player_streaming_start.js",
  "../internal/server/static/player_streaming_end.js",
  "../../../packages/webassets/static/player-controls.js",
  "../../../packages/webassets/static/player-devices.js",
  "../../../packages/webassets/static/player-progress.js",
], import.meta.url);
