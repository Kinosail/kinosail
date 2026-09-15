jest.mock('./airplay-button', () => ({ AirPlayButton: () => null }));
import type { MusicQueue } from '@/core/use-music-queue';
import React from 'react';
import { act, fireEvent, render, waitFor } from '@testing-library/react-native';
import { routeItem, routeSource } from '@/testing/route-fixtures';
import { defaultPlaybackPreferences } from '@/core/media-preferences';
import type { PlaybackExperience } from '@/core/use-playback-experience';
import { CompatibilityPlayer } from './compatibility-player';

const mockSeek = jest.fn();
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
    default: ReactModule.forwardRef((props: object, ref: unknown) => {
      ReactModule.useImperativeHandle(ref, () => ({
        seek: mockSeek,
        getTracks: jest.fn(),
      }));
      return ReactModule.createElement(View, {
        ...props,
        testID: 'audio-engine',
      });
    }),
  };
});
let experience: PlaybackExperience;
let save: jest.Mock;
beforeEach(() => {
  jest.clearAllMocks();
  experience = {
    loaded: true,
    saving: false,
    error: '',
    preferences: { ...defaultPlaybackPreferences },
    bookmarks: [{ id: 'a'.repeat(64), title: 'Favorite passage', seconds: 75 }],
    change: jest.fn(),
    reset: jest.fn(),
    bookmark: jest.fn(),
    removeBookmark: jest.fn(),
  };
  save = jest.fn(async (event) => ({ ...routeItem.progress, ...event }));
});
const setup = async (withChapters = true) => {
  const view = await render(
    <CompatibilityPlayer
      item={{
        ...routeItem,
        kind: 'audiobook',
        title: 'The Long Journey',
        artist: 'Sample Author',
      }}
      source={{
        ...routeSource,
        start: 20,
        duration: 300,
        details: {
          download: '',
          next: '',
          chapters: withChapters
            ? [
                { title: 'The beginning', start: 0, end: 100 },
                { title: 'The road', start: 100, end: 300 },
              ]
            : [],
        },
      }}
      experience={experience}
      progressMessage=""
      onBack={jest.fn()}
      onRecover={jest.fn()}
      saveProgress={save}
    />,
  );
  return view;
};

it('shows a book cover fallback and music-style transport without video controls', async () => {
  const view = await setup();
  expect(view.getByLabelText('No book cover')).toBeTruthy();
  expect(view.getByText('Sample Author')).toBeTruthy();
  expect(view.getByRole('button', { name: 'Back 30 seconds' })).toBeTruthy();
  expect(
    view.queryByRole('button', { name: 'Audio, subtitles & chapters' }),
  ).toBeNull();
  expect(view.queryByRole('button', { name: 'Next episode' })).toBeNull();
  await fireEvent.press(view.getByRole('button', { name: 'Back 30 seconds' }));
  await fireEvent.press(
    view.getByRole('button', { name: 'Bookmark this moment' }),
  );
  expect(mockSeek).not.toHaveBeenCalled();
  expect(experience.bookmark).not.toHaveBeenCalled();
});
it('bounds 30-second skips, updates progress immediately and bookmarks that spot', async () => {
  const view = await setup();
  await fireEvent(view.getByTestId('audio-engine'), 'playing');
  await fireEvent.press(view.getByRole('button', { name: 'Back 30 seconds' }));
  expect(mockSeek).toHaveBeenLastCalledWith(0);
  await fireEvent.press(
    view.getByRole('button', { name: 'Forward 30 seconds' }),
  );
  expect(mockSeek).toHaveBeenLastCalledWith(30);
  expect(view.getByRole('adjustable').props.accessibilityValue.now).toBe(30);
  await fireEvent.press(
    view.getByRole('button', { name: 'Bookmark this moment' }),
  );
  expect(experience.bookmark).toHaveBeenCalledWith(30);
  await fireEvent(view.getByTestId('audio-engine'), 'progress', {
    currentTime: 295,
  });
  await fireEvent.press(
    view.getByRole('button', { name: 'Forward 30 seconds' }),
  );
  expect(mockSeek).toHaveBeenLastCalledWith(300);
});
it('saves the latest native position when playback pauses', async () => {
  const view = await setup();
  await fireEvent(view.getByTestId('audio-engine'), 'playing');
  await fireEvent(view.getByTestId('audio-engine'), 'progress', {
    currentTime: 65,
  });
  await fireEvent(view.getByTestId('audio-engine'), 'progress', {
    currentTime: 66,
  });
  await fireEvent(view.getByTestId('audio-engine'), 'paused');
  await act(async () => {});
  expect(save).toHaveBeenLastCalledWith(
    expect.objectContaining({ seconds: 66 }),
  );
  expect(view.getByRole('button', { name: 'Play' })).toBeTruthy();
});
it('applies listening speed without offering subtitles', async () => {
  const view = await setup();
  await fireEvent.press(
    view.getByRole('button', { name: 'Playback speed, 1 times' }),
  );
  await fireEvent.press(view.getByRole('button', { name: 'Speed 1.5 times' }));
  expect(experience.change).toHaveBeenCalledWith({
    ...defaultPlaybackPreferences,
    rate: 1.5,
  });
  expect(view.queryByRole('button', { name: 'Close' })).toBeNull();
});
it('selects the chapter end for sleep and jumps to saved passages', async () => {
  const view = await setup();
  await fireEvent(view.getByTestId('audio-engine'), 'playing');
  await fireEvent.press(view.getByRole('button', { name: 'Sleep timer, off' }));
  await fireEvent.press(view.getByRole('button', { name: 'End of chapter' }));
  expect(view.getByTestId('audio-engine').props.sleepPosition).toBe(100);
  await fireEvent.press(
    view.getByRole('button', { name: 'Bookmarks, 1 saved' }),
  );
  await fireEvent.press(view.getByRole('button', { name: 'Favorite passage' }));
  expect(mockSeek).toHaveBeenLastCalledWith(75);
  expect(view.getByRole('adjustable').props.accessibilityValue.now).toBe(75);
});
it('explains missing chapter markers without inventing chapters', async () => {
  const view = await setup(false);
  await fireEvent.press(view.getByRole('button', { name: 'Chapters' }));
  expect(
    view.getByText(
      'This audiobook has no chapter markers. You can still scrub or skip 30 seconds.',
    ),
  ).toBeTruthy();
  expect(view.queryByRole('button', { name: /The beginning/ })).toBeNull();
});
it('can seek back from a finished audiobook without restarting at zero', async () => {
  const view = await setup();
  await fireEvent(view.getByTestId('audio-engine'), 'playing');
  await fireEvent(view.getByTestId('audio-engine'), 'end');
  await fireEvent.press(view.getByRole('button', { name: 'Back 30 seconds' }));
  expect(view.getByTestId('audio-engine').props.source.start).toBe(270);
  expect(view.getByRole('button', { name: 'Play' })).toBeTruthy();
  expect(save).toHaveBeenLastCalledWith(
    expect.objectContaining({ seconds: 270, watched: false }),
  );
});

