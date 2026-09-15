import { render } from '@testing-library/react-native';
import React from 'react';
import { StyleSheet } from 'react-native';

import { palette } from '@/design/tokens';

import { BrandMark } from './brand-mark';

let mockScheme: 'dark' | 'light' = 'dark';
jest.mock('@/design/theme-context', () => ({
  useThemePreference: () => ({ scheme: mockScheme }),
}));

describe('native brand mark', () => {
  it.each([
    ['dark', false],
    ['dark', true],
    ['light', false],
    ['light', true],
  ] as const)(
    'preserves the %s lockup with compact=%s',
    async (scheme, compact) => {
      mockScheme = scheme;
      const colors = palette[scheme];
      const view = await render(
        compact ? <BrandMark compact /> : <BrandMark />,
      );
      expect(view.getByRole('image', { name: 'Kinosail Player' })).toBeTruthy();
      const lockup = view.root!;
      expect(lockup.children).toHaveLength(compact ? 1 : 2);
      const mark = lockup.children[0];
      if (typeof mark === 'string') throw new Error('The mark must be a view.');
      expect(mark.children).toHaveLength(2);
      const [sail, wake] = mark.children;
      if (typeof sail === 'string' || typeof wake === 'string') {
        throw new Error('The sail and wake must be views.');
      }
      expect(StyleSheet.flatten(lockup.props.style)).toEqual({
        alignItems: 'center',
        flexDirection: 'row',
        gap: 12,
      });
      expect(StyleSheet.flatten(mark.props.style)).toEqual({
        alignItems: 'center',
        borderRadius: 10,
        justifyContent: 'center',
        overflow: 'hidden',
        width: compact ? 34 : 44,
        height: compact ? 34 : 44,
        backgroundColor: colors.signal,
      });
      expect(StyleSheet.flatten(sail.props.style)).toEqual({
        borderLeftColor: 'transparent',
        borderLeftWidth: 0,
        borderRightColor: 'transparent',
        height: 0,
        marginLeft: 4,
        width: 0,
        borderBottomColor: colors.signalInk,
        borderRightWidth: compact ? 7 : 9,
        borderBottomWidth: compact ? 16 : 21,
      });
      expect(StyleSheet.flatten(wake.props.style)).toEqual({
        height: 2,
        marginTop: 4,
        width: 22,
        backgroundColor: colors.signalInk,
      });
      if (compact) {
        expect(view.queryByText('KINOSAIL')).toBeNull();
        expect(view.queryByText('PLAYER')).toBeNull();
      } else {
        expect(
          StyleSheet.flatten(view.getByText('KINOSAIL').props.style),
        ).toEqual({
          fontSize: 15,
          fontWeight: '900',
          letterSpacing: 2.2,
          color: colors.text,
        });
        expect(
          StyleSheet.flatten(view.getByText('PLAYER').props.style),
        ).toEqual({
          fontSize: 10,
          fontWeight: '800',
          letterSpacing: 3,
          color: colors.muted,
        });
      }
    },
  );
});
