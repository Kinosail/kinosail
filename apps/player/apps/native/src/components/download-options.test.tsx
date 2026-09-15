import React from 'react';
import { fireEvent, render, within } from '@testing-library/react-native';
import { DownloadOptions } from './download-options';
import { routeItem } from '@/testing/route-fixtures';
jest.mock('react-native-safe-area-context', () => ({
  SafeAreaView: require('react-native').View,
}));
it('defaults to Original and explains the selected network', async () => {
  const onDownload = jest.fn().mockResolvedValue(undefined);
  const screen = await render(
    <DownloadOptions
      item={routeItem}
      wifiOnly
      onDownload={onDownload}
      onClose={jest.fn()}
    />,
  );
  expect(screen.getByText(/Network: Wi-Fi only/)).toBeTruthy();
  await fireEvent.press(screen.getByRole('button', { name: 'Start download' }));
  expect(onDownload).toHaveBeenCalledWith('original');
});
it('offers reduced video quality with its track and preparation tradeoffs', async () => {
  const onDownload = jest.fn().mockResolvedValue(undefined);
  const screen = await render(
    <DownloadOptions
      item={routeItem}
      wifiOnly={false}
      onDownload={onDownload}
      onClose={jest.fn()}
    />,
  );
  expect(screen.getByText(/Network: Wi-Fi \+ cellular/)).toBeTruthy();
  await fireEvent.press(screen.getByRole('button', { name: '720p' }));
  expect(
    screen.getByText(
      /selected audio and subtitles; all tracks are included by default/,
    ),
  ).toBeTruthy();
  await fireEvent.press(screen.getByRole('button', { name: 'Start download' }));
  expect(onDownload).toHaveBeenCalledWith('720p');
});
it('does not offer video resolutions for an audiobook', async () => {
  const screen = await render(
    <DownloadOptions
      item={{ ...routeItem, kind: 'audiobook' }}
      wifiOnly
      onDownload={jest.fn()}
      onClose={jest.fn()}
    />,
  );
  expect(screen.queryByRole('button', { name: '1080p' })).toBeNull();
});
it('downloads a season or all episodes with one quality choice', async () => {
  const showId = 'a'.repeat(16);
  const item = { ...routeItem, show: 'Example', showId, season: 1, episode: 1 };
  const episodes = [
    item,
    { ...item, id: 'second', episode: 2 },
    { ...item, id: 'next-season', season: 2 },
  ];
  const onDownload = jest.fn().mockResolvedValue(undefined);
  const screen = await render(
    <DownloadOptions
      item={item}
      episodes={episodes}
      wifiOnly
      onDownload={onDownload}
      onClose={jest.fn()}
    />,
  );
  await fireEvent.press(
    screen.getByRole('button', { name: 'Entire season 1' }),
  );
  expect(
    screen.getByText(/2 episodes · Existing downloads are skipped/),
  ).toBeTruthy();
  await fireEvent.press(screen.getByRole('button', { name: '720p' }));
  await fireEvent.press(
    screen.getByRole('button', { name: 'Download 2 episodes' }),
  );
  expect(onDownload).toHaveBeenLastCalledWith('720p', episodes.slice(0, 2));
  await fireEvent.press(screen.getByRole('button', { name: 'All episodes' }));
  await fireEvent.press(
    screen.getByRole('button', { name: 'Download 3 episodes' }),
  );
  expect(onDownload).toHaveBeenLastCalledWith('720p', episodes);
});
it('keeps episode download available while loading the show or after a show error', async () => {
  const screen = await render(
    <DownloadOptions
      item={{ ...routeItem, show: 'Example', showId: 'a'.repeat(16) }}
      episodesError="Could not load episodes"
      wifiOnly
      onDownload={jest.fn()}
      onClose={jest.fn()}
    />,
  );
  expect(screen.getByRole('button', { name: 'All episodes' })).toBeDisabled();
  expect(
    screen.getByRole('button', { name: 'Start download' }),
  ).not.toBeDisabled();
  expect(screen.getByText('Could not load episodes')).toBeTruthy();
});
it('shows the original size before starting a download', async () => {
  const screen = await render(
    <DownloadOptions
      item={{ ...routeItem, size: 2_000_000_000 }}
      wifiOnly
      onDownload={jest.fn()}
      onClose={jest.fn()}
    />,
  );
  expect(screen.getByText('Original size: 2.0 GB')).toBeTruthy();
});

it('can close while starting a download without starting another request', async () => {
  const onClose = jest.fn();
  const onDownload = jest.fn(() => new Promise<void>(() => {}));
  const screen = await render(
    <DownloadOptions
      item={routeItem}
      wifiOnly
      onDownload={onDownload}
      onClose={onClose}
    />,
  );
  await fireEvent.press(screen.getByRole('button', { name: 'Start download' }));
  await fireEvent.press(screen.getByRole('button', { name: 'Close' }));
  expect(onClose).toHaveBeenCalledTimes(1);
  expect(onDownload).toHaveBeenCalledTimes(1);
});

it('pins the current size and Start action outside scrolling options with one dismissal', async () => {
  const view = await render(
    <DownloadOptions
      item={{ ...routeItem, size: 2_000_000_000 }}
      wifiOnly
      onDownload={jest.fn()}
      onClose={jest.fn()}
    />,
  );
  const body = within(
    view.container.queryAll((node) => /ScrollView$/.test(node.type))[0],
  );
  expect(body.queryByRole('button', { name: 'Start download' })).toBeNull();
  expect(body.queryByText('Original size: 2.0 GB')).toBeNull();
  expect(view.getAllByRole('button', { name: 'Cancel' })).toHaveLength(1);
  expect(view.queryByRole('button', { name: 'Close' })).toBeNull();
});
