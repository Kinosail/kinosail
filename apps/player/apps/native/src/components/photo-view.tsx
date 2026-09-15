import { Image } from 'expo-image';
import React, { useState } from 'react';
import { ScrollView, Text, View, useWindowDimensions } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import type { MediaItem } from '@/core/contract';
import { artworkSource } from '@/core/artwork-source';
import { useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';

export function PhotoView({
  item,
  uri,
  headers,
  onBack,
}: {
  item: MediaItem;
  uri: string;
  headers: Record<string, string>;
  onBack(): void;
}) {
  const theme = useKinoTheme();
  const { width, height } = useWindowDimensions();
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState('loading');
  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: theme.background }}>
      <View style={{ padding: 20, gap: 12 }}>
        <ActionButton label="Back" onPress={onBack} quiet />
        <Text
          accessibilityRole="header"
          style={{ color: theme.text, fontSize: 24 }}
        >
          {item.title}
        </Text>
        {state !== 'ready' ? (
          <Text accessibilityLiveRegion="polite" style={{ color: theme.muted }}>
            {state === 'error'
              ? 'Could not load this photo.'
              : 'Loading photo…'}
          </Text>
        ) : null}
        {state === 'error' ? (
          <ActionButton
            label="Try again"
            onPress={() => {
              setState('loading');
              setAttempt((value) => value + 1);
            }}
          />
        ) : null}
      </View>
      <ScrollView
        maximumZoomScale={4}
        minimumZoomScale={1}
        centerContent
        contentContainerStyle={{ flexGrow: 1, justifyContent: 'center' }}
      >
        <Image
          key={attempt}
          source={artworkSource(uri, headers)}
          accessibilityLabel={item.title}
          alt={item.title}
          contentFit="contain"
          cachePolicy="memory"
          onLoad={() => setState('ready')}
          onError={() => setState('error')}
          style={{ width, height: Math.max(200, height - 240) }}
        />
      </ScrollView>
    </SafeAreaView>
  );
}
