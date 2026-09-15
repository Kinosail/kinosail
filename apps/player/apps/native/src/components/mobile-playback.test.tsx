import React from 'react';
import { act, fireEvent, render } from '@testing-library/react-native';
import { Alert, AppState, Platform, StyleSheet } from 'react-native';
import { defaultPlaybackPreferences } from '@/core/media-preferences';
import type { MediaItem, PlaybackSource } from '@/core/contract';
import { PlaybackView } from './playback-view';
import { AirPlayButton } from './airplay-button';
import { DetailView } from './detail-view';

const mockEnter = jest.fn().mockResolvedValue(undefined);
const mockExit = jest.fn().mockResolvedValue(undefined);
const mockVideoMount = jest.fn();
const mockVideoUnmount = jest.fn();
const mockListeners: Record<string, (event: any) => void> = {};
const mockPlayer = {
  status: 'idle',
  currentTime: 12,
  playing: true,
  availableAudioTracks: [],
  availableSubtitleTracks: [],
  isExternalPlaybackActive: false,
  allowsExternalPlayback: false,
  play: jest.fn(),
  addListener: jest.fn((name: string, listener: (event: any) => void) => {
    mockListeners[name] = listener;
    return { remove: jest.fn() };
  }),
};
jest.mock('expo-video', () => {
  const React = jest.requireActual('react');
  const { View } = jest.requireActual('react-native');
  return {
    useVideoPlayer: (
      _source: unknown,
      initialize: (player: unknown) => void,
    ) => {
      return React.useMemo(() => {
        initialize(mockPlayer);
        return mockPlayer;
      }, [JSON.stringify(_source)]);
    },
    VideoView: React.forwardRef((props: object, ref: unknown) => {
      React.useEffect(() => {
        mockVideoMount();
        return mockVideoUnmount;
      }, []);
      React.useImperativeHandle(ref, () => ({
        enterFullscreen: mockEnter,
        exitFullscreen: mockExit,
      }));
      return React.createElement(View, { ...props, testID: 'native-video' });
    }),
    VideoAirPlayButton: (props: object) =>
      React.createElement(View, { ...props, testID: 'airplay-picker' }),
  };
});
jest.mock('expo-image', () => ({ Image: 'Image' }));
jest.mock('expo-linear-gradient', () => ({ LinearGradient: 'LinearGradient' }));
jest.mock('@/core/protected-media', () => ({
  localCompatibilityAvailable: true,
}));
jest.mock('./compatibility-player', () => ({
  CompatibilityPlayer: 'CompatibilityPlayer',
}));
jest.mock('expo-crypto', () => ({ randomUUID: () => 'playback-session' }));
jest.mock('@/core/playback-metrics', () => ({
  measurePlayback: () => ({
    finish: jest.fn(),
    buffering: jest.fn(),
    firstFrame: jest.fn(),
    position: jest.fn(),
  }),
}));
const mockWrite = jest.fn();
jest.mock('@/core/progress-writer', () => ({
  createProgressWriter: () => ({
    write: mockWrite,
    drain: jest.fn().mockResolvedValue(undefined),
  }),
}));
const item = {
  id: 'arrival',
  kind: 'video',
  title: 'Arrival',
  container: 'mp4',
  artwork: '',
  backdrop: '',
  progress: { seconds: 12, revision: 2 },
} as MediaItem;
const source = {
  uri: 'https://kino.example/media/arrival',
  headers: { Authorization: 'Bearer viewer' },
  contentType: 'video/mp4',
  start: 12,
  duration: 120,
  plan: { allowed: true, mode: 'direct', reason: 'compatible' },
} as PlaybackSource;
const experience = {
  loaded: true,
  preferences: defaultPlaybackPreferences,
  error: '',
  saving: false,
  bookmarks: [],
  change: jest.fn(),
  reset: jest.fn(),
  bookmark: jest.fn(),
  removeBookmark: jest.fn(),
};
const mockBack = jest.fn();
const mockAlert = jest.spyOn(Alert, 'alert').mockImplementation(() => {});
const playback = () =>
  render(
    <PlaybackView
      item={item}
      source={source}
      experience={experience}
      progressMessage=""
      onBack={mockBack}
      saveProgress={jest.fn()}
    />,
  );
