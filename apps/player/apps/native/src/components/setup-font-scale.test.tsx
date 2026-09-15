import { fireEvent, render } from '@testing-library/react-native';
import React from 'react';
import * as ReactNative from 'react-native';

import { SetupFlow } from './setup-flow';

describe('SetupFlow text-size changes', () => {
  afterEach(() => jest.restoreAllMocks());

  it.each([false, true])(
    'preserves connection state when approval has started=%s',
    async (approving) => {
      const dimensions = jest
        .spyOn(
          jest.requireActual<typeof ReactNative>('react-native'),
          'useWindowDimensions',
        )
        .mockReturnValue({ width: 390, height: 844, scale: 3, fontScale: 1 });
      const startQuickConnect = jest.fn().mockResolvedValue({
        code: '381204',
        secret: 'pending-secret',
      });
      const pollQuickConnect = jest.fn(() => new Promise<null>(() => {}));
      const createClient = () => ({
        startQuickConnect,
        pollQuickConnect,
        cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
      });
      const onConnected = jest.fn();
      const component = (
        <SetupFlow createClient={createClient} onConnected={onConnected} />
      );
      const view = await render(component);
      await fireEvent.changeText(
        view.getByLabelText('Kinosail Server URL'),
        'https://kino.example',
      );
      if (approving) {
        await fireEvent.press(view.getByRole('button', { name: 'Connect' }));
        await view.findByText('381 204');
      }
      for (const fontScale of [3.2, 1]) {
        dimensions.mockReturnValue({
          width: 390,
          height: 844,
          scale: 3,
          fontScale,
        });
        await view.rerender(
          <SetupFlow createClient={createClient} onConnected={onConnected} />,
        );
        if (approving) {
          expect(view.getByText('381 204')).toBeTruthy();
          expect(
            view.getByRole('button', { name: 'Use another server' }),
          ).toBeTruthy();
        } else {
          expect(view.getByLabelText('Kinosail Server URL').props.value).toBe(
            'https://kino.example',
          );
        }
        expect(startQuickConnect).toHaveBeenCalledTimes(approving ? 1 : 0);
        expect(pollQuickConnect).toHaveBeenCalledTimes(approving ? 1 : 0);
        expect(onConnected).not.toHaveBeenCalled();
      }
    },
  );
});
