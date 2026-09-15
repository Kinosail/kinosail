import { act, fireEvent, render } from '@testing-library/react-native';
import React from 'react';
import { AppState, type AppStateStatus } from 'react-native';

import type { MediaItem, PlaybackSource } from '@/core/contract';
import { defaultPlaybackPreferences } from '@/core/media-preferences';

import { PlaybackView } from './playback-view';

jest.mock('expo-crypto', () => ({ randomUUID: jest.fn(() => 'test-session') }));
jest.mock('@/core/protected-media', () => ({
  localCompatibilityAvailable: false,
}));
jest.mock('@/core/verified-transfers', () => ({
  prioritizePlayback: jest.fn(),
}));

const mockListeners: Record<string, jest.Mock> = {};
const mockPlayer = {
  addListener: jest.fn((event: string, listener: jest.Mock) => {
    mockListeners[event] = listener;
    return { remove: jest.fn() };
  }),
  availableAudioTracks: [],
  availableSubtitleTracks: [],
  currentTime: 0,
  play: jest.fn(),
  timeUpdateEventInterval: 0,
};
type InitializePlayer = (player: typeof mockPlayer) => void;
const mockUseVideoPlayer = jest.fn(
  (source: object, initialize: InitializePlayer) => {
    initialize(mockPlayer);
    return mockPlayer;
  },
);

expect.addSnapshotSerializer({
  test: (value) => value === mockPlayer,
  print: () => '[Native video player]',
});

jest.mock('expo-video', () => {
  const ReactModule = jest.requireActual<typeof React>('react');
  const { View } = jest.requireActual('react-native');
  return {
    useVideoPlayer: (source: object, initialize: InitializePlayer) =>
      ReactModule.useMemo(
        () => mockUseVideoPlayer(source, initialize),
        [JSON.stringify(source)],
      ),
    VideoView: ReactModule.forwardRef((props: object, ref) => {
      ReactModule.useImperativeHandle(ref, () => ({
        enterFullscreen: jest.fn().mockResolvedValue(undefined),
        exitFullscreen: jest.fn().mockResolvedValue(undefined),
      }));
      return ReactModule.createElement(View, { ...props, testID: 'video' });
    }),
  };
});

const item = {
  id: 'arrival',
  title: 'Arrival',
  progress: { seconds: 12, watched: false, session: '', revision: 2 },
} as MediaItem;
const source: PlaybackSource = {
  uri: 'https://kino.example/media/arrival',
  headers: { Authorization: 'Bearer viewer-token' },
  contentType: 'video/mp4',
  duration: 120,
  start: 12,
  progressToken: '',
  plan: { allowed: true, mode: 'direct', reason: 'compatible' },
};

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

