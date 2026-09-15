import { act, fireEvent, render, waitFor } from '@testing-library/react-native';
import { router, useLocalSearchParams } from 'expo-router';
import React from 'react';

import ItemScreen from '@/app/item/[id]';
import type { MediaItem } from '@/core/contract';
import {
  loadMediaItem,
  loadMediaPlayback,
  readMediaItem,
} from '@/core/media-loader';
import { KinosailClient } from '@/core/server-client';
import { useSession } from '@/core/session-context';
import {
  deferred,
  routeItem,
  routeSession,
  routeSource,
} from '@/testing/route-fixtures';

import { DetailView } from './detail-view';
import { PhotoView } from './photo-view';
jest.mock('./photo-view', () => ({ PhotoView: jest.fn(() => null) }));

jest.mock('expo-router', () => ({
  router: {
    back: jest.fn(),
    push: jest.fn(),
    canGoBack: jest.fn(),
    replace: jest.fn(),
  },
  useLocalSearchParams: jest.fn(),
}));
jest.mock('@/core/session-context', () => ({ useSession: jest.fn() }));
jest.mock('@/core/media-loader', () => ({
  readMediaItem: jest.fn(),
  loadMediaItem: jest.fn(),
  loadMediaPlayback: jest.fn(),
}));
jest.mock('./detail-view', () => ({ DetailView: jest.fn(() => null) }));

let client: KinosailClient;
const details = () => jest.mocked(DetailView).mock.calls.at(-1)![0];

