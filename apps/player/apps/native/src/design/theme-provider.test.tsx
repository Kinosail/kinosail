import { act, renderHook } from '@testing-library/react-native';
import * as Native from 'react-native';

import { deferred } from '@/testing/route-fixtures';

import { ThemePreferenceProvider, useThemePreference } from './theme-context';

const mockGet = jest.fn<Promise<string | null>, [string]>();
const mockSet = jest.fn<Promise<void>, [string, string]>();
jest.mock('@/core/platform-storage', () => ({
  platformStorage: {
    get: (key: string) => mockGet(key),
    set: (key: string, value: string) => mockSet(key, value),
  },
}));

describe('theme provider', () => {
  beforeEach(() => {
    mockGet.mockReset().mockResolvedValue(null);
    mockSet.mockReset().mockResolvedValue(undefined);
    jest.spyOn(Native, 'useColorScheme').mockReturnValue('light');
  });
  afterEach(() => jest.restoreAllMocks());

  it('has a safe dark default outside the provider', async () => {
    const { result } = await renderHook(useThemePreference);
    expect(result.current.preference).toBe('dark');
    expect(result.current.scheme).toBe('dark');
    await act(() => result.current.cycle());
    expect(mockSet).not.toHaveBeenCalled();
  });

  it.each(['dark', 'light', 'system', 'invalid', '', null])(
    'restores only supported saved preferences: %s',
    async (stored) => {
      mockGet.mockResolvedValue(stored);
      const { result } = await renderHook(useThemePreference, {
        wrapper: ThemePreferenceProvider,
      });
      expect(result.current.preference).toBe(
        stored === 'light' || stored === 'system' ? stored : 'dark',
      );
      expect(result.current.scheme).toBe(
        stored === 'light' || stored === 'system' ? 'light' : 'dark',
      );
      expect(mockGet).toHaveBeenCalledWith('kinosail.player.theme.v1');
      expect(mockSet).not.toHaveBeenCalled();
    },
  );

  it('cycles and persists each preference and follows system changes', async () => {
    const { result, rerender } = await renderHook(useThemePreference, {
      wrapper: ThemePreferenceProvider,
    });
    for (const preference of ['light', 'system', 'dark']) {
      await act(() => result.current.cycle());
      expect(result.current.preference).toBe(preference);
      expect(result.current.scheme).toBe(
        preference === 'dark' ? 'dark' : 'light',
      );
      expect(mockSet).toHaveBeenLastCalledWith(
        'kinosail.player.theme.v1',
        preference,
      );
    }
    await act(() => result.current.cycle());
    await act(() => result.current.cycle());
    jest.mocked(Native.useColorScheme).mockReturnValue('dark');
    await rerender(undefined);
    expect(result.current.preference).toBe('system');
    expect(result.current.scheme).toBe('dark');
  });

  it('ignores a late restore after unmount', async () => {
    const load = deferred<string | null>();
    mockGet.mockReturnValue(load.promise);
    const { result, unmount } = await renderHook(useThemePreference, {
      wrapper: ThemePreferenceProvider,
    });
    await unmount();
    await act(async () => load.resolve('light'));
    expect(result.current.preference).toBe('dark');
    expect(mockSet).not.toHaveBeenCalled();
  });
});
