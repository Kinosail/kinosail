import { fireEvent, render, waitFor } from '@testing-library/react-native';
import React from 'react';
import * as ReactNative from 'react-native';

import { SetupFlow } from './setup-flow';

describe('SetupFlow interaction', () => {
  afterEach(() => jest.restoreAllMocks());

  it('rejects an invalid server before creating a request', async () => {
    const start = jest.fn();
    const view = await render(
      <SetupFlow
        createClient={() => ({
          startQuickConnect: start,
          cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
          pollQuickConnect: jest.fn(),
        })}
        onConnected={jest.fn()}
      />,
    );

    await fireEvent.changeText(
      view.getByLabelText('Kinosail Server URL'),
      'http://public.example',
    );
    await fireEvent.press(view.getByRole('button', { name: 'Connect' }));

    expect(
      await view.findByText('Enter a valid Kinosail Server URL.'),
    ).toBeTruthy();
    expect(start).not.toHaveBeenCalled();
  });

  it('shows the six-digit code while authorization is pending', async () => {
    const startQuickConnect = jest.fn().mockResolvedValue({
      code: '381204',
      secret: 'secret-value',
    });
    const view = await render(
      <SetupFlow
        createClient={() => ({
          startQuickConnect,
          cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
          pollQuickConnect: jest.fn().mockResolvedValue(null),
        })}
        onConnected={jest.fn()}
      />,
    );

    await fireEvent.changeText(
      view.getByLabelText('Kinosail Server URL'),
      'https://kino.example',
    );
    await fireEvent.press(view.getByRole('button', { name: 'Connect' }));

    const code = await view.findByText('381 204');
    expect(ReactNative.StyleSheet.flatten(code.parent!.props.style)).toEqual({
      gap: 24,
    });
    expect(code.props.selectable).toBe(true);
    expect(ReactNative.StyleSheet.flatten(code.props.style)).toEqual({
      color: '#C8F169',
      fontSize: 48,
      fontVariant: ['tabular-nums'],
      fontWeight: '900',
      letterSpacing: 6,
      marginVertical: 16,
    });
    expect(view.getByText('STEP 2 OF 2').props.style).toEqual([
      { fontSize: 11, fontWeight: '900', letterSpacing: 1.8 },
      { color: '#9CA391' },
    ]);
    expect(
      view.getByRole('header', { name: 'Sign in with your phone' }).props.style,
    ).toEqual([
      { fontSize: 28, fontWeight: '800', letterSpacing: -0.6 },
      { color: '#F6F8EF' },
    ]);
    expect(view.getByText('Waiting for approval…').props).toMatchObject({
      accessibilityLiveRegion: 'polite',
      style: [{ fontSize: 14, fontWeight: '700' }, { color: '#9CA391' }],
    });
    expect(
      view.getByText(
        'Approve from a signed-in Player app or browser.',
      ).props.style,
    ).toEqual([{ fontSize: 16, lineHeight: 24 }, { color: '#9CA391' }]);
    expect(startQuickConnect).toHaveBeenCalledTimes(1);
  });

  it('clears a polling error when choosing another server', async () => {
    const view = await render(
      <SetupFlow
        createClient={() => ({
          startQuickConnect: jest.fn().mockResolvedValue({
            code: '381204',
            secret: 'secret-value',
          }),
          cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
          pollQuickConnect: jest
            .fn()
            .mockRejectedValue(
              new Error('Kinosail Server could not complete the request.'),
            ),
        })}
        onConnected={jest.fn()}
      />,
    );

    await fireEvent.changeText(
      view.getByLabelText('Kinosail Server URL'),
      'https://kino.example',
    );
    await fireEvent.press(view.getByRole('button', { name: 'Connect' }));
    expect(
      await view.findByText('Kinosail Server could not complete the request.'),
    ).toBeTruthy();

    expect(
      view.getByText('Kinosail Server could not complete the request.').props
        .style,
    ).toEqual([
      { fontSize: 14, fontWeight: '700', lineHeight: 20 },
      { color: '#FF6B67' },
    ]);

    expect(view.queryByText('Waiting for approval…')).toBeNull();
    expect(view.getByText('Approval check stopped.')).toBeTruthy();
    expect(view.getByRole('button', { name: 'Retry approval' })).toBeTruthy();

    await fireEvent.press(
      view.getByRole('button', { name: 'Use another server' }),
    );

    expect(
      view.queryByText('Kinosail Server could not complete the request.'),
    ).toBeNull();
    expect(view.queryByText('Stryker was here!')).toBeNull();
    expect(view.getByLabelText('Kinosail Server URL').props.value).toBe(
      'https://kino.example',
    );
  });

  it('returns the approved session', async () => {
    const onConnected = jest.fn();
    const view = await render(
      <SetupFlow
        createClient={() => ({
          startQuickConnect: jest.fn().mockResolvedValue({
            code: '381204',
            secret: 'secret-value',
          }),
          cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
          pollQuickConnect: jest.fn().mockResolvedValue('viewer-token'),
        })}
        onConnected={onConnected}
      />,
    );
    await fireEvent.changeText(
      view.getByLabelText('Kinosail Server URL'),
      'https://kino.example',
    );
    await fireEvent.press(view.getByRole('button', { name: 'Connect' }));
    await view.findByText('381 204');

    await waitFor(() =>
      expect(onConnected).toHaveBeenCalledWith({
        baseURL: 'https://kino.example',
        token: 'viewer-token',
      }),
    );
  });

  it('waits for a pending poll before scheduling the next request', async () => {
    jest.useFakeTimers();
    let finishPoll: ((value: null) => void) | undefined;
    const pollQuickConnect = jest.fn(
      () =>
        new Promise<null>((resolve) => {
          finishPoll = resolve;
        }),
    );
    const view = await render(
      <SetupFlow
        createClient={() => ({
          startQuickConnect: jest.fn().mockResolvedValue({
            code: '381204',
            secret: 'secret-value',
          }),
          cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
          pollQuickConnect,
        })}
        onConnected={jest.fn()}
      />,
    );

    await fireEvent.changeText(
      view.getByLabelText('Kinosail Server URL'),
      'https://kino.example',
    );
    await fireEvent.press(view.getByRole('button', { name: 'Connect' }));
    await view.findByText('381 204');
    expect(pollQuickConnect).toHaveBeenCalledTimes(1);

    await jest.advanceTimersByTimeAsync(5000);
    expect(pollQuickConnect).toHaveBeenCalledTimes(1);
    finishPoll?.(null);
    await Promise.resolve();
    await jest.advanceTimersByTimeAsync(1000);
    expect(pollQuickConnect).toHaveBeenCalledTimes(2);
    jest.useRealTimers();
  });
});
