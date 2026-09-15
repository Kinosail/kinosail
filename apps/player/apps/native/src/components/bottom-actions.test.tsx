import React from 'react';
import { fireEvent, render } from '@testing-library/react-native';
import { BottomActions } from './bottom-actions';
import { ActionButton } from './action-button';

it('places Back last and preserves action callbacks', async () => {
  const onBack = jest.fn(),
    onPlay = jest.fn();
  const view = await render(
    <BottomActions onBack={onBack}>
      <ActionButton label="Play" onPress={onPlay} />
    </BottomActions>,
  );
  expect(
    view
      .getAllByRole('button')
      .map((button) => button.props.accessibilityLabel),
  ).toEqual(['Play', 'Back']);
  await fireEvent.press(view.getByRole('button', { name: 'Back' }));
  expect(onBack).toHaveBeenCalledTimes(1);
  expect(onPlay).not.toHaveBeenCalled();
});

it('does not invent a back action for root screens', async () => {
  const view = await render(
    <BottomActions>
      <ActionButton label="Save" onPress={jest.fn()} />
    </BottomActions>,
  );
  expect(view.queryByRole('button', { name: 'Back' })).toBeNull();
});
