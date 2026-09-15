import type { DownloadTrackSelection } from './download-tracks';
import type { DownloadQuality } from './download-quality';
import type { KinosailClient } from './server-client';
import type { MediaItem, PlaybackSource, Progress } from './contract';
import type { DownloadEntry } from './downloads.types';
export type { DownloadEntry } from './downloads.types';
export const downloadsAvailable = false;
export const backgroundDownloadsAvailable = false;
export async function listDownloads(
  _client: KinosailClient,
): Promise<DownloadEntry[]> {
  return [];
}
export async function downloadMedia(
  _client: KinosailClient,
  _item: MediaItem,
  _change: () => void,
  _quality?: DownloadQuality,
  _tracks?: DownloadTrackSelection,
): Promise<void> {
  throw new Error('Downloads are available in the mobile app.');
}
export async function pauseDownload(_client?: KinosailClient, _id?: string) {}
export async function removeDownload(
  _client: KinosailClient,
  _id: string,
  _canRemove?: () => boolean,
): Promise<void> {}
export async function clearDownloads(): Promise<void> {}
export async function downloadedPlayback(
  _client: KinosailClient,
  _id: string,
): Promise<{
  item: MediaItem;
  source: PlaybackSource;
  baseline?: Progress;
} | null> {
  return null;
}

export async function saveDownloadedProgress(
  _client: KinosailClient,
  _id: string,
  _input: Pick<Progress, 'seconds' | 'session' | 'revision'> & {
    watched?: boolean;
  },
): Promise<Progress> {
  throw new Error('Offline playback is unavailable.');
}

export async function acknowledgeDownloadedProgress(
  _client: KinosailClient,
  _id: string,
  _snapshot: Progress,
): Promise<void> {}

export function downloadStorage(): { total: number; free: number } | null {
  return null;
}

export async function checkDownloadedMedia(
  _client: KinosailClient,
  _id: string,
): Promise<void> {
  throw new Error('Offline checks require the native Player build.');
}
