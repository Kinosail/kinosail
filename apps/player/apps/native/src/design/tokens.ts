import { createContext, useContext } from 'react';
import { useThemePreference } from './theme-context';

export const palette = {
  dark: {
    background: '#090A08',
    surface: '#12140F',
    raised: '#1A1D16',
    text: '#F6F8EF',
    muted: '#9CA391',
    line: '#2B3024',
    signal: '#C8F169',
    signalInk: '#11150A',
    focus: '#E4FF9C',
    danger: '#FF6B67',
  },
  light: {
    background: '#F7F8F3',
    surface: '#FFFFFF',
    raised: '#ECEFE6',
    text: '#171A13',
    muted: '#626957',
    line: '#D4D9CB',
    signal: '#365500',
    signalInk: '#FFFFFF',
    focus: '#365500',
    danger: '#B4232B',
  },
} as const;

export const spacing = {
  fine: 4,
  one: 8,
  two: 16,
  three: 24,
  four: 32,
  five: 40,
  six: 48,
  eight: 64,
};

export const radius = { control: 10, card: 16, panel: 18 };

export type KinoTheme = { [Key in keyof typeof palette.dark]: string };
export const ArtworkThemeContext = createContext<KinoTheme | null>(null);

export const useKinoTheme = () => {
  const { scheme } = useThemePreference();
  const artwork = useContext(ArtworkThemeContext);
  return artwork ?? (scheme === 'light' ? palette.light : palette.dark);
};
