import { router, useLocalSearchParams } from 'expo-router';
import React, { useEffect, useState } from 'react';
import { FlatList, Text, TextInput } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { ActionButton } from '@/components/action-button';
import { LibraryView } from '@/components/library-view';
import { BrowseSkeleton } from '@/components/browse-skeleton';
import { collectionsCache } from '@/core/browse-cache';
import { rememberMediaItems } from '@/core/media-loader';
import { ScreenState } from '@/components/screen-state';
import { collectionName } from '@/core/collections';
import type { MediaItem } from '@/core/contract';
import type { KinosailClient } from '@/core/server-client';
import { useSession } from '@/core/session-context';
import { useKinoTheme } from '@/design/tokens';

export default function CollectionsScreen() {
  const { name } = useLocalSearchParams();
  let valid = name === undefined;
  try {
    if (name !== undefined) {
      collectionName(name);
      valid = true;
    }
  } catch {
    valid = false;
  }
  if (!valid)
    return (
      <ScreenState
        message="This collection is invalid."
        action="Collections"
        onAction={() => router.replace('/collections')}
      />
    );
  return (
    <CollectionsContent
      key={(name as string | undefined) ?? ''}
      name={name as string | undefined}
    />
  );
}
function CollectionsContent({ name }: { name?: string }) {
  const { client } = useSession();
  const theme = useKinoTheme();
  const [query, setQuery] = useState('');
  const [offset, setOffset] = useState(0);
  const [retry, setRetry] = useState(0);
  const [result, setResult] = useState<{
    client: KinosailClient;
    names: string[];
    items: MediaItem[];
    error: string;
  } | null>(null);
  const saved = client ? collectionsCache.read(client, name ?? '') : null;
  const current =
    result?.client === client
      ? result
      : saved && client
        ? { client, ...saved, error: '' }
        : null;
  const visibleItems = current?.items;
  useEffect(() => {
    if (client && visibleItems)
      rememberMediaItems(client, visibleItems.slice(offset, offset + 60));
  }, [client, visibleItems, offset]);
  const back = () =>
    name ? router.replace('/collections') : router.replace('/');
  useEffect(() => {
    if (!client) return;
    setOffset(0);
    const controller = new AbortController();
    const request =
      name === undefined
        ? client
            .loadCollections(controller.signal)
            .then((names) => ({ names, items: [] as MediaItem[] }))
        : client
            .loadCollection(name, controller.signal)
            .then((items) => ({ items, names: [] as string[] }));
    request.then(
      (data) => {
        if (!controller.signal.aborted) {
          collectionsCache.write(client, name ?? '', data);
          setResult({ client, ...data, error: '' });
        }
      },
      (error) => {
        if (!controller.signal.aborted) {
          collectionsCache.clear(client);
          setResult({
            client,
            names: [],
            items: [],
            error:
              error instanceof Error
                ? error.message
                : 'Could not load collections.',
          });
        }
      },
    );
    return () => controller.abort();
  }, [client, name, retry]);
  if (!client)
    return (
      <ScreenState
        message="Connect to your Kinosail Server to browse collections."
        action="Go home"
        onAction={() => router.replace('/')}
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
        secondaryAction="Back"
        onSecondaryAction={back}
      />
    );
  if (name !== undefined)
    return (
      <LibraryView
        collection
        title={name}
        items={current?.items.slice(offset, offset + 60) ?? []}
        total={current?.items.length ?? 0}
        query={{ offset }}
        busy={!current}
        error=""
        hasMore={offset + 60 < (current?.items.length ?? 0)}
        headers={client.authorizationHeaders()}
        mediaURL={(path) => client.mediaURL(path)}
        onChange={() => {}}
        onMore={() => setOffset(offset + 60)}
        onPrevious={() => setOffset(Math.max(0, offset - 60))}
        onRetry={() => setRetry((value) => value + 1)}
        onBack={back}
        onOpen={(id) => router.push({ pathname: '/item/[id]', params: { id } })}
      />
    );
  const names = (current?.names ?? []).filter((value) =>
    value.toLowerCase().includes(query.trim().toLowerCase()),
  );
  return (
    <SafeAreaView
      style={{
        flex: 1,
        backgroundColor: theme.background,
        padding: 20,
        gap: 16,
      }}
    >
      <Text
        accessibilityRole="header"
        style={{ color: theme.text, fontSize: 28, fontWeight: '700' }}
      >
        Collections
      </Text>
      <TextInput
        accessibilityLabel="Search collections"
        value={query}
        onChangeText={setQuery}
        maxLength={64}
        placeholder="Search collections"
        placeholderTextColor={theme.muted}
        style={{
          color: theme.text,
          borderColor: theme.line,
          borderWidth: 1,
          borderRadius: 8,
          padding: 12,
          minHeight: 48,
        }}
      />
      <FlatList
        data={names}
        keyExtractor={(value) => value}
        contentContainerStyle={{ gap: 12 }}
        renderItem={({ item }) => (
          <ActionButton
            label={item}
            quiet
            onPress={() =>
              router.push({ pathname: '/collections', params: { name: item } })
            }
          />
        )}
        ListEmptyComponent={
          !current ? (
            <BrowseSkeleton
              variant="row"
              label="Loading collections…"
              rowGap={12}
            />
          ) : (
            <Text style={{ color: theme.muted }}>
              {query ? 'No matching collections.' : 'No collections yet.'}
            </Text>
          )
        }
      />
    </SafeAreaView>
  );
}
