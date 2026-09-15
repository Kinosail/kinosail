import React from 'react';
import { Platform, TVFocusGuideView, View, type ViewProps } from 'react-native';

// Remember the last focused child when returning from details or another shelf.
export function FocusGroup({
  children,
  horizontal = false,
  ...props
}: ViewProps & { horizontal?: boolean }) {
  return Platform.isTV ? (
    <TVFocusGuideView
      autoFocus
      trapFocusLeft={horizontal}
      trapFocusRight={horizontal}
      {...props}
    >
      {children}
    </TVFocusGuideView>
  ) : (
    <View {...props}>{children}</View>
  );
}
