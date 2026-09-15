import { render } from '@testing-library/react-native';
import React from 'react';
import { Platform } from 'react-native';

import RootLayout from '@/app/_layout';

const mockTheme = jest.fn();
const mockStatus = jest.fn();
const mockStack = jest.fn();
const mockScreen = jest.fn();
const mockHead = jest.fn();
let mockScheme = 'dark';

jest.mock('@/components/bottom-navigation', () => ({
  BottomNavigation: () => null,
}));
jest.mock('../global.css', () => ({}));
jest.mock('@/core/session-context', () => ({
  useSession: () => ({ client: null }),
  SessionProvider: ({ children }: { children: React.ReactNode }) => children,
}));
jest.mock('@/design/theme-context', () => ({
  ThemePreferenceProvider: ({ children }: { children: React.ReactNode }) =>
    children,
  useThemePreference: () => ({ scheme: mockScheme }),
}));
jest.mock('expo-router', () => ({
  usePathname: () => '/',
  DarkTheme: { dark: true },
  DefaultTheme: { dark: false },
  ThemeProvider: ({
    children,
    value,
  }: {
    children: React.ReactNode;
    value: { dark: boolean };
  }) => {
    mockTheme(value);
    return children;
  },
  Stack: Object.assign(
    (props: {
      screenOptions: { headerShown: boolean };
      children: React.ReactNode;
    }) => {
      mockStack(props);
      return props.children;
    },
    {
      Screen: (props: {
        name: string;
        options: { gestureEnabled: boolean };
      }) => {
        mockScreen(props);
        return null;
      },
    },
  ),
}));
jest.mock('expo-status-bar', () => ({
  StatusBar: (props: { style: 'dark' | 'light' }) => {
    mockStatus(props);
    return null;
  },
}));
jest.mock('expo-router/head', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => {
    mockHead(children);
    return null;
  },
}));

describe('root navigation', () => {
  const originalOS = Platform.OS;
  afterEach(() => {
    Platform.OS = originalOS;
    jest.clearAllMocks();
  });

  it.each(['light', 'dark'])(
    'uses the %s navigation and status theme',
    async (scheme) => {
      Platform.OS = 'ios';
      mockScheme = scheme;
      await render(<RootLayout />);
      expect(mockTheme).toHaveBeenLastCalledWith({ dark: scheme === 'dark' });
      expect(mockStatus).toHaveBeenLastCalledWith({
        style: scheme === 'dark' ? 'light' : 'dark',
      });
      expect(mockStack).toHaveBeenLastCalledWith(
        expect.objectContaining({
          screenOptions: {
            headerShown: false,
            contentStyle: {
              backgroundColor: scheme === 'light' ? '#F7F8F3' : '#090A08',
            },
          },
        }),
      );
      expect(mockScreen).toHaveBeenCalledTimes(5);
      for (const name of ['index', 'library', 'downloads', 'settings']) {
        expect(mockScreen).toHaveBeenCalledWith({
          name,
          options: { animation: 'none' },
        });
      }
      expect(mockScreen).toHaveBeenCalledWith({
        name: 'watch/[id]',
        options: { gestureEnabled: false },
      });
      expect(mockHead).not.toHaveBeenCalled();
    },
  );

  it('provides document metadata only on web', async () => {
    Platform.OS = 'web';
    await render(<RootLayout />);
    expect(mockHead).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'title',
        props: { children: 'Kinosail Player' },
      }),
    );
  });
});
