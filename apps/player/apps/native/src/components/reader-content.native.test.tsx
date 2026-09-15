import React from 'react';
import { render } from '@testing-library/react-native';
const mockNativeView = jest.fn();
jest.mock('expo', () => ({
  requireNativeView: (...args: unknown[]) => mockNativeView(...args),
}));
jest.mock('react-native', () => {
  const actual = jest.requireActual('react-native');
  return Object.defineProperty(Object.create(actual), 'Platform', {
    value: { ...actual.Platform, OS: 'ios', isTV: true },
  });
});
import { ReaderContent } from './reader-content.native';
it('keeps Apple TV usable without requesting an unavailable native reader', async () => {
  const screen = await render(
    <ReaderContent
      server="https://example.com"
      authorization="Bearer test"
      id="book"
      path="/read/book/file"
      type="epub"
      theme="light"
      fontSize={20}
      onError={() => {}}
    />,
  );
  expect(mockNativeView).not.toHaveBeenCalled();
  expect(
    screen.getByText('Read this book in the iPhone, iPad, or web app.'),
  ).toBeTruthy();
});
