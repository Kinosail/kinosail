import { act, fireEvent, render, waitFor } from '@testing-library/react-native';
import { router, useLocalSearchParams } from 'expo-router';
import React from 'react';

import WatchScreen from '@/app/watch/[id]';
import type { MediaItem } from '@/core/contract';
import { pendingProgress, saveSyncedProgress } from '@/core/progress-sync';
import { KinosailClient } from '@/core/server-client';
import { useSession } from '@/core/session-context';
import {
  deferred,
  routeItem,
  routeSession,
  routeSource,
} from '@/testing/route-fixtures';

import { PlaybackView } from './playback-view';

jest.mock('expo-router', () => ({
  router: { back: jest.fn(), canGoBack: jest.fn(() => true), replace: jest.fn() },
  useLocalSearchParams: jest.fn(),
}));
jest.mock('@/core/session-context', () => ({ useSession: jest.fn() }));
jest.mock('@/core/progress-sync', () => ({
  pendingProgress: jest.fn(),
  saveSyncedProgress: jest.fn(),
}));
jest.mock('@/core/use-playback-experience', () => ({
  usePlaybackExperience: jest.fn(() => ({})),
}));
jest.mock('@/core/smart-downloads', () => ({ setPlayingDownload: jest.fn() }));
const loadMediaItem = jest.fn();
const loadMediaPlayback = jest.fn();
jest.mock('./playback-view', () => ({ PlaybackView: jest.fn(() => null) }));

let client: KinosailClient;
const playback = () => jest.mocked(PlaybackView).mock.calls.at(-1)![0];

