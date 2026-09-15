import React from 'react';
import { render, waitFor } from '@testing-library/react-native';
import { downloadedPlayback } from '@/core/downloads';
import WatchScreen from '@/app/watch/[id]';
import { PlaybackView } from './playback-view';
import { routeItem, routeSource } from '@/testing/route-fixtures';
import type { KinosailClient } from '@/core/server-client';
import {
  pendingProgress,
  saveSyncedProgress,
  flushProgress,
} from '@/core/progress-sync';
let mockParams: Record<string, unknown>;
let mockClient: KinosailClient;
jest.mock('expo-router', () => ({
  router: { back: jest.fn(), canGoBack: jest.fn(() => true), replace: jest.fn() },
  useLocalSearchParams: () => mockParams,
}));
jest.mock('@/core/session-context', () => ({
  useSession: () => ({ client: mockClient }),
}));
jest.mock('@/core/use-playback-experience', () => ({
  usePlaybackExperience: jest.fn(() => ({})),
}));
jest.mock('@/core/smart-downloads', () => ({ setPlayingDownload: jest.fn() }));
jest.mock('@/core/progress-sync', () => ({
  pendingProgress: jest.fn(),
  flushProgress: jest.fn(),
  saveSyncedProgress: jest.fn(),
}));
jest.mock('@/core/downloads', () => ({
  downloadedPlayback: jest.fn(),
  saveDownloadedProgress: jest.fn(),
}));
jest.mock('./playback-view', () => ({ PlaybackView: jest.fn(() => null) }));
const playback = () => jest.mocked(PlaybackView).mock.calls.at(-1)![0];
beforeEach(() => {
  jest.clearAllMocks();
  mockParams = { id: 'arrival' };
  mockClient = {
    mediaURL: (path: string) => `https://kino.example${path}`,
    authorizationHeaders: () => ({ Authorization: 'Bearer test-token' }),
    loadItem: jest.fn().mockResolvedValue(routeItem),
    loadPlayback: jest.fn().mockResolvedValue(routeSource),
  } as unknown as KinosailClient;
  jest.mocked(pendingProgress).mockResolvedValue([]);
  jest
    .mocked(flushProgress)
    .mockImplementation(() => pendingProgress(mockClient));
});
it('keeps normal resume and starts only the explicit beginning request at zero', async () => {
  const view = await render(<WatchScreen />);
  await waitFor(() => expect(PlaybackView).toHaveBeenCalled());
  expect(playback().source.start).toBe(12);
  mockParams = { id: 'arrival', start: 'beginning' };
  await view.rerender(<WatchScreen />);
  await waitFor(() => expect(playback().source.start).toBe(0));
  expect(routeSource.start).toBe(12);
  expect(saveSyncedProgress).not.toHaveBeenCalled();
});
it('overrides queued resume for an explicit restart without deleting queued progress', async () => {
  jest.mocked(pendingProgress).mockResolvedValue([
    {
      id: 'arrival',
      playbackToken: routeSource.progressToken,
      progress: { ...routeItem.progress, seconds: 45 },
      conflict: false,
    },
  ] as Awaited<ReturnType<typeof pendingProgress>>);
  mockParams = { id: 'arrival', start: 'beginning' };
  await render(<WatchScreen />);
  await waitFor(() => expect(PlaybackView).toHaveBeenCalled());
  expect(playback().source.start).toBe(0);
  expect(saveSyncedProgress).not.toHaveBeenCalled();
});
it('honors an explicit restart even with a saved conflict', async () => {
  jest.mocked(pendingProgress).mockResolvedValue([
    {
      id: 'arrival',
      playbackToken: routeSource.progressToken,
      progress: routeItem.progress,
      conflict: true,
    },
  ] as Awaited<ReturnType<typeof pendingProgress>>);
  mockParams = { id: 'arrival', start: 'beginning' };
  const view = await render(<WatchScreen />);
  await waitFor(() => expect(PlaybackView).toHaveBeenCalled());
  expect(playback().source.start).toBe(0);
  expect(view.queryByText(/Choose which saved position/)).toBeNull();
  expect(saveSyncedProgress).not.toHaveBeenCalled();
});
it.each([
  '',
  '0',
  'resume',
  'BEGINNING',
  ' beginning ',
  null,
  ['beginning'],
  ['beginning', 'beginning'],
  'x'.repeat(4097),
])('rejects invalid start %p before playback work', async (start) => {
  mockParams = { id: 'arrival', start };
  const view = await render(<WatchScreen />);
  expect(
    view.getByText('This playback start option is not valid.'),
  ).toBeTruthy();
  expect(mockClient.loadItem).not.toHaveBeenCalled();
  expect(mockClient.loadPlayback).not.toHaveBeenCalled();
  expect(pendingProgress).not.toHaveBeenCalled();
  expect(saveSyncedProgress).not.toHaveBeenCalled();
  expect(PlaybackView).not.toHaveBeenCalled();
});

