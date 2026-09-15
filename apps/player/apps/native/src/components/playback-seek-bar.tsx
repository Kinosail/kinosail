import React, { useRef, useState } from 'react';
import { Platform, StyleSheet, Text, View } from 'react-native';
import { spacing, useKinoTheme } from '@/design/tokens';
const clock = (seconds: number) => {
  const value = Math.floor(Math.max(0, seconds));
  const hours = Math.floor(value / 3600);
  const minutes = Math.floor(value / 60) % 60;
  return `${hours ? `${hours}:${String(minutes).padStart(2, '0')}` : minutes}:${String(value % 60).padStart(2, '0')}`;
};
export function PlaybackSeekBar({
  seconds,
  duration,
  onSeek,
}: {
  seconds: number;
  duration: number;
  onSeek(seconds: number): void;
}) {
  const theme = useKinoTheme(),
    width = useRef(0),
    [preview, setPreview] = useState<number | null>(null);
  if (!Number.isFinite(duration) || duration <= 0 || !Number.isFinite(seconds))
    return null;
  const value = Math.min(duration, Math.max(0, preview ?? seconds));
  const seekAt = (x: number) =>
    Number.isFinite(x) && width.current > 0
      ? Math.min(duration, Math.max(0, (x / width.current) * duration))
      : null;
  return (
    <View style={{ paddingHorizontal: spacing.two }}>
      <View
        accessible
        accessibilityRole="adjustable"
        accessibilityLabel="Playback position"
        accessibilityValue={{
          min: 0,
          max: duration,
          now: value,
          text: `${clock(value)} of ${clock(duration)}`,
        }}
        accessibilityActions={[
          { name: 'increment', label: 'Forward 10 seconds' },
          { name: 'decrement', label: 'Back 10 seconds' },
        ]}
        onAccessibilityAction={({ nativeEvent: { actionName } }) => {
          if (actionName === 'increment' || actionName === 'decrement')
            onSeek(
              Math.max(
                0,
                Math.min(
                  duration,
                  seconds + (actionName === 'increment' ? 10 : -10),
                ),
              ),
            );
        }}
        onLayout={({ nativeEvent: { layout } }) => {
          if (Number.isFinite(layout.width) && layout.width > 0)
            width.current = layout.width;
        }}
        onStartShouldSetResponder={() => true}
        onResponderGrant={({ nativeEvent }) =>
          setPreview(seekAt(nativeEvent.locationX))
        }
        onResponderMove={({ nativeEvent }) =>
          setPreview(seekAt(nativeEvent.locationX))
        }
        onResponderRelease={({ nativeEvent }) => {
          const next = seekAt(nativeEvent.locationX);
          setPreview(null);
          if (next !== null) onSeek(next);
        }}
        onResponderTerminate={() => setPreview(null)}
        style={styles.target}
      >
        <View style={[styles.track, { backgroundColor: theme.line }]}>
          <View
            style={[
              styles.fill,
              {
                backgroundColor: theme.signal,
                width: `${(value / duration) * 100}%`,
              },
            ]}
          />
        </View>
      </View>
      <View style={styles.times}>
        <Text
          style={{
            color: theme.muted,
            fontSize: Platform.isTV ? 22 : undefined,
          }}
        >
          {clock(value)}
        </Text>
        <Text
          style={{
            color: theme.muted,
            fontSize: Platform.isTV ? 22 : undefined,
          }}
        >
          −{clock(duration - value)}
        </Text>
      </View>
    </View>
  );
}
const styles = StyleSheet.create({
  target: { height: 44, justifyContent: 'center' },
  track: { height: 5, borderRadius: 3, overflow: 'hidden' },
  fill: { height: 5 },
  times: { flexDirection: 'row', justifyContent: 'space-between' },
});
