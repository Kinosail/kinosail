import React from 'react';
import { fireEvent, render } from '@testing-library/react-native';
import { LanguagePicker } from './language-picker';
it('searches beyond the initial five languages and commits only a selected option', async () => {
  const onChange = jest.fn();
  const view = await render(
    <LanguagePicker value="auto" disabled={false} onChange={onChange} />,
  );
  await fireEvent.press(
    view.getByRole('button', { name: 'Audio language: Automatic' }),
  );
  await fireEvent.changeText(view.getByLabelText('Find a language'), 'Polish');
  expect(onChange).not.toHaveBeenCalled();
  await fireEvent.press(view.getByRole('button', { name: 'Polish' }));
  expect(onChange).toHaveBeenCalledWith('pl');
  expect(view.queryByLabelText('Find a language')).toBeNull();
});
it('offers Off only for subtitles and does not save a failed search or dismissal', async () => {
  const onChange = jest.fn();
  const view = await render(
    <LanguagePicker
      subtitles
      value="off"
      disabled={false}
      onChange={onChange}
    />,
  );
  await fireEvent.press(
    view.getByRole('button', { name: 'Subtitle language: Off' }),
  );
  expect(
    view.getByRole('button', { name: 'Off' }).props.accessibilityState.selected,
  ).toBe(true);
  await fireEvent.changeText(
    view.getByLabelText('Find a language'),
    'no-such-language',
  );
  expect(view.getByText(/No matching languages/)).toBeTruthy();
  await fireEvent.press(view.getByRole('button', { name: 'Close' }));
  expect(onChange).not.toHaveBeenCalled();
});
