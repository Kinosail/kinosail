import { fireEvent, render } from '@testing-library/react-native';
import React from 'react';
import * as ReactNative from 'react-native';

import type { MediaItem } from '@/core/contract';

import { DetailView } from './detail-view';

jest.mock('expo-image', () => {
  const ReactModule = jest.requireActual<typeof import('react')>('react');
  const { View } =
    jest.requireActual<typeof import('react-native')>('react-native');
  return { Image: (props: object) => ReactModule.createElement(View, props) };
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
describe('DetailView contracts', () => {
  afterEach(() => jest.restoreAllMocks());

  it.each([390, 1024, 1920])(
    'leads with artwork at width %i and preserves fallback actions',
    async (width) => {
      jest
        .spyOn(
          jest.requireActual<typeof ReactNative>('react-native'),
          'useWindowDimensions',
        )
        .mockReturnValue({ width, height: 844, scale: 3, fontScale: 1 });
      const headers = { Authorization: 'Bearer token' };
      const onBack = jest.fn(),
        onPlay = jest.fn(),
        onDownload = jest.fn();
      const view = await render(
        <DetailView
          headers={headers}
          item={item}
          mediaURL={(path) => `https://kino.example${path}`}
          onBack={onBack}
          onPlay={onPlay}
          onDownload={onDownload}
        />,
      );
      const backdrop = view.getByLabelText('Backdrop for Arrival');
      expect(backdrop.props.source).toEqual({
        uri: 'https://kino.example/backdrop/arrival',
        cacheKey: expect.stringContaining('https://kino.example/backdrop/arrival'),
        headers,
      });
      expect(view.queryByLabelText('Poster for Arrival')).toBeNull();
      expect(view.getByText(item.plot)).toBeTruthy();
      expect(view.getByText('2016 · PG-13 · MKV')).toBeTruthy();
      await fireEvent(backdrop, 'error');
      const poster = view.getByLabelText('Poster for Arrival');
      expect(poster.props.source).toEqual({
        uri: 'https://kino.example/art/arrival',
        cacheKey: expect.stringContaining('https://kino.example/art/arrival'),
        headers,
      });
      expect(view.queryByLabelText('Backdrop for Arrival')).toBeNull();
      await fireEvent(poster, 'error');
      expect(view.queryByLabelText('Poster for Arrival')).toBeNull();
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
    },
  );

  it('keeps sparse episodic metadata useful', async () => {
    const view = await render(
      <DetailView
        headers={{}}
        item={{
          ...item,
          artwork: '',
          backdrop: '',
          container: 'mp4',
          director: '',
          genres: '',
          plot: '',
          progress: { ...item.progress, seconds: 0 },
          rating: '',
          show: 'Visitors',
          season: 2,
          episode: 3,
          tagline: '',
          year: '',
        }}
        mediaURL={(path) => path}
        onBack={jest.fn()}
        onPlay={jest.fn()}
      />,
    );
    expect(view.queryByLabelText('Backdrop for Arrival')).toBeNull();
    expect(view.queryByLabelText('Poster for Arrival')).toBeNull();
    expect(view.getByText('S2 E3 · MP4')).toBeTruthy();
    expect(
      view.getByText('No description is available for this title.'),
    ).toBeTruthy();
    expect(view.getByRole('button', { name: 'Play' })).toBeTruthy();
    expect(view.queryByText('Directed by')).toBeNull();
  });
});
