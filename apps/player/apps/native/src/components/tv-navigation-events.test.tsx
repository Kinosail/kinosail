import { act, renderHook } from '@testing-library/react-native';
import { BackHandler, Platform, TVEventControl } from 'react-native';
import { router } from 'expo-router';
import { useTVNavigation } from './tv-navigation-events.native';
let mockPath = '/';
let back: (() => boolean | null | undefined) | undefined;
jest.mock('expo-router', () => ({
  usePathname: () => mockPath,
  router: { canGoBack: jest.fn(), back: jest.fn(), replace: jest.fn() },
}));
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
beforeEach(() => {
  jest.clearAllMocks();
  mockPath = '/';
  jest.spyOn(TVEventControl, 'enableTVMenuKey').mockImplementation(() => {});
  jest.spyOn(TVEventControl, 'disableTVMenuKey').mockImplementation(() => {});
  jest
    .spyOn(BackHandler, 'addEventListener')
    .mockImplementation((_, listener) => {
      back = () => listener({ type: 'hardwareBackPress', timeStamp: 0 });
      return { remove: jest.fn() };
    });
});
afterEach(() => jest.restoreAllMocks());
it('leaves Home to tvOS and returns nested routes to their previous screen', async () => {
  const view = await renderHook(() => useTVNavigation());
  expect(TVEventControl.disableTVMenuKey).toHaveBeenCalled();
  expect(back?.()).toBe(false);
  mockPath = '/library';
  await view.rerender(undefined);
  expect(TVEventControl.enableTVMenuKey).toHaveBeenCalled();
  jest.mocked(router.canGoBack).mockReturnValue(true);
  await act(() => {
    expect(back?.()).toBe(true);
  });
  expect(router.back).toHaveBeenCalledTimes(1);
  expect(BackHandler.addEventListener).toHaveBeenCalledTimes(1);
  await view.unmount();
  expect(TVEventControl.disableTVMenuKey).toHaveBeenCalled();
});
it('returns a deep link to Home when there is no navigation history', async () => {
  mockPath = '/item/example';
  jest.mocked(router.canGoBack).mockReturnValue(false);
  await renderHook(() => useTVNavigation());
  await act(() => {
    expect(back?.()).toBe(true);
  });
  expect(router.replace).toHaveBeenCalledWith('/');
});
it('does not change phone remote handling', async () => {
  const descriptor = Object.getOwnPropertyDescriptor(Platform, 'isTV')!;
  Object.defineProperty(Platform, 'isTV', { configurable: true, value: false });
  try {
    await renderHook(() => useTVNavigation());
    expect(BackHandler.addEventListener).not.toHaveBeenCalled();
    expect(TVEventControl.enableTVMenuKey).not.toHaveBeenCalled();
  } finally {
    Object.defineProperty(Platform, 'isTV', descriptor);
  }
});
