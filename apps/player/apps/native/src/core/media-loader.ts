import type { MediaItem, PlaybackSource } from './contract';
import type { KinosailClient } from './server-client';

type MediaClient = Pick<KinosailClient, 'loadItem' | 'loadPlayback'>;
type Entry<T> = { expires: number; value: Promise<T>; resolved?: T };
type MediaCache = {
  items: Map<string, Entry<MediaItem>>;
  playback: Map<string, Entry<PlaybackSource>>;
};

const cacheLifetime = 60_000;
const cacheLimit = 60;
const caches = new WeakMap<MediaClient, MediaCache>();

const validID = (id: string) =>
  typeof id === 'string' &&
  Boolean(id) &&
  id.length <= 2048 &&
  !/[\r\n]/.test(id);

const cacheFor = (client: MediaClient) => {
  let cache = caches.get(client);
  if (!cache) {
    cache = { items: new Map(), playback: new Map() };
    caches.set(client, cache);
  }
  return cache;
};

const cached = <T>(
  store: Map<string, Entry<T>>,
  id: string,
  load: () => Promise<T>,
  limit: number,
) => {
  const hit = store.get(id);
  if (hit && hit.expires > Date.now()) return hit.value;
  const value = load()
    .then((loaded) => {
      const entry = store.get(id);
      if (entry?.value === value) entry.resolved = loaded;
      return loaded;
    })
    .catch((error) => {
      if (store.get(id)?.value === value) store.delete(id);
      throw error;
    });
  store.set(id, { expires: Date.now() + cacheLifetime, value });
  if (store.size > limit) store.delete(store.keys().next().value!);
  return value;
};

export const readMediaItem = (client: MediaClient, id: string) => {
  if (!validID(id)) return null;
  const entry = caches.get(client)?.items.get(id);
  return entry && entry.expires > Date.now() ? (entry.resolved ?? null) : null;
};

export const loadMediaItem = (client: MediaClient, id: string) => {
  if (!validID(id)) return client.loadItem(id);
  return cached(
    cacheFor(client).items,
    id,
    () => client.loadItem(id),
    cacheLimit,
  );
};

export const loadMediaPlayback = (client: MediaClient, id: string) => {
  if (!validID(id)) return client.loadPlayback(id);
  return cached(
    cacheFor(client).playback,
    id,
    () => client.loadPlayback(id),
    32,
  );
};

export const rememberMediaItems = (
  client: MediaClient,
  items: readonly MediaItem[],
) => {
  const store = cacheFor(client).items;
  for (const item of items) {
    if (!validID(item.id)) continue;
    store.delete(item.id);
    store.set(item.id, {
      expires: Date.now() + cacheLifetime,
      value: Promise.resolve(item),
      resolved: item,
    });
    if (store.size > cacheLimit) store.delete(store.keys().next().value!);
  }
};
