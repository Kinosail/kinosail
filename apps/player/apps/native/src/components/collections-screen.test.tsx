import React from 'react';
import { act, fireEvent, render, waitFor } from '@testing-library/react-native';
import { router, useLocalSearchParams } from 'expo-router';
import CollectionsScreen from '@/app/collections';
import { KinosailClient } from '@/core/server-client';
import { useSession } from '@/core/session-context';
import { readMediaItem } from '@/core/media-loader';
import { deferred, routeItem, routeSession } from '@/testing/route-fixtures';
import { LibraryView } from './library-view';

jest.mock('expo-router', () => ({
  router: { replace: jest.fn(), push: jest.fn() },
  useLocalSearchParams: jest.fn(),
}));
jest.mock('@/core/session-context', () => ({ useSession: jest.fn() }));
jest.mock('./library-view', () => ({ LibraryView: jest.fn(() => null) }));
let client: KinosailClient;
beforeEach(() => {
  jest.clearAllMocks();
  client = new KinosailClient('https://kino.example', 'viewer');
  jest.mocked(useSession).mockReturnValue(routeSession(client));
  jest.mocked(useLocalSearchParams).mockReturnValue({});
});
afterEach(() => jest.restoreAllMocks());
it('searches and opens collection names', async () => {
  jest.spyOn(client, 'loadCollections').mockResolvedValue(['Family', 'Films']);
  const view = await render(<CollectionsScreen />);
  await waitFor(() =>
    expect(view.getByRole('button', { name: 'Family' })).toBeTruthy(),
  );
  await fireEvent.changeText(
    view.getByLabelText('Search collections'),
    'Family',
  );
  expect(view.queryByRole('button', { name: 'Films' })).toBeNull();
  await fireEvent.press(view.getByRole('button', { name: 'Family' }));
  expect(router.push).toHaveBeenCalledWith({
    pathname: '/collections',
    params: { name: 'Family' },
  });
});
it('paginates collection members and opens their normal detail route', async () => {
  jest.mocked(useLocalSearchParams).mockReturnValue({ name: 'Family' });
  const items = Array.from({ length: 61 }, (_, id) => ({
    ...routeItem,
    id: String(id),
  }));
  jest.spyOn(client, 'loadCollection').mockResolvedValue(items);
  await render(<CollectionsScreen />);
  await waitFor(() =>
    expect(jest.mocked(LibraryView).mock.calls.at(-1)?.[0].busy).toBe(false),
  );
  const props = jest.mocked(LibraryView).mock.calls.at(-1)![0];
  expect(props.items).toHaveLength(60);
  expect(props.hasMore).toBe(true);
  expect(props.title).toBe('Family');
  props.onOpen('0');
  expect(router.push).toHaveBeenCalledWith({
    pathname: '/item/[id]',
    params: { id: '0' },
  });
  props.onBack();
  expect(router.replace).toHaveBeenCalledWith('/collections');
});
it.each([[['A', 'B']], ['../'], ['']])(
  'rejects invalid route names without requesting collections: %p',
  async (name) => {
    jest.mocked(useLocalSearchParams).mockReturnValue({ name });
    const request = jest.spyOn(client, 'loadCollection');
    const view = await render(<CollectionsScreen />);
    expect(view.getByText('This collection is invalid.')).toBeTruthy();
    expect(request).not.toHaveBeenCalled();
  },
);
it('recovers from a collection request failure', async () => {
  const request = jest
    .spyOn(client, 'loadCollections')
    .mockRejectedValueOnce(new Error('Offline'))
    .mockResolvedValueOnce([]);
  const view = await render(<CollectionsScreen />);
  await waitFor(() => expect(view.getByText('Offline')).toBeTruthy());
  await fireEvent.press(view.getByRole('button', { name: 'Try again' }));
  await waitFor(() =>
    expect(view.getByText('No collections yet.')).toBeTruthy(),
  );
  expect(request).toHaveBeenCalledTimes(2);
});

it('keeps the collection heading and search in place during a cold load', async () => {
  const pending = deferred<string[]>();
  jest.spyOn(client, 'loadCollections').mockReturnValue(pending.promise);
  const view = await render(<CollectionsScreen />);
  expect(view.getByRole('header', { name: 'Collections' })).toBeTruthy();
  expect(view.getByLabelText('Search collections')).toBeTruthy();
  expect(
    view.getByRole('progressbar', { name: 'Loading collections…' }),
  ).toBeTruthy();
  expect(view.queryByText('No collections yet.')).toBeNull();
  await act(async () => pending.resolve([]));
  expect(view.queryByRole('progressbar')).toBeNull();
  expect(view.getByText('No collections yet.')).toBeTruthy();
});

it('uses the collection grid during loading and primes visible detail items', async () => {
  jest.mocked(useLocalSearchParams).mockReturnValue({ name: 'Family' });
  const pending = deferred<(typeof routeItem)[]>();
  jest.spyOn(client, 'loadCollection').mockReturnValue(pending.promise);
  await render(<CollectionsScreen />);
  expect(jest.mocked(LibraryView).mock.calls.at(-1)![0]).toMatchObject({
    title: 'Family',
    items: [],
    busy: true,
  });
  await act(async () => pending.resolve([routeItem]));
  expect(jest.mocked(LibraryView).mock.calls.at(-1)![0]).toMatchObject({
    items: [routeItem],
    busy: false,
  });
  expect(readMediaItem(client, routeItem.id)).toBe(routeItem);
});

it('reuses collection names immediately while refreshing and isolates replacement sessions', async () => {
  const load = jest
    .spyOn(client, 'loadCollections')
    .mockResolvedValue(['Family']);
  const first = await render(<CollectionsScreen />);
  await first.findByRole('button', { name: 'Family' });
  await first.unmount();
  const pending = deferred<string[]>();
  load.mockReturnValue(pending.promise);
  const next = await render(<CollectionsScreen />);
  expect(next.getByRole('button', { name: 'Family' })).toBeTruthy();
  expect(next.queryByRole('progressbar')).toBeNull();
  const other = new KinosailClient('https://kino.example', 'another-viewer');
  const otherLoad = deferred<string[]>();
  jest.spyOn(other, 'loadCollections').mockReturnValue(otherLoad.promise);
  jest.mocked(useSession).mockReturnValue(routeSession(other));
  await next.rerender(<CollectionsScreen />);
  expect(next.queryByRole('button', { name: 'Family' })).toBeNull();
  expect(next.getByRole('progressbar')).toBeTruthy();
  await act(async () => {
    pending.resolve(['Late old result']);
    otherLoad.resolve(['New collection']);
  });
  expect(next.queryByRole('button', { name: 'Late old result' })).toBeNull();
  expect(next.getByRole('button', { name: 'New collection' })).toBeTruthy();
});