describe('PlaybackView', () => {
  let appStateListener: ((state: AppStateStatus) => void) | undefined;

  beforeEach(() => {
    jest.clearAllMocks();
    for (const event of Object.keys(mockListeners)) delete mockListeners[event];
    jest
      .spyOn(AppState, 'addEventListener')
      .mockImplementation((_event, listener) => {
        appStateListener = listener;
        return { remove: jest.fn() };
      });
  });
  afterEach(() => jest.restoreAllMocks());

  it('starts an authorized source without persistent caching and waits for the first frame', async () => {
    const view = await render(
      <PlaybackView
        experience={experience}
        progressMessage=""
        item={item}
        onBack={jest.fn()}
        saveProgress={jest.fn()}
        source={source}
      />,
    );

    expect(mockUseVideoPlayer).toHaveBeenCalledWith(
      {
        uri: source.uri,
        headers: source.headers,
        contentType: 'progressive',
        metadata: { title: 'Arrival' },
        useCaching: false,
      },
      expect.any(Function),
    );
    expect(mockPlayer.currentTime).toBe(12);
    expect(mockPlayer.timeUpdateEventInterval).toBe(1);
    expect(mockPlayer.play).toHaveBeenCalledTimes(1);
    expect(AppState.addEventListener).toHaveBeenCalledWith(
      'change',
      expect.any(Function),
    );
    expect(view.getByTestId('video').props.nativeControls).toBe(true);
    expect(view.queryByLabelText('Starting playback')).toBeNull();
    expect(view.toJSON()).toMatchSnapshot('starting direct playback');

    await fireEvent(view.getByTestId('video'), 'firstFrameRender');
    expect(view.queryByLabelText('Starting direct playback')).toBeNull();
    expect(view.toJSON()).toMatchSnapshot('first frame ready');
  });

  it('replaces the loading state with direct-play recovery', async () => {
    const view = await render(
      <PlaybackView
        experience={experience}
        progressMessage=""
        item={item}
        onBack={jest.fn()}
        saveProgress={jest.fn()}
        source={source}
      />,
    );

    expect(view.getByRole('button', { name: 'Back' })).toBeTruthy();
    await act(async () => mockListeners.statusChange({ status: 'error' }));

    expect(view.queryByLabelText('Starting direct playback')).toBeNull();
    expect(view.queryByRole('button', { name: 'Back' })).toBeNull();
    expect(view.getByText('Playback stopped')).toBeTruthy();
    expect(
      view.getByText('Playback could not continue on this device.'),
    ).toBeTruthy();
    expect(view.toJSON()).toMatchSnapshot('direct playback recovery');
  });

  it('saves the latest position when the app backgrounds', async () => {
    const saveProgress = jest.fn().mockResolvedValue(item.progress);
    await render(
      <PlaybackView
        experience={experience}
        progressMessage=""
        item={item}
        onBack={jest.fn()}
        saveProgress={saveProgress}
        source={source}
      />,
    );

    await act(async () =>
      mockListeners.statusChange({ status: 'readyToPlay' }),
    );
    await act(async () => mockListeners.timeUpdate({ currentTime: 42 }));
    expect(saveProgress).toHaveBeenCalledTimes(1);
    await act(async () => appStateListener?.('background'));

    expect(saveProgress).toHaveBeenCalledTimes(2);
    expect(saveProgress).toHaveBeenLastCalledWith({
      seconds: 42,
      session: expect.stringMatching(/^native-.+/),
      revision: 4,
      watched: false,
      playbackToken: '',
    });
  });

  it('keeps authenticated HLS uncached without changing its URL or headers', async () => {
    const hls = {
      ...source,
      contentType: 'application/vnd.apple.mpegurl',
      uri: 'https://kino.example/media/live.m3u8',
    };
    await render(
      <PlaybackView
        experience={experience}
        progressMessage=""
        item={item}
        onBack={jest.fn()}
        saveProgress={jest.fn()}
        source={hls}
      />,
    );
    expect(mockUseVideoPlayer).toHaveBeenCalledWith(
      expect.objectContaining({
        uri: hls.uri,
        headers: hls.headers,
        contentType: 'hls',
        useCaching: false,
      }),
      expect.any(Function),
    );
  });

  it('saves completion and removes native listeners when leaving playback', async () => {
    const saveProgress = jest.fn().mockResolvedValue(item.progress);
    const view = await render(
      <PlaybackView
        experience={experience}
        progressMessage=""
        item={item}
        onBack={jest.fn()}
        saveProgress={saveProgress}
        source={source}
      />,
    );
    await act(async () =>
      mockListeners.statusChange({ status: 'readyToPlay' }),
    );
    expect(view.queryByText('Playback stopped')).toBeNull();
    await act(async () => appStateListener?.('active'));
    expect(saveProgress).not.toHaveBeenCalled();
    await act(async () => mockListeners.playToEnd());
    expect(saveProgress).toHaveBeenCalledWith(
      expect.objectContaining({ seconds: 120, watched: true }),
    );
    await view.unmount();
    expect(saveProgress).toHaveBeenCalledTimes(2);
    expect(saveProgress).toHaveBeenLastCalledWith(
      expect.objectContaining({ seconds: 120 }),
    );
    for (const result of mockPlayer.addListener.mock.results) {
      expect(result.value.remove).toHaveBeenCalledTimes(1);
    }
    expect(
      jest.mocked(AppState.addEventListener).mock.results[0].value.remove,
    ).toHaveBeenCalledTimes(1);
  });

  it('rebinds progress saves when the route supplies a new writer', async () => {
    const first = jest.fn().mockResolvedValue(item.progress);
    const second = jest.fn().mockResolvedValue(item.progress);
    const view = await render(
      <PlaybackView
        experience={experience}
        progressMessage=""
        item={item}
        onBack={jest.fn()}
        saveProgress={first}
        source={source}
      />,
    );
    await act(async () =>
      mockListeners.statusChange({ status: 'readyToPlay' }),
    );
    await act(async () => mockListeners.timeUpdate({ currentTime: 12 }));
    await view.rerender(
      <PlaybackView
        experience={experience}
        progressMessage=""
        item={item}
        onBack={jest.fn()}
        saveProgress={second}
        source={source}
      />,
    );
    expect(first).toHaveBeenCalledWith(
      expect.objectContaining({ seconds: 12 }),
    );
    await act(async () =>
      mockListeners.statusChange({ status: 'readyToPlay' }),
    );
    await act(async () => mockListeners.timeUpdate({ currentTime: 44 }));
    expect(second).toHaveBeenCalledWith(
      expect.objectContaining({ seconds: 44 }),
    );
    expect(first).toHaveBeenCalledTimes(2);
  });
});
