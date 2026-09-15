import React from 'react';
import { Platform, StyleSheet, Text, View } from 'react-native';

import { spacing, useKinoTheme } from '@/design/tokens';

import { ActionButton } from './action-button';
import { BrandMark } from './brand-mark';
import { Skeleton } from './skeleton';

export function ScreenState({
  message,
  children,
  action,
  actionQuiet = false,
  loading = false,
  onAction,
  secondaryAction,
  onSecondaryAction,
}: {
  message: string;
  children?: React.ReactNode;
  action?: string;
  actionQuiet?: boolean;
  loading?: boolean;
  onAction?: () => void;
  secondaryAction?: string;
  onSecondaryAction?: () => void;
}) {
  const theme = useKinoTheme();
  const error = !loading;
  return (
    <View
      role="main"
      style={[styles.screen, { backgroundColor: theme.background }]}
    >
      <BrandMark />
      {!error ? <Skeleton variant="content" label={message} /> : null}
      <Text
        accessibilityLiveRegion={error ? 'assertive' : 'polite'}
        style={[styles.message, { color: error ? theme.danger : theme.muted }]}
      >
        {message}
      </Text>
      {children}
      {action && onAction ? (
        <View style={styles.actions}>
          <ActionButton
            preferredFocus={loading || !actionQuiet}
            quiet={actionQuiet}
            label={action}
            onPress={onAction}
          />
          {secondaryAction && onSecondaryAction ? (
            <ActionButton
              label={secondaryAction}
              onPress={onSecondaryAction}
              quiet
            />
          ) : null}
        </View>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  screen: {
    alignItems: 'center',
    flex: 1,
    gap: spacing.three,
    justifyContent: 'center',
    padding: spacing.four,
  },
  actions: {
    alignItems: 'center',
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.one,
    justifyContent: 'center',
  },
  message: {
    fontSize: Platform.isTV ? 28 : 16,
    lineHeight: Platform.isTV ? 40 : 24,
    maxWidth: Platform.isTV ? 840 : 480,
    textAlign: 'center',
  },
});
