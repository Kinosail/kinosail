import React from 'react';
import { render } from '@testing-library/react-native';
import { StyleSheet } from 'react-native';
import { BrowseSkeleton } from './browse-skeleton';

it.each([
  ['poster', 2 / 3, 16],
  ['square', 1, 16],
  ['album', 1, 14],
] as const)(
  'matches %s card dimensions without inset padding',
  async (variant, aspectRatio, borderRadius) => {
    const view = await render(
      <BrowseSkeleton
        variant={variant}
        label="Loading results"
        width={148}
        columns={2}
      />,
    );
    const artwork = view.getAllByTestId('browse-skeleton-artwork');
    expect(artwork).toHaveLength(4);
    expect(StyleSheet.flatten(artwork[0].props.style)).toMatchObject({
      width: '100%',
      aspectRatio,
      borderRadius,
    });
    expect(StyleSheet.flatten(artwork[0].parent!.props.style)).toMatchObject({
      width: 148,
    });
    expect(
      StyleSheet.flatten(artwork[0].parent!.props.style).padding,
    ).toBeUndefined();
    const progress = view.getByRole('progressbar', { name: 'Loading results' });
    expect(progress.props.pointerEvents).toBe('none');
    expect(progress.props.accessibilityState).toEqual({ busy: true });
    expect(view.getAllByRole('progressbar')).toHaveLength(1);
    expect(view.queryAllByRole('button')).toHaveLength(0);
  },
);

it.each([
  ['track', 64],
  ['row', 48],
] as const)('matches %s row height', async (variant, minHeight) => {
  const view = await render(
    <BrowseSkeleton variant={variant} label="Loading rows" rowGap={12} />,
  );
  expect(view.getAllByTestId('browse-skeleton-row')).toHaveLength(6);
  expect(
    StyleSheet.flatten(
      view.getAllByTestId('browse-skeleton-row')[0].props.style,
    ),
  ).toMatchObject({ width: '100%', minHeight });
  expect(view.queryAllByTestId('browse-skeleton-artwork')).toHaveLength(0);
  expect(view.queryAllByRole('button')).toHaveLength(0);
});
