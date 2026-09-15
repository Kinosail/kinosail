import { fireEvent, render } from '@testing-library/react-native';
import React from 'react';
import { StyleSheet } from 'react-native';

import { palette } from '@/design/tokens';

import { ScreenState } from './screen-state';

let mockScheme: 'dark' | 'light' = 'dark';
jest.mock('@/design/theme-context', () => ({
  useThemePreference: () => ({ scheme: mockScheme }),
}));

describe('native recovery and loading state', () => {
  it.each(['dark', 'light'] as const)(
    'announces loading and preserves the %s screen geometry',
    async (scheme) => {
      mockScheme = scheme;
      const colors = palette[scheme];
      const message = 'Preparing direct playback…';
      const view = await render(<ScreenState loading message={message} />);
      const root = view.root!;
      expect(root.props.role).toBe('main');
      expect(StyleSheet.flatten(root.props.style)).toEqual({
        alignItems: 'center',
        flex: 1,
        gap: 24,
        justifyContent: 'center',
        padding: 32,
        backgroundColor: colors.background,
      });
      expect(view.getByRole('image', { name: 'Kinosail Player' })).toBeTruthy();
      expect(view.getByLabelText(message).props).toMatchObject({
        accessibilityLabel: message,
        accessibilityRole: 'progressbar',
        accessibilityState: { busy: true },
      });
      const status = view.getByText(message);
      expect(status.props.accessibilityLiveRegion).toBe('polite');
      expect(StyleSheet.flatten(status.props.style)).toEqual({
        fontSize: 16,
        lineHeight: 24,
        maxWidth: 480,
        textAlign: 'center',
        color: colors.muted,
      });
      expect(view.queryAllByRole('button')).toHaveLength(0);
    },
  );

  it.each(['dark', 'light'] as const)(
    'shows distinct, working recovery actions in %s',
    async (scheme) => {
      mockScheme = scheme;
      const colors = palette[scheme];
      const retry = jest.fn();
      const changeServer = jest.fn();
      const message = 'Could not reach Kinosail Server.';
      const view = await render(
        <ScreenState
          message={message}
          action="Try again"
          onAction={retry}
          secondaryAction="Use another server"
          onSecondaryAction={changeServer}
        />,
      );
      expect(view.queryByLabelText(message)).toBeNull();
      const status = view.getByText(message);
      expect(status.props.accessibilityLiveRegion).toBe('assertive');
      expect(StyleSheet.flatten(status.props.style).color).toBe(colors.danger);
      const actions = view.getAllByRole('button');
      expect(actions).toHaveLength(2);
      expect(StyleSheet.flatten(actions[0].parent!.props.style)).toEqual({
        alignItems: 'center',
        flexDirection: 'row',
        flexWrap: 'wrap',
        gap: 8,
        justifyContent: 'center',
      });
      const primary = view.getByRole('button', { name: 'Try again' });
      const secondary = view.getByRole('button', {
        name: 'Use another server',
      });
      expect(primary.parent).toBe(secondary.parent);
      expect(StyleSheet.flatten(primary.props.style).backgroundColor).toBe(
        colors.signal,
      );
      expect(StyleSheet.flatten(secondary.props.style).backgroundColor).toBe(
        colors.surface,
      );
      expect(StyleSheet.flatten(secondary.props.style).borderColor).toBe(
        colors.line,
      );
      await fireEvent.press(primary);
      await fireEvent.press(secondary);
      expect(retry).toHaveBeenCalledTimes(1);
      expect(changeServer).toHaveBeenCalledTimes(1);
    },
  );

  it('does not expose an action without its handler', async () => {
    const view = await render(
      <ScreenState message="Server unavailable" action="Retry" />,
    );
    expect(view.queryAllByRole('button')).toHaveLength(0);
    expect(view.queryByLabelText('Server unavailable')).toBeNull();
  });

  it('does not expose secondary recovery without a primary recovery action', async () => {
    const view = await render(
      <ScreenState
        message="Loading library…"
        onAction={jest.fn()}
        secondaryAction="Use another server"
        onSecondaryAction={jest.fn()}
      />,
    );
    expect(view.queryAllByRole('button')).toHaveLength(0);
    expect(view.queryByRole('progressbar')).toBeNull();
  });

  it.each(['label', 'handler'] as const)(
    'omits incomplete secondary recovery without its %s',
    async (missing) => {
      const view = await render(
        <ScreenState
          message="Server unavailable"
          action="Retry"
          onAction={jest.fn()}
          secondaryAction={
            missing === 'label' ? undefined : 'Use another server'
          }
          onSecondaryAction={missing === 'handler' ? undefined : jest.fn()}
        />,
      );
      expect(view.getAllByRole('button')).toHaveLength(1);
      expect(view.getByRole('button', { name: 'Retry' })).toBeTruthy();
      expect(
        view.queryByRole('button', { name: 'Use another server' }),
      ).toBeNull();
    },
  );
});

it('keeps an escapable loading screen polite, busy and visually distinct from errors', async () => {
  mockScheme = 'dark';
  const back = jest.fn();
  const view = await render(
    <ScreenState
      loading
      message="Opening book…"
      action="Back"
      actionQuiet
      onAction={back}
    />,
  );
  expect(view.getByRole('progressbar', { name: 'Opening book…' })).toBeTruthy();
  const message = view.getByText('Opening book…');
  expect(message.props.accessibilityLiveRegion).toBe('polite');
  expect(StyleSheet.flatten(message.props.style).color).toBe(
    palette.dark.muted,
  );
  await fireEvent.press(view.getByRole('button', { name: 'Back' }));
  expect(back).toHaveBeenCalledTimes(1);
  await view.unmount();
});

it('does not imply loading for a message without an action', async () => {
  const screen = await render(
    <ScreenState message="Connect to your Kinosail Server to browse music." />,
  );
  expect(screen.queryByRole('progressbar')).toBeNull();
});
