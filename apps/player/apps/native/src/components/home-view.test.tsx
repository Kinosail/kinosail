import { fireEvent, render } from '@testing-library/react-native';
import React from 'react';
import * as ReactNative from 'react-native';

import type { Home } from '@/core/contract';

import { HomeView } from './home-view';
import { homeViewStyles } from './home-view.styles';

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
describe('HomeView', () => {
  const originalTV = Object.getOwnPropertyDescriptor(
    ReactNative.Platform,
    'isTV',
  )!;
  const originalOS = Object.getOwnPropertyDescriptor(
    ReactNative.Platform,
    'OS',
  )!;
  afterEach(() => {
    jest.restoreAllMocks();
    Object.defineProperty(ReactNative.Platform, 'isTV', originalTV);
    Object.defineProperty(ReactNative.Platform, 'OS', originalOS);
  });

  it('keeps the compact home design geometry exact', () => {
    const expected = {
      screen: { flex: 1 },
      backdrop: {
        height: '70%',
        opacity: 1,
        position: 'absolute',
        left: 0,
        right: 0,
        top: 0,
      },
      safeArea: { flex: 1 },
      header: {
        alignItems: 'center',
        flexDirection: 'row',
        justifyContent: 'space-between',
        minHeight: 76,
        paddingVertical: 8,
      },
      headerAccessible: {
        alignItems: 'flex-start',
        flexDirection: 'column',
      },
      account: {
        alignItems: 'center',
        flexDirection: 'row',
        gap: 8,
        flexShrink: 1,
      },
      accountAccessible: { alignItems: 'flex-start', flexWrap: 'wrap' },
      server: { fontSize: 12, fontWeight: '700' },
      viewer: { fontSize: 13, fontWeight: '600' },
      signOut: { minHeight: 44, paddingHorizontal: 16 },
      theme: { minHeight: 44, paddingHorizontal: 16 },
      content: { gap: 48, paddingBottom: 64 },
      contentCompact: { gap: 24 },
      quickActions: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
      quickAction: { borderRadius: 24, paddingHorizontal: 16, minHeight: 44 },
      accountButton: {
        borderRadius: 24,
        minHeight: 44,
        paddingHorizontal: 16,
        maxWidth: 240,
        flexShrink: 1,
      },
      heroCompact: {
        minHeight: 470,
        paddingTop: 220,
        paddingBottom: 24,
        justifyContent: 'flex-end',
        gap: 16,
      },
      heroTitleCompact: { fontSize: 42, lineHeight: 47, letterSpacing: -1.2 },
      heroSummaryCompact: { fontSize: 15, lineHeight: 22 },
      hero: { gap: 16, maxWidth: 680, minHeight: 260, paddingTop: 40 },
      eyebrow: { fontSize: 11, fontWeight: '700', letterSpacing: 1.6 },
      heroTitle: {
        fontSize: 42,
        fontWeight: '700',
        letterSpacing: -1.4,
        lineHeight: 46,
      },
      heroSummary: { fontSize: 16, lineHeight: 24, maxWidth: 620 },
      heroAction: { alignSelf: 'flex-start', minWidth: 148 },
      empty: {
        gap: 16,
        justifyContent: 'center',
        maxWidth: 660,
        minHeight: 320,
      },
      shelf: { gap: 16 },
      shelfTitle: { fontSize: 21, fontWeight: '600', letterSpacing: -0.4 },
      shelfRow: { paddingBottom: 4 },
    } as const;
    expect(Object.keys(homeViewStyles)).toEqual(Object.keys(expected));
    for (const name of Object.keys(expected) as (keyof typeof expected)[]) {
      expect(ReactNative.StyleSheet.flatten(homeViewStyles[name])).toEqual(
        expected[name],
      );
    }
  });

  it.each([720, 1080])(
    'keeps TV browsing in the header and bounds the hero at %s pixels',
    async (height) => {
      Object.defineProperty(ReactNative.Platform, 'isTV', {
        configurable: true,
        value: true,
      });
      jest
        .spyOn(
          jest.requireActual<typeof ReactNative>('react-native'),
          'useWindowDimensions',
        )
        .mockReturnValue({
          width: (height * 16) / 9,
          height,
          scale: 1,
          fontScale: 1,
        });
      const onBrowse = jest.fn();
      const onPlay = jest.fn();
      const view = await render(
        <HomeView
          home={home}
          mediaURL={(path) => path}
          onPlay={onPlay}
          onOpen={jest.fn()}
          onBrowse={onBrowse}
        />,
      );
      const scroll = view.container.queryAll((node) =>
        /ScrollView$/.test(node.type),
      )[0];
      expect(
        scroll.queryAll((node) => node.props.label === 'Search & browse'),
      ).toHaveLength(0);
      expect(
        scroll.queryAll((node) => node.props.alt === 'Backdrop for Arrival'),
      ).toHaveLength(0);
      expect(
        ReactNative.StyleSheet.flatten(
          view.getByTestId('tv-feature').props.style,
        ).height,
      ).toBeLessThan(height / 2);
      await fireEvent.press(view.getByRole('button', { name: 'Search' }));
      expect(onBrowse).toHaveBeenCalledTimes(1);
      expect(view.getByLabelText('Continue watching shelf')).toBeTruthy();
      const failedBackdrop = view.getByLabelText('Backdrop for Arrival');
      await fireEvent(failedBackdrop, 'error');
      expect(view.queryByLabelText('Backdrop for Arrival')).toBeNull();
      expect(view.getByRole('button', { name: 'Resume' })).toBeTruthy();
    },
  );

  // The first FlatList mount also transforms React Native modules under mutation instrumentation.
  it('provides fixed row geometry for virtualized shelves', async () => {
    const native = jest.requireActual<typeof ReactNative>('react-native');
    jest
      .spyOn(native, 'useWindowDimensions')
      .mockReturnValue({ width: 390, height: 844, fontScale: 1, scale: 3 });
    const layouts: { index: number; length: number; offset: number }[] = [];
    const BaseList = native.FlatList;
    class MeasuredList<T> extends BaseList<T> {
      render() {
        if (this.props.getItemLayout)
          layouts.push(this.props.getItemLayout(this.props.data, 3));
        return super.render();
      }
    }
    jest.spyOn(native, 'FlatList', 'get').mockReturnValue(MeasuredList);
    const view = await render(
      <HomeView
        home={{
          ...home,
          continueWatching: [...home.continueWatching, ...home.recent],
        }}
        mediaURL={(path) => path}
        onOpen={jest.fn()}
      />,
    );
    const shelf = view.getByLabelText('Continue watching shelf');
    expect(shelf.props).toMatchObject({
      accessibilityLabel: 'Continue watching shelf',
      accessibilityRole: 'list',
      contentContainerStyle: { paddingBottom: 4 },
      horizontal: true,
      initialNumToRender: 4,
      maxToRenderPerBatch: 6,
      removeClippedSubviews: true,
      showsHorizontalScrollIndicator: false,
      updateCellsBatchingPeriod: 16,
      windowSize: 3,
    });
    expect(shelf.props.keyExtractor(home.continueWatching[0])).toBe('arrival');
    expect(shelf.props.ItemSeparatorComponent().props.style).toEqual({
      width: 12,
    });
    const rendered = shelf.props.renderItem({
      index: 0,
      item: home.continueWatching[0],
      separators: {},
    });
    expect(rendered.props.role).toBe('listitem');
    expect(rendered.props.children.props).toMatchObject({
      headers: undefined,
      imageURL: '/backdrop/arrival',
      item: home.continueWatching[0],
      width: 280.8,
    });
    expect(
      layouts.some(
        (layout) =>
          layout.index === 3 &&
          Math.abs(layout.length - 292.8) < 0.001 &&
          Math.abs(layout.offset - 878.4) < 0.001,
      ),
    ).toBe(true);
  }, 30000);

  it.each([false, true])(
    'keeps shelf virtualization and actions usable on TV=%s',
    async (tv) => {
      Object.defineProperty(ReactNative.Platform, 'isTV', {
        configurable: true,
        value: tv,
      });
      jest
        .spyOn(
          jest.requireActual<typeof ReactNative>('react-native'),
          'useWindowDimensions',
        )
        .mockReturnValue({ width: 1280, height: 720, scale: 1, fontScale: 1 });
      const onOpen = jest.fn();
      const onTheme = jest.fn();
      const onSignOut = jest.fn();
      const headers = { Authorization: 'Bearer test-viewer' };
      const view = await render(
        <HomeView
          home={home}
          mediaURL={(path) => `https://kino.example${path}`}
          headers={headers}
          onOpen={onOpen}
          onTheme={onTheme}
          themeLabel="Dark"
          onSignOut={onSignOut}
        />,
      );
      expect(Boolean(view.queryByText('Den Server'))).toBe(!tv);
      if (tv)
        await fireEvent.press(
          view.getByRole('button', { name: 'Account options for Mike' }),
        );
      expect(view.getByText('Mike')).toBeTruthy();
      if (tv)
        await fireEvent.press(view.getByRole('button', { name: 'Close' }));
      if (!tv) {
        expect(view.getByText('Den Server').props).toMatchObject({
          numberOfLines: 1,
          style: [{ fontSize: 12, fontWeight: '700' }, { color: '#9CA391' }],
        });
      }
      if (tv)
        expect(
          view.getByLabelText('Backdrop for Arrival').props.source,
        ).toMatchObject({
          uri: 'https://kino.example/backdrop/arrival',
          headers,
        });
      const shelf = view.getByLabelText('Continue watching shelf');
      if (tv) expect(shelf.props.removeClippedSubviews).toBe(false);
      else expect(shelf.props.initialNumToRender).toBe(4);
      expect(shelf.props.getItemLayout(shelf.props.data, 3)).toEqual({
        index: 3,
        length: (tv ? 269 : 360) + (tv ? 24 : 12),
        offset: ((tv ? 269 : 360) + (tv ? 24 : 12)) * 3,
      });
      expect(
        ReactNative.StyleSheet.flatten(
          view.getByRole('button', {
            name: tv
              ? 'Arrival, 2016, In progress'
              : 'Resume, Arrival, 2016, In progress',
          }).props.style,
        ).width,
      ).toBe(tv ? 269 : 360);
      await fireEvent.press(
        view.getByRole('button', {
          name: tv ? 'View details' : 'Resume, Arrival, 2016, In progress',
        }),
      );
      expect(onOpen).toHaveBeenCalledWith('arrival');
      if (tv)
        await fireEvent.press(
          view.getByRole('button', { name: 'Account options for Mike' }),
        );
      await fireEvent.press(view.getByRole('button', { name: 'Theme: Dark' }));
      await fireEvent.press(view.getByRole('button', { name: 'Sign out' }));
      expect(onTheme).toHaveBeenCalledTimes(1);
      expect(onSignOut).toHaveBeenCalledTimes(1);
    },
  );
});

