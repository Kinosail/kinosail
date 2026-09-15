import { requireOptionalNativeModule } from 'expo';
import { playbackCapabilitiesQuery } from './playback-capabilities';

const capabilities = requireOptionalNativeModule<{
  getCapabilities(): Promise<unknown>;
}>('PlaybackCapabilities');
// This describes the platform recovery stream, not all formats VLC can open.
// Missing/failed detection retains the conservative stream contract.
export async function getPlaybackQuery(): Promise<string> {
  if (capabilities) {
    let timeout: ReturnType<typeof setTimeout> | undefined;
    try {
      const result = await Promise.race([
        capabilities.getCapabilities(),
        new Promise((resolve) => {
          timeout = setTimeout(() => resolve(null), 750);
        }),
      ]);
      return playbackCapabilitiesQuery(result);
    } catch {
      /* A missing capability must not prevent original playback. */
    } finally {
      clearTimeout(timeout);
    }
  }
  return 'videoCodecs=h264&audioCodecs=aac';
}
