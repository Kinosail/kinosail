import React, { useState } from 'react';
import { Image } from 'expo-image';
import {
  Pressable,
  StyleSheet,
  Text,
  View,
  useWindowDimensions,
} from 'react-native';
import { artworkSource } from '@/core/artwork-source';
import type { MediaItem } from '@/core/contract';
import { useKinoTheme } from '@/design/tokens';
import { NavigationIcon } from './navigation-icon';

export function AudioButton({
  label,
  icon,
  onPress,
  caption,
  primary = false,
  disabled = false,
  selected,
}: {
  label: string;
  icon: React.ComponentProps<typeof NavigationIcon>['name'];
  onPress(): void;
  caption?: string;
  primary?: boolean;
  disabled?: boolean;
  selected?: boolean;
}) {
  const theme = useKinoTheme();
  const [focused, setFocused] = useState(false);
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={label}
      accessibilityState={{ disabled, selected }}
      disabled={disabled}
      focusable={!disabled}
      onPress={onPress}
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
      style={({ pressed }) => ({
        width: caption ? undefined : primary ? 72 : 48,
        minWidth: caption ? 64 : undefined,
        flexBasis: caption ? '22%' : undefined,
        gap: caption ? 8 : 0,
        paddingVertical: caption ? 4 : 0,
        minHeight: primary ? 72 : caption ? 64 : 48,
        borderRadius: primary ? 36 : 12,
        alignItems: 'center',
        justifyContent: 'center',
        backgroundColor: primary ? theme.signal : 'transparent',
        borderWidth: 2,
        borderColor: focused ? theme.focus : 'transparent',
        opacity: disabled ? 0.35 : pressed ? 0.65 : 1,
      })}
    >
      <NavigationIcon
        name={icon}
        color={primary ? theme.signalInk : selected ? theme.signal : theme.text}
        size={caption ? 24 : 32}
      />
      {caption ? (
        <Text style={{ color: theme.text, fontSize: 12, textAlign: 'center' }}>
          {caption}
        </Text>
      ) : null}
    </Pressable>
  );
}

export function AudioArtwork({
  item,
  uri,
  headers,
}: {
  item: MediaItem;
  uri?: string;
  headers?: Record<string, string>;
}) {
  const theme = useKinoTheme();
  const { width, height } = useWindowDimensions();
  const [failed, setFailed] = useState<string>();
  const audiobook = item.kind === 'audiobook';
  const size = Math.max(
    96,
    Math.min(width - 48, height * (audiobook ? 0.24 : 0.4), 440),
  );
  return (
    <View style={[styles.stage, { minHeight: size + 48 }]}>
      <View
        style={{
          width: size,
          height: size,
          maxWidth: '100%',
          borderRadius: 16,
          overflow: 'hidden',
          backgroundColor: theme.raised,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        {uri && failed !== uri ? (
          <Image
            source={artworkSource(uri, headers)}
            accessibilityLabel={`${audiobook ? 'Book cover' : 'Album artwork'} for ${item.title}`}
            contentFit={audiobook ? 'contain' : 'cover'}
            onError={() => setFailed(uri)}
            style={StyleSheet.absoluteFill}
          />
        ) : (
          <View
            accessible
            accessibilityLabel={
              audiobook ? 'No book cover' : 'No album artwork'
            }
            style={{ transform: [{ scale: 3 }] }}
          >
            <NavigationIcon
              name={audiobook ? 'books' : 'music'}
              color={theme.muted}
            />
          </View>
        )}
      </View>
    </View>
  );
}
const styles = StyleSheet.create({
  stage: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    padding: 24,
  },
});