it('resumes directly from a landscape card and links each media shelf to its category', async () => {
  const onPlay = jest.fn(),
    onBrowse = jest.fn();
  const series = { ...home.recent[0], id: 'series', show: 'Visitors' };
  const view = await render(
    <HomeView
      home={{ ...home, recent: [...home.recent, series] }}
      mediaURL={(path) => path}
      onOpen={jest.fn()}
      onPlay={onPlay}
      onBrowse={onBrowse}
    />,
  );
  await fireEvent.press(
    view.getByRole('button', { name: 'Resume, Arrival, 2016, In progress' }),
  );
  expect(onPlay).toHaveBeenCalledWith('arrival');
  await fireEvent.press(view.getByRole('button', { name: 'See all movies' }));
  expect(onBrowse).toHaveBeenLastCalledWith('movies');
  await fireEvent.press(view.getByRole('button', { name: 'See all tv shows' }));
  expect(onBrowse).toHaveBeenLastCalledWith('shows');
});

it('keeps shelves clear when the overlay header grows', async () => {
  const view = await render(
    <HomeView home={home} mediaURL={(path) => path} onOpen={jest.fn()} />,
  );
  const backdrop = view.getByLabelText('Backdrop for Arrival');
  expect(backdrop.props.contentPosition).toBe('center');
  expect(ReactNative.StyleSheet.flatten(backdrop.props.style)).toMatchObject({
    left: 0,
    right: 0,
  });
  expect(
    ReactNative.StyleSheet.flatten(backdrop.props.style).width,
  ).toBeUndefined();
  expect(
    ReactNative.StyleSheet.flatten(
      view.getByRole('image', { name: 'Kinosail Player' }).parent?.props.style,
    ),
  ).toMatchObject({ backgroundColor: '#12140F', padding: 6 });
  const header = view.container
    .queryAll(() => true)
    .find(
      (node) =>
        typeof node.props.onLayout === 'function' &&
        ReactNative.StyleSheet.flatten(node.props.style)?.zIndex === 2,
    )!;
  expect(ReactNative.StyleSheet.flatten(header.props.style)).toMatchObject({
    position: 'absolute',
    backgroundColor: 'transparent',
  });
  expect(header.props.pointerEvents).toBe('box-none');
  await fireEvent(header, 'layout', {
    nativeEvent: { layout: { height: 180 } },
  });
  const scroll = view.container.queryAll((node) =>
    /ScrollView$/.test(node.type),
  )[0];
  const style = ReactNative.StyleSheet.flatten(
    scroll.props.contentContainerStyle,
  );
  expect(style.paddingTop).toBe(180);
  expect(style.paddingBottom).toBeGreaterThanOrEqual(160);
  expect(
    view.getByRole('button', { name: 'Resume, Arrival, 2016, In progress' }),
  ).toBeTruthy();
});
