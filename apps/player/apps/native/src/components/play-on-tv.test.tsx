import React from 'react';
import { fireEvent, render, waitFor } from '@testing-library/react-native';
import { PlayOnTV } from './play-on-tv';
import type { CastingClient } from '@/core/casting';
jest.mock('@/core/google-cast', () => ({
  googleCastAvailable: false,
  GoogleCastPicker: () => null,
}));
jest.mock('./airplay-button', () => ({ AirPlayButton: () => null }));
const device = {
  id: 'a'.repeat(32),
  name: 'Living room',
  protocol: 'dlna' as const,
};
const props = () => ({
  itemId: 'movie',
  video: true,
  getPosition: () => 30,
  onConnected: jest.fn(),
});
it('discovers devices when opened and starts the chosen TV in one action', async () => {
  const scanCastDevices = jest.fn().mockResolvedValue([device]);
  const startCast = jest
    .fn()
    .mockResolvedValue({ id: 'session', duration: 100 });
  const input = props();
  const client = {
    scanCastDevices,
    startCast,
    castStatus: jest.fn(),
    castCommand: jest.fn(),
    endCast: jest.fn(),
  } as unknown as CastingClient;
  const view = await render(
    <PlayOnTV {...input} client={client} initiallyOpen />,
  );
  await fireEvent.press(
    await view.findByRole('button', { name: 'Living room · DLNA' }),
  );
  expect(startCast).toHaveBeenCalledWith('movie', {
    protocol: 'dlna',
    deviceId: device.id,
    position: 30,
    playbackToken: '',
  });
  await waitFor(() => expect(input.onConnected).toHaveBeenCalledTimes(1));
});
it('retains a selected device after failure and permits a targeted retry', async () => {
  const startCast = jest
    .fn()
    .mockRejectedValue(new Error('TV is unavailable.'));
  const client = {
    scanCastDevices: jest.fn().mockResolvedValue([device]),
    startCast,
    endCast: jest.fn(),
  } as unknown as CastingClient;
  const input = props();
  const view = await render(
    <PlayOnTV {...input} client={client} initiallyOpen />,
  );
  await fireEvent.press(
    await view.findByRole('button', { name: 'Living room · DLNA' }),
  );
  await view.findByText('TV is unavailable.');
  expect(input.onConnected).not.toHaveBeenCalled();
  expect(
    view.getByRole('button', { name: 'Living room · DLNA' }),
  ).not.toBeDisabled();
});
it('does not start playback when discovery fails or the picker is dismissed', async () => {
  const startCast = jest.fn();
  const client = {
    scanCastDevices: jest.fn().mockRejectedValue(new Error('offline')),
    startCast,
  } as unknown as CastingClient;
  const view = await render(
    <PlayOnTV {...props()} client={client} initiallyOpen />,
  );
  await view.findByText(/Could not find TVs/);
  await fireEvent.press(view.getByRole('button', { name: 'Close' }));
  expect(startCast).not.toHaveBeenCalled();
});
