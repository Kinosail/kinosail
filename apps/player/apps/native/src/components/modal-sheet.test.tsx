import React from 'react';
import { fireEvent, render, within } from '@testing-library/react-native';
import { Keyboard, Text } from 'react-native';
import { ModalSheet } from './modal-sheet';

jest.mock('react-native-safe-area-context', () => ({
  SafeAreaView: jest.requireActual('react-native').View,
}));

it('keeps Close and the footer outside the scrolling body', async () => {
  const onClose = jest.fn();
  const dismiss = jest.spyOn(Keyboard, 'dismiss').mockImplementation(() => {});
  const screen = await render(
    <ModalSheet
      title="Search & browse"
      onClose={onClose}
      footer={<Text>Results</Text>}
    >
      <Text>{'Long content '.repeat(200)}</Text>
    </ModalSheet>,
  );
  const body = within(
    screen.container.queryAll((node) => /ScrollView$/.test(node.type))[0],
  );
  expect(body.queryByRole('button', { name: 'Close' })).toBeNull();
  expect(body.queryByText('Results')).toBeNull();
  const footer = screen.container
    .queryAll(() => true)
    .find((element) =>
      element.props.style?.some?.(
        (style: { borderTopColor?: string }) => style?.borderTopColor,
      ),
    );
  expect(footer).toBeTruthy();
  expect(within(footer!).getByRole('button', { name: 'Close' })).toBeTruthy();
  await fireEvent.press(screen.getByRole('button', { name: 'Close' }));
  expect(dismiss).toHaveBeenCalled();
  expect(onClose).toHaveBeenCalledTimes(1);
  dismiss.mockRestore();
});

it('dismisses through the backdrop, system back, and accessibility escape', async () => {
  const onClose = jest.fn();
  const screen = await render(
    <ModalSheet title="More" onClose={onClose}>
      <Text>Library destinations</Text>
    </ModalSheet>,
  );
  const backdrop = screen.container
    .queryAll(
      (node) =>
        typeof node.props.onClick === 'function' ||
        typeof node.props.onPress === 'function',
    )
    .find((element) => element.props.accessible === false)!;
  await fireEvent.press(backdrop);
  await fireEvent(
    screen.container.queryAll(
      (node) => typeof node.props.onRequestClose === 'function',
    )[0],
    'requestClose',
  );
  const sheet = screen.container
    .queryAll(() => true)
    .find((element) => element.props.accessibilityViewIsModal)!;
  await fireEvent(sheet, 'accessibilityEscape');
  expect(onClose).toHaveBeenCalledTimes(3);
});

it('does not show a closed sheet', async () => {
  const screen = await render(
    <ModalSheet visible={false} title="Hidden" onClose={jest.fn()}>
      <Text>Private content</Text>
    </ModalSheet>,
  );
  expect(screen.queryByRole('button', { name: 'Close' })).toBeNull();
  expect(screen.queryByText('Private content')).toBeNull();
});

it('supports a motion-free listening sheet without removing the playing surface', async () => {
  const screen = await render(
    <ModalSheet
      title="Listening options"
      animationType="none"
      onClose={jest.fn()}
    >
      <Text>Speed</Text>
    </ModalSheet>,
  );
  expect(
    screen.container.queryAll(
      (node) => typeof node.props.onRequestClose === 'function',
    )[0].props.animationType,
  ).toBe('none');
  expect(
    screen.container.queryAll(
      (node) => typeof node.props.onRequestClose === 'function',
    )[0].props.transparent,
  ).toBe(true);
});
