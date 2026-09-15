import { fireEvent, render } from '@testing-library/react-native';
import React from 'react';
import * as ReactNative from 'react-native';

import type { Home } from '@/core/contract';

import { HomeView } from './home-view';

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
      ReactModule.createElement(View, { ...props, testID: 'home-gradient' }),
  };
});

const home: Home = {
  server: 'Den Server',
  viewer: { id: 'viewer-1', name: 'Mike', owner: false },
  continueWatching: [
    {
      id: 'arrival',
      kind: 'video',
      title: 'Arrival',
      year: '2016',
      plot: 'A linguist works to understand visitors from another world.',
      rating: 'PG-13',
      tagline: '',
      genres: 'Science Fiction',
      director: 'Denis Villeneuve',
      studio: '',
      artist: '',
      album: '',
      show: '',
      season: 0,
      episode: 0,
      artwork: '/art/arrival',
      backdrop: '/backdrop/arrival',
      container: 'mkv',
      progress: { seconds: 1820, watched: false, session: '', revision: 2 },
    },
  ],
  recent: [
    {
      id: 'moon',
      kind: 'video',
      title: 'Moon',
      year: '2009',
      plot: '',
      rating: '',
      tagline: '',
      genres: '',
      director: '',
      studio: '',
      artist: '',
      album: '',
      show: '',
      season: 0,
      episode: 0,
      artwork: '',
      backdrop: '',
      container: 'mp4',
      progress: { seconds: 0, watched: false, session: '', revision: 0 },
    },
  ],
};
describe('HomeView content', () => {
  afterEach(() => jest.restoreAllMocks());

  it('shows useful recovery when no media exists', async () => {
    const view = await render(
      <HomeView
        home={{ ...home, recent: [], continueWatching: [] }}
        mediaURL={(path) => path}
        onOpen={jest.fn()}
      />,
    );
    const title = view.getByRole('header', {
      name: 'Your library is ready for its first title.',
    });
    expect(title.props.style).toEqual([
      {
        fontSize: 42,
        fontWeight: '700',
        letterSpacing: -1.4,
        lineHeight: 46,
      },
      { color: '#F6F8EF' },
    ]);
    expect(
      view.getByText('Add media on Kinosail Server, then refresh this screen.')
        .props.style,
    ).toEqual([
      { fontSize: 16, lineHeight: 24, maxWidth: 620 },
      { color: '#9CA391' },
    ]);
    expect(view.queryByLabelText('Recently added shelf')).toBeNull();
    expect(view.queryByRole('button', { name: 'Resume' })).toBeNull();
  });

  it.each(['2009', ''])(
    'uses recent media when resume history is empty %#',
    async (year) => {
      jest
        .spyOn(
          jest.requireActual<typeof ReactNative>('react-native'),
          'useWindowDimensions',
        )
        .mockReturnValue({ width: 320, height: 640, scale: 1, fontScale: 0.8 });
      const onOpen = jest.fn();
      const view = await render(
        <HomeView
          home={{
            ...home,
            continueWatching: [],
            recent: [{ ...home.recent[0], year }],
          }}
          mediaURL={(path) => path}
          onOpen={onOpen}
          onTheme={jest.fn()}
        />,
      );
      expect(
        view.getByRole('button', { name: 'Account options for Mike' }),
      ).toBeTruthy();
      expect(
        view.getAllByText(year || 'Ready to play.').length,
      ).toBeGreaterThan(0);
      await fireEvent.press(view.getByRole('button', { name: 'View details' }));
      expect(onOpen).toHaveBeenCalledWith('moon');
    },
  );

  it('shows both year and genre when featured media has no plot', async () => {
    const view = await render(
      <HomeView
        home={{
          ...home,
          continueWatching: [],
          recent: [
            {
              ...home.recent[0],
              genres: 'Science Fiction',
              year: '2009',
            },
          ],
        }}
        mediaURL={(path) => path}
        onOpen={jest.fn()}
      />,
    );
    expect(view.getByText('2009 · Science Fiction')).toBeTruthy();
  });

  it('lets an empty library refresh without leaving the app', async () => {
    const onRefresh = jest.fn();
    const view = await render(
      <HomeView
        home={{ ...home, recent: [], continueWatching: [] }}
        mediaURL={(path) => path}
        onOpen={jest.fn()}
        onRefresh={onRefresh}
      />,
    );
    await fireEvent.press(
      view.getByRole('button', { name: 'Refresh library' }),
    );
    expect(onRefresh).toHaveBeenCalledTimes(1);
  });

  it('keeps account actions and shelf cards usable with accessibility text', async () => {
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
      <HomeView
        home={home}
        mediaURL={(path) => path}
        onOpen={jest.fn()}
        onSignOut={jest.fn()}
        onTheme={jest.fn()}
        themeLabel="Dark"
      />,
    );

    expect(view.queryByText('Den Server')).toBeNull();
    expect(
      view.getByRole('button', { name: 'Account options for Mike' }),
    ).toBeTruthy();
    await fireEvent.press(
      view.getByRole('button', { name: 'Account options for Mike' }),
    );
    expect(view.getByRole('button', { name: 'Theme: Dark' })).toBeTruthy();
    expect(view.getByRole('button', { name: 'Sign out' })).toBeTruthy();
    expect(
      view.getByRole('button', {
        name: 'Resume, Arrival, 2016, In progress',
      }).props.style,
    ).toEqual(
      expect.arrayContaining([expect.objectContaining({ width: 360 })]),
    );
    const cardTitle = view.getAllByText('Arrival')[1];
    expect(
      ReactNative.StyleSheet.flatten(cardTitle.props.style).height,
    ).toBeUndefined();
    expect(cardTitle.props.maxFontSizeMultiplier).toBe(2);
  });

  it('puts resume content before recent additions', async () => {
    const view = await render(
      <HomeView home={home} mediaURL={(path) => path} onOpen={jest.fn()} />,
    );

    const headings = view
      .getAllByRole('header')
      .map((heading) => heading.props.children);
    expect(headings).toEqual(
      expect.arrayContaining(['Continue watching', 'Recently added']),
    );
    expect(view.getAllByText('Arrival')).toHaveLength(2);
    expect(view.getByText('2016 · 30 min watched')).toBeTruthy();
    expect(view.getAllByText('Moon')).toHaveLength(2);
    expect(view.getByLabelText('Continue watching shelf')).toBeTruthy();
    expect(view.getByLabelText('Recently added shelf')).toBeTruthy();
  });

  it('opens the selected media item', async () => {
    const onOpen = jest.fn();
    const view = await render(
      <HomeView home={home} mediaURL={(path) => path} onOpen={onOpen} />,
    );

    await fireEvent.press(
      view.getByRole('button', { name: 'Resume, Arrival, 2016, In progress' }),
    );

    expect(onOpen).toHaveBeenCalledWith('arrival');
  });
});
