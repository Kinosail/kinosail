import { act, renderHook } from '@testing-library/react-native';
import { AccessibilityInfo } from 'react-native';
import { useReducedMotion } from './use-reduced-motion';
afterEach(() => jest.restoreAllMocks());
it('uses the OS setting and responds to changes with listener cleanup', async () => {
  let change!: (value: boolean) => void;
  const remove = jest.fn();
  jest
    .spyOn(AccessibilityInfo, 'isReduceMotionEnabled')
    .mockResolvedValue(true);
  jest
    .spyOn(AccessibilityInfo, 'addEventListener')
    .mockImplementation((event, handler) => {
      if (event === 'reduceMotionChanged')
        change = handler as (value: boolean) => void;
      return { remove };
    });
  const hook = await renderHook(() => useReducedMotion());
  expect(hook.result.current).toBe(true);
  await act(() => change(false));
  expect(hook.result.current).toBe(false);
  await hook.unmount();
  expect(remove).toHaveBeenCalled();
});
it('does not replace a newer accessibility event with a stale initial read', async () => {
  let finish!: (value: boolean) => void, change!: (value: boolean) => void;
  jest.spyOn(AccessibilityInfo, 'isReduceMotionEnabled').mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  jest
    .spyOn(AccessibilityInfo, 'addEventListener')
    .mockImplementation((event, handler) => {
      if (event === 'reduceMotionChanged')
        change = handler as (value: boolean) => void;
      return { remove: jest.fn() };
    });
  const hook = await renderHook(() => useReducedMotion());
  await act(() => change(true));
  await act(async () => finish(false));
  expect(hook.result.current).toBe(true);
});
