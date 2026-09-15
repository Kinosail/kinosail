import { fireEvent, render, waitFor } from '@testing-library/react-native';
import React from 'react';
import { StyleSheet } from 'react-native';

import { SetupFlow } from './setup-flow';

afterEach(() => jest.restoreAllMocks());

it.each([320, 359, 360, 390])(
  'retries stopped approval at width %s without requesting another device code',
  async (width) => {
    jest
      .spyOn(
        jest.requireActual<typeof import('react-native')>('react-native'),
        'useWindowDimensions',
      )
      .mockReturnValue({ width, height: 844, fontScale: 1, scale: 3 });
    const startQuickConnect = jest
      .fn()
      .mockResolvedValue({ code: '123456', secret: 'challenge' });
    const pollQuickConnect = jest
      .fn()
      .mockRejectedValueOnce(new Error('Server unavailable.'))
      .mockResolvedValue('viewer-token');
    const onConnected = jest.fn();
    const view = await render(
      <SetupFlow
        createClient={() => ({
          startQuickConnect,
          pollQuickConnect,
          cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
        })}
        onConnected={onConnected}
      />,
    );
    expect(view.getByText('The shortest path to play.')).toBeTruthy();
    await fireEvent.changeText(
      view.getByLabelText('Kinosail Server URL'),
      'https://kino.example',
    );
    await fireEvent.press(view.getByRole('button', { name: 'Connect' }));
    expect(await view.findByText('Approval check stopped.')).toBeTruthy();
    expect(view.queryByText('The shortest path to play.')).toBeNull();
    await fireEvent.press(view.getByRole('button', { name: 'Retry approval' }));
    await waitFor(() =>
      expect(onConnected).toHaveBeenCalledWith({
        baseURL: 'https://kino.example',
        token: 'viewer-token',
      }),
    );
    expect(view.queryByText('Server unavailable.')).toBeNull();
    expect(view.queryByText('Approval check stopped.')).toBeNull();
    expect(view.getByText('123 456')).toBeTruthy();
    expect(
      StyleSheet.flatten(view.getByText('123 456').props.style),
    ).toMatchObject({
      fontSize: width < 360 ? 40 : 48,
      letterSpacing: width < 360 ? 4 : 6,
    });
    expect(startQuickConnect).toHaveBeenCalledTimes(1);
    expect(pollQuickConnect).toHaveBeenCalledTimes(2);
    expect(pollQuickConnect).toHaveBeenLastCalledWith('challenge');
  },
);
