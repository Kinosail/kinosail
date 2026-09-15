import { act, renderHook } from '@testing-library/react-native';
import { AccessibilityInfo, BackHandler } from 'react-native';
import { useTVPlaybackChrome } from './use-tv-playback-chrome';

let back: (() => boolean | null | undefined) | undefined;
let screenReaderChanged: ((value: boolean) => void) | undefined;
const remove = jest.fn();
beforeEach(() => {
  jest.useFakeTimers();
  jest
    .spyOn(AccessibilityInfo, 'isScreenReaderEnabled')
    .mockResolvedValue(false);
  jest
    .spyOn(AccessibilityInfo, 'addEventListener')
    .mockImplementation((_, listener) => {
      screenReaderChanged = listener as unknown as (value: boolean) => void;
      return { remove } as unknown as ReturnType<
        typeof AccessibilityInfo.addEventListener
      >;
    });
  jest
    .spyOn(BackHandler, 'addEventListener')
    .mockImplementation((_, listener) => {
      back = () => listener({ type: 'hardwareBackPress', timeStamp: 0 });
      return { remove };
    });
});
afterEach(() => {
  jest.useRealTimers();
  jest.restoreAllMocks();
});
it('hides idle controls and resets the timeout after remote activity', async () => {
  const { result } = await renderHook(() =>
    useTVPlaybackChrome(true, false, false, jest.fn()),
  );
  await act(() => jest.advanceTimersByTime(4000));
  expect(result.current.visible).toBe(true);
  await act(() => result.current.reveal());
  await act(() => jest.advanceTimersByTime(4000));
  expect(result.current.visible).toBe(true);
  await act(() => jest.advanceTimersByTime(1000));
  expect(result.current.visible).toBe(false);
  await act(() => result.current.reveal());
  expect(result.current.visible).toBe(true);
});
it('keeps controls visible while paused, buffering, or reading options', async () => {
  const { result, rerender } = await renderHook(
    ({ pinned }: { pinned: boolean }) =>
      useTVPlaybackChrome(true, pinned, false, jest.fn()),
    { initialProps: { pinned: false } },
  );
  await act(() => jest.advanceTimersByTime(5000));
  expect(result.current.visible).toBe(false);
  await rerender({ pinned: true });
  await act(() => jest.advanceTimersByTime(30000));
  expect(result.current.visible).toBe(true);
});
it('closes options before controls and lets a subsequent Back return to the route', async () => {
  const close = jest.fn();
  const { result, rerender } = await renderHook(
    ({ options }: { options: boolean }) =>
      useTVPlaybackChrome(true, options, options, close),
    { initialProps: { options: true } },
  );
  await act(() => {
    expect(back?.()).toBe(true);
  });
  expect(close).toHaveBeenCalledTimes(1);
  await rerender({ options: false });
  await act(() => {
    expect(back?.()).toBe(true);
  });
  expect(result.current.visible).toBe(false);
  expect(back?.()).toBe(false);
});
it('does not hide controls from a screen reader or intercept mobile Back', async () => {
  const { result, rerender, unmount } = await renderHook(
    ({ enabled }: { enabled: boolean }) =>
      useTVPlaybackChrome(enabled, false, false, jest.fn()),
    { initialProps: { enabled: true } },
  );
  await act(() => screenReaderChanged?.(true));
  await act(() => jest.advanceTimersByTime(30000));
  expect(result.current.visible).toBe(true);
  expect(back?.()).toBe(false);
  await rerender({ enabled: false });
  expect(remove).toHaveBeenCalled();
  await unmount();
});