describe('watch route', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    jest.spyOn(KinosailClient.prototype, 'loadItem').mockImplementation(loadMediaItem);
    jest.spyOn(KinosailClient.prototype, 'loadPlayback').mockImplementation(loadMediaPlayback);
    jest.mocked(pendingProgress).mockResolvedValue([]);
    jest.mocked(saveSyncedProgress).mockImplementation((activeClient, id, _title, _baseline, progress) => activeClient.saveProgress(id, progress));
    client = new KinosailClient('https://kino.example', 'viewer-token');
    jest.mocked(useLocalSearchParams).mockReturnValue({ id: 'arrival' });
    jest.mocked(useSession).mockReturnValue(routeSession(client));
    jest.mocked(loadMediaItem).mockResolvedValue(routeItem);
    jest.mocked(loadMediaPlayback).mockResolvedValue(routeSource);
  });
  afterEach(() => jest.restoreAllMocks());

  it.each(['session', 'id'])(
    'avoids playback requests without %s',
    async (missing) => {
      if (missing === 'session')
        jest.mocked(useSession).mockReturnValue(routeSession(null));
      else jest.mocked(useLocalSearchParams).mockReturnValue({});
      const view = await render(<WatchScreen />);
      await fireEvent.press(
        view.getByRole('button', { name: 'Return to details' }),
      );
      expect(router.back).toHaveBeenCalledTimes(1);
      expect(loadMediaItem).not.toHaveBeenCalled();
      expect(loadMediaPlayback).not.toHaveBeenCalled();
    },
  );

  it('loads item and source concurrently and saves progress for the active title', async () => {
    const pending = deferred<MediaItem>();
    jest.mocked(loadMediaItem).mockReturnValue(pending.promise);
    const save = jest
      .spyOn(client, 'saveProgress')
      .mockResolvedValue(routeItem.progress);
    const view = await render(<WatchScreen />);
    expect(view.getByText('Preparing direct playback…')).toBeTruthy();
    expect(loadMediaPlayback).toHaveBeenCalledWith('arrival');
    expect(PlaybackView).not.toHaveBeenCalled();
    await act(async () => {
      pending.resolve(routeItem);
    });
    expect(playback().item).toEqual(routeItem);
    expect(playback().source).toEqual(routeSource);
    const progress = {
      seconds: 24,
      session: 'native-test',
      revision: 2,
      playbackToken: 'progress-token',
    };
    await expect(playback().saveProgress(progress)).resolves.toEqual(
      routeItem.progress,
    );
    expect(save).toHaveBeenCalledWith('arrival', progress);
    playback().onBack();
    expect(router.back).toHaveBeenCalledTimes(1);
  });

  it.each([
    ['error', new Error('offline')],
    ['other', 'failure'],
  ])('offers recovery when preparation fails: %s', async (_name, reason) => {
    jest.mocked(loadMediaPlayback).mockRejectedValue(reason);
    const view = await render(<WatchScreen />);
    expect(
      await view.findByText(
        reason instanceof Error ? reason.message : 'Playback could not start.',
      ),
    ).toBeTruthy();
    await fireEvent.press(
      view.getByRole('button', { name: 'Return to details' }),
    );
    expect(router.back).toHaveBeenCalledTimes(1);
    expect(PlaybackView).not.toHaveBeenCalled();
  });

  it.each(['resolve', 'reject'] as const)(
    'discards late preparation %s after leaving',
    async (outcome) => {
      const pending = deferred<MediaItem>();
      jest.mocked(loadMediaItem).mockReturnValue(pending.promise);
      const view = await render(<WatchScreen />);
      await view.unmount();
      await act(async () => {
        if (outcome === 'resolve') pending.resolve(routeItem);
        else pending.reject(new Error('late failure'));
      });
      expect(PlaybackView).not.toHaveBeenCalled();
    },
  );

  it.each(['title', 'server'])(
    'never attaches the previous video when the %s changes',
    async (change) => {
      const originalSave = jest
        .spyOn(client, 'saveProgress')
        .mockResolvedValue(routeItem.progress);
      const view = await render(<WatchScreen />);
      await waitFor(() => expect(PlaybackView).toHaveBeenCalled());
      const pending = deferred<MediaItem>();
      jest.mocked(loadMediaItem).mockReturnValue(pending.promise);
      const nextID = change === 'title' ? 'moon' : 'arrival';
      const nextClient =
        change === 'server'
          ? new KinosailClient('https://second.example', 'second-token')
          : client;
      const save =
        nextClient === client
          ? originalSave
          : jest
              .spyOn(nextClient, 'saveProgress')
              .mockResolvedValue(routeItem.progress);
      if (change === 'title')
        jest.mocked(useLocalSearchParams).mockReturnValue({ id: nextID });
      else jest.mocked(useSession).mockReturnValue(routeSession(nextClient));
      jest.mocked(PlaybackView).mockClear();
      await view.rerender(<WatchScreen />);
      expect(view.getByText('Preparing direct playback…')).toBeTruthy();
      expect(PlaybackView).not.toHaveBeenCalled();
      await act(async () => {
        pending.resolve({ ...routeItem, id: nextID, title: 'Moon' });
      });
      expect(playback().item.id).toBe(nextID);
      await playback().saveProgress(routeItem.progress);
      expect(save).toHaveBeenCalledWith(nextID, routeItem.progress);
      if (nextClient !== client) expect(originalSave).not.toHaveBeenCalled();
    },
  );

  it.each(['resolve', 'reject'] as const)(
    'keeps the new video when the previous request finishes with %s',
    async (outcome) => {
      const previous = deferred<MediaItem>();
      jest.mocked(loadMediaItem).mockReturnValueOnce(previous.promise);
      const view = await render(<WatchScreen />);
      jest.mocked(useLocalSearchParams).mockReturnValue({ id: 'moon' });
      jest
        .mocked(loadMediaItem)
        .mockResolvedValue({ ...routeItem, id: 'moon' });
      await view.rerender(<WatchScreen />);
      await waitFor(() => expect(playback().item.id).toBe('moon'));
      const rendered = jest.mocked(PlaybackView).mock.calls.length;
      await act(async () => {
        if (outcome === 'resolve') previous.resolve(routeItem);
        else previous.reject(new Error('previous playback failed'));
      });
      expect(view.queryByText('Preparing direct playback…')).toBeNull();
      expect(view.queryByText('previous playback failed')).toBeNull();
      expect(PlaybackView).toHaveBeenCalledTimes(rendered);
      expect(playback().item.id).toBe('moon');
    },
  );

  it('does not carry a playback error into the next title', async () => {
    jest
      .mocked(loadMediaPlayback)
      .mockRejectedValueOnce(new Error('unavailable source'));
    const view = await render(<WatchScreen />);
    await view.findByText('unavailable source');
    jest.mocked(useLocalSearchParams).mockReturnValue({ id: 'moon' });
    await view.rerender(<WatchScreen />);
    await waitFor(() => expect(PlaybackView).toHaveBeenCalled());
    expect(view.queryByText('unavailable source')).toBeNull();
  });
});


