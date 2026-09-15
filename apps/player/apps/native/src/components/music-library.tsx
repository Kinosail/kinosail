import { MusicTrackRow } from './music-track-row';
import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  FlatList,
  Platform,
  Pressable,
  StyleSheet,
  Text,
  TextInput,
  View,
  type PressableProps,
  type ViewStyle,
  useWindowDimensions,
} from 'react-native';
import {
  SafeAreaView,
  useSafeAreaInsets,
} from 'react-native-safe-area-context';
import { Image } from 'expo-image';
import { router } from 'expo-router';
import { artworkSource } from '@/core/artwork-source';
import type { Album } from '@/core/music-catalog';
import type { MediaItem } from '@/core/contract';
import type { KinosailClient } from '@/core/server-client';
import { useSession } from '@/core/session-context';
import { useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';
import { NavigationIcon } from './navigation-icon';
import { ScreenState } from './screen-state';
import { BrowseSkeleton } from './browse-skeleton';
import { albumCatalogCache, albumTracksCache } from '@/core/browse-cache';

function MusicPressable({
  style,
  ...props
}: Omit<PressableProps, 'style'> & {
  style(state: { pressed: boolean; focused: boolean }): ViewStyle;
}) {
  const [focused, setFocused] = useState(false);
  return (
    <Pressable
      {...props}
      focusable={props.focusable ?? true}
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
      style={({ pressed }) => style({ pressed, focused })}
    />
  );
}

function Cover({
  album,
  client,
  size,
}: {
  album: Album;
  client: KinosailClient;
  size: number;
}) {
  const theme = useKinoTheme();
  const [failed, setFailed] = useState('');
  return (
    <View
      style={{
        width: size,
        aspectRatio: 1,
        borderRadius: 14,
        overflow: 'hidden',
        backgroundColor: theme.raised,
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      {album.artwork && failed !== album.artwork ? (
        <Image
          source={artworkSource(
            client.mediaURL(album.artwork),
            client.authorizationHeaders(),
          )}
          accessibilityLabel={`Album artwork for ${album.title}`}
          contentFit="cover"
          cachePolicy="memory-disk"
          onError={() => setFailed(album.artwork)}
          style={StyleSheet.absoluteFill}
        />
      ) : (
        <NavigationIcon name="music" color={theme.muted} size={48} />
      )}
    </View>
  );
}

export function MusicLibrary({
  searchOpen = false,
  onSongs,
}: {
  searchOpen?: boolean;
  onSongs(): void;
}) {
  const { client } = useSession();
  const theme = useKinoTheme();
  const insets = useSafeAreaInsets();
  const { width, fontScale } = useWindowDimensions();
  const searchInput = useRef<TextInput>(null);
  useEffect(() => {
    if (searchOpen) searchInput.current?.focus();
  }, [searchOpen]);
  const [query, setQuery] = useState('');
  const [mode, setMode] = useState<'albums' | 'artists'>('albums');
  const [artist, setArtist] = useState<string | null>(null);
  const [selection, setSelection] = useState<{
    client: KinosailClient;
    album: Album;
  } | null>(null);
  const selected = selection?.client === client ? selection.album : null;
  const [retry, setRetry] = useState(0);
  const [catalog, setCatalog] = useState<{
    client: KinosailClient;
    albums: Album[];
    error: string;
  } | null>(null);
  const [detail, setDetail] = useState<{
    client: KinosailClient;
    id: string;
    tracks: MediaItem[];
    error: string;
  } | null>(null);
  useEffect(() => {
    if (!client) return;
    const controller = new AbortController();
    client.loadAlbums(controller.signal).then(
      (albums) => {
        if (!controller.signal.aborted) {
          albumCatalogCache.write(client, 'albums', albums);
          setCatalog({ client, albums, error: '' });
        }
      },
      () => {
        if (!controller.signal.aborted) {
          albumCatalogCache.clear(client);
          setCatalog({ client, albums: [], error: 'Could not load albums.' });
        }
      },
    );
    return () => controller.abort();
  }, [client, retry]);
  useEffect(() => {
    if (!client || !selected) return;
    const controller = new AbortController();
    client.loadAlbum(selected.id, controller.signal).then(
      (album) => {
        if (!controller.signal.aborted) {
          albumTracksCache.write(client, selected.id, album.tracks);
          setDetail({
            client,
            id: selected.id,
            tracks: album.tracks,
            error: '',
          });
        }
      },
      () => {
        if (!controller.signal.aborted) {
          albumTracksCache.clear(client);
          setDetail({
            client,
            id: selected.id,
            tracks: [],
            error: 'Could not load album tracks.',
          });
        }
      },
    );
    return () => controller.abort();
  }, [client, selected, retry]);
  const savedAlbums = client ? albumCatalogCache.read(client, 'albums') : null;
  const currentCatalog =
    catalog?.client === client
      ? catalog
      : savedAlbums && client
        ? { client, albums: savedAlbums, error: '' }
        : null;
  const albums = currentCatalog?.albums ?? [];
  const savedTracks =
    client && selected ? albumTracksCache.read(client, selected.id) : null;
  const current =
    detail?.client === client && detail.id === selected?.id
      ? detail
      : savedTracks && client && selected
        ? { client, id: selected.id, tracks: savedTracks, error: '' }
        : null;
  const search = query.trim().toLocaleLowerCase();
  const filtered = useMemo(
    () =>
      albums
        .filter(
          (album) =>
            (artist === null || album.artist === artist) &&
            `${album.title} ${album.artist}`
              .toLocaleLowerCase()
              .includes(search),
        )
        .sort((a, b) => a.title.localeCompare(b.title)),
    [albums, artist, search],
  );
  const artists = useMemo(
    () =>
      [...new Set(albums.map((album) => album.artist))]
        .filter((name) => name.toLocaleLowerCase().includes(search))
        .sort((a, b) => a.localeCompare(b)),
    [albums, search],
  );
  const inset = Platform.isTV ? 64 : 20;
  const columns = Math.max(
    1,
    Math.floor(
      (width - inset * 2 + 16) /
        ((Platform.isTV ? 240 : 144 * Math.min(fontScale, 2)) + 16),
    ),
  );
  const size = (width - inset * 2 - (columns - 1) * 16) / columns;
  if (!client)
    return (
      <ScreenState message="Connect to your Kinosail Server to browse music." />
    );
  const error = selected ? current?.error : currentCatalog?.error;
  const loading = selected ? !current : !currentCatalog;
  const reload = () => {
    setCatalog(null);
    setDetail(null);
    setRetry((value) => value + 1);
  };
  const play = (id: string, shuffle = false) =>
    router.push({
      pathname: '/watch/[id]',
      params: {
        id,
        album: selected!.id,
        start: 'beginning',
        ...(shuffle ? { shuffle: '1' } : {}),
      },
    });
  const header = (
    <View style={{ gap: 16, paddingBottom: 24 }}>
      <View
        style={{
          flexDirection: 'row',
          flexWrap: 'wrap',
          alignItems: 'center',
          gap: 12,
        }}
      >
        {selected || artist !== null ? (
          <ActionButton
            label="Back"
            quiet
            onPress={() => (selected ? setSelection(null) : setArtist(null))}
          />
        ) : null}
        <Text
          accessibilityRole="header"
          style={{
            color: theme.text,
            fontSize: Platform.isTV ? 40 : 30,
            fontWeight: '700',
            flexShrink: 1,
          }}
        >
          {selected?.title || artist || 'Music'}
        </Text>
      </View>
      {selected ? (
        <>
          <Cover
            album={selected}
            client={client}
            size={Math.min(width - inset * 2, Platform.isTV ? 240 : 180)}
          />
          <Text style={{ color: theme.muted, fontSize: 18 }}>
            {selected.artist || 'Unknown artist'}
          </Text>
          <Text style={{ color: theme.muted }}>
            {current ? `${current.tracks.length} tracks` : 'Loading tracks…'}
          </Text>
          <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 12 }}>
            <ActionButton
              label="Play album"
              disabled={!current?.tracks.length}
              onPress={() => play(current!.tracks[0].id)}
            />
            <ActionButton
              label="Shuffle"
              quiet
              disabled={!current?.tracks.length}
              onPress={() => play(current!.tracks[0].id, true)}
            />
          </View>
        </>
      ) : (
        <>
          <TextInput
            ref={searchInput}
            accessibilityLabel="Search albums and artists"
            placeholder="Search albums and artists"
            placeholderTextColor={theme.muted}
            value={query}
            onChangeText={setQuery}
            maxLength={512}
            style={{
              color: theme.text,
              backgroundColor: theme.surface,
              borderColor: theme.line,
              borderWidth: 1,
              borderRadius: 12,
              minHeight: 48,
              padding: 12,
            }}
          />
          {artist === null ? (
            <View style={{ flexDirection: 'row', gap: 12 }}>
              <ActionButton
                label="Albums"
                selected={mode === 'albums'}
                quiet={mode !== 'albums'}
                onPress={() => setMode('albums')}
              />
              <ActionButton label="Songs" quiet onPress={onSongs} />
              <ActionButton
                label="Artists"
                selected={mode === 'artists'}
                quiet={mode !== 'artists'}
                onPress={() => setMode('artists')}
              />
            </View>
          ) : null}
        </>
      )}
      {error ? (
        <>
          <Text accessibilityRole="alert" style={{ color: theme.text }}>
            {error}
          </Text>
          <ActionButton label="Try again" onPress={reload} />
        </>
      ) : null}
    </View>
  );
  const contentStyle = {
    padding: inset,
    paddingBottom: Platform.isTV ? 64 : 160 + insets.bottom,
  };
  return (
    <SafeAreaView
      style={{ flex: 1, backgroundColor: theme.background }}
      edges={['top', 'left', 'right']}
    >
      {selected ? (
        <FlatList
          key={selected.id}
          data={current?.tracks ?? []}
          keyExtractor={(item) => item.id}
          contentContainerStyle={contentStyle}
          ListHeaderComponent={header}
          renderItem={({ item, index }) => (
            <MusicTrackRow
              item={item}
              number={index + 1}
              onPress={() => play(item.id)}
            />
          )}
          ListEmptyComponent={
            loading ? (
              <BrowseSkeleton
                variant="track"
                label="Loading tracks…"
                rowGap={0}
              />
            ) : !error ? (
              <Text style={{ color: theme.muted }}>
                This album has no playable tracks.
              </Text>
            ) : null
          }
        />
      ) : mode === 'artists' && artist === null ? (
        <FlatList
          key="artists"
          data={artists}
          keyExtractor={(name) => name}
          contentContainerStyle={contentStyle}
          ListHeaderComponent={header}
          renderItem={({ item }) => (
            <ActionButton
              label={item || 'Unknown artist'}
              quiet
              onPress={() => setArtist(item)}
            />
          )}
          ItemSeparatorComponent={() => <View style={{ height: 12 }} />}
          ListEmptyComponent={
            loading ? (
              <BrowseSkeleton
                variant="row"
                label="Loading music…"
                rowGap={12}
              />
            ) : (
              <Text style={{ color: theme.muted }}>
                {error ? '' : 'No artists found.'}
              </Text>
            )
          }
        />
      ) : (
        <FlatList
          key={`albums-${columns}`}
          data={filtered}
          numColumns={columns}
          keyExtractor={(item) => item.id}
          contentContainerStyle={contentStyle}
          ListHeaderComponent={header}
          columnWrapperStyle={columns > 1 ? { gap: 16 } : undefined}
          renderItem={({ item }) => (
            <MusicPressable
              accessibilityRole="button"
              accessibilityLabel={`${item.title}, ${item.artist || 'Unknown artist'}`}
              onPress={() => setSelection({ client, album: item })}
              style={({ pressed, focused }) => ({
                width: size,
                marginBottom: 24,
                gap: 8,
                opacity: pressed ? 0.7 : 1,
                borderRadius: 16,
                outlineWidth: focused ? 2 : 0,
                outlineColor: theme.focus,
              })}
            >
              <Cover album={item} client={client} size={size} />
              <Text
                numberOfLines={2}
                style={{
                  color: theme.text,
                  fontSize: Platform.isTV ? 28 : 16,
                  fontWeight: '600',
                }}
              >
                {item.title}
              </Text>
              <Text numberOfLines={1} style={{ color: theme.muted }}>
                {item.artist || 'Unknown artist'}
              </Text>
            </MusicPressable>
          )}
          ListEmptyComponent={
            loading ? (
              <BrowseSkeleton
                variant="album"
                label="Loading music…"
                width={size}
                columns={columns}
              />
            ) : (
              <Text style={{ color: theme.muted }}>
                {error
                  ? ''
                  : 'No albums found. Try another search or add tagged music to your Server.'}
              </Text>
            )
          }
        />
      )}
    </SafeAreaView>
  );
}
