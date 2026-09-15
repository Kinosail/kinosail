import React from 'react';
import { fireEvent, render } from '@testing-library/react-native';
import { DownloadsView, downloadStatus, downloadBytes } from './downloads-view';
import { routeItem } from '@/testing/route-fixtures';
import type { DownloadEntry } from '@/core/downloads.types';
jest.mock('./navigation-icon', () => ({ NavigationIcon: () => null }));
jest.mock('react-native-safe-area-context', () => ({
  SafeAreaView: require('react-native').View,
}));
const entry: DownloadEntry = {
  item: routeItem,
  bytes: 4,
  total: 8,
  status: 'downloading',
  duration: 120,
  contentType: 'video/mp4',
  version: 'one',
  error: '',
};
const props = () => ({
  entries: [entry],
  background: true,
  error: '',
  onPause: jest.fn().mockResolvedValue(undefined),
  onResume: jest.fn().mockResolvedValue(undefined),
  onRemove: jest.fn().mockResolvedValue(undefined),
  onPlay: jest.fn(),
  onBrowse: jest.fn(),
});
it('shows progress and opens title-specific actions in a sheet', async () => {
  const actions = props();
  const screen = await render(<DownloadsView {...actions} />);
  expect(
    screen.getByLabelText('Arrival download progress').props.accessibilityValue,
  ).toEqual({
    min: 0,
    max: 100,
    now: 50,
  });
  await fireEvent.press(
    screen.getByRole('button', { name: /Arrival.*Download options/ }),
  );
  await fireEvent.press(screen.getByRole('button', { name: 'Pause download' }));
  expect(actions.onPause).toHaveBeenCalledWith(entry);
  expect(actions.onRemove).not.toHaveBeenCalled();
});
it('offers listening offline for a completed audiobook', async () => {
  const actions = props(),
    book: DownloadEntry = {
      ...entry,
      item: { ...routeItem, kind: 'audiobook' },
      status: 'complete',
      bytes: 8,
    };
  const screen = await render(<DownloadsView {...actions} entries={[book]} />);
  await fireEvent.press(
    screen.getByRole('button', { name: /Arrival.*Download options/ }),
  );
  await fireEvent.press(screen.getByRole('button', { name: 'Listen offline' }));
  expect(actions.onPlay).toHaveBeenCalledWith(book);
});
it('explains foreground-only behavior on unsupported platforms', async () => {
  const screen = await render(
    <DownloadsView {...props()} background={false} entries={[]} />,
  );
  expect(screen.getByText(/Keep the app open/)).toBeTruthy();
  expect(screen.queryByText(/Downloads continue in the background/)).toBeNull();
  expect(
    screen.getByRole('button', { name: 'Find something to download' }),
  ).toBeTruthy();
});
it.each([
  ['queued', 'Queued · starts automatically'],
  ['waiting', 'Waiting for a connection'],
  ['pausing', 'Pausing…'],
  ['paused', 'Paused · 50%'],
  ['complete', 'Ready offline'],
] as const)('describes %s accurately', (status, label) => {
  expect(downloadStatus({ ...entry, status })).toBe(label);
});
it('keeps small downloads legible instead of showing zero GB', () => {
  expect(downloadBytes(5 * 1024 ** 2)).toBe('5 MB');
  expect(downloadBytes(1.5 * 1024 ** 3)).toBe('1.5 GB');
});

it('shows observed speed and ETA only for an active download', async () => {
  const progress = { [entry.item.id]: '1.0 MB/s · About 2 min left' };
  const screen = await render(
    <DownloadsView {...props()} progress={progress} />,
  );
  expect(screen.getByText(progress[entry.item.id])).toBeTruthy();
  await screen.rerender(
    <DownloadsView
      {...props()}
      entries={[{ ...entry, status: 'paused' }]}
      progress={progress}
    />,
  );
  expect(screen.queryByText(progress[entry.item.id])).toBeNull();
});

it('shows preparation without a fabricated byte percentage', async () => {
  const preparing: DownloadEntry = {
    ...entry,
    status: 'preparing',
    quality: '720p',
    jobID: 'a'.repeat(16),
    bytes: 0,
    total: 0,
  };
  const screen = await render(
    <DownloadsView {...props()} entries={[preparing]} />,
  );
  expect(screen.getByText('Preparing on the Server')).toBeTruthy();
  expect(screen.getByText(/Size available after preparation/)).toBeTruthy();
  expect(screen.queryByRole('progressbar')).toBeNull();
  expect(downloadStatus({ ...preparing, status: 'paused' })).toBe(
    'Download paused',
  );
});

it('shows the configured storage limit instead of a hard-coded quota', async () => {
  const screen = await render(<DownloadsView {...props()} limitGiB={5} />);
  expect(screen.getByText(/5 GB download limit/)).toBeTruthy();
  expect(screen.queryByText(/20 GB download limit/)).toBeNull();
});

it('names unlimited storage without displaying a zero GB quota', async () => {
  const screen = await render(<DownloadsView {...props()} limitGiB={0} />);
  expect(screen.getByText(/No download storage limit/)).toBeTruthy();
  expect(screen.queryByText(/0 GB download limit/)).toBeNull();
});

it('closes download details without pausing or removing the download', async () => {
  const actions = props();
  const screen = await render(<DownloadsView {...actions} />);
  await fireEvent.press(
    screen.getByRole('button', { name: /Arrival.*Download options/ }),
  );
  await fireEvent.press(screen.getByRole('button', { name: 'Close' }));
  expect(screen.queryByRole('button', { name: 'Remove download' })).toBeNull();
  expect(actions.onPause).not.toHaveBeenCalled();
  expect(actions.onRemove).not.toHaveBeenCalled();
});

it('requires confirmation and lets the user keep their offline copy', async () => {
  const actions = props();
  const screen = await render(<DownloadsView {...actions} />);
  await fireEvent.press(
    screen.getByRole('button', { name: /Arrival.*Download options/ }),
  );
  await fireEvent.press(
    screen.getByRole('button', { name: 'Remove download' }),
  );
  expect(actions.onRemove).not.toHaveBeenCalled();
  expect(screen.getByText(/Remove Arrival from this device/)).toBeTruthy();
  await fireEvent.press(screen.getByRole('button', { name: 'Keep download' }));
  expect(actions.onRemove).not.toHaveBeenCalled();
  await fireEvent.press(
    screen.getByRole('button', { name: 'Remove download' }),
  );
  await fireEvent.press(
    screen.getByRole('button', { name: 'Remove from device' }),
  );
  expect(actions.onRemove).toHaveBeenCalledTimes(1);
  expect(actions.onRemove).toHaveBeenCalledWith(entry);
});
it('keeps removal confirmation open when removal fails', async () => {
  const actions = props();
  actions.onRemove.mockRejectedValue(new Error('Storage unavailable'));
  const screen = await render(<DownloadsView {...actions} />);
  await fireEvent.press(
    screen.getByRole('button', { name: /Arrival.*Download options/ }),
  );
  await fireEvent.press(
    screen.getByRole('button', { name: 'Remove download' }),
  );
  await fireEvent.press(
    screen.getByRole('button', { name: 'Remove from device' }),
  );
  expect(screen.getByRole('button', { name: 'Keep download' })).toBeTruthy();
});
