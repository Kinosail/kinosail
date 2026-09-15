import React from 'react';
import { act, fireEvent, render, waitFor } from '@testing-library/react-native';
import { Keyboard, Platform, StyleSheet } from 'react-native';
import { BottomNavigation } from './bottom-navigation';
import { palette } from '@/design/tokens';

const originalTV = Object.getOwnPropertyDescriptor(Platform, 'isTV')!;
const mockReplace = jest.fn();
const mockNavigate = jest.fn();
const mockPush = jest.fn();
const mockGet = jest.fn();
const mockSet = jest.fn();
jest.mock('@/core/platform-storage', () => ({
  platformStorage: {
    get: (key: string) => mockGet(key),
    set: (key: string, value: string) => mockSet(key, value),
  },
}));
let mockPath = '/';
let mockParams: { view?: string | string[] } = {};
let mockClient: object | null = {};
jest.mock('expo-router', () => ({
  router: {
    replace: (...args: unknown[]) => mockReplace(...args),
    navigate: (...args: unknown[]) => mockNavigate(...args),
    push: (...args: unknown[]) => mockPush(...args),
  },
  usePathname: () => mockPath,
  useGlobalSearchParams: () => mockParams,
}));
jest.mock('@/core/downloads', () => ({ downloadsAvailable: true }));
jest.mock('@/core/session-context', () => ({
  useSession: () => ({ client: mockClient }),
}));
jest.mock('react-native-safe-area-context', () => ({
  SafeAreaView: jest.requireActual('react-native').View,
  useSafeAreaInsets: () => ({ top: 0, bottom: 34, left: 0, right: 0 }),
}));
jest.mock('./navigation-icon', () => ({ NavigationIcon: () => null }));
beforeEach(() => {
  mockPath = '/';
  mockParams = {};
  mockClient = {};
  mockReplace.mockClear();
  mockGet.mockReset().mockResolvedValue(null);
  mockSet.mockReset().mockResolvedValue(undefined);
});
afterEach(() => {
  jest.restoreAllMocks();
  Object.defineProperty(Platform, 'isTV', originalTV);
});
it('puts Downloads in the five-tab bar with Home selected', async () => {
  const view = await render(<BottomNavigation />);
  expect(view.getAllByRole('tab')).toHaveLength(5);
  expect(
    view.getByRole('tab', { name: 'Home' }).props.accessibilityState.selected,
  ).toBe(true);
  await fireEvent.press(view.getByRole('tab', { name: 'Home' }));
  expect(mockReplace).not.toHaveBeenCalled();
  for (const [name, category] of [
    ['Movies', 'movies'],
    ['TV', 'shows'],
  ]) {
    await fireEvent.press(view.getByRole('tab', { name }));
    expect(mockReplace).toHaveBeenLastCalledWith({
      pathname: '/library',
      params: { view: category },
    });
  }
  await fireEvent.press(view.getByRole('tab', { name: 'Downloads' }));
  expect(mockReplace).toHaveBeenLastCalledWith('/downloads');
});
it('returns home and keeps category selection independent of keyboard focus', async () => {
  mockPath = '/library';
  mockParams = { view: 'music' };
  const view = await render(<BottomNavigation />);
  expect(
    view.getByRole('tab', { name: 'More' }).props.accessibilityState.selected,
  ).toBe(true);
  await fireEvent(view.getByRole('tab', { name: 'Movies' }), 'focus');
  expect(
    view.getByRole('tab', { name: 'More' }).props.accessibilityState.selected,
  ).toBe(true);
  await fireEvent.press(view.getByRole('tab', { name: 'Home' }));
  expect(mockReplace).toHaveBeenCalledWith('/');
});
it.each(['/watch/arrival', '/item/arrival'])(
  'leaves %s unobstructed',
  async (path) => {
    mockPath = path;
    expect((await render(<BottomNavigation />)).queryByRole('tab')).toBeNull();
  },
);
it('hides the bar before connection and on TV', async () => {
  mockClient = null;
  const view = await render(<BottomNavigation />);
  expect(view.queryByRole('tab')).toBeNull();
  mockClient = {};
  Object.defineProperty(Platform, 'isTV', { configurable: true, value: true });
  await view.rerender(<BottomNavigation />);
  expect(view.queryByRole('tab')).toBeNull();
});
it('hides while the native keyboard is open and removes listeners', async () => {
  const handlers: Record<string, () => void> = {};
  const remove = jest.fn();
  jest.spyOn(Keyboard, 'addListener').mockImplementation((event, handler) => {
    handlers[event] = handler as () => void;
    return { remove } as unknown as ReturnType<typeof Keyboard.addListener>;
  });
  const view = await render(<BottomNavigation />);
  await act(() => handlers.keyboardDidShow());
  expect(view.queryByRole('tab')).toBeNull();
  await act(() => handlers.keyboardDidHide());
  expect(view.getAllByRole('tab')).toHaveLength(5);
  await view.unmount();
  expect(remove).toHaveBeenCalledTimes(2);
});

