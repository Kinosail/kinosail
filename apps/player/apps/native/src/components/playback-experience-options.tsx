import React from 'react';
import { Text, View } from 'react-native';
import type { PlaybackExperience } from '@/core/use-playback-experience';
import { useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';
import { PlaybackPreferenceControls } from './playback-preference-controls';
export function PlaybackExperienceOptions({
  section,
  canSeek = true,
  experience,
  position,
  onSeek,
  audioAvailable,
  sleep,
  onSleep,
  chapterEnd,
  progressMessage,
}: {
  section?: 'sleep' | 'bookmarks';
  canSeek?: boolean;
  experience: PlaybackExperience;
  position: number;
  onSeek(seconds: number): void;
  audioAvailable: boolean;
  sleep: string;
  onSleep(minutes: number): void;
  chapterEnd: boolean;
  progressMessage: string;
}) {
  const theme = useKinoTheme();
  return (
    <View style={{ gap: 12, padding: 16, backgroundColor: theme.surface }}>
      {!section ? (
        <>
          <PlaybackPreferenceControls
            value={experience.preferences}
            onChange={experience.change}
            audioAvailable={audioAvailable}
            disabled={!experience.loaded}
          />
          <ActionButton
            label="Use default playback preferences"
            quiet
            onPress={experience.reset}
            disabled={experience.saving}
          />
        </>
      ) : null}
      {section !== 'bookmarks' ? (
        <>
          <Text
            accessibilityRole="header"
            style={{ color: theme.text, fontSize: 20 }}
          >
            Sleep timer{sleep ? ` · ${sleep}` : ''}
          </Text>
          <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
            {[0, 15, 30, 60].map((minutes) => (
              <ActionButton
                key={minutes}
                label={minutes ? `${minutes} minutes` : 'Off'}
                selected={sleep === (minutes ? `${minutes} minutes` : '')}
                quiet={sleep !== (minutes ? `${minutes} minutes` : '')}
                onPress={() => onSleep(minutes)}
              />
            ))}
            {chapterEnd ? (
              <ActionButton
                label="End of chapter"
                selected={sleep === 'End of chapter'}
                quiet={sleep !== 'End of chapter'}
                onPress={() => onSleep(-1)}
              />
            ) : null}
          </View>
        </>
      ) : null}
      {section !== 'sleep' ? (
        <>
          <Text
            accessibilityRole="header"
            style={{ color: theme.text, fontSize: 20 }}
          >
            Bookmarks
          </Text>
          <ActionButton
            label="Bookmark this moment"
            onPress={() => experience.bookmark(position)}
            disabled={experience.saving || !canSeek}
          />
          {experience.bookmarks
            .filter((bookmark) => bookmark.seconds !== undefined)
            .map((bookmark) => (
              <View
                key={bookmark.id}
                style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}
              >
                <ActionButton
                  label={bookmark.title}
                  disabled={!canSeek}
                  quiet
                  onPress={() => onSeek(bookmark.seconds!)}
                />
                <ActionButton
                  label={`Remove bookmark ${bookmark.title}`}
                  quiet
                  onPress={() => experience.removeBookmark(bookmark.id)}
                  disabled={experience.saving}
                />
              </View>
            ))}
        </>
      ) : null}
      {experience.error ? (
        <Text accessibilityRole="alert" style={{ color: theme.text }}>
          {experience.error}
        </Text>
      ) : null}
      {progressMessage ? (
        <Text accessibilityLiveRegion="polite" style={{ color: theme.text }}>
          {progressMessage}
        </Text>
      ) : null}
    </View>
  );
}
