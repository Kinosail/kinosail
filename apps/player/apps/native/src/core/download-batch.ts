import { parseMediaItem, type InputValue, type MediaItem } from './contract';
import { downloadLimit, validateDownloadID } from './download-policy';
import { parseDownloadQuality, type DownloadQuality } from './download-quality';
import { downloadMedia, listDownloads } from './downloads';
import type { KinosailClient } from './server-client';

export type DownloadScope = 'episode' | 'season' | 'show';

export function selectDownloadEpisodes(
  item: MediaItem,
  episodes: readonly MediaItem[],
  scope: DownloadScope,
): MediaItem[] {
  if (!['episode', 'season', 'show'].includes(scope))
    throw new Error('Choose an episode, season, or show.');
  if (scope === 'episode') return [item];
  if (
    !item.showId ||
    !episodes.length ||
    episodes.length > downloadLimit ||
    episodes.some(
      (episode) => episode.kind !== 'video' || episode.showId !== item.showId,
    ) ||
    !episodes.some((episode) => episode.id === item.id)
  )
    throw new Error('The show’s episodes could not be loaded.');
  return episodes
    .filter((episode) => scope === 'show' || episode.season === item.season)
    .sort(
      (a, b) =>
        a.season - b.season ||
        a.episode - b.episode ||
        a.id.localeCompare(b.id),
    );
}

// Validate the entire selection before starting any individual download. The
// existing downloader remains responsible for authorization, quota and transfer.
export async function downloadBatch(
  client: KinosailClient,
  input: readonly MediaItem[],
  quality: DownloadQuality,
  change: () => void,
  progress: (completed: number, total: number) => void,
  cancelled: () => boolean = () => false,
): Promise<void> {
  parseDownloadQuality(quality);
  if (!Array.isArray(input) || !input.length || input.length > downloadLimit)
    throw new Error(`Choose between 1 and ${downloadLimit} episodes.`);
  const items = input.map((value) =>
    parseMediaItem(value as unknown as InputValue),
  );
  const ids = new Set<string>();
  for (const item of items) {
    validateDownloadID(item.id);
    if (
      ids.has(item.id) ||
      !['video', 'audio', 'audiobook'].includes(item.kind) ||
      (quality !== 'original' && item.kind !== 'video')
    )
      throw new Error('The download selection is invalid.');
    ids.add(item.id);
  }
  const existing = await listDownloads(client);
  const saved = new Set(existing.map((entry) => entry.item.id));
  const pending = items.filter((item) => !saved.has(item.id));
  if (existing.length + pending.length > downloadLimit)
    throw new Error(
      `There is room for ${Math.max(0, downloadLimit - existing.length)} more downloads. Remove downloads before adding this selection.`,
    );
  let completed = 0;
  progress(completed, pending.length);
  for (const item of pending) {
    if (cancelled())
      throw new Error(
        'Stopped adding episodes. Downloads already added remain in Downloads.',
      );
    try {
      await downloadMedia(client, item, change, quality);
    } catch (reason) {
      throw new Error(
        `${completed} of ${pending.length} added. ${item.title}: ${reason instanceof Error ? reason.message : 'Could not start the download.'} Try again to add the remaining episodes.`,
      );
    }
    completed++;
    progress(completed, pending.length);
  }
}