it('can leave preparation without starting a late video', async () => {
  const pending = deferred<MediaItem>();
  jest.mocked(useLocalSearchParams).mockReturnValue({ id: 'arrival' });
  jest.mocked(useSession).mockReturnValue(routeSession(new KinosailClient('https://kino.example', 'viewer-token')));
  const itemSpy = jest.spyOn(KinosailClient.prototype, 'loadItem').mockReturnValue(pending.promise);
  const playbackSpy = jest.spyOn(KinosailClient.prototype, 'loadPlayback').mockResolvedValue(routeSource);
  jest.mocked(pendingProgress).mockResolvedValue([]);
  jest.mocked(PlaybackView).mockClear();
  jest.mocked(router.back).mockClear();
  const view = await render(<WatchScreen />);
  expect(view.getByRole('progressbar', { name: 'Preparing direct playback…' })).toBeTruthy();
  await fireEvent.press(view.getByRole('button', { name: 'Return to details' }));
  expect(router.back).toHaveBeenCalledTimes(1);
  await view.unmount();
  await act(() => pending.resolve(routeItem));
  expect(PlaybackView).not.toHaveBeenCalled();
  itemSpy.mockRestore();
  playbackSpy.mockRestore();
});

it('retries preparation in place after a transient failure', async () => {
  jest.mocked(useLocalSearchParams).mockReturnValue({ id: 'arrival' });
  jest.mocked(useSession).mockReturnValue(routeSession(new KinosailClient('https://kino.example', 'viewer-token')));
  const itemSpy = jest.spyOn(KinosailClient.prototype, 'loadItem').mockResolvedValue(routeItem);
  const playbackSpy = jest.spyOn(KinosailClient.prototype, 'loadPlayback')
    .mockRejectedValueOnce(new Error('Connection interrupted'))
    .mockResolvedValue(routeSource);
  jest.mocked(pendingProgress).mockResolvedValue([]);
  jest.mocked(PlaybackView).mockClear();
  jest.mocked(router.back).mockClear();
  jest.mocked(saveSyncedProgress).mockClear();
  const view = await render(<WatchScreen />);
  await view.findByText('Connection interrupted');
  await fireEvent.press(view.getByRole('button', { name: 'Try again' }));
  await waitFor(() => expect(PlaybackView).toHaveBeenCalled());
  expect(playback().source).toEqual(routeSource);
  expect(playbackSpy).toHaveBeenCalledTimes(2);
  expect(router.back).not.toHaveBeenCalled();
  expect(saveSyncedProgress).not.toHaveBeenCalled();
  await view.unmount();
  itemSpy.mockRestore();
  playbackSpy.mockRestore();
});


it('returns a directly opened watch screen to details without navigation history', async () => {
  jest.mocked(router.canGoBack).mockReturnValueOnce(false);
  jest.mocked(router.replace).mockClear();
  jest.mocked(useLocalSearchParams).mockReturnValue({ id: 'arrival' });
  jest.mocked(useSession).mockReturnValue(routeSession(new KinosailClient('https://kino.example', 'viewer-token')));
  const itemSpy = jest.spyOn(KinosailClient.prototype, 'loadItem').mockResolvedValue(routeItem);
  const playbackSpy = jest.spyOn(KinosailClient.prototype, 'loadPlayback').mockRejectedValue(new Error('offline'));
  jest.mocked(pendingProgress).mockResolvedValue([]);
  const view = await render(<WatchScreen />);
  await view.findByText('offline');
  await fireEvent.press(view.getByRole('button', { name: 'Return to details' }));
  expect(router.replace).toHaveBeenCalledWith({ pathname: '/item/[id]', params: { id: 'arrival' } });
  await view.unmount();
  itemSpy.mockRestore();
  playbackSpy.mockRestore();
});
