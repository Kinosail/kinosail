import React from 'react';
import { fireEvent, render } from '@testing-library/react-native';
import { parseItem, type Home } from '@/core/contract';
import { TVHomeView } from './tv-home-view';

jest.mock('react-native/Libraries/Utilities/Platform', () => {
  const platform = jest.requireActual(
    'react-native/Libraries/Utilities/Platform',
  );
  Object.defineProperty(platform.default, 'isTV', {
    configurable: true,
    value: true,
  });
  return platform;
});

const movie = parseItem({
  item: {
    id: 'arrival',
    kind: 'video',
    title: 'Arrival',
    backdrop: '/arrival.jpg',
    progress: { seconds: 42 },
  },
});
const episode = parseItem({
  item: {
    id: 'episode',
    kind: 'video',
    title:
      'A very long episode title with enough words to wrap across two lines',
    show: 'Visitors',
    season: 2,
    episode: 3,
  },
});
const home: Home = {
  server: 'Den',
  viewer: { id: 'viewer', name: 'Mike', owner: false },
  continueWatching: [movie],
  recent: [movie, episode],
};
const props = () => ({
  home,
  mediaURL: (path: string) => path,
  onOpen: jest.fn(),
  onPlay: jest.fn(),
  onBrowse: jest.fn(),
  onSignOut: jest.fn(),
  onRefresh: jest.fn(),
});
it('routes every primary destination and keeps account actions out of browsing', async () => {
  const p = props();
  const view = await render(<TVHomeView {...p} />);
  for (const [label, destination] of [
    ['Movies', 'movies'],
    ['TV shows', 'shows'],
    ['My List', 'list'],
  ] as const) {
    await fireEvent.press(view.getByRole('button', { name: label }));
    expect(p.onBrowse).toHaveBeenLastCalledWith(destination);
  }
  await fireEvent.press(view.getByRole('button', { name: 'Search' }));
  expect(p.onBrowse).toHaveBeenLastCalledWith();
  expect(view.queryByRole('button', { name: 'Sign out' })).toBeNull();
  await fireEvent.press(
    view.getByRole('button', { name: 'Account options for Mike' }),
  );
  await fireEvent.press(view.getByRole('button', { name: 'Sign out' }));
  expect(p.onSignOut).toHaveBeenCalledTimes(1);
});
it('resumes directly and keeps a separate route to details', async () => {
  const p = props();
  const view = await render(<TVHomeView {...p} />);
  await fireEvent.press(view.getByRole('button', { name: /^Resume, Arrival/ }));
  expect(p.onPlay).toHaveBeenCalledWith('arrival');
  expect(p.onOpen).not.toHaveBeenCalled();
  await fireEvent.press(view.getByRole('button', { name: 'Details' }));
  expect(p.onOpen).toHaveBeenCalledWith('arrival');
});
it('updates the featured title on focus without opening or playing it', async () => {
  const p = props();
  const view = await render(<TVHomeView {...p} />);
  await fireEvent(
    view.getAllByRole('button', { name: /A very long episode/ })[0],
    'focus',
  );
  expect(view.getByRole('header', { name: episode.title })).toBeTruthy();
  expect(p.onPlay).not.toHaveBeenCalled();
  expect(p.onOpen).not.toHaveBeenCalled();
  await fireEvent.press(view.getByRole('button', { name: 'View details' }));
  expect(p.onOpen).toHaveBeenCalledWith('episode');
});
it('recovers from empty libraries without exposing playback actions', async () => {
  const p = props();
  const view = await render(
    <TVHomeView {...p} home={{ ...home, recent: [], continueWatching: [] }} />,
  );
  expect(view.queryByRole('button', { name: 'Resume' })).toBeNull();
  await fireEvent.press(view.getByRole('button', { name: 'Refresh library' }));
  expect(p.onRefresh).toHaveBeenCalledTimes(1);
  expect(p.onPlay).not.toHaveBeenCalled();
});
it('falls back to details when direct playback is unavailable', async () => {
  const p = props();
  const view = await render(<TVHomeView {...p} onPlay={undefined} />);
  await fireEvent.press(view.getAllByRole('button', { name: /^Arrival/ })[0]);
  expect(p.onOpen).toHaveBeenCalledWith('arrival');
});
