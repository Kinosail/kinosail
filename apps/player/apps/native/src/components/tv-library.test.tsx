import React from 'react';
import { fireEvent, render } from '@testing-library/react-native';
import { LibraryView } from './library-view';
import { routeItem } from '@/testing/route-fixtures';
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
jest.mock('react-native-safe-area-context', () => ({
  ...jest.requireActual('react-native-safe-area-context'),
  useSafeAreaInsets: () => ({ top: 0, left: 0, right: 0, bottom: 0 }),
}));
const props = () => ({
  items: [routeItem],
  total: 1,
  query: { view: 'movies' as const },
  busy: false,
  error: '',
  hasMore: true,
  mediaURL: (path: string) => path,
  headers: {},
  onChange: jest.fn(),
  onOpen: jest.fn(),
  onMore: jest.fn(),
  onPrevious: jest.fn(),
  onRetry: jest.fn(),
  onBack: jest.fn(),
});
it('prioritizes posters and opens search and filters on demand', async () => {
  const p = props();
  const view = await render(<LibraryView {...p} />);
  expect(view.queryByLabelText('Search your library')).toBeNull();
  await fireEvent.press(
    view.getByRole('button', { name: new RegExp(routeItem.title) }),
  );
  expect(p.onOpen).toHaveBeenCalledWith(routeItem.id);
  await fireEvent.press(view.getByRole('button', { name: 'Search & filters' }));
  await fireEvent.changeText(
    view.getByLabelText('Search your library'),
    'Moon',
  );
  expect(p.onChange).toHaveBeenCalledWith({
    view: 'movies',
    q: 'Moon',
    offset: 0,
  });
  await fireEvent.press(view.getByRole('button', { name: 'Close' }));
  expect(view.queryByLabelText('Search your library')).toBeNull();
});
it('opens search from its home destination and keeps pagination reachable', async () => {
  const p = props();
  const view = await render(<LibraryView {...p} searchOpen />);
  expect(view.getByLabelText('Search your library')).toBeTruthy();
  await fireEvent.press(view.getByRole('button', { name: 'Show 1 title' }));
  await fireEvent.press(view.getByRole('button', { name: 'Next page' }));
  expect(p.onMore).toHaveBeenCalledTimes(1);
});
it('offers retry and Back when the server fails', async () => {
  const p = props();
  const view = await render(
    <LibraryView {...p} items={[]} error="Server unavailable" />,
  );
  await fireEvent.press(view.getByRole('button', { name: 'Try again' }));
  await fireEvent.press(view.getByRole('button', { name: 'Back' }));
  expect(p.onRetry).toHaveBeenCalledTimes(1);
  expect(p.onBack).toHaveBeenCalledTimes(1);
});
