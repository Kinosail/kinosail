import React from 'react';
import { act, fireEvent, render, waitFor } from '@testing-library/react-native';
import LibraryScreen from '@/app/library';
import { LibraryView } from '@/components/library-view';
import { libraryCache } from '@/core/browse-cache';
import type { KinosailClient } from '@/core/server-client';
const mockBrowse = jest
  .fn()
  .mockResolvedValue({ items: [], total: 0, offset: 0 });
let mockView: unknown;
let mockSearch: unknown;
const mockClient = {
  browseLibrary: mockBrowse,
  mediaURL: (path: string) => path,
  authorizationHeaders: () => ({}),
};
jest.mock('expo-router', () => ({
  useLocalSearchParams: () => ({ view: mockView, search: mockSearch }),
  router: { replace: jest.fn(), push: jest.fn() },
}));
jest.mock('@/core/session-context', () => ({
  useSession: () => ({ client: mockClient }),
}));
jest.mock('@/core/media-loader', () => ({ rememberMediaItems: jest.fn() }));
jest.mock('@/components/music-library', () => ({
  MusicLibrary: ({ onSongs }: { onSongs(): void }) => {
    const { Button } = require('react-native');
    return <Button title="Songs" onPress={onSongs} />;
  },
}));
jest.mock('@/components/library-view', () => ({
  LibraryView: jest.fn(() => null),
}));
beforeEach(() => {
  libraryCache.clear(mockClient as unknown as KinosailClient);
  jest.mocked(LibraryView).mockClear();
  mockBrowse.mockClear();
  mockSearch = undefined;
});
it.each([undefined, 'movies', 'shows', 'audiobooks', 'books'])(
  'requests the selected category %s',
  async (view) => {
    mockView = view;
    await render(<LibraryScreen />);
    await waitFor(() =>
      expect(mockBrowse).toHaveBeenCalledWith(
        { view: view ?? 'all', sort: 'title' },
        expect.anything(),
      ),
    );
  },
);
it.each([
  '',
  'unknown',
  'MOVIES',
  ' movies ',
  'x'.repeat(1025),
  ['movies'],
  ['movies', 'music'],
])('rejects malformed category %# without a library request', async (view) => {
  mockView = view;
  const screen = await render(<LibraryScreen />);
  expect(
    screen.getByText('The requested library category is invalid.'),
  ).toBeTruthy();
  expect(mockBrowse).not.toHaveBeenCalled();
});
it('resets the request when switching media categories', async () => {
  mockView = 'movies';
  const screen = await render(<LibraryScreen />);
  await waitFor(() => expect(mockBrowse).toHaveBeenCalled());
  mockView = 'books';
  await screen.rerender(<LibraryScreen />);
  await waitFor(() =>
    expect(mockBrowse).toHaveBeenLastCalledWith(
      { view: 'books', sort: 'title' },
      expect.anything(),
    ),
  );
});

it.each(['', '0', 'true', ['1'], ['1', '1'], 'x'.repeat(1025)])(
  'rejects malformed search activation %# before a library request',
  async (search) => {
    mockView = undefined;
    mockSearch = search;
    await render(<LibraryScreen />);
    expect(mockBrowse).not.toHaveBeenCalled();
  },
);
it('opens search with the supported flag and default category', async () => {
  mockView = undefined;
  mockSearch = '1';
  await render(<LibraryScreen />);
  await waitFor(() =>
    expect(mockBrowse).toHaveBeenCalledWith(
      { view: 'all', sort: 'title' },
      expect.anything(),
    ),
  );
});

it('starts music at the album browser and keeps all songs reachable', async () => {
  mockView = 'music';
  const screen = await render(<LibraryScreen />);
  expect(mockBrowse).not.toHaveBeenCalled();
  await fireEvent.press(screen.getByText('Songs'));
  await waitFor(() =>
    expect(mockBrowse).toHaveBeenCalledWith(
      { view: 'music', sort: 'title' },
      expect.anything(),
    ),
  );
});

it('shows a saved library page immediately while a return visit refreshes', async () => {
  mockView = 'movies';
  const page = { items: [], total: 42, offset: 0, limit: 60, letters: [] };
  mockBrowse.mockResolvedValueOnce(page);
  const first = await render(<LibraryScreen />);
  await waitFor(() =>
    expect(jest.mocked(LibraryView).mock.calls.at(-1)?.[0].total).toBe(42),
  );
  await first.unmount();
  let finish!: (result: typeof page) => void;
  mockBrowse.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  await render(<LibraryScreen />);
  const returned = jest.mocked(LibraryView).mock.calls.at(-1)![0];
  expect(returned.total).toBe(42);
  expect(returned.busy).toBe(false);
  await waitFor(() => expect(finish).toBeDefined());
  await act(async () => finish({ ...page, total: 43 }));
  await waitFor(() =>
    expect(jest.mocked(LibraryView).mock.calls.at(-1)?.[0].total).toBe(43),
  );
});

it('surfaces refresh failures instead of hiding them behind saved results', async () => {
  mockView = 'movies';
  libraryCache.write(
    mockClient as unknown as KinosailClient,
    'q=&view=movies&sort=title&offset=0&limit=60',
    { items: [], total: 42, offset: 0, limit: 60, letters: [] },
  );
  mockBrowse.mockRejectedValueOnce(
    new Error('This device session has expired.'),
  );
  await render(<LibraryScreen />);
  await waitFor(() =>
    expect(jest.mocked(LibraryView).mock.calls.at(-1)?.[0].error).toBe(
      'This device session has expired.',
    ),
  );
  expect(
    libraryCache.read(
      mockClient as unknown as KinosailClient,
      'q=&view=movies&sort=title&offset=0&limit=60',
    ),
  ).toBeNull();
});
