import { act, fireEvent, render } from '@testing-library/react-native';
import React from 'react';
import * as ReactNative from 'react-native';

import { SetupFlow } from './setup-flow';
import { setupFlowStyles } from './setup-flow.styles';

describe('SetupFlow', () => {
  afterEach(() => jest.restoreAllMocks());

  it('keeps the setup design geometry exact', () => {
    const expected = {
      screen: { flex: 1 },
      scroll: {
        alignItems: 'stretch',
        flexGrow: 1,
        gap: 48,
        justifyContent: 'center',
        padding: 32,
      },
      scrollCompact: { gap: 24, padding: 24 },
      intro: { alignSelf: 'center', gap: 24, maxWidth: 640 },
      introCompact: { gap: 16 },
      eyebrow: {
        fontSize: 12,
        fontWeight: '900',
        letterSpacing: 2.5,
        marginTop: 32,
      },
      eyebrowCompact: { marginTop: 8 },
      title: {
        fontSize: 48,
        fontWeight: '900',
        letterSpacing: -1.5,
        lineHeight: 51,
        maxWidth: 540,
      },
      titleCompact: { fontSize: 40, lineHeight: 43 },
      summary: { fontSize: 19, lineHeight: 29, maxWidth: 540 },
      rule: { height: 1, maxWidth: 540 },
      note: { fontSize: 13, lineHeight: 20, maxWidth: 480 },
      panel: {
        alignSelf: 'center',
        borderRadius: 18,
        borderWidth: 1,
        flexBasis: 460,
        gap: 24,
        maxWidth: 520,
        padding: 32,
        width: '100%',
      },
      panelCompact: { flexBasis: 'auto', padding: 24 },
      step: { fontSize: 11, fontWeight: '900', letterSpacing: 1.8 },
      form: { gap: 16 },
      challenge: { gap: 24 },
      panelTitle: { fontSize: 28, fontWeight: '800', letterSpacing: -0.6 },
      body: { fontSize: 16, lineHeight: 24 },
      label: { fontSize: 13, fontWeight: '800', marginTop: 8 },
      input: {
        borderRadius: 10,
        borderWidth: 1,
        fontSize: 16,
        minHeight: 52,
        paddingHorizontal: 16,
      },
      error: { fontSize: 14, fontWeight: '700', lineHeight: 20 },
      code: {
        fontSize: 48,
        fontVariant: ['tabular-nums'],
        fontWeight: '900',
        letterSpacing: 6,
        marginVertical: 16,
      },
      waiting: { fontSize: 14, fontWeight: '700' },
      codeCompact: { fontSize: 40, letterSpacing: 4 },
    } as const;
    expect(Object.keys(setupFlowStyles)).toEqual(Object.keys(expected));
    for (const name of Object.keys(expected) as (keyof typeof expected)[]) {
      expect(ReactNative.StyleSheet.flatten(setupFlowStyles[name])).toEqual(
        expected[name],
      );
    }
  });

  it('keeps connection progress visible and ignores repeated keyboard submits', async () => {
    let reject: (error: Error) => void = () => {};
    const startQuickConnect = jest.fn(
      () =>
        new Promise<never>((_, fail) => {
          reject = fail;
        }),
    );
    const view = await render(
      <SetupFlow
        createClient={() => ({
          startQuickConnect,
          cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
          pollQuickConnect: jest.fn(),
        })}
        onConnected={jest.fn()}
      />,
    );
    const input = view.getByLabelText('Kinosail Server URL');
    await fireEvent.changeText(input, 'https://kino.example');
    await act(() => {
      void input.props.onSubmitEditing();
    });
    await act(() => {
      void input.props.onSubmitEditing();
    });
    expect(startQuickConnect).toHaveBeenCalledTimes(1);
    expect(view.getByText('Connecting…')).toBeTruthy();
    expect(
      view.getByRole('button', { name: 'Connecting…' }).props
        .accessibilityState,
    ).toMatchObject({ busy: true, disabled: true });
    await act(async () => reject(new Error('Server unavailable. Try again.')));
    expect(
      await view.findByText('Server unavailable. Try again.'),
    ).toBeTruthy();
    expect(input.props.value).toBe('https://kino.example');
    expect(input.props.editable).toBe(true);
    expect(ReactNative.StyleSheet.flatten(input.props.style).borderColor).toBe(
      '#FF6B67',
    );
    expect(
      view.getByText('Server unavailable. Try again.').props
        .accessibilityLiveRegion,
    ).toBe('assertive');
    expect(
      view.getByText('Server unavailable. Try again.').props.style,
    ).toEqual([
      { fontSize: 14, fontWeight: '700', lineHeight: 20 },
      { color: '#FF6B67' },
    ]);
    expect(
      view.getByRole('button', { name: 'Connect' }).props.accessibilityState,
    ).toMatchObject({ busy: false, disabled: false });
  });
});
