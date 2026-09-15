import { act, renderHook, waitFor } from '@testing-library/react-native';
import { usePlaybackExperience } from './use-playback-experience';
import { readExperienceCache, writeExperienceCache } from './experience-cache';
import { defaultPlaybackPreferences } from './media-preferences';
import { deferred } from '@/testing/route-fixtures';
import type { KinosailClient } from './server-client';

jest.mock('./experience-cache', () => ({
  readExperienceCache: jest.fn(),
  writeExperienceCache: jest.fn(),
}));
const playback = { ...defaultPlaybackPreferences, dialogueBoost: true };
const client = () =>
  ({
    loadPlaybackPreferences: jest.fn().mockResolvedValue({ playback }),
    loadBookmarks: jest.fn().mockResolvedValue([]),
  }) as unknown as KinosailClient;
beforeEach(() => {
  jest.clearAllMocks();
  jest.mocked(readExperienceCache).mockResolvedValue(null);
  jest.mocked(writeExperienceCache).mockResolvedValue(undefined);
});
it('does not serialize the Server response behind slow device storage', async () => {
  jest.mocked(readExperienceCache).mockReturnValue(new Promise(() => {}));
  jest.mocked(writeExperienceCache).mockReturnValue(new Promise(() => {}));
  const value = client();
  const hook = await renderHook(() => usePlaybackExperience(value, 'movie'));
  await waitFor(() => expect(hook.result.current.loaded).toBe(true));
  expect(hook.result.current.preferences.dialogueBoost).toBe(true);
});
it('does not replace a live preference with defaults when persisting the cache fails', async () => {
  jest
    .mocked(writeExperienceCache)
    .mockRejectedValue(new Error('storage unavailable'));
  const value = client();
  const hook = await renderHook(() => usePlaybackExperience(value, 'movie'));
  await waitFor(() => expect(hook.result.current.loaded).toBe(true));
  expect(hook.result.current.preferences).toEqual(playback);
  expect(hook.result.current.error).toBe('');
});
it('waits for saved preferences on network failure and ignores a previous item response', async () => {
  const cache = deferred<typeof playback | null>();
  jest.mocked(readExperienceCache).mockReturnValueOnce(cache.promise);
  const value = client();
  jest
    .mocked(value.loadPlaybackPreferences)
    .mockRejectedValueOnce(new Error('offline'));
  const hook = await renderHook(
    ({ id }: { id: string }) => usePlaybackExperience(value, id),
    {
      initialProps: { id: 'first' },
    },
  );
  expect(hook.result.current.loaded).toBe(false);
  await hook.rerender({ id: 'second' });
  await waitFor(() => expect(hook.result.current.loaded).toBe(true));
  await act(async () => cache.resolve(defaultPlaybackPreferences));
  expect(hook.result.current.preferences).toEqual(playback);
});
