import type { Album } from './music-catalog';
import type { Home, MediaItem } from './contract';
import type { KinosailClient } from './server-client';

// Only validated server results enter these session-local snapshots. Reopening a
// screen still refreshes from the server; snapshots cover that network wait.
export function createBrowseCache<T>(limit: number) {
  const clients = new WeakMap<
    KinosailClient,
    Map<string, { value: T; expires: number }>
  >();
  return {
    read(client: KinosailClient, key: string): T | null {
      const entries = clients.get(client);
      const entry = entries?.get(key);
      if (!entry) return null;
      if (entry.expires <= Date.now()) {
        entries!.delete(key);
        return null;
      }
      return entry.value;
    },
    write(client: KinosailClient, key: string, value: T) {
      let entries = clients.get(client);
      if (!entries) {
        entries = new Map();
        clients.set(client, entries);
      }
      entries.delete(key);
      entries.set(key, { value, expires: Date.now() + 30 * 60_000 });
      if (entries.size > limit) entries.delete(entries.keys().next().value!);
    },
    clear(client: KinosailClient) {
      clients.delete(client);
    },
  };
}

export const homeCache = createBrowseCache<Home>(1);
export const libraryCache =
  createBrowseCache<Awaited<ReturnType<KinosailClient['browseLibrary']>>>(12);

export const albumCatalogCache = createBrowseCache<Album[]>(1);
export const albumTracksCache = createBrowseCache<MediaItem[]>(12);
export const collectionsCache = createBrowseCache<{
  names: string[];
  items: MediaItem[];
}>(8);
