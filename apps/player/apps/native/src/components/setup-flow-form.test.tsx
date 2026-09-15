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

describe('SetupFlow form', () => {
  afterEach(() => jest.restoreAllMocks());

  it('preserves the compact form hierarchy and input contract', async () => {
    exposeKeyboardAvoidingView();
    jest.replaceProperty(ReactNative.Platform, 'OS', 'ios');
    jest
      .spyOn(
        jest.requireActual<typeof ReactNative>('react-native'),
        'useWindowDimensions',
      )
      .mockReturnValue({ width: 390, height: 844, scale: 3, fontScale: 1 });
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
    expect(ReactNative.StyleSheet.flatten(view.root!.props.style)).toEqual({
      backgroundColor: '#090A08',
      flex: 1,
    });
    const main = child(view.root!, 0);
    expect(main.props.role).toBe('main');
    expect(main.props.behavior).toBe('padding');
    expect(ReactNative.StyleSheet.flatten(main.props.style)).toEqual({
      flex: 1,
    });
    const panelTitle = view.getByRole('header', { name: 'Find your server' });
    const form = panelTitle.parent!;
    const panel = form.parent!;
    const scroll = panel.parent!;
    const intro = child(scroll, 0);
    let scrollView: NativeNode | null = scroll;
    while (scrollView && scrollView.props.contentContainerStyle === undefined) {
      scrollView = scrollView.parent as NativeNode | null;
    }
    expect(scrollView).not.toBeNull();
    expect(
      ReactNative.StyleSheet.flatten(scrollView!.props.contentContainerStyle),
    ).toEqual({
      alignItems: 'stretch',
      flexDirection: 'column',
      flexGrow: 1,
      gap: 24,
      justifyContent: 'center',
      padding: 24,
    });
    expect(ReactNative.StyleSheet.flatten(intro.props.style)).toEqual({
      alignSelf: 'center',
      flexBasis: 'auto',
      flexShrink: 0,
      gap: 16,
      maxWidth: 640,
    });
    expect(view.getByText('YOUR MEDIA · DIRECT').props.style).toEqual([
      {
        fontSize: 12,
        fontWeight: '900',
        letterSpacing: 2.5,
        marginTop: 32,
      },
      { marginTop: 8 },
      { color: '#C8F169' },
    ]);
    expect(view.getByText('The shortest path to play.').props.style).toEqual([
      {
        fontSize: 48,
        fontWeight: '900',
        letterSpacing: -1.5,
        lineHeight: 51,
        maxWidth: 540,
      },
      { fontSize: 40, lineHeight: 43 },
      { color: '#F6F8EF' },
    ]);
    expect(
      view.getByText(
        'Connect to your Kinosail Server. Your library and viewing history stay under your control.',
      ).props.style,
    ).toEqual([
      { fontSize: 19, lineHeight: 29, maxWidth: 540 },
      { color: '#9CA391' },
    ]);
    expect(ReactNative.StyleSheet.flatten(panel.props.style)).toEqual({
      alignSelf: 'center',
      backgroundColor: '#12140F',
      borderColor: '#2B3024',
      borderRadius: 18,
      borderWidth: 1,
      flexBasis: 'auto',
      gap: 24,
      maxWidth: 520,
      padding: 24,
      width: '100%',
    });
    expect(ReactNative.StyleSheet.flatten(form.props.style)).toEqual({
      gap: 16,
    });
    expect(view.getByText('STEP 1 OF 2').props.style).toEqual([
      { fontSize: 11, fontWeight: '900', letterSpacing: 1.8 },
      { color: '#9CA391' },
    ]);
    expect(panelTitle.props.style).toEqual([
      { fontSize: 28, fontWeight: '800', letterSpacing: -0.6 },
      { color: '#F6F8EF' },
    ]);
    expect(
      view.getByText(
        'Use the trusted HTTPS address, or a private local network address.',
      ).props.style,
    ).toEqual([{ fontSize: 16, lineHeight: 24 }, { color: '#9CA391' }]);
    const input = view.getByLabelText('Kinosail Server URL');
    expect(input.props).toMatchObject({
      accessibilityLabel: 'Kinosail Server URL',
      autoCapitalize: 'none',
      autoCorrect: false,
      editable: true,
      keyboardType: 'url',
      placeholder: 'https://player.example.com',
      placeholderTextColor: '#9CA391',
      returnKeyType: 'go',
      value: '',
    });
    expect(typeof input.props.onChangeText).toBe('function');
    expect(typeof input.props.onSubmitEditing).toBe('function');
    expect(ReactNative.StyleSheet.flatten(input.props.style)).toEqual({
      backgroundColor: '#090A08',
      borderColor: '#2B3024',
      borderRadius: 10,
      borderWidth: 1,
      color: '#F6F8EF',
      fontSize: 16,
      minHeight: 52,
      paddingHorizontal: 16,
    });
    expect(view.getByText('Kinosail Server URL').props.style).toEqual([
      { fontSize: 13, fontWeight: '800', marginTop: 8 },
      { color: '#F6F8EF' },
    ]);
    expect(view.queryByText('Connection failed.')).toBeNull();
    expect(view.queryByText('Stryker was here!')).toBeNull();
  });
});
