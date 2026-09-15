import React, { useState } from 'react';
import { Text, View } from 'react-native';
import { ModalSheet } from './modal-sheet';
import type { PlaybackSource } from '@/core/contract';
import type { PlaybackExperience } from '@/core/use-playback-experience';
import { formatPosition } from '@/core/media-preferences';
import { useKinoTheme } from '@/design/tokens';
import { AudioButton } from './audio-player-controls';
import { ActionButton } from './action-button';
import { PlaybackExperienceOptions } from './playback-experience-options';
import { PlaybackPreferenceControls } from './playback-preference-controls';

type Panel = 'speed' | 'sleep' | 'chapters' | 'bookmarks' | 'audio';
type Props = {
  source: PlaybackSource;
  position: number;
  chapterIndex: number;
  canSeek: boolean;
  experience: PlaybackExperience;
  sleep: string;
  chapterEnd: boolean;
  progressMessage: string;
  onSeek(seconds: number): void;
  onSleep(minutes: number): void;
  onNext?: () => void;
};

export function AudiobookProgress({
  source,
  position,
  chapterIndex,
  rate,
}: Pick<Props, 'source' | 'position' | 'chapterIndex'> & { rate: number }) {
  const theme = useKinoTheme();
  const chapters = source.details?.chapters ?? [];
  const chapter = chapters[chapterIndex];
  const minutes = Math.ceil(
    Math.max(0, source.duration - position) / rate / 60,
  );
  return (
    <View style={{ paddingHorizontal: 24, paddingTop: 12, gap: 4 }}>
      {chapter ? (
        <Text style={{ color: theme.text }}>
          Chapter {chapterIndex + 1} of {chapters.length}
          {chapter.title ? ` · ${chapter.title}` : ''}
        </Text>
      ) : null}
      {source.duration > 0 ? (
        <Text style={{ color: theme.muted }}>
          {Math.floor(
            Math.min(1, Math.max(0, position / source.duration)) * 100,
          )}
          % of book · {minutes >= 60 ? `${Math.floor(minutes / 60)}h ` : ''}
          {minutes % 60}m left at {rate}×
        </Text>
      ) : (
        <Text style={{ color: theme.muted }}>Duration unavailable</Text>
      )}
    </View>
  );
}

export function AudiobookTools(props: Props) {
  const { source, chapterIndex, experience, sleep, onSeek, onSleep, onNext } =
    props;
  const theme = useKinoTheme();
  const [panel, setPanel] = useState<Panel | null>(null);
  const chapters = source.details?.chapters ?? [];
  const count = experience.bookmarks.filter(
    (mark) => mark.seconds !== undefined,
  ).length;
  const choosePosition = (seconds: number) => {
    onSeek(seconds);
    setPanel(null);
  };
  return (
    <>
      <View
        style={{
          flexDirection: 'row',
          flexWrap: 'wrap',
          justifyContent: 'space-between',
          gap: 4,
          paddingHorizontal: 16,
          paddingBottom: 8,
        }}
      >
        <AudioButton
          icon="speed"
          caption={`${experience.preferences.rate}×`}
          label={`Playback speed, ${experience.preferences.rate} times`}
          onPress={() => setPanel('speed')}
        />
        <AudioButton
          icon="sleep"
          caption={sleep || 'Sleep'}
          label={sleep ? `Sleep timer, ${sleep}` : 'Sleep timer, off'}
          onPress={() => setPanel('sleep')}
        />
        <AudioButton
          icon="chapters"
          caption="Chapters"
          label="Chapters"
          onPress={() => setPanel('chapters')}
        />
        <AudioButton
          icon="bookmark"
          caption={`Bookmarks (${count})`}
          label={`Bookmarks, ${count} saved`}
          onPress={() => setPanel('bookmarks')}
        />
      </View>
      {props.progressMessage ? (
        <Text
          accessibilityLiveRegion="polite"
          style={{ padding: 16, color: theme.muted }}
        >
          {props.progressMessage}
        </Text>
      ) : null}
      {experience.error ? (
        <Text
          accessibilityRole="alert"
          style={{ padding: 16, color: theme.text }}
        >
          {experience.error}
        </Text>
      ) : null}
      {panel ? (
        <ModalSheet
          title="Listening options"
          animationType="none"
          onClose={() => setPanel(null)}
          footer={
            panel !== 'audio' ? (
              <ActionButton
                label="Audio settings"
                quiet
                onPress={() => setPanel('audio')}
              />
            ) : (
              <ActionButton
                label="Use default playback preferences"
                quiet
                disabled={experience.saving}
                onPress={experience.reset}
              />
            )
          }
        >
          {panel === 'speed' || panel === 'audio' ? (
            <View style={{ padding: 16 }}>
              <PlaybackPreferenceControls
                section={panel}
                value={experience.preferences}
                audioAvailable
                disabled={!experience.loaded || experience.saving}
                onChange={(value) => {
                  experience.change(value);
                  if (panel === 'speed') setPanel(null);
                }}
              />
            </View>
          ) : null}
          {panel === 'sleep' || panel === 'bookmarks' ? (
            <PlaybackExperienceOptions
              {...props}
              section={panel}
              audioAvailable
              onSeek={choosePosition}
              onSleep={(minutes) => {
                onSleep(minutes);
                setPanel(null);
              }}
            />
          ) : null}
          {panel === 'chapters' ? (
            <View style={{ padding: 16, gap: 12 }}>
              <Text
                accessibilityRole="header"
                style={{ color: theme.text, fontSize: 20 }}
              >
                Chapters
              </Text>
              {!chapters.length ? (
                <Text style={{ color: theme.muted }}>
                  This audiobook has no chapter markers. You can still scrub or
                  skip 30 seconds.
                </Text>
              ) : null}
              {chapters.map((chapter, index) => (
                <ActionButton
                  key={index}
                  label={`${chapter.title || `Chapter ${index + 1}`} · ${formatPosition(chapter.start)}`}
                  disabled={!props.canSeek}
                  selected={index === chapterIndex}
                  quiet={index !== chapterIndex}
                  onPress={() => choosePosition(chapter.start)}
                />
              ))}
              {onNext ? (
                <ActionButton label="Next part" quiet onPress={onNext} />
              ) : null}
            </View>
          ) : null}
        </ModalSheet>
      ) : null}
    </>
  );
}
