import React, { useState } from 'react';
import { Text, TextInput } from 'react-native';
import { mediaLanguages } from '@/core/media-languages';
import { useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';
import { ModalSheet } from './modal-sheet';

export function LanguagePicker({
  value,
  subtitles = false,
  disabled,
  onChange,
}: {
  value: string;
  subtitles?: boolean;
  disabled: boolean;
  onChange(value: string): void;
}) {
  const theme = useKinoTheme();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const title = subtitles ? 'Subtitle language' : 'Audio language';
  const options = [
    ['auto', 'Automatic'],
    ...(subtitles ? [['off', 'Off']] : []),
    ...mediaLanguages
      .map(([tag, name]) => [tag, name])
      .sort((a, b) => a[1].localeCompare(b[1])),
  ];
  const label = options.find(([tag]) => tag === value)?.[1] ?? value;
  const matches = options.filter(([tag, name]) =>
    `${tag} ${name}`.toLowerCase().includes(query.trim().toLowerCase()),
  );
  return (
    <>
      <ActionButton
        label={`${title}: ${label}`}
        quiet
        disabled={disabled}
        onPress={() => {
          setQuery('');
          setOpen(true);
        }}
      />
      <ModalSheet visible={open} title={title} onClose={() => setOpen(false)}>
        <TextInput
          accessibilityLabel="Find a language"
          placeholder="Find a language"
          placeholderTextColor={theme.muted}
          value={query}
          onChangeText={setQuery}
          maxLength={64}
          autoCapitalize="none"
          autoCorrect={false}
          style={{
            minHeight: 48,
            padding: 12,
            borderWidth: 1,
            borderColor: theme.line,
            color: theme.text,
            borderRadius: 12,
          }}
        />
        {matches.map(([tag, name]) => (
          <ActionButton
            key={tag}
            label={name}
            quiet
            selected={value === tag}
            disabled={disabled}
            onPress={() => {
              onChange(tag);
              setOpen(false);
            }}
          />
        ))}
        {!matches.length ? (
          <Text accessibilityLiveRegion="polite" style={{ color: theme.muted }}>
            No matching languages. Try a language name or code.
          </Text>
        ) : null}
      </ModalSheet>
    </>
  );
}
