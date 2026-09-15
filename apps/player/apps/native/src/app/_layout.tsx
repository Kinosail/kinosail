import { DarkTheme, DefaultTheme, Stack, ThemeProvider } from 'expo-router';
import Head from 'expo-router/head';
import { StatusBar } from 'expo-status-bar';
import React from 'react';
import { Platform, View } from 'react-native';
import { useTVNavigation } from '@/components/tv-navigation-events';
import { TVApprovalPrompt } from '@/components/tv-approval-prompt';
import { BottomNavigation } from '@/components/bottom-navigation';

import { SessionProvider } from '@/core/session-context';
import {
  ThemePreferenceProvider,
  useThemePreference,
} from '@/design/theme-context';

import { ArtworkThemeContext, palette } from '@/design/tokens';

import '../global.css';

function AppNavigator() {
  useTVNavigation();
  const { scheme } = useThemePreference();
  const contentTheme = scheme === 'light' ? palette.light : palette.dark;
  return (
    <SessionProvider>
      <ThemeProvider value={scheme === 'light' ? DefaultTheme : DarkTheme}>
        {Platform.OS === 'web' ? (
          <Head>
            <title>Kinosail Player</title>
          </Head>
        ) : null}
        <StatusBar style={scheme === 'light' ? 'dark' : 'light'} />
        <View style={{ flex: 1, backgroundColor: contentTheme.background }}>
          <ArtworkThemeContext.Provider value={contentTheme}>
            <Stack
              screenOptions={{
                headerShown: false,
                contentStyle: { backgroundColor: contentTheme.background },
              }}
            >
              <Stack.Screen name="index" options={{ animation: 'none' }} />
              <Stack.Screen name="library" options={{ animation: 'none' }} />
              <Stack.Screen name="downloads" options={{ animation: 'none' }} />
              <Stack.Screen name="settings" options={{ animation: 'none' }} />
              <Stack.Screen
                name="watch/[id]"
                options={{ gestureEnabled: false }}
              />
            </Stack>
          </ArtworkThemeContext.Provider>
          <BottomNavigation />
          <TVApprovalPrompt />
        </View>
      </ThemeProvider>
    </SessionProvider>
  );
}

export default function RootLayout() {
  return (
    <ThemePreferenceProvider>
      <AppNavigator />
    </ThemePreferenceProvider>
  );
}
