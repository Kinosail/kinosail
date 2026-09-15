import { render } from '@testing-library/react-native';
import React from 'react';

import { HomeBackdrop } from './home-backdrop';

let mockScheme: 'dark' | 'light' = 'dark';
jest.mock('@/design/theme-context', () => ({
  useThemePreference: () => ({ scheme: mockScheme }),
}));

describe('home branding', () => {
  it.each(['dark', 'light'] as const)(
    'keeps the %s backdrop decorative and outside remote or touch interaction',
    async (scheme) => {
      mockScheme = scheme;
      const view = await render(<HomeBackdrop />);
      expect(
        view.getByTestId('home-brand-backdrop', { includeHiddenElements: true })
          .props,
      ).toMatchObject({
        pointerEvents: 'none',
        accessibilityElementsHidden: true,
        importantForAccessibility: 'no-hide-descendants',
      });
      expect(view.queryAllByRole('button')).toHaveLength(0);
      expect(view.queryAllByRole('image')).toHaveLength(0);
    },
  );
});
