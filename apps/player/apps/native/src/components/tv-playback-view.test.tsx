import React from 'react';
import { act, fireEvent, render } from '@testing-library/react-native';
import type { CastingClient, CastSession, CastStatus } from '@/core/casting';
import { routeItem } from '@/testing/route-fixtures';
import { TVPlaybackView } from './tv-playback-view';

jest.mock('expo-crypto', () => ({ randomUUID: () => 'cast-progress' }));
const session: CastSession = {
  id: 'cast-1', url: 'https://kino.example/cast/1', contentType: 'video/mp4',
  title: 'Arrival', position: 12, duration: 120, expiresAt: '',
  protocol: 'google-cast', tracks: [],
};
beforeEach(() => jest.useFakeTimers());
afterEach(() => jest.useRealTimers());

it('reports observed loading, playing, paused and stopped states without inventing progress', async () => {
  let state: CastStatus['state'] = 'buffering';
  const controller = {
    name: 'Living room TV',
    status: jest.fn(async () => ({ state, position: 12, duration: 120 })),
    command: jest.fn().mockResolvedValue(undefined),
  };
  const client = { endCast: jest.fn().mockResolvedValue(undefined) } as unknown as CastingClient;
  const saveProgress = jest.fn().mockResolvedValue(routeItem.progress);
  const view = await render(
    <TVPlaybackView playback={{ session, controller }} client={client} item={routeItem}
      saveProgress={saveProgress} onDone={jest.fn()} />,
  );
  expect(view.getByRole('progressbar', { name: 'Loading on Living room TV' })).toBeTruthy();
  expect(view.queryByText('Playing on Living room TV')).toBeNull();
  expect(saveProgress).not.toHaveBeenCalled();
  for (const [next, label] of [['playing', 'Playing'], ['paused', 'Paused'], ['stopped', 'Stopped']] as const) {
    state = next;
    await act(async () => jest.advanceTimersByTime(2000));
    expect(view.getByText(`${label} on Living room TV`)).toBeTruthy();
    expect(view.queryByRole('progressbar')).toBeNull();
  }
  await view.unmount();
});

it('hides the loading indicator when polling fails and keeps stop available', async () => {
  const controller = {
    name: 'Living room TV', status: jest.fn().mockRejectedValue(new Error('offline')),
    command: jest.fn().mockResolvedValue(undefined),
  };
  const endCast = jest.fn().mockResolvedValue(undefined);
  const done = jest.fn();
  const view = await render(
    <TVPlaybackView playback={{ session, controller }}
      client={{ endCast } as unknown as CastingClient} item={routeItem}
      saveProgress={jest.fn().mockResolvedValue(routeItem.progress)} onDone={done} />,
  );
  expect(view.queryByRole('progressbar')).toBeNull();
  expect(view.getByRole('alert')).toBeTruthy();
  await fireEvent.press(view.getByRole('button', { name: 'Stop casting and return' }));
  expect(endCast).toHaveBeenCalledWith('cast-1');
  expect(done).toHaveBeenCalledTimes(1);
  await view.unmount();
});
