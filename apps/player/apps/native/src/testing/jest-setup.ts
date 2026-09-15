// Unit tests supply their HTTP responses; do not load Expo's native fetch during teardown.
Object.defineProperty(globalThis, 'fetch', {
  configurable: true,
  writable: true,
  value: async () => { throw new Error('Unexpected HTTP request in native unit test.'); },
});

jest.mock(
  'react-native-safe-area-context',
  () => jest.requireActual('react-native-safe-area-context/jest/mock').default,
);

jest.mock('expo-network', () => ({
  NetworkStateType: Object.fromEntries(
    [
      'NONE',
      'UNKNOWN',
      'CELLULAR',
      'WIFI',
      'BLUETOOTH',
      'ETHERNET',
      'WIMAX',
      'VPN',
      'OTHER',
    ].map((type) => [type, type]),
  ),
  getNetworkStateAsync: jest
    .fn()
    .mockResolvedValue({
      isConnected: true,
      isInternetReachable: true,
      type: 'WIFI',
    }),
  addNetworkStateListener: jest.fn(() => ({ remove: jest.fn() })),
}));
