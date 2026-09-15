import React, { useRef, useState } from 'react';
import {
  Platform,
  Pressable,
  StyleSheet,
  Text,
  type ViewStyle,
  type StyleProp,
} from 'react-native';

import { radius, spacing, useKinoTheme } from '@/design/tokens';
import { Skeleton } from './skeleton';

type Props = {
  label: string;
  accessibilityLabel?: string;
  onPress: () => void;
  busy?: boolean;
  expanded?: boolean;
  selected?: boolean;
  disabled?: boolean;
  quiet?: boolean;
  preferredFocus?: boolean;
  style?: StyleProp<ViewStyle>;
};

export function ActionButton({
  label,
  accessibilityLabel,
  onPress,
  busy = false,
  expanded,
  selected,
  disabled = false,
  quiet = false,
  preferredFocus = false,
  style,
}: Props) {
  const theme = useKinoTheme();
  const [focused, setFocused] = useState(false);
  const inactive = disabled || busy;
  const preferredUsed = useRef(false);
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel ?? label}
      aria-busy={busy}
      aria-expanded={expanded}
      accessibilityState={{ disabled: inactive, selected }}
      aria-pressed={selected}
      disabled={inactive}
      focusable={!inactive}
      hasTVPreferredFocus={
        Platform.isTV && preferredFocus && !preferredUsed.current
      }
      onBlur={() => setFocused(false)}
      onFocus={() => {
        preferredUsed.current = true;
        setFocused(true);
      }}
      onPress={onPress}
      style={({ pressed }) => [
        styles.button,
        {
          minHeight: Platform.isTV ? 80 : 48,
          backgroundColor: quiet
            ? selected
              ? theme.raised
              : theme.surface
            : theme.signal,
          borderColor: focused
            ? theme.focus
            : selected
              ? theme.signal
              : quiet
                ? theme.line
                : theme.signal,
          opacity: inactive ? 0.45 : pressed ? 0.76 : 1,
        },
        style,
        focused && Platform.isTV
          ? { borderColor: theme.focus, borderWidth: 3 }
          : null,
      ]}
    >
      {busy ? <Skeleton variant="inline" /> : null}
      {selected ? (
        <Text
          accessible={false}
          aria-hidden
          style={{ color: quiet ? theme.text : theme.signalInk }}
        >
          ✓
        </Text>
      ) : null}
      <Text
        style={[styles.label, { color: quiet ? theme.text : theme.signalInk }]}
      >
        {label}
      </Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: {
    alignItems: 'center',
    maxWidth: '100%',
    borderRadius: radius.control,
    borderWidth: 2,
    justifyContent: 'center',
    flexDirection: 'row',
    gap: spacing.one,
    paddingHorizontal: spacing.three,
  },
  label: {
    flexShrink: 1,
    textAlign: 'center',
    fontSize: Platform.isTV ? 28 : 16,
    fontWeight: '800',
    letterSpacing: 0.2,
  },
});
