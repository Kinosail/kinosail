import { fireEvent, render, userEvent } from '@testing-library/react-native';
import React from 'react';
import { Platform, StyleSheet } from 'react-native';

import type { MediaItem } from '@/core/contract';
import { palette } from '@/design/tokens';

import { MediaCard } from './media-card';

jest.mock('expo-image', () => {
  const ReactModule = jest.requireActual<typeof import('react')>('react');
  const { View } =
    jest.requireActual<typeof import('react-native')>('react-native');
  return {
    Image: (props: object) => ReactModule.createElement(View, props),
  };
});

const item: MediaItem = {
  id: 'arrival',
  kind: 'video',
  title: 'Arrival',
  year: '2016',
  plot: '',
  rating: 'PG-13',
  tagline: '',
  genres: 'Science Fiction',
  director: '',
  studio: '',
  artist: '',
  album: '',
  show: '',
  season: 0,
  episode: 0,
  artwork: '/art/arrival',
  backdrop: '',
  container: 'mp4',
  progress: { seconds: 0, watched: false, session: '', revision: 0 },
};
type NativeNode = NonNullable<Awaited<ReturnType<typeof render>>['root']>;
const child = (node: NativeNode, index: number): NativeNode => {
  const value = node.children[index];
  if (typeof value === 'string') throw new Error('Expected a native element.');
  return value;
};

describe('MediaCard', () => {
  const originalTV = Object.getOwnPropertyDescriptor(Platform, 'isTV')!;
  beforeEach(() => jest.useFakeTimers());
  afterEach(() => {
    Object.defineProperty(Platform, 'isTV', originalTV);
    jest.useRealTimers();
  });

  it.each([false, true])('restores focus feedback on TV=%s', async (tv) => {
    Object.defineProperty(Platform, 'isTV', { configurable: true, value: tv });
    const onPress = jest.fn();
    const view = await render(
      <MediaCard item={item} imageURL="" width={132} onPress={onPress} />,
    );
    const button = () => view.getByRole('button', { name: 'Arrival, 2016' });
    const poster = () => child(button(), 0);
    expect(view.getByText('A')).toBeTruthy();
    expect(button().props.focusable).toBe(true);
    expect(StyleSheet.flatten(button().props.style)).toEqual({
      gap: 4,
      opacity: 1,
      width: 132,
    });
    expect(StyleSheet.flatten(poster().props.style)).toEqual({
      aspectRatio: 2 / 3,
      backgroundColor: palette.dark.raised,
      borderColor: palette.dark.line,
      borderRadius: 16,
      borderWidth: tv ? 3 : 1,
      overflow: 'hidden',
    });
    await fireEvent(button(), 'focus');
    expect(StyleSheet.flatten(button().props.style).transform).toBeUndefined();
    expect(StyleSheet.flatten(poster().props.style).borderColor).toBe(
      palette.dark.focus,
    );
    expect(StyleSheet.flatten(poster().props.style).borderWidth).toBe(
      tv ? 3 : 2,
    );
    await userEvent.press(button());
    expect(onPress).toHaveBeenCalledTimes(1);
    await fireEvent(button(), 'blur');
    expect(StyleSheet.flatten(button().props.style).transform).toBeUndefined();
  });

  it.each([
    [{ show: 'Visitors', season: 2, episode: 3 }, 'S2 E3'],
    [{ year: '', artist: 'Composer' }, 'Composer'],
    [{ year: '', artist: '' }, 'video'],
  ])('announces alternate media metadata %#', async (fields, subtitle) => {
    const view = await render(
      <MediaCard
        item={{
          ...item,
          ...fields,
          progress: { ...item.progress, seconds: 10, watched: true },
        }}
        imageURL=""
        width={164}
        onPress={jest.fn()}
      />,
    );
    expect(
      view.getByRole('button', { name: `Arrival, ${subtitle}` }),
    ).toBeTruthy();
    expect(view.queryByText('IN PROGRESS')).toBeNull();
  });

  it('keeps progress, fallback, and text hierarchy exact', async () => {
    const view = await render(
      <MediaCard
        item={{
          ...item,
          progress: { ...item.progress, seconds: 10 },
        }}
        imageURL=""
        width={164}
        onPress={jest.fn()}
      />,
    );
    const button = view.getByRole('button', {
      name: 'Arrival, 2016, In progress',
    });
    const poster = child(button, 0);
    const fallback = child(poster, 0);
    const badge = child(poster, 1);
    const title = view.getByText('Arrival');
    const subtitle = view.getByText('2016');
    const label = view.getByText('A');
    const progress = view.getByText('IN PROGRESS');
    expect(StyleSheet.flatten(fallback.props.style)).toEqual({
      alignItems: 'center',
      flex: 1,
      justifyContent: 'center',
    });
    expect(StyleSheet.flatten(label.props.style)).toEqual({
      color: palette.dark.muted,
      fontSize: 44,
      fontWeight: '300',
    });
    expect(StyleSheet.flatten(badge.props.style)).toEqual({
      backgroundColor: palette.dark.signal,
      bottom: 0,
      left: 0,
      paddingHorizontal: 8,
      paddingVertical: 4,
      position: 'absolute',
    });
    expect(progress.props.maxFontSizeMultiplier).toBe(1.5);
    expect(StyleSheet.flatten(progress.props.style)).toEqual({
      color: palette.dark.signalInk,
      fontSize: 9,
      fontWeight: '900',
      letterSpacing: 1.1,
    });
    expect(title.props).toMatchObject({
      maxFontSizeMultiplier: 2,
      numberOfLines: 2,
    });
    expect(StyleSheet.flatten(title.props.style)).toEqual({
      color: palette.dark.text,
      fontSize: 15,
      fontWeight: '800',
      lineHeight: 19,
      marginTop: 8,
      minHeight: 38,
    });
    expect(subtitle.props).toMatchObject({
      maxFontSizeMultiplier: 2,
      numberOfLines: 1,
    });
    expect(StyleSheet.flatten(subtitle.props.style)).toEqual({
      color: palette.dark.muted,
      fontSize: 12,
      fontWeight: '600',
      textTransform: 'uppercase',
    });
  });

  it('keeps recycled shelf artwork decoded and cached', async () => {
    const view = await render(
      <MediaCard
        imageURL="https://kino.example/art/arrival"
        item={item}
        onPress={jest.fn()}
        width={132}
      />,
    );

    const image = view.getByLabelText('Poster for Arrival');
    expect(image.props).toMatchObject({
      accessibilityLabel: 'Poster for Arrival',
      alt: 'Poster for Arrival',
      cachePolicy: 'memory-disk',
      contentFit: 'cover',
      enforceEarlyResizing: true,
      recyclingKey: 'arrival',
      source: {
        uri: 'https://kino.example/art/arrival',
      },
    });
    expect(StyleSheet.flatten(image.props.style)).toEqual(
      StyleSheet.absoluteFill,
    );
    expect(view.getByText('Arrival').props.numberOfLines).toBe(2);
  });
  it('keeps failed artwork usable and loads a replacement URL', async () => {
    const onPress = jest.fn();
    const view = await render(
      <MediaCard
        item={item}
        imageURL="https://kino.example/broken"
        width={132}
        onPress={onPress}
      />,
    );
    await fireEvent(view.getByLabelText('Poster for Arrival'), 'error', {
      error: 'Unavailable',
    });
    expect(view.queryByLabelText('Poster for Arrival')).toBeNull();
    expect(view.getByText('A')).toBeTruthy();
    expect(onPress).not.toHaveBeenCalled();
    await fireEvent.press(view.getByRole('button', { name: 'Arrival, 2016' }));
    expect(onPress).toHaveBeenCalledTimes(1);
    await view.rerender(
      <MediaCard
        item={item}
        imageURL="https://kino.example/replacement"
        width={132}
        onPress={onPress}
      />,
    );
    expect(view.getByLabelText('Poster for Arrival').props.source.uri).toBe(
      'https://kino.example/replacement',
    );
    expect(view.queryByText('A')).toBeNull();
  });
});

