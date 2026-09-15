import {
  saveSyncedProgress,
  flushProgress,
  pendingProgress,
  parsePendingProgress,
} from './progress-sync';
import type { KinosailClient } from './server-client';
import type { Progress } from './contract';
import { readMediaState, writeMediaState } from './media-state-storage';
const mockState = new Map<string, string>();
jest.mock('./media-state-storage', () => ({
  readMediaState: jest.fn(async (_client, name) => mockState.get(name) ?? null),
  writeMediaState: jest.fn(async (_client, name, value) => {
    mockState.set(name, value);
  }),
}));
jest.mock('./downloads', () => ({
  acknowledgeDownloadedProgress: jest.fn(async () => {}),
}));
const zero: Progress = { seconds: 0, watched: false, session: '', revision: 0 };
const event = (seconds: number, revision = 1): Progress => ({
  seconds,
  watched: false,
  session: 'unique-session',
  revision,
});
describe('durable progress synchronization', () => {
  beforeEach(() => mockState.clear());
  it('persists before attempting the network and flushes later', async () => {
    const syncProgress = jest
      .fn()
      .mockImplementationOnce(async () => {
        expect(mockState.has('progress-outbox')).toBe(true);
        throw new Error('offline');
      })
      .mockResolvedValueOnce({ conflict: false, progress: event(10) });
    const client = { syncProgress } as unknown as KinosailClient;
    expect(
      await saveSyncedProgress(client, 'one', 'Title', zero, event(10)),
    ).toEqual(event(10));
    expect((await pendingProgress(client))[0].expected).toEqual(zero);
    await flushProgress(client);
    expect(await pendingProgress(client)).toEqual([]);
  });
  it('keeps the original server baseline while coalescing offline events', async () => {
    const client = {
      syncProgress: jest.fn(async () => {
        throw new Error('offline');
      }),
    } as unknown as KinosailClient;
    await saveSyncedProgress(client, 'one', 'Title', zero, event(10));
    await saveSyncedProgress(client, 'one', 'Title', event(10), event(20, 2));
    const [pending] = await pendingProgress(client);
    expect(pending.expected).toEqual(zero);
    expect(pending.progress).toEqual(event(20, 2));
  });
  it.each([50, 10])(
    'accepts server progress at %s when it is at least as far',
    async (seconds) => {
      const remote = {
        ...event(seconds),
        session: 'another-device',
        revision: 9,
      };
      const syncProgress = jest
        .fn()
        .mockResolvedValue({ conflict: true, progress: remote });
      const client = { syncProgress } as unknown as KinosailClient;
      expect(
        await saveSyncedProgress(client, 'one', 'Title', zero, event(10)),
      ).toEqual(remote);
      expect(syncProgress).toHaveBeenCalledTimes(1);
      expect(await pendingProgress(client)).toEqual([]);
    },
  );
  it('retries the furthest local position against the returned server baseline', async () => {
    const remote = { ...event(5), session: 'another-device', revision: 9 };
    const syncProgress = jest
      .fn()
      .mockResolvedValueOnce({ conflict: true, progress: remote })
      .mockResolvedValueOnce({ conflict: false, progress: event(10) });
    const client = { syncProgress } as unknown as KinosailClient;
    await saveSyncedProgress(client, 'one', 'Title', zero, event(10));
    expect(syncProgress.mock.calls[1][2]).toEqual(remote);
    expect(await pendingProgress(client)).toEqual([]);
  });
  it.each(['offline', 'race'])(
    'keeps local progress queued after a retry encounters %s',
    async (failure) => {
      const remote = { ...event(5), session: 'another-device' };
      const syncProgress = jest
        .fn()
        .mockResolvedValue({ conflict: true, progress: remote });
      if (failure === 'offline')
        syncProgress
          .mockResolvedValueOnce({ conflict: true, progress: remote })
          .mockRejectedValueOnce(new Error('offline'));
      const client = { syncProgress } as unknown as KinosailClient;
      expect(
        await saveSyncedProgress(client, 'one', 'Title', zero, event(10)),
      ).toEqual(event(10));
      expect(syncProgress).toHaveBeenCalledTimes(2);
      expect((await pendingProgress(client))[0].progress).toEqual(event(10));
      syncProgress.mockResolvedValue({ conflict: false, progress: event(10) });
      await flushProgress(client);
      expect(await pendingProgress(client)).toEqual([]);
    },
  );
  it('automatically flushes conflicts saved by older clients', async () => {
    mockState.set(
      'progress-outbox',
      JSON.stringify([
        {
          id: 'one',
          title: 'Title',
          expected: zero,
          progress: event(10),
          playbackToken: '',
          conflict: true,
        },
      ]),
    );
    const client = {
      syncProgress: jest
        .fn()
        .mockResolvedValue({ conflict: false, progress: event(10) }),
    } as unknown as KinosailClient;
    await flushProgress(client);
    expect(await pendingProgress(client)).toEqual([]);
  });
  it.each([
    null,
    {},
    [],
    { seconds: -1, watched: false, session: 's', revision: 1 },
    { seconds: 1, watched: false, session: 's', revision: 1, unknown: true },
  ])('rejects invalid imported progress %p', (progress) => {
    expect(() =>
      parsePendingProgress(
        JSON.stringify([
          {
            id: 'one',
            title: 'Title',
            expected: zero,
            progress,
            playbackToken: '',
            conflict: false,
          },
        ]),
      ),
    ).toThrow();
  });
  it('rejects duplicate and oversized imported queues', () => {
    const entry = {
      id: 'one',
      title: 'Title',
      expected: zero,
      progress: event(1),
      playbackToken: '',
      conflict: false,
    };
    expect(() =>
      parsePendingProgress(JSON.stringify([entry, entry])),
    ).toThrow();
    expect(() => parsePendingProgress(' '.repeat(262145))).toThrow();
  });
  it('rejects malformed inputs without storage or network side effects', () => {
    const client = { syncProgress: jest.fn() } as unknown as KinosailClient;
    expect(() =>
      saveSyncedProgress(client, 'one', 'Bad\nTitle', zero, event(1)),
    ).toThrow();
    expect(mockState.size).toBe(0);
    expect(client.syncProgress).not.toHaveBeenCalled();
  });
});

it('flushes only the requested title and preserves other queued progress', async () => {
  mockState.clear();
  const syncProgress = jest.fn().mockRejectedValue(new Error('offline'));
  const client = { syncProgress } as unknown as KinosailClient;
  await saveSyncedProgress(client, 'one', 'First', zero, event(10));
  await saveSyncedProgress(client, 'two', 'Second', zero, event(20));
  syncProgress.mockClear();
  syncProgress.mockResolvedValue({ conflict: false, progress: event(10) });
  await flushProgress(client, 'one');
  expect(syncProgress).toHaveBeenCalledTimes(1);
  expect(syncProgress.mock.calls[0][0]).toBe('one');
  expect((await pendingProgress(client)).map((entry) => entry.id)).toEqual([
    'two',
  ]);
});
it.each(['', 'bad\nID', 'x'.repeat(2049), null, 1, ['one']])(
  'rejects invalid scoped flush ID %p before any side effects',
  (id) => {
    jest.mocked(readMediaState).mockClear();
    jest.mocked(writeMediaState).mockClear();
    const client = { syncProgress: jest.fn() } as unknown as KinosailClient;
    expect(() => flushProgress(client, id as string)).toThrow();
    expect(readMediaState).not.toHaveBeenCalled();
    expect(writeMediaState).not.toHaveBeenCalled();
    expect(client.syncProgress).not.toHaveBeenCalled();
  },
);
