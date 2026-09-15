import { render } from '@testing-library/react-native';
import React from 'react';
import * as ReactNative from 'react-native';

import type { Home } from '@/core/contract';

import { HomeView } from './home-view';
import { HomeSkeleton } from './home-skeleton';

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
type NativeNode = NonNullable<Awaited<ReturnType<typeof render>>['root']>;
const child = (node: NativeNode, index: number): NativeNode => {
  const value = node.children[index];
  if (typeof value === 'string') throw new Error('Expected a native element.');
  return value;
};

describe('HomeView layout', () => {
  const originalOS = Object.getOwnPropertyDescriptor(
    ReactNative.Platform,
    'OS',
  )!;
  afterEach(() => {
    jest.restoreAllMocks();
    Object.defineProperty(ReactNative.Platform, 'OS', originalOS);
  });

  it.each([
    [359, 1, false, true, 258.48],
    [360, 1, false, true, 259.2],
    [419, 1, false, true, 301.68],
    [420, 1, false, true, 302.4],
    [979, 1, false, true, 360],
    [980, 1, true, true, 360],
    [1366, 1.5, false, true, 360],
  ] as const)(
    'honors home reflow at width=%s scale=%s',
    async (width, fontScale, wide, viewer, cardWidth) => {
      jest
        .spyOn(
          jest.requireActual<typeof ReactNative>('react-native'),
          'useWindowDimensions',
        )
        .mockReturnValue({ width, height: 900, scale: 2, fontScale });
      const view = await render(
        <HomeView home={home} mediaURL={(path) => path} onOpen={jest.fn()} />,
      );
      expect(Boolean(view.queryByText('KINOSAIL'))).toBe(
        width >= 360 || fontScale >= 1.5,
      );
      expect(Boolean(view.queryByText('Den Server'))).toBe(wide);
      expect(Boolean(view.queryByText('Mike'))).toBe(viewer);
      expect(
        ReactNative.StyleSheet.flatten(
          view.getByRole('button', {
            name: 'Resume, Arrival, 2016, In progress',
          }).props.style,
        ).width,
      ).toBe(cardWidth);
      expect(view.getAllByText('Arrival')[0].props.numberOfLines).toBe(
        fontScale >= 1.5 ? undefined : 2,
      );
    },
  );

  it.each([
    [320, 844, 1],
    [390, 844, 1],
    [844, 390, 1],
    [1024, 1366, 1],
    [390, 844, 2],
  ])(
    'keeps loading geometry aligned with Home at %s x %s scale %s',
    async (width, height, fontScale) => {
      jest
        .spyOn(
          jest.requireActual<typeof ReactNative>('react-native'),
          'useWindowDimensions',
        )
        .mockReturnValue({ width, height, fontScale, scale: 2 });
      const loaded = await render(
        <HomeView home={home} mediaURL={(path) => path} onOpen={jest.fn()} />,
      );
      const heroStyle = ReactNative.StyleSheet.flatten(
        loaded.getByText('PICK UP WHERE YOU LEFT OFF').parent!.props.style,
      );
      const resume = ReactNative.StyleSheet.flatten(
        loaded.getByRole('button', {
          name: 'Resume, Arrival, 2016, In progress',
        }).props.style,
      );
      const poster = ReactNative.StyleSheet.flatten(
        loaded.getAllByRole('button', {
          name: 'Moon, 2009',
        })[0].props.style,
      );
      await loaded.unmount();
      const loading = await render(<HomeSkeleton />);
      expect(
        ReactNative.StyleSheet.flatten(
          loading.getByTestId('home-skeleton-hero').props.style,
        ),
      ).toEqual(heroStyle);
      expect(
        ReactNative.StyleSheet.flatten(
          loading.getAllByTestId('home-skeleton-resume')[0].props.style,
        ),
      ).toMatchObject({ width: resume.width, aspectRatio: 16 / 9 });
      expect(
        ReactNative.StyleSheet.flatten(
          loading.getAllByTestId('home-skeleton-poster')[0].props.style,
        ),
      ).toMatchObject({ width: poster.width, aspectRatio: 2 / 3 });
      expect(loading.getAllByRole('progressbar')).toHaveLength(1);
      expect(
        loading.getByRole('progressbar', { name: 'Loading your library…' })
          .props.accessibilityState,
      ).toEqual({ busy: true });
      expect(loading.queryAllByRole('button')).toHaveLength(0);
    },
  );

  it('preserves the home composition and platform list behavior', async () => {
    Object.defineProperty(ReactNative.Platform, 'OS', {
      configurable: true,
      value: 'web',
    });
    jest
      .spyOn(
        jest.requireActual<typeof ReactNative>('react-native'),
        'useWindowDimensions',
      )
      .mockReturnValue({ width: 390, height: 844, scale: 3, fontScale: 1 });
    const view = await render(
      <HomeView
        home={home}
        mediaURL={(path) => `https://kino.example${path}`}
        headers={{ Authorization: 'Bearer token' }}
        onOpen={jest.fn()}
        onTheme={jest.fn()}
        themeLabel="Dark"
        onSignOut={jest.fn()}
      />,
    );
    expect(view.root!.props.role).toBe('main');
    expect(ReactNative.StyleSheet.flatten(view.root!.props.style)).toEqual({
      backgroundColor: '#090A08',
      flex: 1,
    });
    const backdrop = view.getByLabelText('Backdrop for Arrival');
    expect(backdrop.props).toMatchObject({
      accessibilityIgnoresInvertColors: true,
      accessibilityLabel: 'Backdrop for Arrival',
      alt: 'Backdrop for Arrival',
      cachePolicy: 'memory-disk',
      contentFit: 'cover',
      enforceEarlyResizing: true,
      priority: 'high',
      source: {
        uri: 'https://kino.example/backdrop/arrival',
        headers: { Authorization: 'Bearer token' },
      },
    });
    expect(ReactNative.StyleSheet.flatten(backdrop.props.style)).toEqual({
      height: expect.any(Number),
      opacity: 1,
      position: 'absolute',
      right: 0,
      top: 0,
      left: 0,
    });
    expect(view.getAllByTestId('home-gradient').at(-1)!.props).toMatchObject({
      colors: ['#090A0810', '#090A08B8', '#090A08'],
      locations: [0, 0.5, 1],
      pointerEvents: 'none',
    });
    const brand = view.getByRole('image', { name: 'Kinosail Player' });
    const header = brand.parent!.parent!;
    expect(ReactNative.StyleSheet.flatten(header.props.style)).toEqual({
      alignItems: 'center',
      flexDirection: 'row',
      justifyContent: 'space-between',
      minHeight: 76,
      paddingHorizontal: 20,
      paddingVertical: 8,
    });
    expect(
      ReactNative.StyleSheet.flatten(child(header, 1).props.style),
    ).toEqual({
      alignItems: 'center',
      flexDirection: 'row',
      gap: 8,
      flexShrink: 1,
    });
    expect(
      view.getByLabelText('Continue watching shelf').props
        .removeClippedSubviews,
    ).toBe(false);
    expect(view.getByText('PICK UP WHERE YOU LEFT OFF').props.style).toEqual([
      { fontSize: 11, fontWeight: '700', letterSpacing: 1.6 },
      { color: '#9CA391' },
    ]);
    expect(view.getByText(home.continueWatching[0].plot).props).toMatchObject({
      numberOfLines: 2,
    });
    expect(view.getByText(home.continueWatching[0].plot).props.style).toEqual([
      { fontSize: 16, lineHeight: 24, maxWidth: 620 },
      { fontSize: 15, lineHeight: 22 },
      { color: '#9CA391' },
    ]);
    expect(view.getAllByText('Arrival')[0].props).toMatchObject({
      numberOfLines: 2,
      style: [
        {
          fontSize: 42,
          fontWeight: '700',
          letterSpacing: -1.4,
          lineHeight: 46,
        },
        { fontSize: 42, lineHeight: 47, letterSpacing: -1.2 },
        { color: '#F6F8EF' },
      ],
    });
    expect(view.getByText('Continue watching').props.style).toEqual([
      {
        fontSize: 21,
        fontWeight: '600',
        letterSpacing: -0.4,
      },
      { color: '#F6F8EF', flexShrink: 1 },
    ]);
    expect(
      view.getByRole('button', { name: 'Account options for Mike' }),
    ).toBeTruthy();
    let content = view.getByText('PICK UP WHERE YOU LEFT OFF').parent;
    while (content && content.props.contentContainerStyle === undefined) {
      content = content.parent;
    }
    expect(content).not.toBeNull();
    expect(
      ReactNative.StyleSheet.flatten(content!.props.contentContainerStyle),
    ).toEqual({
      gap: 24,
      paddingBottom: 160,
      paddingHorizontal: 20,
      paddingTop: 76,
    });
  });

  it.each([
    [jest.fn(), undefined],
    [undefined, 'Dark'],
  ] as const)(
    'requires both theme action inputs %#',
    async (onTheme, label) => {
      const view = await render(
        <HomeView
          home={home}
          mediaURL={(path) => path}
          onOpen={jest.fn()}
          onTheme={onTheme}
          themeLabel={label}
        />,
      );
      expect(view.queryByRole('button', { name: 'Theme: Dark' })).toBeNull();
    },
  );
});
