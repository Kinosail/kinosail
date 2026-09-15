import React, { type ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';
import { useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';

// Place after the scrolling body, inside the screen's safe area.
export function BottomActions({
  children,
  onBack,
}: {
  children?: ReactNode;
  onBack?: () => void;
}) {
  const theme = useKinoTheme();
  return (
    <View
      style={[
        styles.bar,
        { backgroundColor: theme.background, borderTopColor: theme.line },
      ]}
    >
      {children ? <View style={styles.actions}>{children}</View> : null}
      {onBack ? <ActionButton label="Back" quiet onPress={onBack} /> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  bar: {
    flexShrink: 0,
    flexDirection: 'row',
    flexWrap: 'wrap',
    alignItems: 'center',
    justifyContent: 'flex-end',
    padding: 12,
    gap: 8,
    borderTopWidth: 1,
  },
  actions: {
    flex: 1,
    flexDirection: 'row',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: 8,
  },
});