it('preserves progress when a restart source cannot load', async () => {
  mockParams = { id: 'arrival', start: 'beginning' };
  jest
    .mocked(mockClient.loadPlayback)
    .mockRejectedValueOnce(new Error('Source unavailable'));
  const view = await render(<WatchScreen />);
  await view.findByText('Source unavailable');
  expect(PlaybackView).not.toHaveBeenCalled();
  expect(saveSyncedProgress).not.toHaveBeenCalled();
});

it.each([
  { album: '' },
  { album: '../bad' },
  { album: ['album'] },
  { album: 'x'.repeat(129) },
  { shuffle: '1' },
  { album: 'album', shuffle: 'true' },
  { album: 'album', shuffle: ['1'] },
  { album: 'album', offline: '1' },
])(
  'rejects invalid music playback options before requests %#',
  async (extra) => {
    mockParams = { id: 'arrival', ...extra };
    await render(<WatchScreen />);
    expect(mockClient.loadItem).not.toHaveBeenCalled();
    expect(mockClient.loadPlayback).not.toHaveBeenCalled();
    expect(saveSyncedProgress).not.toHaveBeenCalled();
  },
);
it.each(['music', 'audiobook'])(
  'starts songs at zero while preserving audiobook resume for %s',
  async (kind) => {
    jest.mocked(mockClient.loadItem).mockResolvedValue({ ...routeItem, kind });
    await render(<WatchScreen />);
    await waitFor(() => expect(PlaybackView).toHaveBeenCalled());
    expect(playback().source.start).toBe(kind === 'music' ? 0 : 12);
    expect(saveSyncedProgress).not.toHaveBeenCalled();
  },
);

it('rejects an album playback request for a video before mounting playback', async () => {
  mockParams = { id: 'arrival', album: 'album' };
  const view = await render(<WatchScreen />);
  expect(
    await view.findByText('Album playback requires a music track.'),
  ).toBeTruthy();
  expect(PlaybackView).not.toHaveBeenCalled();
  expect(saveSyncedProgress).not.toHaveBeenCalled();
});

it.each([
  [true, 54, 54],
  [true, 5, 12],
  [false, 54, 54],
  [true, 12, 12],
])(
  'resumes automatically with conflict=%s and local seconds=%s',
  async (conflict, seconds, expected) => {
    jest.mocked(pendingProgress).mockResolvedValue([
      {
        id: 'arrival',
        title: 'Arrival',
        expected: routeItem.progress,
        progress: { ...routeItem.progress, seconds: seconds as number },
        playbackToken: 'old-token',
        conflict: conflict as boolean,
      },
    ]);
    const view = await render(<WatchScreen />);
    await waitFor(() => expect(PlaybackView).toHaveBeenCalled());
    expect(playback().source.start).toBe(expected);
    expect(view.queryByText(/Choose which saved position/)).toBeNull();
    expect(flushProgress).toHaveBeenCalledWith(mockClient, 'arrival');
  },
);

it('reloads the server resume point after automatically syncing pending progress', async () => {
  jest.mocked(pendingProgress).mockResolvedValue([
    {
      id: 'arrival',
      title: 'Arrival',
      expected: routeItem.progress,
      progress: { ...routeItem.progress, seconds: 45 },
      playbackToken: 'old-token',
      conflict: false,
    },
  ]);
  jest.mocked(flushProgress).mockImplementation(async () => {
    jest
      .mocked(mockClient.loadPlayback)
      .mockResolvedValue({ ...routeSource, start: 45 });
    return [];
  });
  await render(<WatchScreen />);
  await waitFor(() => expect(PlaybackView).toHaveBeenCalled());
  expect(playback().source.start).toBe(45);
});

it('plays a download from the furthest saved position without a network sync', async () => {
  mockParams = { id: 'arrival', offline: '1' };
  jest.mocked(downloadedPlayback).mockResolvedValue({
    item: routeItem,
    source: routeSource,
    baseline: routeItem.progress,
  });
  jest.mocked(pendingProgress).mockResolvedValue([
    {
      id: 'arrival',
      title: 'Arrival',
      expected: routeItem.progress,
      progress: { ...routeItem.progress, seconds: 3252 },
      playbackToken: 'old-token',
      conflict: true,
    },
  ]);
  await render(<WatchScreen />);
  await waitFor(() => expect(PlaybackView).toHaveBeenCalled());
  expect(playback().source.start).toBe(3252);
  expect(flushProgress).not.toHaveBeenCalled();
  expect(mockClient.loadItem).not.toHaveBeenCalled();
});
