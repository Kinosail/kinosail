import React from 'react';
import { defaultPlaybackPreferences } from '@/core/media-preferences';
import { act, fireEvent, render } from '@testing-library/react-native';
import { routeItem, routeSource } from '@/testing/route-fixtures';
import { CompatibilityPlayer } from './compatibility-player';
import { openProtectedMedia } from '@/core/protected-media';
import { clearPlaybackMetrics, playbackMetrics } from '@/core/playback-metrics';
jest.mock('./airplay-button', () => ({ AirPlayButton: () => null }));
jest.mock('@/core/protected-media', () => ({
  openProtectedMedia: jest
    .fn()
    .mockResolvedValue('http://127.0.0.1:1234/capability'),
  closeProtectedMedia: jest.fn(),
  nextProtectedMediaID: () => 1,
}));
jest.mock('./local-video', () => {
  const ReactModule = jest.requireActual('react');
  const { View } = jest.requireActual('react-native');
  return {
    __esModule: true,
    default: ReactModule.forwardRef((props: object, ref: unknown) =>
      ReactModule.createElement(View, { ...props, ref, testID: 'local-video' }),
    ),
  };
});
beforeEach(() => {
  jest.clearAllMocks();
  clearPlaybackMetrics();
});
const setup = () =>
  render(
    <CompatibilityPlayer
      experience={{
        loaded: true,
        preferences: defaultPlaybackPreferences,
        error: '',
        saving: false,
        bookmarks: [],
        change: jest.fn(),
        reset: jest.fn(),
        bookmark: jest.fn(),
        removeBookmark: jest.fn(),
      }}
      progressMessage=""
      item={routeItem}
      source={{ ...routeSource, duration: 120, start: 30 }}
      onBack={jest.fn()}
      onRecover={jest.fn()}
      saveProgress={jest.fn().mockResolvedValue(routeItem.progress)}
    />,
  );
it('replays from zero after end and discards malformed native track data', async () => {
  const view = await setup();
  const native = await view.findByTestId('local-video');
  await fireEvent(native, 'tracks', null);
  await fireEvent(native, 'end');
  await fireEvent.press(view.getByRole('button', { name: 'Replay' }));
  expect(view.getByTestId('local-video').props.source.start).toBe(0);
  expect(view.getByTestId('local-video').props.paused).toBe(false);
});
it('measures replay independently without reopening the authorized transport', async () => {
  const view = await setup();
  const first = await view.findByTestId('local-video');
  await fireEvent(first, 'firstFrame');
  await fireEvent(first, 'end');
  await fireEvent.press(view.getByRole('button', { name: 'Replay' }));
  await fireEvent(view.getByTestId('local-video'), 'firstFrame');
  await fireEvent(view.getByTestId('local-video'), 'end');
  expect(
    playbackMetrics().filter((sample) => sample.outcome === 'ended'),
  ).toHaveLength(2);
  expect(openProtectedMedia).toHaveBeenCalledTimes(1);
});
it('preserves a pause requested while protected transport is opening', async () => {
  let resolve: (uri: string) => void = () => {};
  (openProtectedMedia as jest.Mock).mockImplementationOnce(
    () =>
      new Promise<string>((done) => {
        resolve = done;
      }),
  );
  const view = await setup();
  await fireEvent.press(view.getByRole('button', { name: 'Pause' }));
  await act(async () => resolve('http://127.0.0.1:1234/capability'));
  expect(view.getByTestId('local-video').props.paused).toBe(true);
});

it('does not treat an intentional pause before the first frame as a startup failure', async () => {
  jest.useFakeTimers();
  try {
    const view = await setup();
    await fireEvent.press(view.getByRole('button', { name: 'Pause' }));
    await act(async () => {
      jest.advanceTimersByTime(31_000);
    });
    expect(view.queryByRole('alert')).toBeNull();
    await view.unmount();
  } finally {
    jest.useRealTimers();
  }
});


it('shows buffering after startup and hides it when paused, resumed, ended or failed', async () => {
  const view = await setup();
  const native = await view.findByTestId('local-video');
  await fireEvent(native, 'firstFrame');
  await fireEvent(native, 'buffering', { active: 'invalid' });
  expect(view.queryByText('Buffering…')).toBeNull();
  await fireEvent(native, 'buffering', { active: true });
  expect(view.getByRole('progressbar', { name: 'Buffering media' })).toBeTruthy();
  await fireEvent(native, 'paused');
  expect(view.queryByText('Buffering…')).toBeNull();
  await fireEvent(native, 'playing');
  expect(view.queryByText('Buffering…')).toBeNull();
  await fireEvent(native, 'buffering', { active: true });
  await fireEvent(native, 'buffering', { active: false });
  expect(view.queryByText('Buffering…')).toBeNull();
  await fireEvent(native, 'buffering', { active: true });
  await fireEvent(native, 'end');
  expect(view.queryByText('Buffering…')).toBeNull();
  await fireEvent(native, 'error');
  expect(view.queryByText('Buffering…')).toBeNull();
  await view.unmount();
});
