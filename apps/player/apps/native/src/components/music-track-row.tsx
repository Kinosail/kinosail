import React, { useState } from 'react';
import { Platform, Pressable, Text, View } from 'react-native';
import type { MediaItem } from '@/core/contract';
import { useKinoTheme } from '@/design/tokens';
import { NavigationIcon } from './navigation-icon';

export function MusicTrackRow({
  item,
  number,
  onPress,
}: {
  item: MediaItem;
  number?: number;
  onPress(): void;
}) {
  const theme = useKinoTheme();
  const [focused, setFocused] = useState(false);
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={`Play ${item.title}${item.artist ? ` by ${item.artist}` : ''}`}
      focusable
      onPress={onPress}
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
      style={({ pressed }) => ({
        flexDirection: 'row',
        alignItems: 'center',
        gap: 16,
        minHeight: Platform.isTV ? 80 : 64,
        padding: 12,
        borderRadius: 10,
        borderWidth: 2,
        borderColor: focused ? theme.focus : 'transparent',
        backgroundColor: pressed ? theme.raised : 'transparent',
      })}
    >
      {number !== undefined ? (
        <Text style={{ color: theme.muted, minWidth: 28 }}>{number}</Text>
      ) : (
        <NavigationIcon name="music" color={theme.muted} />
      )}
      <View style={{ flex: 1, gap: 4 }}>
        <Text style={{ color: theme.text, fontSize: Platform.isTV ? 28 : 17 }}>
          {item.title}
        </Text>
        <Text style={{ color: theme.muted }}>
          {[item.artist, item.album].filter(Boolean).join(' · ') ||
            'Unknown artist'}
        </Text>
      </View>
      <NavigationIcon name="play" color={theme.text} />
    </Pressable>
  );
}
