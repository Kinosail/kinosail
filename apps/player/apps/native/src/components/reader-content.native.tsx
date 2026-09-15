import React from 'react';
import { requireNativeView } from 'expo';
import { Platform, Text } from 'react-native';
import type { ReaderContentProps } from './reader-content.types';
const NativeReader =
  Platform.OS === 'ios' && !Platform.isTV
    ? requireNativeView<ReaderContentProps & { style: object }>(
        'ProtectedReader',
      )
    : null;
export function ReaderContent(props: ReaderContentProps) {
  return NativeReader ? (
    <NativeReader {...props} style={{ flex: 1, overflow: 'hidden' }} />
  ) : (
    <Text>Read this book in the iPhone, iPad, or web app.</Text>
  );
}