it('selects More while browsing an unpinned audiobook category', async () => {
  mockPath = '/library';
  mockParams = { view: 'audiobooks' };
  const view = await render(<BottomNavigation />);
  expect(
    view.getByRole('tab', { name: 'More' }).props.accessibilityState.selected,
  ).toBe(true);
});

it('opens search and downloads from the bottom controls', async () => {
  const view = await render(<BottomNavigation />);
  await fireEvent.press(
    view.getByRole('button', { name: 'Search your library' }),
  );
  expect(mockNavigate).toHaveBeenCalledWith({
    pathname: '/library',
    params: { search: '1' },
  });
  await fireEvent.press(view.getByRole('tab', { name: 'Downloads' }));
  expect(mockReplace).toHaveBeenCalledWith('/downloads');
});
it('keeps navigation on Downloads without a duplicate Downloads button', async () => {
  mockPath = '/downloads';
  const view = await render(<BottomNavigation />);
  expect(view.getAllByRole('tab')).toHaveLength(5);
  expect(view.queryByRole('button', { name: 'Downloads' })).toBeNull();
  expect(
    view.getByRole('tab', { name: 'Downloads' }).props.accessibilityState
      .selected,
  ).toBe(true);
});

async function openEditor(view: Awaited<ReturnType<typeof render>>) {
  await fireEvent.press(view.getByRole('tab', { name: 'More' }));
  await waitFor(() =>
    expect(view.getByRole('button', { name: 'Customize tabs' })).toBeEnabled(),
  );
  await fireEvent.press(view.getByRole('button', { name: 'Customize tabs' }));
}
it('reorders Downloads, persists the order, and restores it on a later mount', async () => {
  const view = await render(<BottomNavigation />);
  await openEditor(view);
  for (let count = 0; count < 3; count++) {
    await fireEvent.press(
      view.getByRole('button', { name: 'Move Downloads earlier' }),
    );
  }
  await fireEvent.press(view.getByRole('button', { name: 'Save' }));
  await waitFor(() =>
    expect(mockSet).toHaveBeenCalledWith(
      'kinosail.player.navigation.v1',
      '["downloads","home","movies","shows"]',
    ),
  );
  expect(
    view.getAllByRole('tab').map((tab) => tab.props.accessibilityLabel),
  ).toEqual(['Downloads', 'Home', 'Movies', 'TV', 'More']);
  await view.unmount();
  mockGet.mockResolvedValue('["downloads","home","movies","shows"]');
  const restored = await render(<BottomNavigation />);
  await waitFor(() =>
    expect(restored.getAllByRole('tab')[0].props.accessibilityLabel).toBe(
      'Downloads',
    ),
  );
});
it('adds and removes tabs without making hidden destinations unreachable', async () => {
  const view = await render(<BottomNavigation />);
  await openEditor(view);
  expect(
    view.getByRole('button', { name: 'Add Music to tab bar' }),
  ).toBeDisabled();
  await fireEvent.press(
    view.getByRole('button', { name: 'Remove TV from tab bar' }),
  );
  await fireEvent.press(
    view.getByRole('button', { name: 'Add Music to tab bar' }),
  );
  await fireEvent.press(view.getByRole('button', { name: 'Save' }));
  await waitFor(() =>
    expect(view.getByRole('tab', { name: 'Music' })).toBeTruthy(),
  );
  await fireEvent.press(view.getByRole('tab', { name: 'More' }));
  await fireEvent.press(view.getByRole('button', { name: 'TV' }));
  expect(mockReplace).toHaveBeenLastCalledWith({
    pathname: '/library',
    params: { view: 'shows' },
  });
});
it('cancels edits without writing and supports restoring defaults', async () => {
  mockGet.mockResolvedValue('["books","downloads"]');
  const view = await render(<BottomNavigation />);
  await openEditor(view);
  await fireEvent.press(view.getByRole('button', { name: 'Restore defaults' }));
  await fireEvent.press(view.getByRole('button', { name: 'Cancel' }));
  expect(mockSet).not.toHaveBeenCalled();
  expect(
    view.getAllByRole('tab').map((tab) => tab.props.accessibilityLabel),
  ).toEqual(['Books', 'Downloads', 'More']);
  await openEditor(view);
  await fireEvent.press(view.getByRole('button', { name: 'Restore defaults' }));
  await fireEvent.press(view.getByRole('button', { name: 'Save' }));
  await waitFor(() =>
    expect(mockSet).toHaveBeenCalledWith(
      'kinosail.player.navigation.v1',
      '["home","movies","shows","downloads"]',
    ),
  );
});
it('preserves the draft after a save failure and allows retry', async () => {
  mockSet.mockRejectedValueOnce(new Error('unavailable'));
  const view = await render(<BottomNavigation />);
  await openEditor(view);
  await fireEvent.press(
    view.getByRole('button', { name: 'Move Downloads earlier' }),
  );
  await fireEvent.press(view.getByRole('button', { name: 'Save' }));
  await waitFor(() =>
    expect(
      view.getByText('Your tabs could not be saved. Try again.'),
    ).toBeTruthy(),
  );
  expect(
    view.getAllByRole('tab').map((tab) => tab.props.accessibilityLabel),
  ).toEqual(['Home', 'Movies', 'TV', 'Downloads', 'More']);
  await fireEvent.press(view.getByRole('button', { name: 'Save' }));
  await waitFor(() =>
    expect(mockSet).toHaveBeenLastCalledWith(
      'kinosail.player.navigation.v1',
      '["home","movies","downloads","shows"]',
    ),
  );
});

