import { nextTheme, resolveTheme } from './theme-context';

describe('theme preference', () => {
  it('defaults to dark and cycles all supported choices', () => {
    expect(resolveTheme(undefined, 'light')).toBe('dark');
    expect(resolveTheme('light', 'dark')).toBe('light');
    expect(resolveTheme('system', 'light')).toBe('light');
    expect(resolveTheme('system', null)).toBe('dark');
    expect(nextTheme('dark')).toBe('light');
    expect(nextTheme('light')).toBe('system');
    expect(nextTheme('system')).toBe('dark');
  });
});
