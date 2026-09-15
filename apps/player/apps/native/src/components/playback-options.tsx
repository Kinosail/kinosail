import React from 'react';
import { ScrollView, Text, View } from 'react-native';
import type { PlaybackSource } from '@/core/contract';
import { spacing, useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';
import { FocusGroup } from './focus-group';

export function PlaybackOptions({
  source,
  onSeek,
  onNext,
}: {
  source: PlaybackSource;
  onSeek(seconds: number): void;
  onNext?: () => void;
}) {
  const theme = useKinoTheme(),
    media = source.details?.media;
  return (
    <ScrollView
      style={{ maxHeight: 260 }}
      contentContainerStyle={{
        padding: spacing.two,
        gap: spacing.one,
        backgroundColor: theme.surface,
      }}
    >
      {media ? (
        <View>
          <Text style={{ color: theme.text }}>Original file</Text>
          <Text style={{ color: theme.muted }}>
            {[
              media.container.toUpperCase(),
              media.codec.toUpperCase(),
              media.width && media.height
                ? `${media.width} × ${media.height}`
                : '',
              media.bitDepth ? `${media.bitDepth}-bit` : '',
              media.hdr,
              media.dolbyVisionProfile
                ? `Dolby Vision profile ${media.dolbyVisionProfile}`
                : '',
            ]
              .filter(Boolean)
              .join(' · ')}
          </Text>
        </View>
      ) : null}
      {onNext ? <ActionButton label="Next episode" onPress={onNext} /> : null}
      {source.details?.chapters.length ? (
        <>
          <Text
            accessibilityRole="header"
            style={{ color: theme.text, fontSize: 20 }}
          >
            Chapters
          </Text>
          <FocusGroup
            style={{ flexDirection: 'row', flexWrap: 'wrap', gap: spacing.one }}
          >
            {source.details.chapters.map((chapter, index) => (
              <ActionButton
                key={index}
                label={chapter.title || `Chapter ${index + 1}`}
                quiet
                onPress={() => onSeek(chapter.start)}
              />
            ))}
          </FocusGroup>
        </>
      ) : null}
    </ScrollView>
  );
}
