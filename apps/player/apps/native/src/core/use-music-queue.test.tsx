import { act, renderHook, waitFor } from '@testing-library/react-native';
import {
  queueNeighbor,
  shuffledTracks,
  useMusicQueue,
} from './use-music-queue';
import { routeItem } from '@/testing/route-fixtures';
import type { KinosailClient } from './server-client';
const tracks = ['one', 'two', 'three'].map((id) => ({
  ...routeItem,
  id,
  kind: 'music',
}));
it('bounds queue movement and wraps only for repeat album', () => {
  expect(
    queueNeighbor({ items: tracks, index: 2, repeat: 'off' }, 1),
  ).toBeUndefined();
  expect(
    queueNeighbor({ items: tracks, index: 2, repeat: 'one' }, 1),
  ).toBeUndefined();
  expect(queueNeighbor({ items: tracks, index: 2, repeat: 'all' }, 1)).toBe(
    'one',
  );
  expect(
    queueNeighbor({ items: tracks, index: 0, repeat: 'off' }, -1),
  ).toBeUndefined();
  expect(queueNeighbor({ items: tracks, index: 0, repeat: 'all' }, -1)).toBe(
    'three',
  );
  expect(
    queueNeighbor({ items: [], index: -1, repeat: 'all' }, 1),
  ).toBeUndefined();
});
it('shuffles without dropping, duplicating, or mutating tracks', () => {
  const before = [...tracks];
  expect(
    shuffledTracks(tracks)
      .map((item) => item.id)
      .sort(),
  ).toEqual(['one', 'three', 'two']);
  expect(tracks).toEqual(before);
});
it('loads an album once, retains queue state across tracks, and rejects unknown selections', async () => {
  const loadAlbum = jest.fn().mockResolvedValue({ tracks });
  const client = { loadAlbum } as unknown as KinosailClient;
  const onSelect = jest.fn();
  const { result, rerender } = await renderHook(
    ({ current }: { current: string }) =>
      useMusicQueue({
        client,
        root: 'one',
        current,
        enabled: true,
        album: 'album',
        initialShuffle: false,
        onSelect,
      }),
    { initialProps: { current: 'one' } },
  );
  await waitFor(() => expect(result.current.items).toHaveLength(3));
  await act(() => result.current.select('unknown'));
  expect(onSelect).not.toHaveBeenCalled();
  await act(() => result.current.select('two'));
  expect(onSelect).toHaveBeenCalledWith('two');
  await rerender({ current: 'two' });
  expect(result.current.index).toBe(1);
  expect(loadAlbum).toHaveBeenCalledTimes(1);
  await act(() => result.current.shuffle());
  expect(result.current.items[0].id).toBe('two');
  await act(() => result.current.cycleRepeat());
  expect(result.current.repeat).toBe('all');
});
it('reports queue failure without inventing tracks and supports retry', async () => {
  const loadMusicQueue = jest
    .fn()
    .mockRejectedValueOnce(new Error('offline'))
    .mockResolvedValue(tracks);
  const client = { loadMusicQueue } as unknown as KinosailClient;
  const onSelect = jest.fn();
  const { result } = await renderHook(() =>
    useMusicQueue({
      client,
      root: 'one',
      current: 'one',
      enabled: true,
      initialShuffle: false,
      onSelect,
    }),
  );
  await waitFor(() =>
    expect(result.current.error).toContain('This track can still play'),
  );
  expect(result.current.items).toEqual([]);
  await act(() => result.current.retry());
  await waitFor(() => expect(result.current.items).toHaveLength(3));
});
it('does not fetch a music queue for audiobook or offline playback', async () => {
  const loadMusicQueue = jest.fn();
  const client = { loadMusicQueue } as unknown as KinosailClient;
  await renderHook(() =>
    useMusicQueue({
      client,
      root: 'one',
      current: 'one',
      enabled: false,
      initialShuffle: false,
      onSelect: jest.fn(),
    }),
  );
  expect(loadMusicQueue).not.toHaveBeenCalled();
});
