import { fireEvent, render, waitFor } from '@testing-library/react-native';
import { router, useLocalSearchParams } from 'expo-router';
import React from 'react';
import ShowScreen from '@/app/show/[id]';
import { KinosailClient } from '@/core/server-client';
import { useSession } from '@/core/session-context';
import { routeItem, routeSession } from '@/testing/route-fixtures';

jest.mock('expo-router', () => ({
  router: {
    push: jest.fn(),
    back: jest.fn(),
    canGoBack: jest.fn(),
    replace: jest.fn(),
  },
  useLocalSearchParams: jest.fn(),
}));
jest.mock('@/core/session-context', () => ({ useSession: jest.fn() }));
const id = 'a'.repeat(16);
let load: jest.SpyInstance;
beforeEach(() => {
  jest.clearAllMocks();
  const client = new KinosailClient('https://kino.example', 'viewer-token');
  jest.mocked(useSession).mockReturnValue(routeSession(client));
  jest.mocked(useLocalSearchParams).mockReturnValue({ id });
  load = jest.spyOn(client, 'loadShowEpisodes').mockResolvedValue([
    {
      ...routeItem,
      id: 'second',
      show: 'Example',
      showId: id,
      season: 2,
      episode: 1,
    },
    { ...routeItem, show: 'Example', showId: id, season: 1, episode: 2 },
  ]);
});
afterEach(() => jest.restoreAllMocks());
it.each([undefined, '', 'invalid', 'a'.repeat(17), ['a'.repeat(16)]])(
  'rejects invalid show id %p without a request',
  async (invalid) => {
    jest
      .mocked(useLocalSearchParams)
      .mockReturnValue({ id: invalid } as ReturnType<
        typeof useLocalSearchParams
      >);
    const view = await render(<ShowScreen />);
    expect(view.getByText('This show is not available.')).toBeTruthy();
    expect(load).not.toHaveBeenCalled();
  },
);
it('groups episodes into seasons and opens episode details', async () => {
  const view = await render(<ShowScreen />);
  await waitFor(() => expect(view.getByText('Season 1')).toBeTruthy());
  expect(view.getByText('Season 2')).toBeTruthy();
  await fireEvent.press(
    view.getByRole('button', { name: `E2 · ${routeItem.title}` }),
  );
  expect(router.push).toHaveBeenCalledWith({
    pathname: '/item/[id]',
    params: { id: routeItem.id },
  });
});
it('offers retry after a failed request', async () => {
  load.mockRejectedValueOnce(new Error('unavailable'));
  const view = await render(<ShowScreen />);
  await waitFor(() =>
    expect(view.getByText('Could not load this show.')).toBeTruthy(),
  );
  await fireEvent.press(view.getByRole('button', { name: 'Try again' }));
  await waitFor(() => expect(view.getByText('Season 1')).toBeTruthy());
});

it('lets TV viewers resume directly and choose a season without traversing every episode', async () => {
  const { Platform } =
    jest.requireActual<typeof import('react-native')>('react-native');
  const descriptor = Object.getOwnPropertyDescriptor(Platform, 'isTV')!;
  Object.defineProperty(Platform, 'isTV', { configurable: true, value: true });
  try {
    const view = await render(<ShowScreen />);
    await fireEvent.press(
      await view.findByRole('button', { name: 'Resume S2 E1' }),
    );
    expect(router.push).toHaveBeenCalledWith({
      pathname: '/watch/[id]',
      params: { id: 'second' },
    });
    await fireEvent.press(view.getByRole('button', { name: 'Season 1' }));
    expect(
      view.getByRole('button', { name: `E2 · ${routeItem.title}` }),
    ).toBeTruthy();
    expect(
      view.queryByRole('button', { name: `E1 · ${routeItem.title}` }),
    ).toBeNull();
    await view.unmount();
  } finally {
    Object.defineProperty(Platform, 'isTV', descriptor);
  }
});
