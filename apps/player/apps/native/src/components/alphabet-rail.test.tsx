import React from 'react';
import { fireEvent, render } from '@testing-library/react-native';
import { AlphabetRail } from './alphabet-rail';

const letters = [
  { label: 'A', offset: 0, count: 207 },
  { label: 'Z', offset: 207, count: 8 },
];
it('jumps through the full library with accessible adjustment and clamps the ends', async () => {
  const onJump = jest.fn();
  const screen = await render(
    <AlphabetRail letters={letters} offset={0} onJump={onJump} />,
  );
  const rail = screen.getByLabelText('Library alphabet');
  expect(rail.props.accessibilityValue.text).toBe('A');
  await fireEvent(rail, 'accessibilityAction', {
    nativeEvent: { actionName: 'increment' },
  });
  expect(onJump).toHaveBeenLastCalledWith(207);
  await fireEvent(rail, 'accessibilityAction', {
    nativeEvent: { actionName: 'decrement' },
  });
  expect(onJump).toHaveBeenLastCalledWith(0);
});
it('previews a drag, fetches only on release, and cancels interrupted gestures', async () => {
  const onJump = jest.fn();
  const screen = await render(
    <AlphabetRail letters={letters} offset={0} onJump={onJump} />,
  );
  const rail = () => screen.getByLabelText('Library alphabet');
  await fireEvent(rail(), 'responderGrant', { nativeEvent: { pageY: 0 } });
  await fireEvent(rail(), 'responderMove', { nativeEvent: { pageY: 1000 } });
  expect(onJump).not.toHaveBeenCalled();
  expect(rail().props.accessibilityValue.text).toBe('Z');
  await fireEvent(rail(), 'responderRelease');
  expect(onJump).toHaveBeenCalledTimes(1);
  expect(onJump).toHaveBeenCalledWith(207);
  await fireEvent(rail(), 'responderGrant', { nativeEvent: { pageY: 0 } });
  await fireEvent(rail(), 'responderTerminate');
  await fireEvent(rail(), 'responderRelease');
  expect(onJump).toHaveBeenCalledTimes(1);
});