describe('item route', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    jest.mocked(readMediaItem).mockReturnValue(null);
    jest.mocked(router.canGoBack).mockReturnValue(true);
    client = new KinosailClient('https://kino.example', 'viewer-token');
    jest.mocked(useLocalSearchParams).mockReturnValue({ id: 'arrival' });
    jest.mocked(useSession).mockReturnValue(routeSession(client));
    jest.mocked(loadMediaItem).mockResolvedValue(routeItem);
    jest.mocked(loadMediaPlayback).mockResolvedValue(routeSource);
  });
  afterEach(() => jest.restoreAllMocks());

  it('renders cached details before the asynchronous loader completes', async () => {
    jest.mocked(readMediaItem).mockReturnValue(routeItem);
    const pending = deferred<MediaItem>();
    jest.mocked(loadMediaItem).mockReturnValue(pending.promise);
    const view = await render(<ItemScreen />);
    expect(details().item).toBe(routeItem);
    expect(view.queryByRole('progressbar')).toBeNull();
    expect(readMediaItem).toHaveBeenCalledWith(client, 'arrival');
    await act(async () => pending.resolve(routeItem));
  });

  it.each(['session', 'id'])(
    'offers recovery without requests when %s is missing',
    async (missing) => {
      if (missing === 'session')
        jest.mocked(useSession).mockReturnValue(routeSession(null));
      else jest.mocked(useLocalSearchParams).mockReturnValue({});
      const view = await render(<ItemScreen />);
      expect(view.getByText('This media item is not available.')).toBeTruthy();
      await fireEvent.press(view.getByRole('button', { name: 'Go back' }));
      expect(router.back).toHaveBeenCalledTimes(1);
      expect(loadMediaItem).not.toHaveBeenCalled();
    },
  );

  it('opens photos without requesting playback or exposing video actions', async () => {
    jest
      .mocked(loadMediaItem)
      .mockResolvedValue({ ...routeItem, kind: 'photo' });
    await render(<ItemScreen />);
    await waitFor(() => expect(PhotoView).toHaveBeenCalled());
    expect(loadMediaPlayback).not.toHaveBeenCalled();
    expect(DetailView).not.toHaveBeenCalled();
    expect(jest.mocked(PhotoView).mock.calls.at(-1)![0]).toMatchObject({
      uri: 'https://kino.example/media/arrival',
      headers: client.authorizationHeaders(),
    });
  });

  it('opens the parent show from an episode', async () => {
    const showId = 'a'.repeat(16);
    jest
      .mocked(loadMediaItem)
      .mockResolvedValue({ ...routeItem, show: 'Example', showId });
    await render(<ItemScreen />);
    await waitFor(() => expect(DetailView).toHaveBeenCalled());
    details().onShow?.();
    expect(router.push).toHaveBeenCalledWith({
      pathname: '/show/[id]',
      params: { id: showId },
    });
  });

  it('does not offer a show destination for a movie', async () => {
    await render(<ItemScreen />);
    await waitFor(() => expect(DetailView).toHaveBeenCalled());
    expect(details().onShow).toBeUndefined();
  });

  it('passes an explicit beginning choice to playback', async () => {
    await render(<ItemScreen />);
    await waitFor(() => expect(DetailView).toHaveBeenCalled());
    details().onPlayFromBeginning?.();
    expect(router.push).toHaveBeenCalledWith({
      pathname: '/watch/[id]',
      params: { id: 'arrival', start: 'beginning' },
    });
  });

  it('loads details before prefetch and wires authenticated navigation', async () => {
    const pending = deferred<MediaItem>();
    jest.mocked(loadMediaItem).mockReturnValue(pending.promise);
    let frame: FrameRequestCallback = () => {};
    jest
      .spyOn(globalThis, 'requestAnimationFrame')
      .mockImplementation((callback) => {
        frame = callback;
        return 17;
      });
    const cancel = jest.spyOn(globalThis, 'cancelAnimationFrame');
    const view = await render(<ItemScreen />);
    expect(view.getByText('Loading details…')).toBeTruthy();
    expect(loadMediaPlayback).not.toHaveBeenCalled();
    await act(async () => {
      pending.resolve(routeItem);
    });
    expect(details().item).toEqual(routeItem);
    expect(details().headers).toEqual({ Authorization: 'Bearer viewer-token' });
    expect(details().mediaURL('/art/arrival')).toBe(
      'https://kino.example/art/arrival',
    );
    details().onBack();
    details().onPlay();
    expect(router.back).toHaveBeenCalledTimes(1);
    expect(router.push).toHaveBeenCalledWith({
      pathname: '/watch/[id]',
      params: { id: 'arrival' },
    });
    jest
      .mocked(loadMediaPlayback)
      .mockRejectedValueOnce(new Error('prefetch unavailable'));
    await act(async () => {
      frame(0);
    });
    expect(loadMediaPlayback).toHaveBeenCalledWith(client, 'arrival');
    expect(view.queryByText('prefetch unavailable')).toBeNull();
    await view.unmount();
    expect(cancel).toHaveBeenCalledWith(17);
    jest.mocked(loadMediaPlayback).mockClear();
    await act(async () => {
      frame(0);
    });
    expect(loadMediaPlayback).not.toHaveBeenCalled();
  });

  it.each([
    ['error', new Error('offline')],
    ['other', 'failure'],
  ])('shows recoverable load errors: %s', async (_name, reason) => {
    jest.mocked(loadMediaItem).mockRejectedValue(reason);
    const view = await render(<ItemScreen />);
    expect(
      await view.findByText(
        reason instanceof Error ? reason.message : 'Could not load this title.',
      ),
    ).toBeTruthy();
    await fireEvent.press(view.getByRole('button', { name: 'Go back' }));
    expect(router.back).toHaveBeenCalledTimes(1);
  });

  it.each(['resolve', 'reject'] as const)(
    'ignores stale load %s after unmount',
    async (outcome) => {
      const pending = deferred<MediaItem>();
      jest.mocked(loadMediaItem).mockReturnValue(pending.promise);
      const cancel = jest.spyOn(globalThis, 'cancelAnimationFrame');
      const view = await render(<ItemScreen />);
      await view.unmount();
      expect(cancel).not.toHaveBeenCalled();
      await act(async () => {
        if (outcome === 'resolve') pending.resolve(routeItem);
        else pending.reject(new Error('late failure'));
      });
      expect(DetailView).not.toHaveBeenCalled();
      expect(loadMediaPlayback).not.toHaveBeenCalled();
    },
  );

  it.each(['resolve', 'reject'] as const)(
    'keeps new details when the previous request finishes with %s',
    async (outcome) => {
      const previous = deferred<MediaItem>();
      jest.mocked(loadMediaItem).mockReturnValueOnce(previous.promise);
      const view = await render(<ItemScreen />);
      jest.mocked(useLocalSearchParams).mockReturnValue({ id: 'moon' });
      jest
        .mocked(loadMediaItem)
        .mockResolvedValue({ ...routeItem, id: 'moon' });
      await view.rerender(<ItemScreen />);
      await waitFor(() => expect(details().item.id).toBe('moon'));
      const rendered = jest.mocked(DetailView).mock.calls.length;
      await act(async () => {
        if (outcome === 'resolve') previous.resolve(routeItem);
        else previous.reject(new Error('previous title failed'));
      });
      expect(view.queryByText('Loading details…')).toBeNull();
      expect(view.queryByText('previous title failed')).toBeNull();
      expect(DetailView).toHaveBeenCalledTimes(rendered);
      expect(details().item.id).toBe('moon');
    },
  );

  it.each(['title', 'server'])(
    'clears the previous title when the %s changes',
    async (change) => {
      const view = await render(<ItemScreen />);
      await waitFor(() => expect(DetailView).toHaveBeenCalled());
      const pending = deferred<MediaItem>();
      jest.mocked(loadMediaItem).mockReturnValue(pending.promise);
      const nextID = change === 'title' ? 'moon' : 'arrival';
      if (change === 'title')
        jest.mocked(useLocalSearchParams).mockReturnValue({ id: nextID });
      else
        jest
          .mocked(useSession)
          .mockReturnValue(
            routeSession(
              new KinosailClient('https://second.example', 'second-token'),
            ),
          );
      jest.mocked(DetailView).mockClear();
      await view.rerender(<ItemScreen />);
      expect(view.getByText('Loading details…')).toBeTruthy();
      expect(DetailView).not.toHaveBeenCalled();
      await act(async () => {
        pending.resolve({ ...routeItem, id: nextID, title: 'Moon' });
      });
      expect(details().item.title).toBe('Moon');
    },
  );

  it('does not carry an error into the next title', async () => {
    jest
      .mocked(loadMediaItem)
      .mockRejectedValueOnce(new Error('missing title'));
    const view = await render(<ItemScreen />);
    await view.findByText('missing title');
    jest.mocked(useLocalSearchParams).mockReturnValue({ id: 'moon' });
    await view.rerender(<ItemScreen />);
    await waitFor(() => expect(DetailView).toHaveBeenCalled());
    expect(view.queryByText('missing title')).toBeNull();
  });
  it('returns a directly opened detail screen to the library', async () => {
    jest.mocked(router.canGoBack).mockReturnValue(false);
    await render(<ItemScreen />);
    await waitFor(() => expect(DetailView).toHaveBeenCalled());
    details().onBack();
    expect(router.replace).toHaveBeenCalledWith('/');
    expect(router.back).not.toHaveBeenCalled();
  });

  it('recovers a missing deep-linked item without a history entry', async () => {
    jest.mocked(router.canGoBack).mockReturnValue(false);
    jest.mocked(useLocalSearchParams).mockReturnValue({});
    const view = await render(<ItemScreen />);
    await fireEvent.press(view.getByRole('button', { name: 'Go back' }));
    expect(router.replace).toHaveBeenCalledWith('/');
    expect(router.back).not.toHaveBeenCalled();
    expect(loadMediaItem).not.toHaveBeenCalled();
  });
});
