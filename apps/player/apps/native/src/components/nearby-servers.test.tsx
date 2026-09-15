jest.mock('@/core/server-discovery', () => ({
  serverDiscoveryAvailable: true,
  discoverServers: jest.fn(),
  stopServerDiscovery: jest.fn(async () => {}),
}));
import React from 'react';
import { act, fireEvent, render } from '@testing-library/react-native';
import { discoverServers, stopServerDiscovery } from '@/core/server-discovery';
import { NearbyServers } from './nearby-servers';
import { SetupFlow } from './setup-flow';

const discover = jest.mocked(discoverServers);
beforeEach(() => {
  jest.clearAllMocks();
});

it('shows discovery progress, selects an endpoint, and stops when unmounted', async () => {
  let resolve!: (servers: { name: string; url: string }[]) => void;
  discover.mockImplementation(
    () =>
      new Promise((done) => {
        resolve = done;
      }),
  );
  const select = jest.fn();
  const view = await render(
    <NearbyServers disabled={false} onSelect={select} />,
  );
  expect(view.getByText('Looking for Kinosail Servers…')).toBeTruthy();
  expect(select).not.toHaveBeenCalled();
  await act(async () =>
    resolve([{ name: 'Living room', url: 'http://server.local:38127' }]),
  );
  await fireEvent.press(
    view.getByRole('button', {
      name: 'Connect to Living room, http://server.local:38127',
    }),
  );
  expect(select).toHaveBeenCalledWith('http://server.local:38127');
  await view.unmount();
  expect(stopServerDiscovery).toHaveBeenCalled();
});

it('recovers from unavailable discovery with another search', async () => {
  discover.mockRejectedValueOnce(new Error('denied')).mockResolvedValueOnce([]);
  const view = await render(
    <NearbyServers disabled={false} onSelect={jest.fn()} />,
  );
  expect(view.getByText(/Local discovery is unavailable/)).toBeTruthy();
  await fireEvent.press(view.getByRole('button', { name: 'Search again' }));
  expect(view.getByText(/No servers found/)).toBeTruthy();
  expect(discover).toHaveBeenCalledTimes(2);
});

it('uses existing device approval after selecting a discovered server', async () => {
  discover.mockResolvedValue([
    { name: 'Living room', url: 'http://server.local:38127' },
  ]);
  const startQuickConnect = jest.fn(async () => ({
    code: '123456',
    secret: 'secret',
  }));
  const createClient = jest.fn(() => ({
    startQuickConnect,
    cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
    pollQuickConnect: jest.fn(() => new Promise<string>(() => {})),
  }));
  const onConnected = jest.fn();
  const view = await render(
    <SetupFlow createClient={createClient} onConnected={onConnected} />,
  );
  expect(createClient).not.toHaveBeenCalled();
  await fireEvent.press(
    view.getByRole('button', {
      name: 'Connect to Living room, http://server.local:38127',
    }),
  );
  expect(createClient).toHaveBeenCalledWith('http://server.local:38127');
  expect(startQuickConnect).toHaveBeenCalledTimes(1);
  expect(view.getByText('Sign in with your phone')).toBeTruthy();
  expect(onConnected).not.toHaveBeenCalled();
  expect(stopServerDiscovery).toHaveBeenCalled();
});

it('preserves manual entry when no server is found', async () => {
  discover.mockResolvedValue([]);
  const startQuickConnect = jest.fn(async () => ({
    code: '123456',
    secret: 'secret',
  }));
  const view = await render(
    <SetupFlow
      createClient={() => ({
        startQuickConnect,
        cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
        pollQuickConnect: jest.fn(() => new Promise<string>(() => {})),
      })}
      onConnected={jest.fn()}
    />,
  );
  await fireEvent.changeText(
    view.getByLabelText('Kinosail Server URL'),
    'https://player.example.com',
  );
  await fireEvent.press(view.getByRole('button', { name: 'Connect' }));
  expect(startQuickConnect).toHaveBeenCalledTimes(1);
});

it('does not connect or restart discovery while pairing is pending', async () => {
  discover.mockResolvedValue([
    { name: 'Living room', url: 'http://server.local:38127' },
  ]);
  const select = jest.fn();
  const view = await render(<NearbyServers disabled onSelect={select} />);
  await fireEvent.press(
    view.getByRole('button', {
      name: 'Connect to Living room, http://server.local:38127',
    }),
  );
  await fireEvent.press(view.getByRole('button', { name: 'Search again' }));
  expect(select).not.toHaveBeenCalled();
  expect(discover).toHaveBeenCalledTimes(1);
});

it('ignores a search result delivered after unmount', async () => {
  let resolve!: (servers: { name: string; url: string }[]) => void;
  discover.mockImplementation(
    () =>
      new Promise((done) => {
        resolve = done;
      }),
  );
  const select = jest.fn();
  const view = await render(
    <NearbyServers disabled={false} onSelect={select} />,
  );
  await view.unmount();
  await act(async () =>
    resolve([{ name: 'Late server', url: 'https://server.local' }]),
  );
  expect(select).not.toHaveBeenCalled();
  expect(stopServerDiscovery).toHaveBeenCalled();
});
