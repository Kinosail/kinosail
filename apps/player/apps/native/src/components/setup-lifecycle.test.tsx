import { act, fireEvent, render } from '@testing-library/react-native';
import React from 'react';

import { deferred } from '@/testing/route-fixtures';

import { SetupFlow } from './setup-flow';

const challenge = { code: '381204', secret: 'qa-secret' };
const start = jest.fn();
const poll = jest.fn();
const connected = jest.fn();
const createClient = jest.fn(() => ({
  startQuickConnect: start,
  cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
  pollQuickConnect: poll,
}));
const mount = async () => {
  const view = await render(
    <SetupFlow createClient={createClient} onConnected={connected} />,
  );
  await fireEvent.changeText(
    view.getByLabelText('Kinosail Server URL'),
    'https://kino.example/',
  );
  await fireEvent.press(view.getByRole('button', { name: 'Connect' }));
  return view;
};

describe('setup request lifecycle', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    start.mockReset().mockResolvedValue(challenge);
    poll.mockReset().mockResolvedValue(null);
    connected.mockReset();
  });
  afterEach(() => jest.useRealTimers());

  it.each([new Error('Server unavailable'), 'non-error rejection'])(
    'recovers from a start failure: %s',
    async (failure) => {
      start.mockRejectedValue(failure);
      const view = await mount();
      expect(
        view.getByText(
          failure instanceof Error ? failure.message : 'Connection failed.',
        ),
      ).toBeTruthy();
      expect(view.getByRole('button', { name: 'Connect' })).toBeEnabled();
      expect(poll).not.toHaveBeenCalled();
      expect(connected).not.toHaveBeenCalled();
      start.mockResolvedValue(challenge);
      await fireEvent.press(view.getByRole('button', { name: 'Connect' }));
      expect(view.queryByText('Connection failed.')).toBeNull();
      expect(view.getByText('381 204')).toBeTruthy();
      expect(start).toHaveBeenLastCalledWith('Kinosail Player');
      expect(createClient).toHaveBeenCalledWith('https://kino.example');
    },
  );

  it('shows a safe fallback for an unstructured polling failure', async () => {
    poll.mockRejectedValue('failure');
    const view = await mount();
    expect(view.getByText('Connection failed.')).toBeTruthy();
    expect(connected).not.toHaveBeenCalled();
  });

  it.each(['approve', 'pending', 'reject'] as const)(
    'ignores a late poll after choosing another server: %s',
    async (outcome) => {
      jest.useFakeTimers();
      const request = deferred<string | null>();
      poll.mockReturnValue(request.promise);
      const view = await mount();
      await fireEvent.press(
        view.getByRole('button', { name: 'Use another server' }),
      );
      await act(async () => {
        if (outcome === 'reject') request.reject(new Error('late failure'));
        else request.resolve(outcome === 'approve' ? 'viewer-token' : null);
      });
      await act(async () => jest.advanceTimersByTimeAsync(5000));
      expect(connected).not.toHaveBeenCalled();
      expect(view.queryByText('late failure')).toBeNull();
      expect(view.getByLabelText('Kinosail Server URL')).toBeTruthy();
      expect(poll).toHaveBeenCalledTimes(1);
    },
  );

  it('cancels scheduled polling when the screen unmounts', async () => {
    jest.useFakeTimers();
    const view = await mount();
    expect(poll).toHaveBeenCalledTimes(1);
    await view.unmount();
    await act(async () => jest.advanceTimersByTimeAsync(5000));
    expect(poll).toHaveBeenCalledTimes(1);
    expect(connected).not.toHaveBeenCalled();
  });
});
