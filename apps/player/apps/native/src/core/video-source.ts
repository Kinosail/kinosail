import type { ContentType } from 'expo-video';

import type { PlaybackSource } from './contract';

const hlsTypes = new Set([
  'application/vnd.apple.mpegurl',
  'application/x-mpegurl',
]);

export function nativeContentType(value: string): ContentType {
  const mime = value.split(';', 1)[0].trim().toLowerCase();
  if (hlsTypes.has(mime)) return 'hls';
  if (mime === 'application/dash+xml') return 'dash';
  if (mime === 'application/vnd.ms-sstr+xml') return 'smoothStreaming';
  return 'progressive';
}

// Used by automatic recovery and manual retries.
// Map source time to the Server's optional automatic-skip presentation.
export function compatiblePlaybackSource(
  source: PlaybackSource,
  position: number,
): PlaybackSource | null {
  const next = source.compatible;
  if (!next) return null;
  const sourceTime = Number.isFinite(position) ? Math.max(0, position) : source.start;
  let start = sourceTime;
  for (const range of next.omitted) {
    start -= Math.max(0, Math.min(sourceTime, range.end) - range.start);
  }
  return {
    ...source,
    uri: next.uri,
    contentType: 'application/vnd.apple.mpegurl',
    duration: next.duration,
    start: Math.min(next.duration, Math.max(0, start)),
    progressToken: next.progressToken,
    plan: next.plan,
    compatible: undefined,
  };
}
