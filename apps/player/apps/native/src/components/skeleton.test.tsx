import { render } from '@testing-library/react-native';
import React from 'react';
import { StyleSheet } from 'react-native';

import { Skeleton } from './skeleton';

jest.mock('@/design/theme-context', () => ({
  useThemePreference: () => ({ scheme: 'dark' }),
}));

it.each(['content', 'media', 'poster', 'inline'] as const)(
  'announces the %s placeholder without accepting touches',
  async (variant) => {
    const view = await render(<Skeleton variant={variant} label="Loading titles" />);
    const placeholder = view.getByRole('progressbar');
    expect(placeholder.props.accessibilityLabel).toBe('Loading titles');
    expect(placeholder.props.accessibilityState).toEqual({ busy: true });
    expect(placeholder.props.pointerEvents).toBe('none');
    expect(StyleSheet.flatten(placeholder.props.style)).toMatchObject(
      variant === 'inline' ? { width: 32 } : { width: '100%', maxWidth: 480 },
    );
    await view.rerender(<></>);
    expect(view.queryByRole('progressbar')).toBeNull();
  },
);
