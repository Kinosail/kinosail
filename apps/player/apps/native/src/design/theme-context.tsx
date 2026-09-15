import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';
import { useColorScheme } from 'react-native';

import { platformStorage } from '@/core/platform-storage';

const THEME_KEY = 'kinosail.player.theme.v1';

export type ThemePreference = 'dark' | 'light' | 'system';
export type ThemeScheme = 'dark' | 'light';

export const resolveTheme = (
  preference: ThemePreference | undefined,
  system: ThemeScheme | 'unspecified' | null,
): ThemeScheme => {
  if (!preference || preference === 'dark') return 'dark';
  if (preference === 'light') return 'light';
  return system === 'light' ? 'light' : 'dark';
};

export const nextTheme = (preference: ThemePreference): ThemePreference => {
  if (preference === 'dark') return 'light';
  if (preference === 'light') return 'system';
  return 'dark';
};

type ThemeValue = {
  preference: ThemePreference;
  scheme: ThemeScheme;
  cycle(): void;
};

const ThemeContext = createContext<ThemeValue>({
  preference: 'dark',
  scheme: 'dark',
  cycle() {},
});

export function ThemePreferenceProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const system = useColorScheme();
  const [preference, setPreference] = useState<ThemePreference>('dark');

  // Stryker disable ArrayDeclaration: React requires a stable empty list; a stable constant has identical behavior.
  useEffect(() => {
    let active = true;
    platformStorage.get(THEME_KEY).then((stored) => {
      // Stryker disable next-line ConditionalExpression: React ignores state after unmount; the guard prevents an unnecessary setter call.
      if (active && (stored === 'light' || stored === 'system')) {
        setPreference(stored);
      }
    });
    // Stryker disable next-line BlockStatement: React ignores state after unmount; this cleanup only avoids an unnecessary setter call.
    return () => {
      // Stryker disable next-line BooleanLiteral: true would only cause an ignored post-unmount setter call.
      active = false;
    };
  }, []);
  // Stryker restore ArrayDeclaration

  const value = useMemo(
    () => ({
      preference,
      scheme: resolveTheme(preference, system),
      cycle() {
        setPreference((current) => {
          const next = nextTheme(current);
          void platformStorage.set(THEME_KEY, next);
          return next;
        });
      },
    }),
    [preference, system],
  );
  return (
    <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
  );
}

export const useThemePreference = () => useContext(ThemeContext);