it.each([NaN, Infinity, -1])(
  'ignores an invalid native playback position %s',
  async (currentTime) => {
    const view = await setup();
    await fireEvent(view.getByTestId('audio-engine'), 'playing');
    await fireEvent(view.getByTestId('audio-engine'), 'progress', {
      currentTime,
    });
    expect(save).not.toHaveBeenCalled();
    expect(view.getByRole('adjustable').props.accessibilityValue.now).toBe(20);
  },
);
it('clears an expired chapter sleep timer', async () => {
  const view = await setup();
  await fireEvent(view.getByTestId('audio-engine'), 'playing');
  await fireEvent.press(view.getByRole('button', { name: 'Sleep timer, off' }));
  await fireEvent.press(view.getByRole('button', { name: 'End of chapter' }));
  await fireEvent(view.getByTestId('audio-engine'), 'progress', {
    currentTime: 100,
  });
  expect(view.getByTestId('audio-engine').props.sleepPosition).toBe(0);
  expect(view.getByRole('button', { name: 'Sleep timer, off' })).toBeTruthy();
});

const musicSetup = async (repeat: MusicQueue['repeat'] = 'off', index = 0) => {
  const items = ['first', 'second'].map((id) => ({
    ...routeItem,
    id,
    kind: 'music',
    album: 'Album',
  }));
  const select = jest.fn();
  const queue: MusicQueue = {
    items,
    index,
    repeat,
    shuffled: false,
    loading: false,
    error: '',
    select,
    shuffle: jest.fn(),
    cycleRepeat: jest.fn(),
    retry: jest.fn(),
  };
  const view = await render(
    <CompatibilityPlayer
      item={items[index]}
      source={{ ...routeSource, start: 0 }}
      musicQueue={queue}
      experience={experience}
      progressMessage=""
      onBack={jest.fn()}
      onRecover={jest.fn()}
      saveProgress={save}
    />,
  );
  await fireEvent(view.getByTestId('audio-engine'), 'playing');
  return { view, select };
};
it('advances album playback after saving the completed song', async () => {
  const { view, select } = await musicSetup();
  await fireEvent(view.getByTestId('audio-engine'), 'end');
  await waitFor(() => expect(select).toHaveBeenCalledWith('second'));
  expect(save).toHaveBeenCalledWith(
    expect.objectContaining({ seconds: 120, watched: true }),
  );
});
it('stops at the last song when repeat is off', async () => {
  const { view, select } = await musicSetup('off', 1);
  await fireEvent(view.getByTestId('audio-engine'), 'end');
  expect(select).not.toHaveBeenCalled();
  expect(view.getByRole('button', { name: 'Replay' })).toBeTruthy();
  expect(
    view.getByRole('button', { name: 'Next track' }).props.accessibilityState
      .disabled,
  ).toBe(true);
});
it('repeats a track from zero without selecting a different song', async () => {
  const { view, select } = await musicSetup('one');
  await fireEvent(view.getByTestId('audio-engine'), 'end');
  expect(select).not.toHaveBeenCalled();
  expect(view.getByTestId('audio-engine').props.source.start).toBe(0);
  expect(view.getByTestId('audio-engine').props.paused).toBe(false);
});
it('exposes an ordered selectable queue and keeps audiobook tools out of music', async () => {
  const { view, select } = await musicSetup();
  expect(view.queryByRole('button', { name: 'Chapters' })).toBeNull();
  await fireEvent.press(view.getByRole('button', { name: 'Queue, 2 tracks' }));
  await fireEvent.press(view.getByRole('button', { name: '2. Arrival' }));
  await waitFor(() => expect(select).toHaveBeenCalledWith('second'));
});
