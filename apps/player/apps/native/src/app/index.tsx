import { router } from 'expo-router';
import React, { useEffect, useState } from 'react';
import { Alert, Platform } from 'react-native';

import { downloadsAvailable } from '@/core/downloads';
import { HomeView } from '@/components/home-view';
import { HomeSkeleton } from '@/components/home-skeleton';
import { ActionButton } from '@/components/action-button';
import { ScreenState } from '@/components/screen-state';
import { SetupFlow } from '@/components/setup-flow';
import type { Home } from '@/core/contract';
import { homeCache } from '@/core/browse-cache';
import { rememberMediaItems } from '@/core/media-loader';
import { KinosailClient } from '@/core/server-client';
import { useSession } from '@/core/session-context';
import { useThemePreference } from '@/design/theme-context';

const createClient = (baseURL: string) => new KinosailClient(baseURL);

export default function HomeScreen() {
  const { booting, bootError, session, client, connect, retryBoot, signOut } =
    useSession();
  const { preference, cycle } = useThemePreference();
  const [result, setResult] = useState<{
    client: KinosailClient;
    home: Home | null;
    error: string;
  } | null>(null);
  const current = result?.client === client ? result : null;
  const home = current
    ? current.home
    : client
      ? homeCache.read(client, 'home')
      : null;
  const error = current?.error;
  const [refresh, setRefresh] = useState({});
  useEffect(() => {
    let active = true;
    if (!client) return;
    client.loadHome().then(
      (loaded) => {
        if (!active) return;
        rememberMediaItems(client, [
          ...loaded.recent,
          ...loaded.continueWatching,
        ]);
        homeCache.write(client, 'home', loaded);
        setResult({ client, home: loaded, error: '' });
      },
      (reason) => {
        if (!active) return;
        homeCache.clear(client);
        setResult({
          client,
          home: null,
          error:
            reason instanceof Error
              ? reason.message
              : 'Could not load the library.',
        });
      },
    );
    return () => {
      active = false;
    };
  }, [client, refresh]);

  const retry = () => {
    setResult(null);
    setRefresh({});
  };
  const leaveServer = async () => {
    try {
      await signOut();
      setResult(null);
    } catch {
      if (client)
        setResult({
          client,
          home: null,
          error: 'Could not sign out. Try again.',
        });
    }
  };
  const changeServer = () => {
    if (!downloadsAvailable) return void leaveServer();
    Alert.alert(
      'Use another server?',
      'Signing out removes downloaded titles from this device.',
      [
        { text: 'Cancel', style: 'cancel' },
        {
          text: 'Sign out',
          style: 'destructive',
          onPress: () => void leaveServer(),
        },
      ],
    );
  };
  const sessionExpired = error === 'This device session has expired.';

  if (booting) return <HomeSkeleton message="Opening your player…" />;
  if (bootError)
    return (
      <ScreenState
        action="Try again"
        message={bootError}
        onAction={retryBoot}
      />
    );
  if (!session || !client)
    return <SetupFlow createClient={createClient} onConnected={connect} />;
  if (error)
    return (
      <ScreenState
        action={sessionExpired ? 'Reconnect' : 'Try again'}
        message={error}
        onAction={sessionExpired ? changeServer : retry}
        onSecondaryAction={
          downloadsAvailable
            ? () => router.push('/downloads')
            : sessionExpired
              ? undefined
              : changeServer
        }
        secondaryAction={
          downloadsAvailable
            ? 'Downloads'
            : sessionExpired
              ? undefined
              : 'Use another server'
        }
      >
        {downloadsAvailable && !sessionExpired ? (
          <ActionButton
            label="Use another server"
            onPress={changeServer}
            quiet
          />
        ) : null}
      </ScreenState>
    );
  if (!home) return <HomeSkeleton />;
  return (
    <HomeView
      headers={client.authorizationHeaders()}
      home={home}
      mediaURL={(path) => client.mediaURL(path)}
      onOpen={(id) => router.push({ pathname: '/item/[id]', params: { id } })}
      onPlay={(id, fromBeginning) =>
        router.push({
          pathname: '/watch/[id]',
          params: { id, ...(fromBeginning ? { start: 'beginning' } : {}) },
        })
      }
      onBrowse={(view) =>
        router.push({
          pathname: '/library',
          params: view ? { view } : Platform.isTV ? { search: '1' } : {},
        })
      }
      onDownloads={
        downloadsAvailable ? () => router.push('/downloads') : undefined
      }
      onRefresh={retry}
      onSignOut={signOut}
      onTheme={cycle}
      themeLabel={preference[0].toUpperCase() + preference.slice(1)}
    />
  );
}
