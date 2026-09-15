import React from 'react';
import { Text, View } from 'react-native';
import { useKinoTheme } from '@/design/tokens';
import type { PlaybackPreferences } from '@/core/media-preferences';
import { ActionButton } from './action-button';

export function PlaybackPreferenceControls({
  value,
  onChange,
  audioAvailable,
  section,
  disabled = false,
}: {
  value: PlaybackPreferences;
  onChange(value: PlaybackPreferences): void;
  audioAvailable: boolean;
  section?: 'speed' | 'audio';
  disabled?: boolean;
}) {
  const theme = useKinoTheme();
  return (
    <View style={{ gap: 12 }}>
      {section !== 'audio' ? (
        <>
          <Text
            accessibilityRole="header"
            style={{ fontSize: 20, color: theme.text }}
          >
            Playback speed
          </Text>
          <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
            {[0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3].map((rate) => (
              <ActionButton
                key={rate}
                label={`${rate}×`}
                accessibilityLabel={`Speed ${rate} times${value.rate === rate ? ', selected' : ''}`}
                selected={value.rate === rate}
                quiet={value.rate !== rate}
                disabled={disabled}
                onPress={() => onChange({ ...value, rate })}
              />
            ))}
          </View>
        </>
      ) : null}
      {section !== 'speed' ? (
        <>
          <Text
            accessibilityRole="header"
            style={{ fontSize: 20, color: theme.text }}
          >
            Audio
          </Text>
          {audioAvailable ? (
            <>
              <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
                <ActionButton
                  label={`Night mode: ${value.nightMode ? 'On' : 'Off'}`}
                  selected={value.nightMode}
                  quiet={!value.nightMode}
                  disabled={disabled}
                  onPress={() =>
                    onChange({ ...value, nightMode: !value.nightMode })
                  }
                />
                <ActionButton
                  label={`Dialogue boost: ${value.dialogueBoost ? 'On' : 'Off'}`}
                  selected={value.dialogueBoost}
                  quiet={!value.dialogueBoost}
                  disabled={disabled}
                  onPress={() =>
                    onChange({ ...value, dialogueBoost: !value.dialogueBoost })
                  }
                />
              </View>
              <Text style={{ color: theme.muted }}>
                Night mode reduces loud peaks. Dialogue boost emphasizes speech
                frequencies. Audio effects use stereo output with a peak
                limiter.
              </Text>
              <Text style={{ color: theme.text }}>Volume boost</Text>
              <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
                {[1, 1.25, 1.5, 1.75, 2].map((volumeBoost) => (
                  <ActionButton
                    key={volumeBoost}
                    label={`${Math.round(volumeBoost * 100)}%`}
                    accessibilityLabel={`Volume boost ${Math.round(volumeBoost * 100)} percent${value.volumeBoost === volumeBoost ? ', selected' : ''}`}
                    selected={value.volumeBoost === volumeBoost}
                    quiet={value.volumeBoost !== volumeBoost}
                    disabled={disabled}
                    onPress={() => onChange({ ...value, volumeBoost })}
                  />
                ))}
              </View>
            </>
          ) : (
            <Text style={{ color: theme.muted }}>
              Audio effects are available in the native Apple player. Your saved
              choices apply on supported playback routes.
            </Text>
          )}
        </>
      ) : null}
    </View>
  );
}
