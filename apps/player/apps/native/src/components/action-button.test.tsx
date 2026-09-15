import { fireEvent, render, userEvent } from '@testing-library/react-native';
import React from 'react';
import { Platform, StyleSheet } from 'react-native';

import { palette } from '@/design/tokens';

import { ActionButton } from './action-button';

const colors = palette.dark;

describe('native action button', () => {
  const originalTV = Object.getOwnPropertyDescriptor(Platform, 'isTV')!;
  beforeEach(() => jest.useFakeTimers());
  afterEach(() => {
    Object.defineProperty(Platform, 'isTV', originalTV);
    jest.useRealTimers();
  });

  it.each([false, true])(
    'keeps focus and pressed feedback on TV=%s',
    async (tv) => {
      Object.defineProperty(Platform, 'isTV', {
        configurable: true,
        value: tv,
      });
      const onPress = jest.fn();
      const view = await render(
        <ActionButton label="Play" onPress={onPress} style={{ width: 200 }} />,
      );
      const button = () => view.getByRole('button', { name: 'Play' });
      const style = () => StyleSheet.flatten(button().props.style);
      expect(style()).toMatchObject({
        minHeight: tv ? 80 : 48,
        width: 200,
        opacity: 1,
      });
      expect(button().props.focusable).toBe(true);
      await fireEvent(button(), 'focus');
      expect(style()).toMatchObject({
        borderColor: colors.focus,
      });
      await fireEvent(button(), 'blur');
      expect(style()).toMatchObject({
        borderColor: colors.signal,
      });
      await userEvent.press(button());
      expect(onPress).toHaveBeenCalledTimes(1);
    },
  );

  it.each([false, true])('blocks busy presses with quiet=%s', async (quiet) => {
    const onPress = jest.fn();
    const view = await render(
      <ActionButton
        label="Connect"
        accessibilityLabel="Connect to server"
        onPress={onPress}
        quiet={quiet}
        busy
      />,
    );
    const button = view.getByRole('button', { name: 'Connect to server' });
    expect(button).toBeDisabled();
    expect(button.props.accessibilityState).toEqual({
      busy: true,
      disabled: true,
    });
    expect(button.props.focusable).toBe(false);
    expect(StyleSheet.flatten(button.props.style).opacity).toBe(0.45);
    expect(view.getByText('Connect')).toBeTruthy();
    expect(view.getByRole('progressbar')).toBeTruthy();
    await fireEvent.press(button);
    expect(onPress).not.toHaveBeenCalled();
  });

  it('styles a quiet disabled action without marking it busy', async () => {
    const onPress = jest.fn();
    const view = await render(
      <ActionButton label="Return" onPress={onPress} quiet disabled />,
    );
    const button = view.getByRole('button', { name: 'Return' });
    expect(button.props.accessibilityState).toEqual({
      busy: false,
      disabled: true,
    });
    expect(StyleSheet.flatten(button.props.style)).toMatchObject({
      backgroundColor: colors.surface,
      borderColor: colors.line,
    });
    expect(view.getByText('Return')).toBeTruthy();
    await fireEvent.press(button);
    expect(onPress).not.toHaveBeenCalled();
  });
});

it('makes a quiet selection visible without color alone', async () => {
  const view = await render(
    <ActionButton label="English" selected quiet onPress={jest.fn()} />,
  );
  const button = view.getByRole('button', { name: 'English' });
  expect(button.props.accessibilityState.selected).toBe(true);
  expect(view.getByText('✓')).toBeTruthy();
  expect(StyleSheet.flatten(button.props.style).borderColor).toBe(
    colors.signal,
  );
  await fireEvent(button, 'focus');
  expect(StyleSheet.flatten(button.props.style).transform).toBeUndefined();
  await view.rerender(
    <ActionButton label="English" quiet selected={false} onPress={jest.fn()} />,
  );
  expect(view.queryByText('✓')).toBeNull();
});
