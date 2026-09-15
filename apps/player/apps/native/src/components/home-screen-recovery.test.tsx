import { act, fireEvent, render } from '@testing-library/react-native';
import React from 'react';
import { Alert } from 'react-native';
import { router } from 'expo-router';

import HomeScreen from '@/app/index';
import { KinosailClient } from '@/core/server-client';
import { useSession } from '@/core/session-context';

jest.mock('@/core/session-context', () => ({ useSession: jest.fn() }));
jest.mock('expo-router', () => ({ router: { push: jest.fn() } }));
let mockDownloadsAvailable = false;
jest.mock('@/core/downloads', () => ({
  get downloadsAvailable() {
    return mockDownloadsAvailable;
  },
}));

const mockUseSession = useSession as jest.MockedFunction<typeof useSession>;

function failedSession(
  message: string,
  signOut: jest.Mock<Promise<void>, []>,
): ReturnType<typeof useSession> {
  const client = new KinosailClient('https://kino.example', 'viewer-token');
  jest.spyOn(client, 'loadHome').mockRejectedValue(new Error(message));
  return {
    booting: false,
    bootError: '',
    client,
    connect: jest.fn().mockResolvedValue(undefined),
    retryBoot: jest.fn(),
    session: { baseURL: 'https://kino.example', token: 'viewer-token' },
    signOut,
  };
}

describe('HomeScreen recovery', () => {
  beforeEach(() => {
    mockDownloadsAvailable = false;
    jest.clearAllMocks();
  });
  afterEach(() => jest.restoreAllMocks());
  it('lets an expired session reconnect instead of retrying it', async () => {
    const signOut = jest.fn().mockResolvedValue(undefined);
    mockUseSession.mockReturnValue(
      failedSession('This device session has expired.', signOut),
    );

    const view = await render(<HomeScreen />);

    expect(
      await view.findByText('This device session has expired.'),
    ).toBeTruthy();
    await fireEvent.press(view.getByRole('button', { name: 'Reconnect' }));
    expect(signOut).toHaveBeenCalledTimes(1);
    expect(view.queryByText('This device session has expired.')).toBeNull();
  });

  it('can retry or leave an unreachable saved server', async () => {
    const signOut = jest.fn().mockResolvedValue(undefined);
    mockUseSession.mockReturnValue(
      failedSession(
        'Could not reach Kinosail Server. Check the address and network, then try again.',
        signOut,
      ),
    );

    const view = await render(<HomeScreen />);

    expect(await view.findByRole('button', { name: 'Try again' })).toBeTruthy();
    await fireEvent.press(
      view.getByRole('button', { name: 'Use another server' }),
    );
    expect(signOut).toHaveBeenCalledTimes(1);
    expect(
      view.queryByText(
        'Could not reach Kinosail Server. Check the address and network, then try again.',
      ),
    ).toBeNull();
  });

  it('keeps offline downloads reachable and confirms leaving a mobile server', async () => {
    mockDownloadsAvailable = true;
    const signOut = jest.fn().mockResolvedValue(undefined);
    const alert = jest.spyOn(Alert, 'alert').mockImplementation(() => {});
    mockUseSession.mockReturnValue(
      failedSession('Server unavailable', signOut),
    );
    const view = await render(<HomeScreen />);
    await fireEvent.press(
      await view.findByRole('button', { name: 'Downloads' }),
    );
    expect(router.push).toHaveBeenCalledWith('/downloads');
    await fireEvent.press(
      view.getByRole('button', { name: 'Use another server' }),
    );
    expect(signOut).not.toHaveBeenCalled();
    expect(alert).toHaveBeenCalledWith(
      'Use another server?',
      expect.stringContaining('removes downloaded titles'),
      expect.any(Array),
    );
    const actions = alert.mock.calls[0][2]!;
    expect(actions[0].style).toBe('cancel');
    expect(actions[0].onPress).toBeUndefined();
    await act(async () => {
      actions[1].onPress?.();
    });
    expect(signOut).toHaveBeenCalledTimes(1);
  });

  it('keeps recovery available when signing out fails', async () => {
    const signOut = jest
      .fn()
      .mockRejectedValue(new Error('storage unavailable'));
    mockUseSession.mockReturnValue(
      failedSession('Server unavailable', signOut),
    );
    const view = await render(<HomeScreen />);
    await fireEvent.press(
      await view.findByRole('button', { name: 'Use another server' }),
    );
    expect(
      await view.findByText('Could not sign out. Try again.'),
    ).toBeTruthy();
    expect(
      view.getByRole('button', { name: 'Use another server' }),
    ).toBeTruthy();
    expect(view.queryByText('storage unavailable')).toBeNull();
  });
});