it('offers customization after unreadable preferences and keeps at least one tab', async () => {
  mockGet.mockResolvedValue('["unknown"]');
  const view = await render(<BottomNavigation />);
  await openEditor(view);
  expect(
    view.getByText('Saved tabs could not be read. Choose your tabs again.'),
  ).toBeTruthy();
  for (const name of ['Home', 'Movies', 'TV']) {
    await fireEvent.press(
      view.getByRole('button', { name: `Remove ${name} from tab bar` }),
    );
  }
  expect(
    view.getByRole('button', { name: 'Remove Downloads from tab bar' }),
  ).toBeDisabled();
  await fireEvent.press(view.getByRole('button', { name: 'Save' }));
  await waitFor(() =>
    expect(mockSet).toHaveBeenCalledWith(
      'kinosail.player.navigation.v1',
      '["downloads"]',
    ),
  );
});
it('opens the editor by holding a tab', async () => {
  const view = await render(<BottomNavigation />);
  await waitFor(() => expect(mockGet).toHaveBeenCalled());
  await fireEvent(view.getByRole('tab', { name: 'Downloads' }), 'longPress');
  expect(view.getByText('Customize tabs')).toBeTruthy();
  expect(mockReplace).not.toHaveBeenCalled();
});

it('keeps Close available while tab preferences are being saved', async () => {
  mockSet.mockReturnValue(new Promise<void>(() => {}));
  const view = await render(<BottomNavigation />);
  await openEditor(view);
  await fireEvent.press(view.getByRole('button', { name: 'Save' }));
  await fireEvent.press(view.getByRole('button', { name: 'Close' }));
  expect(view.queryByRole('button', { name: 'Save' })).toBeNull();
  expect(mockSet).toHaveBeenCalledTimes(1);
});

it('leaves the settings action bar clear while retaining tab navigation', async () => {
  mockPath = '/settings';
  const view = await render(<BottomNavigation />);
  expect(view.getAllByRole('tab')).toHaveLength(5);
  expect(
    view.queryByRole('button', { name: 'Search your library' }),
  ).toBeNull();
});

it.each([
  ['Photos', 'photos'],
  ['Audiobooks', 'audiobooks'],
  ['My List', 'list'],
  ['History', 'history'],
])('opens %s directly from More', async (label, category) => {
  const view = await render(<BottomNavigation />);
  await fireEvent.press(view.getByRole('tab', { name: 'More' }));
  await fireEvent.press(view.getByRole('button', { name: label }));
  expect(mockReplace).toHaveBeenLastCalledWith({
    pathname: '/library',
    params: { view: category },
  });
});
it('opens the collection browser from More', async () => {
  const view = await render(<BottomNavigation />);
  await fireEvent.press(view.getByRole('tab', { name: 'More' }));
  await fireEvent.press(view.getByRole('button', { name: 'Collections' }));
  expect(mockReplace).toHaveBeenLastCalledWith('/collections');
});

it.each(['/', '/library'])(
  'floats one unified navigation bar over scrolling media on %s',
  async (pathname) => {
    mockPath = pathname;
    const view = await render(<BottomNavigation />);
    const overlay = view.getByTestId('bottom-navigation-overlay');
    expect(StyleSheet.flatten(overlay.props.style)).toMatchObject({
      position: 'absolute',
      bottom: 0,
      left: 0,
      right: 0,
    });
    expect(overlay.props.pointerEvents).toBe('box-none');
    const bar = view.getByRole('tab', { name: 'Home' }).parent!;
    expect(StyleSheet.flatten(bar.props.style)).toMatchObject({
      backgroundColor: palette.dark.surface,
      borderColor: palette.dark.line,
      borderRadius: 32,
    });
    expect(bar.props.pointerEvents).toBe('box-none');
    expect(
      StyleSheet.flatten(view.getByRole('tab', { name: 'TV' }).props.style),
    ).toMatchObject({ backgroundColor: 'transparent' });
    expect(
      view.getByRole('button', { name: 'Search your library' }),
    ).toBeTruthy();
  },
);

it.each(['/settings', '/collections', '/downloads'])(
  'keeps reserved navigation space on %s',
  async (pathname) => {
    mockPath = pathname;
    const view = await render(<BottomNavigation />);
    expect(
      view.queryByRole('button', { name: 'Search your library' }),
    ).toBeNull();
    expect(
      view.getByTestId('bottom-navigation-overlay').props.style,
    ).toBeUndefined();
  },
);

it('keeps search inside the selected music category', async () => {
  mockPath = '/library';
  mockParams = { view: 'music' };
  const view = await render(<BottomNavigation />);
  await fireEvent.press(
    view.getByRole('button', { name: 'Search your library' }),
  );
  expect(mockNavigate).toHaveBeenLastCalledWith({
    pathname: '/library',
    params: { search: '1', view: 'music' },
  });
});
