import React from 'react';
import { act, fireEvent, render, waitFor } from '@testing-library/react-native';
import { MusicLibrary } from './music-library';
import { albumCatalogCache, albumTracksCache } from '@/core/browse-cache';
import type { KinosailClient } from '@/core/server-client';
import { deferred, routeItem } from '@/testing/route-fixtures';
const mockPush = jest.fn();
const albums = [
  { id: 'one', title: 'First Album', artist: 'First Artist', artwork: '' },
  { id: 'two', title: 'Second Album', artist: 'Second Artist', artwork: '' },
];
const mockClient = {
  loadAlbums: jest.fn(),
  loadAlbum: jest.fn(),
  mediaURL: (path: string) => path,
  authorizationHeaders: () => ({}),
};
let mockActiveClient = mockClient;
jest.mock('expo-router', () => ({
  router: { push: (...args: unknown[]) => mockPush(...args) },
}));
jest.mock('@/core/session-context', () => ({
  useSession: () => ({ client: mockActiveClient }),
}));
beforeEach(() => {
  jest.clearAllMocks();
  mockActiveClient = mockClient;
  albumCatalogCache.clear(mockClient as unknown as KinosailClient);
  albumTracksCache.clear(mockClient as unknown as KinosailClient);
  mockClient.loadAlbums.mockResolvedValue(albums);
  mockClient.loadAlbum.mockResolvedValue({
    ...albums[0],
    tracks: [
      {
        ...routeItem,
        id: 'track-1',
        kind: 'music',
        title: 'Opening song',
        artist: 'First Artist',
      },
      {
        ...routeItem,
        id: 'track-2',
        kind: 'music',
        title: 'Closing song',
        artist: 'First Artist',
      },
    ],
  });
});
it('browses album tracks in server order and starts the selected song in its album', async () => {
  const view = await render(<MusicLibrary onSongs={jest.fn()} />);
  await fireEvent.press(
    await view.findByRole('button', { name: 'First Album, First Artist' }),
  );
  await fireEvent.press(
    await view.findByRole('button', {
      name: 'Play Closing song by First Artist',
    }),
  );
  expect(mockPush).toHaveBeenCalledWith({
    pathname: '/watch/[id]',
    params: {
      id: 'track-2',
      album: 'one',
      start: 'beginning',
    },
  });
});
it('filters artists and preserves a route to songs without album tags', async () => {
  const onSongs = jest.fn();
  const view = await render(<MusicLibrary onSongs={onSongs} />);
  await view.findByRole('button', { name: 'First Album, First Artist' });
  await fireEvent.changeText(
    view.getByLabelText('Search albums and artists'),
    'second',
  );
  expect(
    view.queryByRole('button', { name: 'First Album, First Artist' }),
  ).toBeNull();
  expect(
    view.getByRole('button', { name: 'Second Album, Second Artist' }),
  ).toBeTruthy();
  await fireEvent.press(view.getByRole('button', { name: 'Artists' }));
  await fireEvent.press(view.getByRole('button', { name: 'Second Artist' }));
  expect(
    view.getByRole('button', { name: 'Second Album, Second Artist' }),
  ).toBeTruthy();
  await fireEvent.press(view.getByRole('button', { name: 'Back' }));
  await fireEvent.press(view.getByRole('button', { name: 'Songs' }));
  expect(onSongs).toHaveBeenCalledTimes(1);
});
it('shows fetch errors and allows retry without starting playback', async () => {
  mockClient.loadAlbums.mockRejectedValueOnce(new Error('offline'));
  const view = await render(<MusicLibrary onSongs={jest.fn()} />);
  await fireEvent.press(await view.findByRole('button', { name: 'Try again' }));
  await waitFor(() =>
    expect(
      view.getByRole('button', { name: 'First Album, First Artist' }),
    ).toBeTruthy(),
  );
  expect(mockPush).not.toHaveBeenCalled();
});
it('does not offer playback for an empty album', async () => {
  mockClient.loadAlbum.mockResolvedValue({ ...albums[0], tracks: [] });
  const view = await render(<MusicLibrary onSongs={jest.fn()} />);
  await fireEvent.press(
    await view.findByRole('button', { name: 'First Album, First Artist' }),
  );
  await view.findByText('This album has no playable tracks.');
  await fireEvent.press(view.getByRole('button', { name: 'Play album' }));
  expect(mockPush).not.toHaveBeenCalled();
});

