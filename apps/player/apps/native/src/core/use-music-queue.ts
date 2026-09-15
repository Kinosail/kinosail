import { useEffect, useState } from 'react';
import type { MediaItem } from './contract';
import type { KinosailClient } from './server-client';

export type MusicQueue = {
  items: MediaItem[];
  index: number;
  shuffled: boolean;
  repeat: 'off' | 'all' | 'one';
  loading: boolean;
  error: string;
  select(id: string): void;
  shuffle(): void;
  cycleRepeat(): void;
  retry(): void;
};
export function shuffledTracks(items: MediaItem[]) {
  const result = [...items];
  for (let i = result.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [result[i], result[j]] = [result[j], result[i]];
  }
  return result;
}
export function queueNeighbor(
  queue: Pick<MusicQueue, 'items' | 'index' | 'repeat'>,
  direction: -1 | 1,
) {
  if (queue.index < 0 || !queue.items.length) return undefined;
  const next = queue.index + direction;
  return (
    queue.items[next]?.id ??
    (queue.repeat === 'all'
      ? queue.items[(next + queue.items.length) % queue.items.length]?.id
      : undefined)
  );
}
export function useMusicQueue({
  client,
  root,
  current,
  enabled,
  album,
  initialShuffle,
  onSelect,
}: {
  client: KinosailClient | null;
  root: string;
  current: string;
  enabled: boolean;
  album?: string;
  initialShuffle: boolean;
  onSelect(id: string): void;
}): MusicQueue {
  const [result, setResult] = useState<{
    client: KinosailClient;
    root: string;
    items: MediaItem[];
    error: string;
  } | null>(null);
  const [order, setOrder] = useState<MediaItem[] | null>(null);
  const [repeat, setRepeat] = useState<MusicQueue['repeat']>('off');
  const [attempt, setAttempt] = useState(0);
  const active =
    result?.client === client && result.root === root ? result : null;
  useEffect(() => {
    if (!enabled || !client) return;
    const controller = new AbortController();
    setOrder(null);
    const request = album
      ? client.loadAlbum(album, controller.signal).then((value) => value.tracks)
      : client.loadMusicQueue(root, controller.signal);
    request
      .then((items) => {
        if (controller.signal.aborted) return;
        if (!items.some((item) => item.id === root))
          throw new Error('This track is no longer in the album.');
        setResult({ client, root, items, error: '' });
        if (initialShuffle) {
          const mixed = shuffledTracks(items);
          setOrder(mixed);
          onSelect(mixed[0].id);
        }
      })
      .catch(() => {
        if (!controller.signal.aborted)
          setResult({
            client,
            root,
            items: [],
            error: 'Could not load the album queue. This track can still play.',
          });
      });
    return () => controller.abort();
  }, [client, root, album, enabled, attempt, initialShuffle, onSelect]);
  const items = active ? (order ?? active.items) : [];
  return {
    items,
    index: items.findIndex((item) => item.id === current),
    shuffled: order !== null,
    repeat,
    loading: enabled && !active,
    error: active?.error ?? '',
    select: (id) => {
      if (items.some((item) => item.id === id)) onSelect(id);
    },
    shuffle: () =>
      setOrder((previous) =>
        previous
          ? null
          : [
              ...items.filter((item) => item.id === current),
              ...shuffledTracks(items.filter((item) => item.id !== current)),
            ],
      ),
    cycleRepeat: () =>
      setRepeat((value) =>
        value === 'off' ? 'all' : value === 'all' ? 'one' : 'off',
      ),
    retry: () => {
      setResult(null);
      setOrder(null);
      setAttempt((value) => value + 1);
    },
  };
}
