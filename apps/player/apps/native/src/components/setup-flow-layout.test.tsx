import { render } from '@testing-library/react-native';
import React from 'react';
import * as ReactNative from 'react-native';

import { SetupFlow } from './setup-flow';

type NativeNode = NonNullable<Awaited<ReturnType<typeof render>>['root']>;
const child = (node: NativeNode, index: number): NativeNode => {
  const value = node.children[index];
  if (typeof value === 'string') throw new Error('Expected a native element.');
  return value;
};
const exposeKeyboardAvoidingView = () => {
  const native = jest.requireActual<typeof ReactNative>('react-native');
  class Exposed extends native.KeyboardAvoidingView {
    override render() {
      return React.createElement(native.View, this.props);
    }
  }
  jest.spyOn(native, 'KeyboardAvoidingView', 'get').mockReturnValue(Exposed);
};

describe('SetupFlow layout', () => {
  afterEach(() => jest.restoreAllMocks());

  it.each([
    [419, 1, true, false, true],
    [420, 1, false, false, true],
    [819, 1, false, false, true],
    [820, 1, false, true, true],
    [1366, 1.49, false, true, true],
    [1366, 1.5, true, false, false],
  ] as const)(
    'honors setup reflow at width=%s scale=%s',
    async (width, fontScale, compact, wide, introduction) => {
      jest
        .spyOn(
          jest.requireActual<typeof ReactNative>('react-native'),
          'useWindowDimensions',
        )
        .mockReturnValue({ width, height: 900, scale: 2, fontScale });
      const view = await render(
        <SetupFlow
          createClient={() => ({
            startQuickConnect: jest.fn(),
            cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
            pollQuickConnect: jest.fn(),
          })}
          onConnected={jest.fn()}
        />,
      );
      const panel = view.getByRole('header', { name: 'Find your server' })
        .parent!.parent!;
      const scroll = panel.parent!;
      const intro = child(scroll, 0);
      let scrollView: NativeNode | null = scroll;
      while (
        scrollView &&
        scrollView.props.contentContainerStyle === undefined
      ) {
        scrollView = scrollView.parent as NativeNode | null;
      }
      expect(scrollView).not.toBeNull();
      expect(
        ReactNative.StyleSheet.flatten(scrollView!.props.contentContainerStyle),
      ).toMatchObject({
        flexDirection: wide ? 'row' : 'column',
        gap: compact ? 24 : 48,
        padding: compact ? 24 : 32,
      });
      expect(ReactNative.StyleSheet.flatten(intro.props.style)).toMatchObject({
        flexBasis: wide ? '48%' : 'auto',
        gap: compact ? 16 : 24,
      });
      expect(Boolean(view.queryByText('The shortest path to play.'))).toBe(
        introduction,
      );
      expect(Boolean(view.queryByText('KINOSAIL'))).toBe(fontScale < 1.5);
      expect(Boolean(view.queryByText(/Your server, your library/))).toBe(
        introduction && !compact,
      );
      if (introduction && !compact) {
        expect(view.getByText(/Your server, your library/).props.style).toEqual(
          [
            { fontSize: 13, lineHeight: 20, maxWidth: 480 },
            { color: '#9CA391' },
          ],
        );
        const rule = child(intro, 4);
        expect(ReactNative.StyleSheet.flatten(rule.props.style)).toEqual({
          backgroundColor: '#2B3024',
          height: 1,
          maxWidth: 540,
        });
      }
    },
  );

  it.each(['ios', 'android'] as const)(
    'uses the correct keyboard behavior on %s',
    async (os) => {
      exposeKeyboardAvoidingView();
      jest.replaceProperty(ReactNative.Platform, 'OS', os);
      const view = await render(
        <SetupFlow
          createClient={() => ({
            startQuickConnect: jest.fn(),
            cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
            pollQuickConnect: jest.fn(),
          })}
          onConnected={jest.fn()}
        />,
      );
      expect(child(view.root!, 0).props.behavior).toBe(
        os === 'ios' ? 'padding' : undefined,
      );
    },
  );

  it.each([390, 1024])(
    'keeps the Android form available at width %s',
    async (width) => {
      jest.replaceProperty(ReactNative.Platform, 'OS', 'android');
      jest
        .spyOn(
          jest.requireActual<typeof ReactNative>('react-native'),
          'useWindowDimensions',
        )
        .mockReturnValue({ width, height: 900, scale: 2, fontScale: 1 });
      const view = await render(
        <SetupFlow
          createClient={() => ({
            startQuickConnect: jest.fn(),
            cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
            pollQuickConnect: jest.fn(),
          })}
          onConnected={jest.fn()}
        />,
      );
      expect(view.getByText('The shortest path to play.')).toBeTruthy();
      expect(view.getByLabelText('Kinosail Server URL')).toBeTruthy();
      expect(view.getByRole('button', { name: 'Connect' })).toBeTruthy();
      expect(Boolean(view.queryByText(/Your server, your library/))).toBe(
        width >= 420,
      );
    },
  );

  it('uses the scrollable compact layout for accessibility text sizes', async () => {
    jest
      .spyOn(
        jest.requireActual<typeof ReactNative>('react-native'),
        'useWindowDimensions',
      )
      .mockReturnValue({
        fontScale: 3.2,
        height: 1024,
        scale: 2,
        width: 1366,
      });

    const view = await render(
      <SetupFlow
        createClient={() => ({
          startQuickConnect: jest.fn(),
          cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
          pollQuickConnect: jest.fn(),
        })}
        onConnected={jest.fn()}
      />,
    );

    expect(view.queryByText('The shortest path to play.')).toBeNull();
    expect(view.queryByText('YOUR MEDIA · DIRECT')).toBeNull();
    expect(view.getByRole('header', { name: 'Find your server' })).toBeTruthy();
    expect(view.getByLabelText('Kinosail Server URL')).toBeTruthy();
    expect(view.getByRole('button', { name: 'Connect' })).toBeTruthy();
    expect(view.queryByText(/Your server, your library/)).toBeNull();
  });
});
