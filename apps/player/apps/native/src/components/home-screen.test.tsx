import { act, fireEvent, render } from '@testing-library/react-native';
import { router } from 'expo-router';
import React from 'react';

import HomeScreen from '@/app/index';
import type { Home } from '@/core/contract';
import * as mediaLoader from '@/core/media-loader';
import { KinosailClient } from '@/core/server-client';
import { useSession } from '@/core/session-context';
import { useThemePreference } from '@/design/theme-context';
import { deferred, routeItem, routeSession } from '@/testing/route-fixtures';

import { HomeView } from './home-view';
import { SetupFlow } from './setup-flow';

jest.mock('expo-router', () => ({ router: { push: jest.fn() } }));
jest.mock('@/core/session-context', () => ({ useSession: jest.fn() }));
jest.mock('./home-view', () => ({ HomeView: jest.fn(() => null) }));
jest.mock('./setup-flow', () => ({ SetupFlow: jest.fn(() => null) }));
jest.mock('@/design/theme-context', () => ({ useThemePreference: jest.fn() }));

const home: Home = {
  server: 'First server',
  viewer: { id: 'viewer', name: 'First viewer', owner: false },
  continueWatching: [routeItem],
  recent: [routeItem],
};

describe('home session isolation', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    jest.spyOn(mediaLoader, 'rememberMediaItems');
    jest.mocked(useThemePreference).mockReturnValue({
      preference: 'light',
      scheme: 'light',
      cycle: jest.fn(),
    });
  });
  afterEach(() => jest.restoreAllMocks());

  it('shows boot progress and retries a failed saved session', async () => {
    const session = { ...routeSession(null), booting: true };
    jest.mocked(useSession).mockReturnValue(session);
    const view = await render(<HomeScreen />);
    expect(view.getByText('Opening your player…')).toBeTruthy();
    expect(SetupFlow).not.toHaveBeenCalled();
    jest.mocked(useSession).mockReturnValue({
      ...session,
      booting: false,
      bootError: 'Storage unavailable',
    });
    await view.rerender(<HomeScreen />);
    expect(view.getByText('Storage unavailable')).toBeTruthy();
    await fireEvent.press(view.getByRole('button', { name: 'Try again' }));
    expect(session.retryBoot).toHaveBeenCalledTimes(1);
  });

  it.each(['session', 'client'])(
    'offers setup when %s is missing',
    async (missing) => {
      const client = new KinosailClient('https://kino.example');
      jest.spyOn(client, 'loadHome').mockReturnValue(new Promise(() => {}));
      const session = routeSession(client);
      jest.mocked(useSession).mockReturnValue({ ...session, [missing]: null });
      await render(<HomeScreen />);
      const props = jest.mocked(SetupFlow).mock.calls.at(-1)![0];
      expect(props.onConnected).toBe(session.connect);
      const next = props.createClient('https://new.example');
      expect(next).toBeInstanceOf(KinosailClient);
      expect(next).toMatchObject({ baseURL: 'https://new.example' });
    },
  );

  it('wires library artwork, navigation, theme and sign-out to the current session', async () => {
    const client = new KinosailClient('https://kino.example', 'viewer-token');
    jest.spyOn(client, 'loadHome').mockResolvedValue(home);
    const session = routeSession(client);
    jest.mocked(useSession).mockReturnValue(session);
    await render(<HomeScreen />);
    const props = jest.mocked(HomeView).mock.calls.at(-1)![0];
    expect(props.headers).toEqual({ Authorization: 'Bearer viewer-token' });
    expect(props.mediaURL('/art/arrival')).toBe(
      'https://kino.example/art/arrival',
    );
    props.onOpen('arrival');
    expect(router.push).toHaveBeenCalledWith({
      pathname: '/item/[id]',
      params: { id: 'arrival' },
    });
    expect(props.themeLabel).toBe('Light');
    expect(mediaLoader.rememberMediaItems).toHaveBeenCalledWith(client, [
      ...home.recent,
      ...home.continueWatching,
    ]);
    expect(props.onTheme).toBeDefined();
    props.onTheme!();
    expect(useThemePreference().cycle).toHaveBeenCalledTimes(1);
    expect(props.onSignOut).toBeDefined();
    props.onSignOut!();
    expect(session.signOut).toHaveBeenCalledTimes(1);
  });

  it('keeps cached home content visible during remount and manual refresh', async () => {
    const client = new KinosailClient('https://kino.example', 'viewer-token');
    const load = jest.spyOn(client, 'loadHome').mockResolvedValue(home);
    jest.mocked(useSession).mockReturnValue(routeSession(client));
    const first = await render(<HomeScreen />);
    await first.unmount();
    const pending = deferred<Home>();
    load.mockReturnValue(pending.promise);
    const second = await render(<HomeScreen />);
    expect(jest.mocked(HomeView).mock.calls.at(-1)![0].home).toBe(home);
    expect(second.queryByRole('progressbar')).toBeNull();
    await act(async () => pending.resolve(home));
    const refreshed = deferred<Home>();
    load.mockReturnValue(refreshed.promise);
    await act(async () =>
      jest.mocked(HomeView).mock.calls.at(-1)![0].onRefresh!(),
    );
    expect(second.queryByRole('progressbar')).toBeNull();
    expect(jest.mocked(HomeView).mock.calls.at(-1)![0].home).toBe(home);
    await act(async () => refreshed.resolve({ ...home, server: 'Refreshed' }));
    expect(jest.mocked(HomeView).mock.calls.at(-1)![0].home.server).toBe(
      'Refreshed',
    );
  });

  it('clears each failed attempt and supports repeated retries', async () => {
    const client = new KinosailClient('https://kino.example', 'viewer-token');
    const retry = deferred<Home>();
    const load = jest
      .spyOn(client, 'loadHome')
      .mockRejectedValueOnce('unstructured failure')
      .mockReturnValueOnce(retry.promise)
      .mockResolvedValue(home);
    jest.mocked(useSession).mockReturnValue(routeSession(client));
    const view = await render(<HomeScreen />);
    expect(view.getByText('Could not load the library.')).toBeTruthy();
    await fireEvent.press(view.getByRole('button', { name: 'Try again' }));
    expect(load).toHaveBeenCalledTimes(2);
    expect(view.queryByText('Could not load the library.')).toBeNull();
    expect(view.getByText('Loading your library…')).toBeTruthy();
    await act(async () => retry.reject(new Error('Still offline')));
    expect(view.getByText('Still offline')).toBeTruthy();
    await fireEvent.press(view.getByRole('button', { name: 'Try again' }));
    expect(load).toHaveBeenCalledTimes(3);
    expect(view.queryByText('Still offline')).toBeNull();
    expect(jest.mocked(HomeView).mock.calls.at(-1)![0].home).toBe(home);
  });

  it.each(['resolve', 'reject'] as const)(
    'ignores late load %s after unmount',
    async (outcome) => {
      const client = new KinosailClient('https://kino.example');
      const load = deferred<Home>();
      jest.spyOn(client, 'loadHome').mockReturnValue(load.promise);
      jest.mocked(useSession).mockReturnValue(routeSession(client));
      const view = await render(<HomeScreen />);
      await view.unmount();
      await act(async () => {
        if (outcome === 'resolve') load.resolve(home);
        else load.reject(new Error('late failure'));
      });
      expect(HomeView).not.toHaveBeenCalled();
      expect(mediaLoader.rememberMediaItems).not.toHaveBeenCalled();
    },
  );

  it('does not show the previous session library while a new session loads', async () => {
    const first = new KinosailClient('https://first.example', 'first-token');
    jest.spyOn(first, 'loadHome').mockResolvedValue(home);
    jest.mocked(useSession).mockReturnValue(routeSession(first));
    const view = await render(<HomeScreen />);
    expect(jest.mocked(HomeView).mock.calls.at(-1)![0].home).toBe(home);
    const second = new KinosailClient('https://second.example', 'second-token');
    const load = deferred<Home>();
    jest.spyOn(second, 'loadHome').mockReturnValue(load.promise);
    jest.mocked(useSession).mockReturnValue(routeSession(second));
    jest.mocked(HomeView).mockClear();
    await view.rerender(<HomeScreen />);
    expect(jest.mocked(HomeView)).not.toHaveBeenCalled();
    expect(view.getByText('Loading your library…')).toBeTruthy();
    const next = { ...home, server: 'Second server' };
    await act(async () => load.resolve(next));
    expect(jest.mocked(HomeView).mock.calls.at(-1)![0].home).toBe(next);
  });

  it('does not carry an expired session error into a replacement session', async () => {
    const first = new KinosailClient('https://kino.example', 'expired-token');
    jest
      .spyOn(first, 'loadHome')
      .mockRejectedValue(new Error('This device session has expired.'));
    jest.mocked(useSession).mockReturnValue(routeSession(first));
    const view = await render(<HomeScreen />);
    expect(view.getByText('This device session has expired.')).toBeTruthy();
    const second = new KinosailClient(
      'https://kino.example',
      'replacement-token',
    );
    const load = deferred<Home>();
    jest.spyOn(second, 'loadHome').mockReturnValue(load.promise);
    jest.mocked(useSession).mockReturnValue(routeSession(second));
    await view.rerender(<HomeScreen />);
    expect(view.queryByText('This device session has expired.')).toBeNull();
    expect(view.getByText('Loading your library…')).toBeTruthy();
    await act(async () => load.resolve(home));
    expect(jest.mocked(HomeView).mock.calls.at(-1)![0].headers).toEqual({
      Authorization: 'Bearer replacement-token',
    });
  });
});
