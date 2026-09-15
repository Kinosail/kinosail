import type { MediaItem } from './contract';
import type { DownloadQuality } from './download-quality';

const sizeLabel = (bytes: number) =>
  bytes >= 1_000_000_000
    ? `${(bytes / 1_000_000_000).toFixed(1)} GB`
    : `${Math.max(1, Math.round(bytes / 1_000_000))} MB`;

export function downloadEstimate(
  items: readonly MediaItem[],
  quality: DownloadQuality,
  durations: Readonly<Record<string, number>>,
): string | null {
  if (!items.length) return null;
  if (quality === 'original') {
    if (
      items.some(
        (item) => !Number.isSafeInteger(item.size) || (item.size ?? 0) <= 0,
      )
    )
      return null;
    const total = items.reduce((sum, item) => sum + item.size!, 0);
    return Number.isSafeInteger(total) ? sizeLabel(total) : null;
  }
  if (
    !['1080p', '720p'].includes(quality) ||
    items.some(
      (item) =>
        !Number.isFinite(durations[item.id]) ||
        durations[item.id] <= 0 ||
        durations[item.id] > 31_536_000,
    )
  )
    return null;
  const duration = items.reduce((sum, item) => sum + durations[item.id], 0);
  // Planning ranges, not an encoder promise: quality-based output varies by
  // codec, content, and Server settings. Include the download's 160 kb/s audio.
  const rates = quality === '1080p' ? [3, 8] : [1.5, 4];
  const sizes = rates.map((rate) => (duration * (rate + 0.16) * 1_000_000) / 8);
  return `About ${sizeLabel(sizes[0])}–${sizeLabel(sizes[1])}`;
}