const originalOS = Platform.OS;
const originalTV = Object.getOwnPropertyDescriptor(Platform, 'isTV')!;
beforeEach(() => {
  jest.clearAllMocks();
  Platform.OS = 'ios';
  Object.defineProperty(Platform, 'isTV', { configurable: true, value: false });
  mockPlayer.status = 'idle';
  mockPlayer.isExternalPlaybackActive = false;
  for (const key of Object.keys(mockListeners)) delete mockListeners[key];
});
afterEach(() => {
  Platform.OS = originalOS;
  Object.defineProperty(Platform, 'isTV', originalTV);
});

it('defaults to an accessible close button instead of embedded expand controls', async () => {
  const view = await playback();
  expect(view.getByTestId('native-video').props.nativeControls).toBe(false);
  const close = view.getByLabelText('Close player');
  expect(StyleSheet.flatten(close.props.style)).toMatchObject({ minHeight: 48, width: 48, paddingHorizontal: 0 });
  await fireEvent.press(close);
  expect(mockBack).toHaveBeenCalledTimes(1);
  await view.unmount();
});
it('hands close controls to AVKit only when fullscreen opens', async () => {
  const view = await playback();
  await fireEvent(view.getByTestId('native-video'), 'layout');
  expect(view.getByLabelText('Close player')).toBeTruthy();
  await fireEvent(view.getByTestId('native-video'), 'fullscreenEnter');
  expect(view.queryByLabelText('Close player')).toBeNull();
  expect(view.getByTestId('native-video').props.nativeControls).toBe(true);
  await fireEvent(view.getByTestId('native-video'), 'fullscreenExit');
  expect(mockBack).toHaveBeenCalledTimes(1);
  await view.unmount();
});
it('opens native fullscreen once, retaining native controls and authorized resume', async () => {
  const view = await playback();
  await fireEvent(view.getByTestId('native-video'), 'layout');
  await fireEvent(view.getByTestId('native-video'), 'layout');
  expect(mockEnter).toHaveBeenCalledTimes(1);
  await fireEvent(view.getByTestId('native-video'), 'fullscreenEnter');
  expect(view.getByTestId('native-video').props.nativeControls).toBe(true);
  expect(mockPlayer.currentTime).toBe(12);
  expect(mockPlayer.allowsExternalPlayback).toBe(true);
  await view.unmount();
});
it('offers a native return action when fullscreen presentation fails', async () => {
  mockEnter.mockRejectedValueOnce(new Error('Presentation unavailable'));
  const view = await playback();
  await fireEvent(view.getByTestId('native-video'), 'layout');
  expect(view.getByTestId('native-video').props.nativeControls).toBe(false);
  await fireEvent.press(view.getByLabelText('Close player'));
  expect(mockBack).toHaveBeenCalledTimes(1);
  expect(view.queryByText('Back')).toBeNull();
  expect(mockAlert).toHaveBeenCalledWith('Unable to open player', expect.any(String), [
    { text: 'Return to details', onPress: mockBack },
  ]);
  await view.unmount();
});
it('leaves AirPlay feedback native and presents errors in a system alert', async () => {
  const view = await playback();
  await act(() =>
    mockListeners.isExternalPlaybackActiveChange({
      isExternalPlaybackActive: true,
    }),
  );
  expect(view.queryByText('Playing via AirPlay')).toBeNull();
  await act(() =>
    mockListeners.isExternalPlaybackActiveChange({
      isExternalPlaybackActive: false,
    }),
  );
  expect(view.queryByText('Playing via AirPlay')).toBeNull();
  await act(() => mockListeners.statusChange({ status: 'error' }));
  expect(mockExit).not.toHaveBeenCalled();
  expect(view.queryByText('Playback stopped')).toBeNull();
  expect(mockAlert).toHaveBeenCalledWith('Playback stopped', expect.any(String), [
    { text: 'Return to details', onPress: mockBack },
  ]);
  await view.unmount();
});
it.each(['android', 'web'] as const)(
  'keeps %s inline without an unsupported AirPlay picker',
  async (os) => {
    Platform.OS = os;
    const view = await playback();
    await fireEvent(view.getByTestId('native-video'), 'layout');
    expect(mockEnter).not.toHaveBeenCalled();
    expect(view.getByTestId('native-video').props.nativeControls).toBe(true);
    expect(view.queryByLabelText('Close player')).toBeNull();
    expect(view.queryByTestId('airplay-picker')).toBeNull();
    await view.unmount();
  },
);
it('does not add mobile casting or automatic fullscreen to TV', async () => {
  Object.defineProperty(Platform, 'isTV', { configurable: true, value: true });
  const view = await playback();
  await fireEvent(view.getByTestId('native-video'), 'layout');
  expect(mockEnter).not.toHaveBeenCalled();
  expect(view.queryByTestId('airplay-picker')).toBeNull();
  await view.unmount();
});
it('provides an accessible system route picker with audio destinations for music', async () => {
  const view = await render(<AirPlayButton video={false} />);
  const button = view.getByTestId('airplay-picker');
  expect(button.props.prioritizeVideoDevices).toBe(false);
  expect(button.props.accessibilityLabel).toBe('AirPlay');
  expect(StyleSheet.flatten(button.props.style)).toMatchObject(
    { position: 'absolute', top: 0, right: 0, bottom: 0, left: 0 },
  );
  expect(button.props.tint).toBe('transparent');
  expect(button.props.activeTint).toBe('transparent');
  const label = view.getByText('AirPlay', { includeHiddenElements: true }).parent!;
  expect(label.props.pointerEvents).toBe('none');
  expect(label.props.accessibilityElementsHidden).toBe(true);
  expect(view.queryByText(/Chromecast/i)).toBeNull();
  expect(button.props.accessibilityHint).toContain('stop AirPlay');
  await view.unmount();
});
it.each(['video', 'music', 'book'] as const)(
  'offers casting on %s details only when relevant',
  async (kind) => {
    const onPlayOnTV = jest.fn();
    const view = await render(
      <DetailView
        item={{ ...item, kind }}
        mediaURL={(path) => path}
        headers={{}}
        onBack={jest.fn()}
        onPlay={jest.fn()}
        onPlayOnTV={onPlayOnTV}
      />,
    );
    if (kind !== 'book')
      await fireEvent.press(
        view.getByRole('button', { name: 'More title actions' }),
      );
    expect(Boolean(view.queryByRole('button', { name: 'Play on TV' }))).toBe(
      kind !== 'book',
    );
    if (kind !== 'book') {
      await fireEvent.press(view.getByRole('button', { name: 'Play on TV' }));
      expect(onPlayOnTV).toHaveBeenCalledTimes(1);
    }
    await view.unmount();
  },
);

