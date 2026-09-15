import { fireEvent, render, within } from '@testing-library/react-native';
import React from 'react';
import * as ReactNative from 'react-native';

import type { MediaItem } from '@/core/contract';

import { DetailView } from './detail-view';

jest.mock('expo-image', () => {
  const ReactModule = jest.requireActual<typeof import('react')>('react');
  const { View } =
    jest.requireActual<typeof import('react-native')>('react-native');
  return {
    Image: (props: object) => ReactModule.createElement(View, props),
  };
});
jest.mock('expo-linear-gradient', () => {
  const ReactModule = jest.requireActual<typeof import('react')>('react');
  const { View } =
    jest.requireActual<typeof import('react-native')>('react-native');
  return {
    LinearGradient: (props: object) =>
      ReactModule.createElement(View, {
        ...props,
        testID: 'backdrop-gradient',
      }),
  };
});

const item: MediaItem = {
  id: 'arrival',
  kind: 'video',
  title: 'Arrival',
  year: '2016',
  plot: 'A linguist works to understand visitors from another world.',
  rating: 'PG-13',
  tagline: 'Why are they here?',
  genres: 'Science Fiction · Drama',
  director: 'Denis Villeneuve',
  studio: 'Paramount',
  artist: '',
  album: '',
  show: '',
  season: 0,
  episode: 0,
  artwork: '/art/arrival',
  backdrop: '/backdrop/arrival',
  container: 'mkv',
  progress: { seconds: 1820, watched: false, session: '', revision: 2 },
};
describe('DetailView', () => {
  const originalTV = Object.getOwnPropertyDescriptor(
    ReactNative.Platform,
    'isTV',
  )!;
  afterEach(() => jest.restoreAllMocks());

  afterEach(() => {
    Object.defineProperty(ReactNative.Platform, 'isTV', originalTV);
  });

  it('uses a wider poster on a regular tablet layout', async () => {
    jest
      .spyOn(
        jest.requireActual<typeof ReactNative>('react-native'),
        'useWindowDimensions',
      )
      .mockReturnValue({ width: 1024, height: 768, scale: 2, fontScale: 1 });
    const view = await render(
      <DetailView
        item={{ ...item, backdrop: '' }}
        mediaURL={(path) => path}
        headers={{}}
        onBack={jest.fn()}
        onPlay={jest.fn()}
      />,
    );
    expect(
      ReactNative.StyleSheet.flatten(
        view.getByLabelText('Poster for Arrival').props.style,
      ).width,
    ).toBe(260);
  });

  it('stacks artwork and copy for accessibility text sizes', async () => {
    jest
      .spyOn(
        jest.requireActual<typeof ReactNative>('react-native'),
        'useWindowDimensions',
      )
      .mockReturnValue({
        fontScale: 3.2,
        height: 1024,
        scale: 2,
        width: 1366,
      });

    const view = await render(
      <DetailView
        headers={{ Authorization: 'Bearer token' }}
        item={{ ...item, backdrop: '' }}
        mediaURL={(path) => path}
        onBack={jest.fn()}
        onPlay={jest.fn()}
      />,
    );

    expect(
      ReactNative.StyleSheet.flatten(
        view.getByLabelText('Poster for Arrival').props.style,
      ),
    ).toEqual(expect.objectContaining({ width: 220 }));
  });

  it('shows useful media facts and resumes playback', async () => {
    const onPlay = jest.fn();
    const view = await render(
      <DetailView
        headers={{ Authorization: 'Bearer token' }}
        item={{ ...item, backdrop: '' }}
        mediaURL={(path) => path}
        onBack={jest.fn()}
        onPlay={onPlay}
      />,
    );

    expect(view.getByRole('header', { name: 'Arrival' })).toBeTruthy();
    expect(view.getByText('2016 · PG-13 · MKV')).toBeTruthy();
    expect(view.getByText(item.plot)).toBeTruthy();
    await fireEvent.press(view.getByRole('button', { name: 'Resume' }));
    expect(onPlay).toHaveBeenCalledTimes(1);
  });

  it.each([
    [false, 799, 1, 'column', 220],
    [false, 800, 1, 'row', 260],
    [false, 800, 1.5, 'column', 220],
    [true, 320, 3.2, 'row', 260],
  ] as const)(
    'uses the exact responsive layout at TV=%s width=%s scale=%s',
    async (tv, width, fontScale, direction, posterWidth) => {
      Object.defineProperty(ReactNative.Platform, 'isTV', {
        configurable: true,
        value: tv,
      });
      jest
        .spyOn(
          jest.requireActual<typeof ReactNative>('react-native'),
          'useWindowDimensions',
        )
        .mockReturnValue({ width, height: 800, scale: 2, fontScale });
      const view = await render(
        <DetailView
          item={{ ...item, backdrop: '' }}
          mediaURL={(path) => path}
          headers={{}}
          onBack={jest.fn()}
          onPlay={jest.fn()}
        />,
      );
      const content = view.getByLabelText('Poster for Arrival').parent!;
      expect(ReactNative.StyleSheet.flatten(content.props.style)).toMatchObject(
        {
          flexDirection: direction,
        },
      );
      expect(
        ReactNative.StyleSheet.flatten(
          view.getByLabelText('Poster for Arrival').props.style,
        ).width,
      ).toBe(posterWidth);
    },
  );
});

