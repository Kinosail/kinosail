jest.mock('./airplay-button', () => ({ AirPlayButton: () => null }));
import React from 'react';
import { act, fireEvent, render } from '@testing-library/react-native';
import { routeItem, routeSource } from '@/testing/route-fixtures';
import { CompatibilityPlayer } from './compatibility-player';
import { openProtectedMedia } from '@/core/protected-media';
jest.mock('@/core/protected-media', () => ({
  openProtectedMedia: jest
    .fn()
    .mockResolvedValue('http://127.0.0.1:1234/capability'),
  closeProtectedMedia: jest.fn(),
  nextProtectedMediaID: () => 1,
}));
jest.mock('./local-video', () => {
  const ReactModule = jest.requireActual('react');
  const { View } = jest.requireActual('react-native');
  return {
    __esModule: true,
    default: ReactModule.forwardRef((props: object, ref: unknown) =>
      ReactModule.createElement(View, { ...props, ref, testID: 'local-video' }),
    ),
  };
});
import { defaultPlaybackPreferences } from '@/core/media-preferences';
const experience = {
  loaded: true,
  preferences: defaultPlaybackPreferences,
  error: '',
  saving: false,
  bookmarks: [],
  change: jest.fn(),
  reset: jest.fn(),
  bookmark: jest.fn(),
  removeBookmark: jest.fn(),
};
it('does not reset progress when opening the original file fails', async () => {
  jest
    .mocked(openProtectedMedia)
    .mockRejectedValueOnce(new Error('Unavailable'));
  const save = jest.fn();
  const view = await render(
    <CompatibilityPlayer
      item={routeItem}
      source={{ ...routeSource, start: 0 }}
      experience={experience}
      progressMessage=""
      onBack={jest.fn()}
      onRecover={jest.fn()}
      saveProgress={save}
    />,
  );
  await view.findByText(
    'The original file could not be opened on this device.',
  );
  await view.unmount();
  expect(save).not.toHaveBeenCalled();
});
