import React from 'react';
import { Platform, StyleSheet, Text, View } from 'react-native';

import { radius, useKinoTheme } from '@/design/tokens';

export function BrandMark({ compact = false }: { compact?: boolean }) {
  const theme = useKinoTheme();
  const size = compact ? 34 : Platform.isTV ? 64 : 44;
  return (
    <View
      accessible
      accessibilityLabel="Kinosail Player"
      accessibilityRole="image"
      style={styles.lockup}
    >
      <View
        style={[
          styles.mark,
          { width: size, height: size, backgroundColor: theme.signal },
        ]}
      >
        <View
          style={[
            styles.sail,
            {
              borderBottomColor: theme.signalInk,
              borderRightWidth: compact ? 7 : Platform.isTV ? 13 : 9,
              borderBottomWidth: compact ? 16 : Platform.isTV ? 30 : 21,
            },
          ]}
        />
        <View style={[styles.wake, { backgroundColor: theme.signalInk }]} />
      </View>
      {!compact && (
        <View>
          <Text style={[styles.name, { color: theme.text }]}>KINOSAIL</Text>
          <Text style={[styles.product, { color: theme.muted }]}>PLAYER</Text>
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  lockup: { alignItems: 'center', flexDirection: 'row', gap: 12 },
  mark: {
    alignItems: 'center',
    borderRadius: radius.control,
    justifyContent: 'center',
    overflow: 'hidden',
  },
  sail: {
    borderLeftColor: 'transparent',
    borderLeftWidth: 0,
    borderRightColor: 'transparent',
    height: 0,
    marginLeft: 4,
    width: 0,
  },
  wake: { height: 2, marginTop: 4, width: Platform.isTV ? 32 : 22 },
  name: {
    fontSize: Platform.isTV ? 22 : 15,
    fontWeight: '900',
    letterSpacing: 2.2,
  },
  product: {
    fontSize: Platform.isTV ? 16 : 10,
    fontWeight: '800',
    letterSpacing: 3,
  },
});