it('preserves saved progress when playback fails before it becomes playable', async () => {
  const view = await playback();
  await act(async () => mockListeners.timeUpdate({ currentTime: 0 }));
  await act(async () => mockListeners.statusChange({ status: 'error' }));
  await view.unmount();
  expect(mockWrite).not.toHaveBeenCalled();
});
it('saves a restart after playback becomes playable and reports its position', async () => {
  const view = await playback();
  await act(async () => mockListeners.statusChange({ status: 'readyToPlay' }));
  await act(async () => mockListeners.timeUpdate({ currentTime: 0 }));
  expect(mockWrite).toHaveBeenCalledWith(0);
  await view.unmount();
  expect(mockWrite).toHaveBeenLastCalledWith(0);
});

it('offers supported casting routes in the Android Play on TV menu', async () => {
  Platform.OS = 'android';
  const view = await playback();
  await fireEvent.press(view.getByRole('button', { name: 'Play on TV' }));
  expect(view.queryByTestId('airplay-picker')).toBeNull();
  expect(view.getByText('Google Cast / Chromecast')).toBeTruthy();
  expect(view.getByRole('button', { name: 'Find DLNA TVs' })).toBeDisabled();
  expect(view.getByText('Screen mirroring / Miracast')).toBeTruthy();
  await view.unmount();
});

it.each([
  { container: 'mkv', contentType: 'video/x-matroska', dialogueBoost: false },
  { container: 'mp4', contentType: 'video/mp4', dialogueBoost: true },
])('keeps iOS video in the native player for $container and audio preferences', async ({ container, contentType, dialogueBoost }) => {
  const view = await render(
    <PlaybackView
      item={{ ...item, container }}
      source={{ ...source, contentType }}
      experience={{ ...experience, preferences: { ...defaultPlaybackPreferences, dialogueBoost } }}
      progressMessage=""
      onBack={jest.fn()}
      saveProgress={jest.fn()}
    />,
  );
  await fireEvent(view.getByTestId('native-video'), 'fullscreenEnter');
  expect(view.getByTestId('native-video').props.nativeControls).toBe(true);
  expect(view.queryByLabelText('Starting playback')).toBeNull();
  await act(() => mockListeners.statusChange({
    status: 'error', error: { message: 'ERROR_CODE_DECODING_FAILED' },
  }));
  expect(view.getByTestId('native-video')).toBeTruthy();
  expect(view.queryByRole('button', { name: 'Try local playback' })).toBeNull();
  expect(view.queryByRole('button', { name: 'Return to details' })).toBeNull();
  expect(mockAlert).toHaveBeenCalledWith('Playback stopped', expect.any(String), expect.any(Array));
  await view.unmount();
});

