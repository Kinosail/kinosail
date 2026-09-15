import * as Network from 'expo-network';
import {
  downloadsAvailable,
  listDownloads,
  downloadMedia,
  removeDownload,
} from './downloads';
import { withPendingProgress } from './progress-sync';
import type { KinosailClient } from './server-client';
let playingID = '';
export function setPlayingDownload(id: string) {
  playingID = id;
}
export async function maintainDownloads(client: KinosailClient) {
  if (!downloadsAvailable) return;
  const preferences = await client.loadMediaPreferences();
  if (!preferences.autoDownloadNext && !preferences.removeWatched) return;
  const network = await Network.getNetworkStateAsync();
  if (
    !network.isConnected ||
    (preferences.wifiOnly &&
      ![
        Network.NetworkStateType.WIFI,
        Network.NetworkStateType.ETHERNET,
      ].includes(network.type ?? Network.NetworkStateType.UNKNOWN))
  )
    return;
  const entries = await listDownloads(client);
  const seen = new Set(entries.map((entry) => entry.item.id));
  const shows = new Set<string>();
  for (const entry of entries) {
    const current = await client.loadItem(entry.item.id);
    if (
      preferences.removeWatched &&
      entry.status === 'complete' &&
      current.progress.watched
    ) {
      await withPendingProgress(client, async (pending) => {
        if (
          !pending.some((value) => value.id === entry.item.id) &&
          entry.item.id !== playingID
        )
          await removeDownload(
            client,
            entry.item.id,
            () => entry.item.id !== playingID,
          );
      });
    }
    if (
      preferences.autoDownloadNext &&
      current.show &&
      !shows.has(current.show)
    ) {
      shows.add(current.show);
      let cursor = current;
      for (let count = 0; count < preferences.autoDownloadNext; count++) {
        const source = await client.loadPlayback(cursor.id),
          next = source.details?.downloadNext ?? source.details?.next;
        if (!next || next === cursor.id) break;
        const item = await client.loadItem(next);
        if (item.show !== current.show || item.progress.watched) break;
        if (!seen.has(item.id)) {
          await downloadMedia(client, item, () => {}, entry.quality);
          seen.add(item.id);
        }
        cursor = item;
      }
    }
  }
}
