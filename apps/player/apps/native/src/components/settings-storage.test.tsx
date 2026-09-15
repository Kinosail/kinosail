import React from 'react';
import { fireEvent, render, within } from '@testing-library/react-native';
import { router } from 'expo-router';
import SettingsScreen from '@/app/settings';
import { defaultMediaPreferences } from '@/core/media-preferences';
const mockClient = {
  loadMediaPreferences: jest.fn().mockResolvedValue(defaultMediaPreferences),
  saveMediaPreferences: jest.fn(async (value) => value),
};
jest.mock('expo-router', () => ({
  router: { back: jest.fn(), canGoBack: jest.fn(() => true) },
}));
jest.mock('@/core/session-context', () => ({
  useSession: () => ({ client: mockClient }),
}));
jest.mock('@/core/downloads', () => ({
  downloadStorage: () => ({ total: 128 * 1024 ** 3, free: 60 * 1024 ** 3 }),
}));
jest.mock('@/core/protected-media', () => ({
  localCompatibilityAvailable: false,
}));
jest.mock('@/core/experience-cache', () => ({
  writeExperienceCache: jest.fn().mockResolvedValue(undefined),
}));
jest.mock('./progress-sync-panel', () => ({ ProgressSyncPanel: () => null }));
jest.mock('./playback-preference-controls', () => ({
  PlaybackPreferenceControls: () => null,
}));
it('offers limits that fit this device and saves an explicit unlimited choice', async () => {
  const view = await render(<SettingsScreen />);
  await view.findByRole('button', { name: '100 GB' });
  expect(view.queryByRole('button', { name: '250 GB' })).toBeNull();
  expect(view.getByText(/60.0 GB free of 128.0 GB/)).toBeTruthy();
  expect(
    view.getByRole('button', { name: '20 GB' }).props.accessibilityState
      .selected,
  ).toBe(true);
  await fireEvent.press(view.getByRole('button', { name: 'No limit' }));
  await fireEvent.press(view.getByRole('button', { name: 'Save settings' }));
  await view.findByText('Settings saved.');
  expect(mockClient.saveMediaPreferences).toHaveBeenCalledWith({
    ...defaultMediaPreferences,
    downloadLimitGiB: 0,
  });
});

it('keeps Save and Back outside scrolling settings and hides Back without history', async () => {
  const view = await render(<SettingsScreen />);
  await view.findByRole('button', { name: '100 GB' });
  const body = within(
    view.container.queryAll((node) => /ScrollView$/.test(node.type))[0],
  );
  expect(body.queryByRole('button', { name: 'Save settings' })).toBeNull();
  expect(body.queryByRole('button', { name: 'Back' })).toBeNull();
  await fireEvent.press(view.getByRole('button', { name: 'Back' }));
  expect(router.back).toHaveBeenCalled();
  jest.mocked(router.canGoBack).mockReturnValue(false);
  await view.rerender(<SettingsScreen />);
  expect(view.queryByRole('button', { name: 'Back' })).toBeNull();
  expect(view.getByRole('button', { name: 'Save settings' })).toBeTruthy();
  jest.mocked(router.canGoBack).mockReturnValue(true);
});

it('keeps advanced preferences collapsed and persists a searched language', async () => {
  const view = await render(<SettingsScreen />);
  await view.findByRole('button', { name: 'Audio language: Automatic' });
  expect(view.queryByText('Audio processing')).toBeNull();
  await fireEvent.press(
    view.getByRole('button', { name: 'Audio language: Automatic' }),
  );
  await fireEvent.changeText(view.getByLabelText('Find a language'), 'Polish');
  await fireEvent.press(view.getByRole('button', { name: 'Polish' }));
  await fireEvent.press(view.getByRole('button', { name: 'Save settings' }));
  await view.findByText('Settings saved.');
  expect(mockClient.saveMediaPreferences).toHaveBeenLastCalledWith({
    ...defaultMediaPreferences,
    playback: { ...defaultMediaPreferences.playback, audioLanguage: 'pl' },
  });
});