it('offers reading for ebooks rather than video playback', async () => {
  const onRead = jest.fn();
  const view = await render(
    <DetailView
      item={{ ...item, kind: 'book', container: 'epub' }}
      headers={{}}
      mediaURL={(path) => path}
      onPlay={onRead}
      onBack={jest.fn()}
    />,
  );
  expect(view.queryByRole('button', { name: 'Play' })).toBeNull();
  expect(
    view.getByText(
      'Opens your server’s reader in your browser. You may need to sign in.',
    ),
  ).toBeTruthy();
  await fireEvent.press(view.getByRole('button', { name: 'Read ebook' }));
  expect(onRead).toHaveBeenCalledTimes(1);
});

it('keeps phone playback, download, and return actions available', async () => {
  const onPlay = jest.fn(),
    onDownload = jest.fn(),
    onBack = jest.fn();
  const view = await render(
    <DetailView
      item={item}
      mediaURL={(path) => path}
      headers={{}}
      onPlay={onPlay}
      onDownload={onDownload}
      onBack={onBack}
    />,
  );
  await fireEvent.press(view.getByRole('button', { name: 'Resume' }));
  await fireEvent.press(
    view.getByRole('button', { name: 'More title actions' }),
  );
  await fireEvent.press(view.getByRole('button', { name: 'Download' }));
  await fireEvent.press(view.getByRole('button', { name: 'Close' }));
  await fireEvent.press(view.getByRole('button', { name: 'Back' }));
  expect(onPlay).toHaveBeenCalledTimes(1);
  expect(onDownload).toHaveBeenCalledTimes(1);
  expect(onBack).toHaveBeenCalledTimes(1);
});

it('contains series artwork and hides an unavailable poster on details', async () => {
  const view = await render(
    <DetailView
      item={{ ...item, show: 'Series', backdrop: '' }}
      mediaURL={(path) => path}
      headers={{}}
      onPlay={jest.fn()}
      onBack={jest.fn()}
    />,
  );
  const image = view.getByLabelText('Poster for Arrival');
  expect(image.props.contentFit).toBe('contain');
  await fireEvent(image, 'error');
  expect(view.queryByLabelText('Poster for Arrival')).toBeNull();
  expect(view.getByRole('button', { name: 'Resume' })).toBeTruthy();
});

it.each([320, 390, 1024])(
  'keeps primary actions outside the scrolling detail at width %s',
  async (width) => {
    const dimensions = jest
      .spyOn(ReactNative, 'useWindowDimensions')
      .mockReturnValue({ width, height: 844, scale: 2, fontScale: 1 });
    const view = await render(
      <DetailView
        item={item}
        headers={{}}
        mediaURL={(path) => path}
        onPlay={jest.fn()}
        onBack={jest.fn()}
      />,
    );
    const body = within(
      view.container.queryAll((node) => /ScrollView$/.test(node.type))[0],
    );
    expect(body.queryByRole('button', { name: 'Resume' })).toBeNull();
    expect(body.queryByRole('button', { name: 'Back' })).toBeNull();
    expect(view.getAllByRole('button', { name: 'Back' })).toHaveLength(1);
    expect(view.getAllByRole('button', { name: 'Resume' })).toHaveLength(1);
    dimensions.mockRestore();
  },
);

it('restarts a completed title instead of resuming at the end', async () => {
  const onPlay = jest.fn(),
    onPlayFromBeginning = jest.fn();
  const view = await render(
    <DetailView
      item={{ ...item, progress: { ...item.progress, watched: true } }}
      mediaURL={(path) => path}
      headers={{}}
      onBack={jest.fn()}
      onPlay={onPlay}
      onPlayFromBeginning={onPlayFromBeginning}
    />,
  );
  expect(view.queryByRole('button', { name: 'Resume' })).toBeNull();
  await fireEvent.press(view.getByRole('button', { name: 'Play again' }));
  expect(onPlayFromBeginning).toHaveBeenCalledTimes(1);
  expect(onPlay).not.toHaveBeenCalled();
});
