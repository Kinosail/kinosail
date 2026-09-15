import { fireEvent, render } from '@testing-library/react-native';
import * as Clipboard from 'expo-clipboard';
import React from 'react';

import { CopyCodeButton } from './copy-code-button';

jest.mock('expo-clipboard', () => ({ setStringAsync: jest.fn() }));

describe('CopyCodeButton', () => {
  beforeEach(() => jest.resetAllMocks());

  it('copies the unformatted authorization code only after pressing the button', async () => {
    jest.mocked(Clipboard.setStringAsync).mockResolvedValue(true);
    const view = await render(<CopyCodeButton code="381204" />);
    expect(Clipboard.setStringAsync).not.toHaveBeenCalled();
    await fireEvent.press(view.getByRole('button', { name: 'Copy code' }));
    expect(Clipboard.setStringAsync).toHaveBeenCalledWith('381204');
    expect(
      (await view.findByText('Code copied.')).props.accessibilityLiveRegion,
    ).toBe('polite');
  });

  it.each(['false', 'rejected'])(
    'allows retry after a %s clipboard result',
    async (result) => {
      const copy = jest.mocked(Clipboard.setStringAsync);
      if (result === 'false') copy.mockResolvedValueOnce(false);
      else copy.mockRejectedValueOnce(new Error('Clipboard unavailable'));
      const view = await render(<CopyCodeButton code="381204" />);
      await fireEvent.press(view.getByRole('button', { name: 'Copy code' }));
      expect(
        await view.findByText('Could not copy. Select the code to copy it.'),
      ).toBeTruthy();
      expect(view.queryByText('Code copied.')).toBeNull();
      copy.mockResolvedValueOnce(true);
      await fireEvent.press(view.getByRole('button', { name: 'Copy code' }));
      expect(await view.findByText('Code copied.')).toBeTruthy();
      expect(copy).toHaveBeenCalledTimes(2);
    },
  );
});
