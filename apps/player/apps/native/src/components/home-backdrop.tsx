import { LinearGradient } from 'expo-linear-gradient';
import React from 'react';
import { Platform, StyleSheet, View } from 'react-native';

import { useKinoTheme } from '@/design/tokens';

/** Local, static brand artwork stays available when the library has no backdrop. */
export function HomeBackdrop() {
  const theme = useKinoTheme();
  return (
    <View
      testID="home-brand-backdrop"
      pointerEvents="none"
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
      style={styles.canvas}
    >
      <LinearGradient
        colors={[theme.raised, theme.background]}
        locations={[0, 1]}
        start={{ x: 1, y: 0 }}
        end={{ x: 0, y: 1 }}
        style={StyleSheet.absoluteFill}
      />
      <View style={styles.emblem}>
        <View style={[styles.sail, { borderBottomColor: theme.signal }]} />
        <View style={[styles.wake, { backgroundColor: theme.signal }]} />
      </View>
      <LinearGradient
        colors={[`${theme.background}00`, theme.background]}
        locations={[0.2, 1]}
        style={StyleSheet.absoluteFill}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  canvas: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    height: Platform.isTV ? 720 : 560,
    overflow: 'hidden',
  },
  emblem: {
    position: 'absolute',
    top: Platform.isTV ? 64 : 96,
    right: Platform.isTV ? 92 : -32,
    opacity: 0.07,
  },
  sail: {
    width: 0,
    height: 0,
    borderRightColor: 'transparent',
    borderRightWidth: Platform.isTV ? 280 : 180,
    borderBottomWidth: Platform.isTV ? 480 : 320,
    marginLeft: 32,
  },
  wake: {
    height: 2,
    marginTop: 24,
    width: Platform.isTV ? 400 : 260,
  },
});
