import { artworkSource } from '@/core/artwork-source';
import { Image } from 'expo-image';
import React, { useRef, useState } from 'react';
import { Platform, Pressable, StyleSheet, Text, View } from 'react-native';

import type { MediaItem } from '@/core/contract';
import { radius, spacing, useKinoTheme } from '@/design/tokens';
import { NavigationIcon } from './navigation-icon';

type Props = {
  item: MediaItem;
  imageURL: string;
  headers?: Record<string, string>;
  onPress(): void;
  onHighlight?: () => void;
  width: number;
  square?: boolean;
  landscape?: boolean;
  resume?: boolean;
  compact?: boolean;
  preferredFocus?: boolean;
};

const mediaSubtitle = (item: MediaItem) => {
  if (item.show) return `S${item.season} E${item.episode}`;
  return item.year || item.artist || item.kind;
};

export const MediaCard = React.memo(function MediaCard({
  item,
  imageURL,
  headers,
  onPress,
  onHighlight,
  width,
  square = false,
  landscape = false,
  resume = landscape,
  compact = false,
  preferredFocus = false,
}: Props) {
  const theme = useKinoTheme();
  const preferredUsed = useRef(false);
  const [focused, setFocused] = useState(false);
  const [failedURL, setFailedURL] = useState<string | null>(null);
  return (
    <Pressable
      accessibilityLabel={[
        resume ? (item.progress.watched ? 'Play again' : 'Resume') : '',
        item.title,
        mediaSubtitle(item),
        item.progress.seconds > 0 && !item.progress.watched
          ? 'In progress'
          : '',
      ]
        .filter(Boolean)
        .join(', ')}
      accessibilityRole="button"
      focusable
      hasTVPreferredFocus={
        Platform.isTV && preferredFocus && !preferredUsed.current
      }
      onBlur={() => setFocused(false)}
      onFocus={() => {
        preferredUsed.current = true;
        setFocused(true);
        onHighlight?.();
      }}
      onPress={onPress}
      style={({ pressed }) => [
        styles.card,
        {
          opacity: pressed ? 0.8 : 1,
          width,
        },
      ]}
    >
      <View
        style={[
          styles.poster,
          {
            aspectRatio: landscape ? 16 / 9 : square ? 1 : 2 / 3,
            backgroundColor: theme.raised,
            borderColor: focused ? theme.focus : theme.line,
            borderWidth: Platform.isTV ? 3 : focused ? 2 : 1,
          },
        ]}
      >
        {imageURL && failedURL !== imageURL ? (
          <Image
            accessibilityLabel={`Poster for ${item.title}`}
            alt={`Poster for ${item.title}`}
            cachePolicy="memory-disk"
            contentFit={item.show && !landscape ? 'contain' : 'cover'}
            enforceEarlyResizing
            recyclingKey={item.id}
            onError={() => setFailedURL(imageURL)}
            source={artworkSource(imageURL, headers)}
            style={StyleSheet.absoluteFill}
          />
        ) : (
          <View style={styles.fallback}>
            <Text style={[styles.fallbackText, { color: theme.muted }]}>
              {item.title.slice(0, 1).toUpperCase()}
            </Text>
          </View>
        )}
        {resume ? (
          <View
            pointerEvents="none"
            style={[
              StyleSheet.absoluteFill,
              { alignItems: 'center', justifyContent: 'center' },
            ]}
          >
            <View
              style={{
                width: 48,
                height: 48,
                borderRadius: 24,
                backgroundColor: '#00000088',
                borderWidth: 2,
                borderColor: '#FFFFFF',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <NavigationIcon name="play" color="#FFFFFF" />
            </View>
          </View>
        ) : null}
        {!landscape && item.progress.seconds > 0 && !item.progress.watched ? (
          <View
            style={[styles.progressBadge, { backgroundColor: theme.signal }]}
          >
            <Text
              maxFontSizeMultiplier={1.5}
              style={[
                styles.progressText,
                compact ? { letterSpacing: 0, fontWeight: '600' } : null,
                { color: theme.signalInk },
              ]}
            >
              {compact ? 'In progress' : 'IN PROGRESS'}
            </Text>
          </View>
        ) : null}
      </View>
      <Text
        maxFontSizeMultiplier={2}
        numberOfLines={compact ? 1 : 2}
        style={[
          styles.title,
          compact
            ? { minHeight: 0, fontSize: 14, fontWeight: '600', marginTop: 4 }
            : null,
          { color: theme.text },
        ]}
      >
        {item.title}
      </Text>
      <Text
        maxFontSizeMultiplier={2}
        numberOfLines={1}
        style={[styles.subtitle, { color: theme.muted }]}
      >
        {mediaSubtitle(item)}
        {resume && item.progress.seconds > 0
          ? ` · ${Math.floor(item.progress.seconds / 60)} min watched`
          : ''}
      </Text>
    </Pressable>
  );
});

const styles = StyleSheet.create({
  card: { gap: spacing.fine },
  poster: { borderRadius: radius.card, overflow: 'hidden' },
  fallback: { alignItems: 'center', flex: 1, justifyContent: 'center' },
  fallbackText: { fontSize: 44, fontWeight: '300' },
  progressBadge: {
    bottom: 0,
    left: 0,
    paddingHorizontal: spacing.one,
    paddingVertical: spacing.fine,
    position: 'absolute',
  },
  progressText: { fontSize: 9, fontWeight: '900', letterSpacing: 1.1 },
  title: {
    fontSize: Platform.isTV ? 21 : 15,
    fontWeight: '800',
    minHeight: Platform.isTV ? 54 : 38,
    lineHeight: Platform.isTV ? 27 : 19,
    marginTop: spacing.one,
  },
  subtitle: {
    fontSize: Platform.isTV ? 17 : 12,
    fontWeight: '600',
    textTransform: 'uppercase',
  },
});
