import React from 'react';
import { Platform } from 'react-native';
import { act, fireEvent, render } from '@testing-library/react-native';
import { PlaybackView } from './playback-view';
import { CompatibilityPlayer } from './compatibility-player';
import { defaultPlaybackPreferences } from '@/core/media-preferences';
import { routeItem, routeSource } from '@/testing/route-fixtures';
import type { PlaybackExperience } from '@/core/use-playback-experience';

const mockListeners: Record<string, (event: object) => void> = {};
const mockPlayer = {
  currentTime: 12,
  status: 'readyToPlay',
  playing: true,
  availableAudioTracks: [],
  availableSubtitleTracks: [],
  play: jest.fn(),
  pause: jest.fn(),
  addListener: jest.fn((name: string, listener: (event: object) => void) => {
    mockListeners[name] = listener;
    return { remove: jest.fn() };
  }),
};
const mockOpen = jest.fn();
const mockEnterFullscreen = jest.fn().mockResolvedValue(undefined);
jest.mock('expo-video', () => {
  const ReactModule = jest.requireActual<typeof React>('react');
  const { View } = jest.requireActual('react-native');
  return {
    useVideoPlayer: (
      source: object,
      initialize: (player: typeof mockPlayer) => void,
    ) => {
      const previous = ReactModule.useRef('');
      const key = JSON.stringify(source);
      if (previous.current !== key) {
        previous.current = key;
        mockOpen(source);
        initialize(mockPlayer);
      }
      return mockPlayer;
    },
    VideoView: ReactModule.forwardRef((props: object, ref) => {
      ReactModule.useImperativeHandle(ref, () => ({
        exitFullscreen: async () => {},
        enterFullscreen: mockEnterFullscreen,
      }));
      return ReactModule.createElement(View, props);
    }),
  };
});
jest.mock('@/core/protected-media', () => ({
  localCompatibilityAvailable: true,
}));
jest.mock('./compatibility-player', () => ({
  CompatibilityPlayer: jest.fn(() => null),
}));
jest.mock('./airplay-button', () => ({ AirPlayButton: () => null }));
const experience: PlaybackExperience = {
  preferences: defaultPlaybackPreferences,
  loaded: true,
  error: '',
  saving: false,
  bookmarks: [],
  change: jest.fn(),
  reset: jest.fn(),
  bookmark: jest.fn(),
  removeBookmark: jest.fn(),
};
const props = {
  item: routeItem,
  source: routeSource,
  experience,
  progressMessage: '',
  onBack: jest.fn(),
  saveProgress: jest.fn().mockResolvedValue(routeItem.progress),
};
const originalOS = Platform.OS;
afterEach(() => {
  Platform.OS = originalOS;
});
beforeEach(() => {
  Platform.OS = 'android';
  jest.clearAllMocks();
  mockPlayer.currentTime = 12;
});

it('does not initialize a player while preferences are unresolved', async () => {
  const view = await render(
    <PlaybackView {...props} experience={{ ...experience, loaded: false }} />,
  );
  expect(mockOpen).not.toHaveBeenCalled();
  await view.rerender(
    <PlaybackView
      {...props}
      experience={{
        ...experience,
        preferences: { ...defaultPlaybackPreferences, dialogueBoost: true },
      }}
    />,
  );
  expect(mockOpen).not.toHaveBeenCalled();
  expect(CompatibilityPlayer).toHaveBeenCalled();
});
it('opens an original MKV directly in the local engine', async () => {
  await render(
    <PlaybackView
      {...props}
      item={{ ...routeItem, container: 'mkv' }}
      source={{ ...routeSource, contentType: 'video/x-matroska' }}
    />,
  );
  expect(mockOpen).not.toHaveBeenCalled();
  expect(CompatibilityPlayer).toHaveBeenCalled();
});
it('leaves unknown errors in the same engine and preserves time for explicit local recovery', async () => {
  const view = await render(<PlaybackView {...props} />);
  await act(async () => mockListeners.timeUpdate({ currentTime: 42 }));
  await act(async () =>
    mockListeners.statusChange({
      status: 'error',
      error: { message: 'The operation could not be completed' },
    }),
  );
  expect(CompatibilityPlayer).not.toHaveBeenCalled();
  await fireEvent.press(
    view.getByRole('button', { name: 'Try local playback' }),
  );
  expect(
    jest.mocked(CompatibilityPlayer).mock.calls.at(-1)![0].source.start,
  ).toBe(42);
});
it('uses one local fallback for explicit decoding failure', async () => {
  await render(<PlaybackView {...props} />);
  await act(async () =>
    mockListeners.statusChange({
      status: 'error',
      error: { message: 'ERROR_CODE_DECODING_FAILED' },
    }),
  );
  expect(CompatibilityPlayer).toHaveBeenCalled();
  expect(
    jest.mocked(CompatibilityPlayer).mock.calls.at(-1)![0].source.uri,
  ).toBe(routeSource.uri);
});

it('does not offer conversion or a decoder switch for denied access', async () => {
  const view = await render(<PlaybackView {...props} />);
  await act(async () =>
    mockListeners.statusChange({
      status: 'error',
      error: { message: 'HTTP 403 forbidden' },
    }),
  );
  expect(CompatibilityPlayer).not.toHaveBeenCalled();
  expect(view.queryByRole('button', { name: 'Try local playback' })).toBeNull();
  expect(
    view.getByText(
      'Your Server denied access to this title. Check your Viewer permissions or reconnect.',
    ),
  ).toBeTruthy();
});

it('fills the TV viewport and keeps pause, seek, tracks, and recovery in the shared controls', async () => {
  const { Platform } =
    jest.requireActual<typeof import('react-native')>('react-native');
  const descriptor = Object.getOwnPropertyDescriptor(Platform, 'isTV')!;
  Object.defineProperty(Platform, 'isTV', { configurable: true, value: true });
  try {
    const view = await render(<PlaybackView {...props} />);
    const video = view.getByLabelText(`Playing ${routeItem.title}`);
    expect(video.props.nativeControls).toBe(false);
    await fireEvent(video, 'layout', {
      nativeEvent: { layout: { width: 1920, height: 1080 } },
    });
    expect(mockEnterFullscreen).not.toHaveBeenCalled();
    expect(
      view.getByRole('button', { name: 'Pause' }).props.accessibilityState
        .disabled,
    ).toBe(true);
    await fireEvent(video, 'firstFrameRender');
    await fireEvent.press(view.getByRole('button', { name: 'Pause' }));
    expect(mockPlayer.pause).toHaveBeenCalledTimes(1);
    await act(() => mockListeners.playingChange({ isPlaying: false }));
    await fireEvent.press(view.getByRole('button', { name: 'Play' }));
    expect(mockPlayer.play).toHaveBeenCalled();
    await act(() => mockListeners.timeUpdate({ currentTime: 42 }));
    await fireEvent.press(
      view.getByRole('button', { name: 'Back 10 seconds' }),
    );
    expect(mockPlayer.currentTime).toBe(32);
    await fireEvent.press(
      view.getByRole('button', { name: 'Forward 10 seconds' }),
    );
    expect(mockPlayer.currentTime).toBe(42);
    await fireEvent.press(
      view.getByRole('button', { name: 'Playback options' }),
    );
    expect(view.getByRole('button', { name: 'Subtitles off' })).toBeTruthy();
    await fireEvent.press(view.getByRole('button', { name: 'Back' }));
    expect(props.onBack).toHaveBeenCalledTimes(1);
    await view.unmount();
  } finally {
    Object.defineProperty(Platform, 'isTV', descriptor);
  }
});
