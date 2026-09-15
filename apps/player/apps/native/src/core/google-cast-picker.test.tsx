import React from 'react';
import { render } from '@testing-library/react-native';
import { GoogleCastPicker } from './google-cast.native';
const mockRemove = jest.fn();
let mockStarted: () => void;
const mockSubscribe = jest.fn((handler: () => void) => {
  mockStarted = handler;
  return { remove: mockRemove };
});
jest.mock('react-native', () => {
  const actual = jest.requireActual('react-native');
  return {
    ...actual,
    NativeModules: { ...actual.NativeModules, RNGCCastContext: {} },
  };
});
jest.mock('react-native-google-cast', () => ({
  __esModule: true,
  CastButton: jest.requireActual('react-native').View,
  default: {
    getSessionManager: () => ({
      onSessionStarted: (handler: () => void) => mockSubscribe(handler),
    }),
  },
}));
it('starts only on a new receiver session and ignores notifications after dismissal', async () => {
  const connected = jest.fn();
  const view = await render(<GoogleCastPicker onConnected={connected} />);
  expect(connected).not.toHaveBeenCalled();
  mockStarted();
  expect(connected).toHaveBeenCalledTimes(1);
  await view.unmount();
  mockStarted();
  expect(connected).toHaveBeenCalledTimes(1);
  expect(mockRemove).toHaveBeenCalledTimes(1);
});