it.each([false, true])(
  'uses readable cover geometry for square=%s',
  async (square) => {
    const onPress = jest.fn();
    const view = await render(
      <MediaCard
        item={item}
        imageURL="https://kino.example/art/arrival"
        width={160}
        square={square}
        onPress={onPress}
      />,
    );
    const image = view.getByLabelText('Poster for Arrival');
    expect(StyleSheet.flatten(image.parent!.props.style).aspectRatio).toBe(
      square ? 1 : 2 / 3,
    );
    await fireEvent.press(view.getByRole('button'));
    expect(onPress).toHaveBeenCalledTimes(1);
  },
);

it('preserves the complete TV artwork inside the portrait frame', async () => {
  const view = await render(
    <MediaCard
      item={{ ...item, show: 'Series' }}
      imageURL="https://kino.example/art/arrival"
      width={132}
      onPress={jest.fn()}
    />,
  );
  expect(view.getByLabelText('Poster for Arrival').props.contentFit).toBe(
    'contain',
  );
});

it('keeps a landscape resume card actionable if its artwork fails', async () => {
  const onPress = jest.fn();
  const view = await render(
    <MediaCard
      item={item}
      imageURL="/backdrop/arrival"
      width={280}
      landscape
      compact
      onPress={onPress}
    />,
  );
  const poster = view.getByLabelText('Poster for Arrival');
  expect(poster.props.contentFit).toBe('cover');
  await fireEvent(poster, 'error');
  const button = view.getByRole('button', { name: /Resume, Arrival/ });
  await fireEvent.press(button);
  expect(onPress).toHaveBeenCalledTimes(1);
  expect(view.getByText('Arrival').props.numberOfLines).toBe(1);
});