it('keeps the music controls visible while loading matching album and artist placeholders', async () => {
  const pending = deferred<typeof albums>();
  mockClient.loadAlbums.mockReturnValue(pending.promise);
  const view = await render(<MusicLibrary onSongs={jest.fn()} />);
  expect(view.getByLabelText('Search albums and artists')).toBeTruthy();
  expect(
    view.getByRole('progressbar', { name: 'Loading music…' }),
  ).toBeTruthy();
  expect(view.getAllByTestId('browse-skeleton-artwork').length).toBeGreaterThan(
    0,
  );
  await fireEvent.press(view.getByRole('button', { name: 'Artists' }));
  expect(view.queryAllByTestId('browse-skeleton-artwork')).toHaveLength(0);
  expect(view.getAllByTestId('browse-skeleton-row')).toHaveLength(6);
  await act(async () => pending.resolve(albums));
  expect(view.queryByRole('progressbar')).toBeNull();
});

it('shows track rows only while a selected album has no loaded tracks', async () => {
  const pending = deferred<{ tracks: (typeof routeItem)[] }>();
  mockClient.loadAlbum.mockReturnValue(pending.promise);
  const view = await render(<MusicLibrary onSongs={jest.fn()} />);
  await fireEvent.press(
    await view.findByRole('button', { name: 'First Album, First Artist' }),
  );
  expect(
    view.getByRole('progressbar', { name: 'Loading tracks…' }),
  ).toBeTruthy();
  expect(view.getAllByTestId('browse-skeleton-row')).toHaveLength(6);
  expect(view.queryAllByTestId('browse-skeleton-artwork')).toHaveLength(0);
  await act(async () => pending.resolve({ tracks: [] }));
  expect(view.queryByRole('progressbar')).toBeNull();
  expect(view.getByText('This album has no playable tracks.')).toBeTruthy();
});

it('returns to cached albums and tracks without skeletons while refreshing', async () => {
  const first = await render(<MusicLibrary onSongs={jest.fn()} />);
  await fireEvent.press(
    await first.findByRole('button', { name: 'First Album, First Artist' }),
  );
  await first.findByRole('button', {
    name: 'Play Opening song by First Artist',
  });
  await first.unmount();
  const catalog = deferred<typeof albums>(),
    tracks = deferred<{ tracks: (typeof routeItem)[] }>();
  mockClient.loadAlbums.mockReturnValue(catalog.promise);
  mockClient.loadAlbum.mockReturnValue(tracks.promise);
  const next = await render(<MusicLibrary onSongs={jest.fn()} />);
  expect(next.queryByRole('progressbar')).toBeNull();
  await fireEvent.press(
    next.getByRole('button', { name: 'First Album, First Artist' }),
  );
  expect(
    next.getByRole('button', { name: 'Play Opening song by First Artist' }),
  ).toBeTruthy();
  expect(next.queryByRole('progressbar')).toBeNull();
  await act(async () => {
    catalog.resolve(albums);
    tracks.resolve({ tracks: [] });
  });
  expect(next.getByText('This album has no playable tracks.')).toBeTruthy();
});

it('surfaces a failed refresh and clears its cached catalog', async () => {
  const first = await render(<MusicLibrary onSongs={jest.fn()} />);
  await first.findByRole('button', { name: 'First Album, First Artist' });
  await first.unmount();
  mockClient.loadAlbums.mockRejectedValueOnce(new Error('expired session'));
  const next = await render(<MusicLibrary onSongs={jest.fn()} />);
  await next.findByText('Could not load albums.');
  expect(next.queryByRole('progressbar')).toBeNull();
  expect(
    next.queryByRole('button', { name: 'First Album, First Artist' }),
  ).toBeNull();
  expect(
    albumCatalogCache.read(mockClient as unknown as KinosailClient, 'albums'),
  ).toBeNull();
});

it('does not show cached albums or tracks after the client changes', async () => {
  const view = await render(<MusicLibrary onSongs={jest.fn()} />);
  await fireEvent.press(
    await view.findByRole('button', { name: 'First Album, First Artist' }),
  );
  await view.findByRole('button', {
    name: 'Play Opening song by First Artist',
  });
  const pending = deferred<typeof albums>();
  mockActiveClient = {
    ...mockClient,
    loadAlbums: jest.fn().mockReturnValue(pending.promise),
  };
  await view.rerender(<MusicLibrary onSongs={jest.fn()} />);
  expect(
    view.queryByRole('button', { name: 'Play Opening song by First Artist' }),
  ).toBeNull();
  expect(
    view.queryByRole('button', { name: 'First Album, First Artist' }),
  ).toBeNull();
  expect(
    view.getByRole('progressbar', { name: 'Loading music…' }),
  ).toBeTruthy();
  await act(async () => pending.resolve([]));
  expect(view.queryByRole('progressbar')).toBeNull();
});
