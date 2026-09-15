import { fireEvent, render } from '@testing-library/react-native';
import React from 'react';
import * as Native from 'react-native';

import type { Home } from '@/core/contract';
import { HomeView } from './home-view';

const emptyHome: Home = {
  server: 'Living room',
  viewer: { id: 'viewer', name: 'A long viewer profile name', owner: false },
  continueWatching: [],
  recent: [],
};

describe('home brand presence', () => {
  const originalTV = Object.getOwnPropertyDescriptor(Native.Platform, 'isTV')!;
  afterEach(() => {
    jest.restoreAllMocks();
    Object.defineProperty(Native.Platform, 'isTV', originalTV);
  });

  it.each([
    [false, 320, 1, true],
    [false, 390, 1, true],
    [false, 320, 2, true],
    [true, 1920, 1, true],
  ] as const)(
    'brands an empty library on TV=%s width=%s scale=%s',
    async (tv, width, fontScale, wordmark) => {
      Object.defineProperty(Native.Platform, 'isTV', {
        configurable: true,
        value: tv,
      });
      jest.spyOn(Native, 'useWindowDimensions').mockReturnValue({
        width,
        height: 1080,
        scale: 1,
        fontScale,
      });
      const refresh = jest.fn();
      const mediaURL = jest.fn((path: string) => path);
      const view = await render(
        <HomeView
          home={emptyHome}
          mediaURL={mediaURL}
          onOpen={jest.fn()}
          onRefresh={refresh}
        />,
      );
      expect(view.getByRole('image', { name: 'Kinosail Player' })).toBeTruthy();
      expect(Boolean(view.queryByText('KINOSAIL'))).toBe(wordmark);
      expect(
        Boolean(view.queryByTestId('home-brand-backdrop', {
          includeHiddenElements: true,
        })),
      ).toBe(!tv);
      expect(mediaURL).not.toHaveBeenCalled();
      await fireEvent.press(
        view.getByRole('button', { name: 'Refresh library' }),
      );
      expect(refresh).toHaveBeenCalledTimes(1);
    },
  );
});
