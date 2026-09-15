import React from 'react';
import { StyleSheet, View } from 'react-native';

import { spacing, useKinoTheme } from '@/design/tokens';

// Static placeholders respect reduced motion on every native platform.
export function Skeleton({
  variant = 'content',
  label = 'Loading…',
}: {
  variant?: 'content' | 'media' | 'poster' | 'inline';
  label?: string;
}) {
  const theme = useKinoTheme();
  const block = { backgroundColor: theme.line, borderRadius: 4 };
  return (
    <View
      accessibilityLabel={label}
      accessibilityRole="progressbar"
      accessibilityState={{ busy: true }}
      accessible
      pointerEvents="none"
      style={variant === 'inline' ? styles.inline : styles.content}
    >
      {variant !== 'inline' ? (
        <View style={[block, variant === 'poster' ? styles.poster : variant === 'media' ? styles.media : styles.heading]} />
      ) : null}
      <View style={[block, styles.line]} />
      {variant !== 'inline' ? <View style={[block, styles.shortLine]} /> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  content: { width: '100%', maxWidth: 480, gap: spacing.two, padding: spacing.two },
  inline: { width: 32, justifyContent: 'center' },
  heading: { width: '55%', height: 28 },
  poster: { width: '100%', aspectRatio: 2 / 3 },
  media: { width: '100%', aspectRatio: 16 / 9 },
  line: { width: '100%', height: 12 },
  shortLine: { width: '70%', height: 12 },
});