it('renders only native video controls after fullscreen and returns through native Done', async () => {
  const view = await playback();
  await fireEvent(view.getByTestId('native-video'), 'fullscreenEnter');
  expect(view.queryAllByRole('button')).toHaveLength(0);
  expect(view.queryByLabelText('Starting playback')).toBeNull();
  await fireEvent(view.getByTestId('native-video'), 'fullscreenExit');
  expect(mockBack).toHaveBeenCalledTimes(1);
  await view.unmount();
});

it('keeps playback mounted when fullscreen transitions to picture in picture', async () => {
  const view = await playback();
  await fireEvent(view.getByTestId('native-video'), 'pictureInPictureStart');
  await fireEvent(view.getByTestId('native-video'), 'fullscreenExit');
  expect(mockBack).not.toHaveBeenCalled();
  await fireEvent(view.getByTestId('native-video'), 'pictureInPictureStop');
  await fireEvent(view.getByTestId('native-video'), 'fullscreenExit');
  expect(mockBack).toHaveBeenCalledTimes(1);
  await view.unmount();
});

const fallback = (mode = 'remux', allowed = true): PlaybackSource => ({
  ...source,
  compatible: {
    uri: 'https://kino.example/compatible/arrival.m3u8',
    duration: 110,
    omitted: [{ start: 20, end: 30 }],
    progressToken: 'compatible-progress',
    plan: { allowed, mode, reason: 'container-unsupported' },
  },
});
const recoveryPlayback = (value: PlaybackSource) => render(
  <PlaybackView item={item} source={value} experience={experience}
    progressMessage="" onBack={mockBack} saveProgress={jest.fn()} />,
);

it.each(['remux', 'audio-transcode', 'transcode'])(
  'automatically resumes %s once without an alert or accidental fullscreen exit',
  async (mode) => {
    const view = await recoveryPlayback(fallback(mode));
    const oldExit = view.getByTestId('native-video').props.onFullscreenExit;
    await act(() => mockListeners.statusChange({ status: 'readyToPlay' }));
    await act(() => mockListeners.timeUpdate({ currentTime: 45 }));
    const failed = mockListeners.statusChange;
    await act(() => {
      failed({ status: 'error' });
      failed({ status: 'error' });
      oldExit();
    });
    expect(mockPlayer.currentTime).toBe(35);
    expect(mockAlert).not.toHaveBeenCalled();
    expect(mockBack).not.toHaveBeenCalled();
    await act(() => mockListeners.statusChange({ status: 'error' }));
    expect(mockAlert).toHaveBeenCalledTimes(1);
    expect(mockAlert.mock.calls[0][2]).toEqual([
      { text: 'Return to details', onPress: mockBack },
    ]);
    await view.unmount();
  },
);

it.each([
  ['denied', false, undefined],
  ['unexpected', true, undefined],
  ['remux', false, undefined],
  ['remux', true, 'HTTP 403 forbidden'],
  ['remux', true, 'HTTP 404 not found'],
  ['remux', true, 'network connection timed out'],
])('does not automatically recover %s allowed=%s error=%s', async (mode, allowed, message) => {
  const view = await recoveryPlayback(fallback(mode as string, allowed as boolean));
  await act(() => mockListeners.statusChange({
    status: 'error', error: message ? { message } : undefined,
  }));
  expect(mockPlayer.currentTime).toBe(12);
  expect(mockAlert).toHaveBeenCalledTimes(1);
  expect(mockBack).not.toHaveBeenCalled();
  await view.unmount();
});

