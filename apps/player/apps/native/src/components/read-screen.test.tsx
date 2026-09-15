import React from 'react';
import { act, fireEvent, render, within } from '@testing-library/react-native';
import { router, useLocalSearchParams } from 'expo-router';
import ReadScreen from '@/app/read/[id]';
import { useSession } from '@/core/session-context';
import { KinosailClient } from '@/core/server-client';
import { defaultMediaPreferences } from '@/core/media-preferences';
import { routeSession, deferred } from '@/testing/route-fixtures';
import { ReaderContent } from './reader-content';

jest.mock('expo-router', () => ({
  router: { back: jest.fn() },
  useLocalSearchParams: jest.fn(),
}));
jest.mock('@/core/session-context', () => ({ useSession: jest.fn() }));
jest.mock('./reader-content', () => ({ ReaderContent: jest.fn(() => null) }));
let client: KinosailClient;
const content = () => jest.mocked(ReaderContent).mock.calls.at(-1)![0];
const mark = { id: 'a'.repeat(64), title: 'My spot', page: 1, offset: 0.4 };
beforeEach(() => {
  jest.clearAllMocks();
  client = new KinosailClient('https://kino.example', 'viewer');
  jest.mocked(useLocalSearchParams).mockReturnValue({ id: 'book' });
  jest.mocked(useSession).mockReturnValue(routeSession(client));
  jest.spyOn(client, 'loadReader').mockResolvedValue({
    id: 'book',
    title: 'Novel',
    type: 'epub',
    pages: [
      { number: 1, title: 'First', url: '/read/book/asset/first.xhtml' },
      { number: 2, title: 'Second', url: '/read/book/asset/second.xhtml' },
    ],
  });
  jest
    .spyOn(client, 'loadReaderProgress')
    .mockResolvedValue({ page: 2, total: 2, offset: 0.5 });
  jest
    .spyOn(client, 'loadMediaPreferences')
    .mockResolvedValue(defaultMediaPreferences);
  jest.spyOn(client, 'loadBookmarks').mockResolvedValue([mark]);
  jest
    .spyOn(client, 'saveReaderProgress')
    .mockResolvedValue({ page: 2, total: 2, offset: 0.7 });
  jest.spyOn(client, 'addBookmark').mockResolvedValue([mark]);
});
afterEach(() => jest.restoreAllMocks());

it('restores the saved chapter and offset and exposes overall progress', async () => {
  const view = await render(<ReadScreen />);
  expect(content().path).toBe('/read/book/asset/second.xhtml');
  expect(content().offset).toBe(0.5);
  expect(view.getByRole('progressbar').props.accessibilityValue.now).toBe(75);
  await act(async () =>
    content().onPosition!({
      nativeEvent: { path: content().path, offset: 0.7 },
    }),
  );
  expect(client.saveReaderProgress).toHaveBeenCalledWith('book', 2, 0.7);
  expect(view.getByText('Reading position saved')).toBeTruthy();
  await fireEvent.press(view.getByRole('button', { name: 'Bookmark my spot' }));
  expect(client.addBookmark).toHaveBeenCalledWith('book', {
    title: 'Chapter 2 · 70%',
    page: 2,
    offset: 0.7,
  });
  expect(view.getByText('Bookmark saved')).toBeTruthy();
});
it('uses the options area for bookmarks and restores a selected spot', async () => {
  const view = await render(<ReadScreen />);
  await fireEvent.press(view.getByRole('button', { name: 'Reader options' }));
  expect(view.getByRole('button', { name: 'My spot' })).toBeTruthy();
  await fireEvent.press(view.getByRole('button', { name: 'My spot' }));
  expect(content().path).toBe('/read/book/asset/first.xhtml');
  expect(content().offset).toBe(0.4);
  expect(
    view.queryByRole('button', { name: 'Close reader options' }),
  ).toBeNull();
  expect(client.saveReaderProgress).toHaveBeenCalledWith('book', 1, 0.4);
});
it.each([NaN, Infinity, -1, 1.1])(
  'ignores invalid native offsets %s without saving',
  async (offset) => {
    await render(<ReadScreen />);
    await act(async () =>
      content().onPosition!({ nativeEvent: { path: content().path, offset } }),
    );
    expect(client.saveReaderProgress).not.toHaveBeenCalled();
  },
);
it('ignores events from an old chapter', async () => {
  await render(<ReadScreen />);
  await act(async () =>
    content().onPosition!({
      nativeEvent: { path: '/read/book/asset/first.xhtml', offset: 0.9 },
    }),
  );
  expect(client.saveReaderProgress).not.toHaveBeenCalled();
});
it('reports save failure and retries the current spot', async () => {
  jest
    .mocked(client.saveReaderProgress)
    .mockRejectedValueOnce(new Error('offline'));
  const view = await render(<ReadScreen />);
  await act(async () =>
    content().onPosition!({
      nativeEvent: { path: content().path, offset: 0.7 },
    }),
  );
  expect(view.queryByText('Reading position saved')).toBeNull();
  await fireEvent.press(
    view.getByRole('button', { name: 'Retry saving position' }),
  );
  expect(client.saveReaderProgress).toHaveBeenLastCalledWith('book', 2, 0.7);
  expect(view.getByText('Reading position saved')).toBeTruthy();
});

it('coalesces scrolling while a save is pending and keeps the latest spot', async () => {
  const first = deferred<{ page: number; total: number; offset: number }>();
  jest.mocked(client.saveReaderProgress).mockReturnValueOnce(first.promise);
  const view = await render(<ReadScreen />);
  await act(async () =>
    content().onPosition!({
      nativeEvent: { path: content().path, offset: 0.6 },
    }),
  );
  await act(async () => {
    content().onPosition!({
      nativeEvent: { path: content().path, offset: 0.7 },
    });
    content().onPosition!({
      nativeEvent: { path: content().path, offset: 0.9 },
    });
  });
  expect(client.saveReaderProgress).toHaveBeenCalledTimes(1);
  await act(async () => first.resolve({ page: 2, total: 2, offset: 0.6 }));
  expect(client.saveReaderProgress).toHaveBeenCalledTimes(2);
  expect(client.saveReaderProgress).toHaveBeenLastCalledWith('book', 2, 0.9);
  expect(view.getByText('Reading position saved')).toBeTruthy();
});

it('rejects a late event from the unmounted previous chapter', async () => {
  const view = await render(<ReadScreen />);
  const previous = content();
  await fireEvent.press(view.getByRole('button', { name: 'Previous' }));
  jest.mocked(client.saveReaderProgress).mockClear();
  await act(async () =>
    previous.onPosition!({
      nativeEvent: { path: previous.path, offset: 0.95 },
    }),
  );
  expect(client.saveReaderProgress).not.toHaveBeenCalled();
});

it('keeps reader options and Back in the bottom bar while options scroll', async () => {
  const view = await render(<ReadScreen />);
  const bar = view;
  await fireEvent.press(bar.getByRole('button', { name: 'Reader options' }));
  const body = within(
    view.container.queryAll((node) => /ScrollView$/.test(node.type))[0],
  );
  expect(body.queryByRole('button', { name: 'Back' })).toBeNull();
  expect(
    body.queryByRole('button', { name: 'Close reader options' }),
  ).toBeNull();
  await fireEvent.press(
    bar.getByRole('button', { name: 'Close reader options' }),
  );
  expect(view.queryByRole('button', { name: 'My spot' })).toBeNull();
  await fireEvent.press(bar.getByRole('button', { name: 'Back' }));
  expect(router.back).toHaveBeenCalledTimes(1);
});
