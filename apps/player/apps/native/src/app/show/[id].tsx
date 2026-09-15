import { router, useLocalSearchParams } from 'expo-router';
import React, { useEffect, useState } from 'react';
import { Platform, SectionList, Text, View } from 'react-native';
import { FocusGroup } from '@/components/focus-group';
import { Image } from 'expo-image';
import { LinearGradient } from 'expo-linear-gradient';
import { artworkSource } from '@/core/artwork-source';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ActionButton } from '@/components/action-button';
import { WatchProgress } from '@/components/watch-progress';
import { ScreenState } from '@/components/screen-state';
import type { MediaItem } from '@/core/contract';
import type { KinosailClient } from '@/core/server-client';
import { useSession } from '@/core/session-context';
import { ArtworkThemeContext, spacing, useKinoTheme } from '@/design/tokens';

const goBack = () => (router.canGoBack() ? router.back() : router.replace('/'));

export default function ShowScreen() {
  const { id } = useLocalSearchParams();
  const { client } = useSession();
  const valid = typeof id === 'string' && /^[a-f0-9]{16}$/.test(id);
  const [retry, setRetry] = useState(0);
  const [selectedSeason, setSelectedSeason] = useState<number | null>(null);
  const [result, setResult] = useState<{
    client: KinosailClient;
    id: string;
    episodes: MediaItem[];
    error: string;
  } | null>(null);
  const current =
    result?.client === client && result?.id === id ? result : null;
  const featured =
    current?.episodes.find(
      (episode) => episode.progress.seconds > 0 && !episode.progress.watched,
    ) ?? current?.episodes[0];
  const headers = client?.authorizationHeaders();
  const theme = useKinoTheme();
  const backdropURI = featured?.backdrop
    ? (client?.mediaURL(featured.backdrop) ?? '')
    : '';
  const [failedBackdrop, setFailedBackdrop] = useState('');
  const hasBackdrop = Boolean(backdropURI && backdropURI !== failedBackdrop);
  useEffect(() => {
    if (!client || !valid) return;
    let active = true;
    client.loadShowEpisodes(id).then(
      (episodes) => active && setResult({ client, id, episodes, error: '' }),
      () =>
        active &&
        setResult({
          client,
          id,
          episodes: [],
          error: 'Could not load this show.',
        }),
    );
    return () => {
      active = false;
    };
  }, [client, id, valid, retry]);
  if (!valid || !client)
    return (
      <ScreenState
        message="This show is not available."
        action="Go back"
        onAction={goBack}
      />
    );
  if (current?.error)
    return (
      <ScreenState
        message={current.error}
        action="Try again"
        onAction={() => {
          setResult(null);
          setRetry((value) => value + 1);
        }}
        secondaryAction="Go back"
        onSecondaryAction={goBack}
      />
    );
  if (!current) return (
    <ScreenState
      loading
      message="Loading seasons and episodes…"
      action="Go back"
      actionQuiet
      onAction={goBack}
    />
  );
  const seasons = new Map<number, MediaItem[]>();
  for (const episode of current.episodes) {
    const entries = seasons.get(episode.season) ?? [];
    entries.push(episode);
    seasons.set(episode.season, entries);
  }
  const sections = [...seasons]
    .sort(([a], [b]) => a - b)
    .map(([season, episodes]) => ({
      season,
      title: season === 0 ? 'Specials' : `Season ${season}`,
      data: episodes.sort(
        (a, b) => a.episode - b.episode || a.title.localeCompare(b.title),
      ),
    }));
  return (
    <ArtworkThemeContext.Provider value={theme}>
      <SafeAreaView style={{ flex: 1, backgroundColor: theme.background }}>
        <SectionList
          sections={
            Platform.isTV
              ? sections.filter(
                  (section) =>
                    section.season === (selectedSeason ?? featured?.season),
                )
              : sections
          }
          keyExtractor={(item) => item.id}
          contentContainerStyle={{
            padding: Platform.isTV ? 64 : spacing.four,
            gap: spacing.two,
          }}
          removeClippedSubviews={!Platform.isTV}
          stickySectionHeadersEnabled={false}
          ListHeaderComponent={
            <View
              style={{
                minHeight: hasBackdrop ? 420 : 0,
                justifyContent: 'flex-end',
                paddingBottom: spacing.three,
              }}
            >
              {hasBackdrop ? (
                <>
                  <Image
                    source={artworkSource(backdropURI, headers)}
                    contentFit="cover"
                    accessibilityLabel={`Backdrop for ${featured?.show}`}
                    onError={() => setFailedBackdrop(backdropURI)}
                    style={{ position: 'absolute', inset: 0 }}
                  />
                  <LinearGradient
                    colors={[`${theme.background}10`, theme.background]}
                    style={{ position: 'absolute', inset: 0 }}
                    pointerEvents="none"
                  />
                </>
              ) : null}
              <Text
                accessibilityRole="header"
                style={{
                  color: theme.text,
                  fontSize: Platform.isTV ? 64 : 42,
                  lineHeight: Platform.isTV ? 72 : 47,
                  fontWeight: '700',
                  letterSpacing: -1.2,
                }}
              >
                {featured?.show}
              </Text>
              {featured ? (
                <WatchProgress
                  id={featured.id}
                  seconds={featured.progress.seconds}
                  watched={featured.progress.watched}
                  mediaURL={(path) => client.mediaURL(path)}
                  headers={headers}
                />
              ) : null}
              {Platform.isTV && featured ? (
                <>
                  <ActionButton
                    preferredFocus
                    label={`${featured.progress.seconds > 0 ? 'Resume' : 'Play'} S${featured.season} E${featured.episode}`}
                    onPress={() =>
                      router.push({
                        pathname: '/watch/[id]',
                        params: { id: featured.id },
                      })
                    }
                    style={{ alignSelf: 'flex-start', marginVertical: 24 }}
                  />
                  <FocusGroup
                    style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 16 }}
                  >
                    {sections.map((section) => (
                      <ActionButton
                        key={section.season}
                        label={section.title}
                        selected={
                          section.season === (selectedSeason ?? featured.season)
                        }
                        quiet={
                          section.season !== (selectedSeason ?? featured.season)
                        }
                        onPress={() => setSelectedSeason(section.season)}
                      />
                    ))}
                  </FocusGroup>
                </>
              ) : null}
            </View>
          }
          renderSectionHeader={({ section }) => (
            <Text
              accessibilityRole="header"
              style={{
                color: theme.text,
                fontSize: Platform.isTV ? 32 : 22,
                fontWeight: '700',
                paddingVertical: spacing.two,
              }}
            >
              {section.title}
            </Text>
          )}
          renderItem={({ item }) => (
            <ActionButton
              label={`E${item.episode} · ${item.title}${item.progress.watched ? ' · Watched' : ''}`}
              quiet
              onPress={() =>
                router.push({ pathname: '/item/[id]', params: { id: item.id } })
              }
            />
          )}
        />
        <ActionButton
          label="Back"
          quiet
          onPress={goBack}
          style={{ margin: spacing.two }}
        />
      </SafeAreaView>
    </ArtworkThemeContext.Provider>
  );
}