describe('video with audio but no rendered picture', () => {
  const originalState = AppState.currentState;
  beforeEach(() => {
    jest.useFakeTimers();
    AppState.currentState = 'active';
    mockPlayer.playing = true;
  });
  afterEach(() => {
    jest.useRealTimers();
    AppState.currentState = originalState;
  });
  const advanceAudio = async (seconds: number) => {
    for (let second = 0; second < seconds; second++) {
      mockPlayer.currentTime += 1;
      await act(() => jest.advanceTimersByTime(1000));
    }
  };
  it('switches once to compatible playback after eight seconds without a frame', async () => {
    const view = await recoveryPlayback(fallback('transcode'));
    await advanceAudio(7);
    expect(mockPlayer.currentTime).toBe(19);
    await advanceAudio(1);
    expect(mockPlayer.currentTime).toBe(20);
    // A fresh player mount starts at the mapped time and consumes the fallback.
    expect(mockPlayer.play).toHaveBeenCalledTimes(2);
    expect(mockAlert).not.toHaveBeenCalled();
    await advanceAudio(10);
    expect(mockPlayer.play).toHaveBeenCalledTimes(2);
    await view.unmount();
  });
  it.each(['frame', 'paused', 'background', 'airplay', 'pip', 'audio', 'stalled'])(
    'does not recover normal %s playback', async (state) => {
      const view = await render(
        <PlaybackView item={{ ...item, kind: state === 'audio' ? 'music' : 'video' }}
          source={{ ...fallback('transcode'), plan: { ...source.plan, mode: 'remux' } }} experience={experience}
          progressMessage="" onBack={mockBack} saveProgress={jest.fn()} />,
      );
      if (state === 'frame') await fireEvent(view.getByTestId('native-video'), 'firstFrameRender');
      if (state === 'paused') mockPlayer.playing = false;
      if (state === 'background') AppState.currentState = 'background';
      if (state === 'airplay') mockPlayer.isExternalPlaybackActive = true;
      if (state === 'pip') await fireEvent(view.getByTestId('native-video'), 'pictureInPictureStart');
      const plays = mockPlayer.play.mock.calls.length;
      if (state === 'stalled') await act(() => jest.advanceTimersByTime(10000));
      else await advanceAudio(10);
      expect(mockPlayer.play).toHaveBeenCalledTimes(plays);
      expect(mockAlert).not.toHaveBeenCalled();
      await view.unmount();
    },
  );
});

it('retains the presented native view and its controls when recovery replaces the player', async () => {
  const view = await recoveryPlayback(fallback());
  await fireEvent(view.getByTestId('native-video'), 'layout');
  await fireEvent(view.getByTestId('native-video'), 'fullscreenEnter');
  await act(() => mockListeners.statusChange({ status: 'readyToPlay' }));
  await act(() => mockListeners.timeUpdate({ currentTime: 45 }));
  await act(() => mockListeners.statusChange({ status: 'error' }));
  await fireEvent(view.getByTestId('native-video'), 'layout');
  expect(mockVideoMount).toHaveBeenCalledTimes(1);
  expect(mockVideoUnmount).not.toHaveBeenCalled();
  expect(mockEnter).toHaveBeenCalledTimes(1);
  expect(mockPlayer.currentTime).toBe(35);
  expect(view.getByTestId('native-video').props.nativeControls).toBe(true);
  await act(() => mockListeners.statusChange({ status: 'readyToPlay' }));
  await act(() => mockListeners.timeUpdate({ currentTime: 36 }));
  expect(mockWrite).toHaveBeenLastCalledWith(36);
  await fireEvent(view.getByTestId('native-video'), 'fullscreenExit');
  expect(mockBack).toHaveBeenCalledTimes(1);
  await view.unmount();
});

it('shows a return action for a load failure that occurred before listeners attached', async () => {
  mockPlayer.status = 'error';
  const view = await playback();
  expect(mockAlert).toHaveBeenCalledWith('Playback stopped', expect.any(String), [
    { text: 'Return to details', onPress: mockBack },
  ]);
  expect(mockWrite).not.toHaveBeenCalled();
  await view.unmount();
});

it('offers explicit recovery when an early failure has no error details', async () => {
  mockPlayer.status = 'error';
  const view = await recoveryPlayback(fallback());
  expect(mockPlayer.play).toHaveBeenCalledTimes(1);
  expect(mockVideoMount).toHaveBeenCalledTimes(1);
  expect(mockAlert).toHaveBeenCalledTimes(1);
  expect(mockAlert.mock.calls[0][2]).toEqual([
    { text: 'Try compatible playback', onPress: expect.any(Function) },
    { text: 'Return to details', onPress: mockBack },
  ]);
  expect(mockWrite).not.toHaveBeenCalled();
  await view.unmount();
});


