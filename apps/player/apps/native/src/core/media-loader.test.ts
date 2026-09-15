import type { MediaItem, PlaybackSource } from './contract';
import { deferred } from '@/testing/route-fixtures';
import {
  loadMediaItem,
  readMediaItem,
  loadMediaPlayback,
  rememberMediaItems,
} from './media-loader';

const item = { id: 'arrival' } as MediaItem;
const playback = {
  uri: 'https://kino.example/media/arrival',
} as PlaybackSource;
const mediaClient = () => ({
  loadItem: jest.fn().mockResolvedValue(item),
  loadPlayback: jest.fn(),
});
const mediaShelf = () =>
  Array.from({ length: 60 }, (_, index) => ({
    ...item,
    id: `item-${index}`,
  }));

describe('media loader', () => {
  afterEach(() => jest.restoreAllMocks());

  it('caches the longest valid item key', async () => {
    const client = {
      loadItem: jest.fn().mockResolvedValue(item),
      loadPlayback: jest.fn(),
    };
    const id = 'a'.repeat(2048);
    await loadMediaItem(client, id);
    await loadMediaItem(client, id);
    expect(client.loadItem).toHaveBeenCalledTimes(1);
  });

  it('retains all 60 request entries until capacity is exceeded', async () => {
    const client = {
      loadItem: jest.fn().mockResolvedValue(item),
      loadPlayback: jest.fn(),
    };
    for (let index = 0; index < 60; index += 1)
      await loadMediaItem(client, `item-${index}`);
    await loadMediaItem(client, 'item-0');
    expect(client.loadItem).toHaveBeenCalledTimes(60);
  });

  it('does not let invalid primed items evict a valid entry', async () => {
    const client = mediaClient();
    const shelf = mediaShelf();
    rememberMediaItems(client, shelf);
    rememberMediaItems(client, [{ ...item, id: '' }]);
    await expect(loadMediaItem(client, 'item-0')).resolves.toBe(shelf[0]);
    expect(client.loadItem).not.toHaveBeenCalled();
  });

  it('moves refreshed shelf data behind older entries before eviction', async () => {
    const client = mediaClient();
    const shelf = mediaShelf();
    rememberMediaItems(client, shelf);
    const refreshed = { ...shelf[0], title: 'Updated title' };
    rememberMediaItems(client, [refreshed, { ...item, id: 'new-title' }]);
    await expect(loadMediaItem(client, 'item-0')).resolves.toBe(refreshed);
    expect(client.loadItem).not.toHaveBeenCalled();
    await loadMediaItem(client, 'item-1');
    expect(client.loadItem).toHaveBeenCalledWith('item-1');
  });

  it('opens every title from a full home response without refetching detail data', async () => {
    const client = {
      loadItem: jest.fn().mockResolvedValue(item),
      loadPlayback: jest.fn(),
    };
    const recent = Array.from({ length: 36 }, (_, index) => ({
      ...item,
      id: `recent-${index}`,
    }));
    const history = Array.from({ length: 24 }, (_, index) => ({
      ...item,
      id: `history-${index}`,
    }));
    const shelf = [...recent, ...history];
    client.loadItem.mockImplementation(async (id: string) =>
      shelf.find((entry) => entry.id === id),
    );
    rememberMediaItems(client, shelf);
    for (const entry of [...shelf].reverse()) {
      await expect(loadMediaItem(client, entry.id)).resolves.toBe(entry);
    }
    expect(client.loadItem).not.toHaveBeenCalled();
  });

  it.each(['', 'x'.repeat(2049), 'bad\rkey', 'bad\nkey'])(
    'does not cache an invalid playback key %#',
    async (id) => {
      const client = {
        loadItem: jest.fn(),
        loadPlayback: jest.fn().mockResolvedValue(playback),
      };
      await loadMediaPlayback(client, id);
      await loadMediaPlayback(client, id);
      expect(client.loadPlayback).toHaveBeenCalledTimes(2);
    },
  );

  it('isolates cached work between authorized clients', async () => {
    const first = {
      loadItem: jest.fn().mockResolvedValue(item),
      loadPlayback: jest.fn().mockResolvedValue(playback),
    };
    const secondItem = { ...item, title: 'Another viewer library' };
    const second = {
      loadItem: jest.fn().mockResolvedValue(secondItem),
      loadPlayback: jest.fn().mockResolvedValue(playback),
    };
    await loadMediaItem(first, 'arrival');
    await expect(loadMediaItem(second, 'arrival')).resolves.toBe(secondItem);
    expect(first.loadItem).toHaveBeenCalledTimes(1);
    expect(second.loadItem).toHaveBeenCalledTimes(1);
  });

  it('evicts old request entries after the bounded cache fills', async () => {
    const client = {
      loadItem: jest.fn().mockResolvedValue(item),
      loadPlayback: jest.fn(),
    };
    for (let index = 0; index < 61; index += 1)
      await loadMediaItem(client, `item-${index}`);
    expect(client.loadItem).toHaveBeenCalledTimes(61);
    await loadMediaItem(client, 'item-60');
    expect(client.loadItem).toHaveBeenCalledTimes(61);
    await loadMediaItem(client, 'item-0');
    expect(client.loadItem).toHaveBeenCalledTimes(62);
  });

  it('keeps the playback plan cache bounded to 32 entries', async () => {
    const client = {
      loadItem: jest.fn(),
      loadPlayback: jest.fn().mockResolvedValue(playback),
    };
    for (let index = 0; index < 33; index += 1)
      await loadMediaPlayback(client, `item-${index}`);
    await loadMediaPlayback(client, 'item-32');
    expect(client.loadPlayback).toHaveBeenCalledTimes(33);
    await loadMediaPlayback(client, 'item-0');
    expect(client.loadPlayback).toHaveBeenCalledTimes(34);
  });

  it('bounds primed shelves and refreshes an existing item', async () => {
    const client = {
      loadItem: jest.fn().mockResolvedValue(item),
      loadPlayback: jest.fn(),
    };
    const shelf = Array.from({ length: 61 }, (_, index) => ({
      ...item,
      id: `item-${index}`,
    }));
    rememberMediaItems(client, shelf);
    await expect(loadMediaItem(client, 'item-32')).resolves.toBe(shelf[32]);
    await loadMediaItem(client, 'item-0');
    expect(client.loadItem).toHaveBeenCalledTimes(1);
    const refreshed = { ...shelf[32], title: 'Updated title' };
    rememberMediaItems(client, [refreshed]);
    await expect(loadMediaItem(client, 'item-32')).resolves.toBe(refreshed);
  });

  it.each(['expired', 'evicted'] as const)(
    'does not invalidate a replacement when an old %s request rejects',
    async (mode) => {
      const now = jest.spyOn(Date, 'now').mockReturnValue(1000);
      const pending = deferred<MediaItem>();
      const client = {
        loadItem: jest
          .fn()
          .mockReturnValueOnce(pending.promise)
          .mockResolvedValue(item),
        loadPlayback: jest.fn(),
      };
      const first = loadMediaItem(client, 'arrival');
      const failure = expect(first).rejects.toThrow('offline');
      if (mode === 'expired') {
        now.mockReturnValue(61_000);
        await loadMediaItem(client, 'arrival');
      } else {
        for (let index = 0; index < 60; index += 1)
          await loadMediaItem(client, `item-${index}`);
      }
      pending.reject(new Error('offline'));
      await failure;
      const before = client.loadItem.mock.calls.length;
      await expect(loadMediaItem(client, 'arrival')).resolves.toBe(item);
      expect(client.loadItem).toHaveBeenCalledTimes(
        before + (mode === 'evicted' ? 1 : 0),
      );
    },
  );
  it('shares item and playback work across detail and watch routes', async () => {
    const client = {
      loadItem: jest.fn().mockResolvedValue(item),
      loadPlayback: jest.fn().mockResolvedValue(playback),
    };

    await Promise.all([
      loadMediaItem(client, 'arrival'),
      loadMediaItem(client, 'arrival'),
      loadMediaPlayback(client, 'arrival'),
      loadMediaPlayback(client, 'arrival'),
    ]);
    await loadMediaItem(client, 'arrival');
    await loadMediaPlayback(client, 'arrival');

    expect(client.loadItem).toHaveBeenCalledTimes(1);
    expect(client.loadPlayback).toHaveBeenCalledTimes(1);
  });

  it('does not retain failed or invalid requests', async () => {
    const client = {
      loadItem: jest
        .fn()
        .mockRejectedValueOnce(new Error('offline'))
        .mockResolvedValue(item),
      loadPlayback: jest.fn().mockResolvedValue(playback),
    };

    await expect(loadMediaItem(client, 'arrival')).rejects.toThrow('offline');
    await expect(loadMediaItem(client, 'arrival')).resolves.toBe(item);
    await loadMediaItem(client, 'bad\nitem');
    await loadMediaItem(client, 'bad\nitem');

    expect(client.loadItem).toHaveBeenCalledTimes(4);
  });

  it('refreshes expired playback plans', async () => {
    const now = jest.spyOn(Date, 'now').mockReturnValue(1000);
    const client = {
      loadItem: jest.fn().mockResolvedValue(item),
      loadPlayback: jest.fn().mockResolvedValue(playback),
    };

    await loadMediaPlayback(client, 'arrival');
    now.mockReturnValue(61_001);
    await loadMediaPlayback(client, 'arrival');

    expect(client.loadPlayback).toHaveBeenCalledTimes(2);
    now.mockRestore();
  });

  it('uses a validated item that was already loaded for a shelf', async () => {
    const client = {
      loadItem: jest.fn().mockResolvedValue(item),
      loadPlayback: jest.fn().mockResolvedValue(playback),
    };

    rememberMediaItems(client, [item, { ...item, id: 'bad\nitem' }]);

    await expect(loadMediaItem(client, 'arrival')).resolves.toBe(item);
    expect(client.loadItem).not.toHaveBeenCalled();
  });
});

it('exposes only resolved unexpired item snapshots for the current client', async () => {
  const client = mediaClient(),
    other = mediaClient();
  const now = jest.spyOn(Date, 'now').mockReturnValue(0);
  const pending = deferred<MediaItem>();
  client.loadItem.mockReturnValue(pending.promise);
  const loading = loadMediaItem(client, item.id);
  expect(readMediaItem(client, item.id)).toBeNull();
  pending.resolve(item);
  await loading;
  expect(readMediaItem(client, item.id)).toBe(item);
  expect(readMediaItem(other, item.id)).toBeNull();
  expect(readMediaItem(client, '')).toBeNull();
  now.mockReturnValue(60_000);
  expect(readMediaItem(client, item.id)).toBeNull();
  now.mockRestore();
});

it('exposes items supplied by a loaded shelf without waiting for a promise', () => {
  const client = mediaClient();
  rememberMediaItems(client, [item]);
  expect(readMediaItem(client, item.id)).toBe(item);
  expect(client.loadItem).not.toHaveBeenCalled();
});
