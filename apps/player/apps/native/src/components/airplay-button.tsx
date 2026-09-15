import React from 'react';
import { VideoAirPlayButton } from 'expo-video';
import { Platform, StyleSheet, Text, View } from 'react-native';
import { radius, spacing, useKinoTheme } from '@/design/tokens';
import { NavigationIcon } from './navigation-icon';

export function AirPlayButton({ video = true }: { video?: boolean }) {
  const theme = useKinoTheme();
  if (Platform.OS !== 'ios' || Platform.isTV) return null;
  return (
    <View
      style={[
        styles.control,
        { backgroundColor: theme.raised, borderColor: theme.line },
      ]}
    >
      <View
        pointerEvents="none"
        accessibilityElementsHidden
        style={styles.label}
      >
        <NavigationIcon name="airplay" color={theme.text} />
        <Text style={{ color: theme.text }}>AirPlay</Text>
      </View>
      {/* The system picker owns the entire tap target, including the label. */}
      <VideoAirPlayButton
        accessibilityLabel="AirPlay"
        accessibilityHint="Choose a device, or choose this iPhone or iPad to stop AirPlay"
        prioritizeVideoDevices={video}
        tint="transparent"
        activeTint="transparent"
        style={StyleSheet.absoluteFill}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  control: {
    minHeight: 56,
    borderWidth: 1,
    borderRadius: radius.control,
    justifyContent: 'center',
  },
  label: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.one,
    paddingHorizontal: spacing.two,
    paddingVertical: spacing.one,
  },
});