it('shows native loading feedback until a frame and while buffering, retaining controls', async () => {
  const view = await playback();
  const loading = () => view.getByTestId('native-video').props.showsLoadingIndicator;
  expect(loading()).toBe(true);
  await fireEvent(view.getByTestId('native-video'), 'layout');
  await act(() => mockListeners.statusChange({ status: 'readyToPlay' }));
  expect(loading()).toBe(true);
  await fireEvent(view.getByTestId('native-video'), 'firstFrameRender');
  expect(loading()).toBe(false);
  await act(() => mockListeners.statusChange({ status: 'loading' }));
  expect(loading()).toBe(true);
  await act(() => mockListeners.statusChange({ status: 'readyToPlay' }));
  expect(loading()).toBe(false);
  await fireEvent(view.getByTestId('native-video'), 'fullscreenEnter');
  expect(view.getByTestId('native-video').props.nativeControls).toBe(true);
  expect(mockEnter).toHaveBeenCalledTimes(1);
  await view.unmount();
});

it('hides loading feedback on failure and during AirPlay', async () => {
  const view = await playback();
  const loading = () => view.getByTestId('native-video').props.showsLoadingIndicator;
  await act(() => mockListeners.isExternalPlaybackActiveChange({ isExternalPlaybackActive: true }));
  expect(loading()).toBe(false);
  await act(() => mockListeners.isExternalPlaybackActiveChange({ isExternalPlaybackActive: false }));
  expect(loading()).toBe(true);
  await act(() => mockListeners.statusChange({ status: 'error' }));
  expect(loading()).toBe(false);
  await view.unmount();
});

it('shows loading again during compatible recovery without replacing the native view', async () => {
  const view = await recoveryPlayback(fallback());
  await fireEvent(view.getByTestId('native-video'), 'firstFrameRender');
  expect(view.getByTestId('native-video').props.showsLoadingIndicator).toBe(false);
  await act(() => mockListeners.statusChange({ status: 'error' }));
  expect(view.getByTestId('native-video').props.showsLoadingIndicator).toBe(true);
  expect(mockVideoMount).toHaveBeenCalledTimes(1);
  await view.unmount();
});

it.each(['android', 'web', 'tv', 'audio'])(
  'does not add iOS video loading feedback to %s', async (platform) => {
    if (platform === 'android' || platform === 'web') Platform.OS = platform;
    if (platform === 'tv') Object.defineProperty(Platform, 'isTV', { configurable: true, value: true });
    const view = await render(
      <PlaybackView item={{ ...item, kind: platform === 'audio' ? 'music' : 'video' }}
        source={source} experience={experience} progressMessage=""
        onBack={mockBack} saveProgress={jest.fn()} />,
    );
    expect(view.queryByTestId('native-video')?.props.showsLoadingIndicator).toBeUndefined();
    await view.unmount();
  },
);


it.each(['android', 'tv'])('shows buffering after the first frame on %s and clears it on recovery or error', async (platform) => {
  Platform.OS = 'android';
  Object.defineProperty(Platform, 'isTV', { configurable: true, value: platform === 'tv' });
  const view = await playback();
  await fireEvent(view.getByTestId('native-video'), 'firstFrameRender');
  await act(() => mockListeners.statusChange({ status: 'loading' }));
  expect(view.getByRole('progressbar', { name: 'Buffering video' })).toBeTruthy();
  expect(view.getByRole('button', { name: 'Back' })).toBeTruthy();
  await act(() => mockListeners.statusChange({ status: 'readyToPlay' }));
  expect(view.queryByText('Buffering…')).toBeNull();
  await act(() => mockListeners.statusChange({ status: 'loading' }));
  await act(() => mockListeners.statusChange({ status: 'error' }));
  expect(view.queryByText('Buffering…')).toBeNull();
  await view.unmount();
});

it('can return to details while playback preferences are still loading', async () => {
  const view = await render(
    <PlaybackView item={item} source={source} experience={{ ...experience, loaded: false }}
      progressMessage="" onBack={mockBack} saveProgress={jest.fn()} />,
  );
  expect(view.getByRole('progressbar', { name: 'Loading playback preferences…' })).toBeTruthy();
  await fireEvent.press(view.getByRole('button', { name: 'Return to details' }));
  expect(mockBack).toHaveBeenCalledTimes(1);
  expect(mockPlayer.play).not.toHaveBeenCalled();
  await view.unmount();
});
