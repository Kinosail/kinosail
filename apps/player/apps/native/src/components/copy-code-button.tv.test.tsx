import React from 'react';
import { render } from '@testing-library/react-native';
import { CopyCodeButton } from './copy-code-button';
jest.mock('react-native', () => {
  const actual = jest.requireActual('react-native');
  return Object.defineProperty(Object.create(actual), 'Platform', {
    value: { ...actual.Platform, isTV: true },
  });
});
jest.mock('expo-clipboard', () => {
  throw new Error('Clipboard is not available on tvOS');
});
it('renders TV setup without loading the unsupported clipboard module', async () => {
  const view = await render(<CopyCodeButton code="123456" />);
  expect(view.queryByRole('button', { name: 'Copy code' })).toBeNull();
});
