import { downloadsAvailable } from '@/core/downloads';
import { MusicLibrary } from '@/components/music-library';
import { router, useLocalSearchParams } from 'expo-router';
import React, { useEffect, useState } from 'react';
import { LibraryView } from '@/components/library-view';
import { ScreenState } from '@/components/screen-state';
import type { MediaItem, LibraryLetter } from '@/core/contract';
import { libraryCache } from '@/core/browse-cache';
import {
  libraryQuery,
  libraryViews,
  type LibraryQuery,
} from '@/core/library-query';
import { rememberMediaItems } from '@/core/media-loader';
import { useSession } from '@/core/session-context';
import type { KinosailClient } from '@/core/server-client';

type Result = {
  client: KinosailClient;
  query: LibraryQuery;
  items: MediaItem[];
  total: number;
  letters: LibraryLetter[];
  next: number;
  error: string;
};
export default function LibraryScreen() {
  const { view, search } = useLocalSearchParams();
  if (
    (view !== undefined &&
      (typeof view !== 'string' ||
        !libraryViews.includes(view as NonNullable<LibraryQuery['view']>))) ||
    (search !== undefined && search !== '1')
  ) {
    return (
      <ScreenState
        message="The requested library category is invalid."
        action="Go home"
        onAction={() => router.replace('/')}
      />
    );
  }
  if (view === 'music') return <MusicRoute searchOpen={search === '1'} />;
  const initialView = (view ?? 'all') as NonNullable<LibraryQuery['view']>;
  return (
    <LibraryContent
      key={initialView}
      initialView={initialView}
      searchOpen={search === '1'}
    />
  );
}
function MusicRoute({ searchOpen }: { searchOpen: boolean }) {
  const [songs, setSongs] = useState(false);
  return songs ? (
    <LibraryContent
      initialView="music"
      searchOpen={searchOpen}
      onAlbums={() => setSongs(false)}
    />
  ) : (
    <MusicLibrary searchOpen={searchOpen} onSongs={() => setSongs(true)} />
  );
}
function LibraryContent({
  initialView,
  searchOpen,
  onAlbums,
}: {
  initialView: NonNullable<LibraryQuery['view']>;
  searchOpen: boolean;
  onAlbums?: () => void;
}) {
  const { client } = useSession();
  const [query, setQuery] = useState<LibraryQuery>({
    view: initialView,
    sort: 'title',
  });
  const [result, setResult] = useState<Result | null>(null);
  const [retry, setRetry] = useState(0);
  let cacheKey: string | null = null;
  try {
    cacheKey = libraryQuery(query);
  } catch {
    // The request reports invalid input through the existing recovery state.
  }
  const saved = client && cacheKey ? libraryCache.read(client, cacheKey) : null;
  const current =
    result?.client === client && result.query === query
      ? result
      : saved && client
        ? {
            client,
            query,
            items: saved.items,
            total: saved.total,
            letters: saved.letters,
            next: saved.offset + saved.items.length,
            error: '',
          }
        : null;
  useEffect(() => {
    if (!client) return;
    const controller = new AbortController();
    // Cancel superseded requests and leave typing responsive during a remote search.
    const timer = setTimeout(
      () => {
        client.browseLibrary(query, controller.signal).then(
          (page) => {
            if (controller.signal.aborted) return;
            if (cacheKey) libraryCache.write(client, cacheKey, page);
            rememberMediaItems(client, page.items);
            setResult({
              client,
              query,
              items: page.items,
              total: page.total,
              letters: page.letters,
              next: page.offset + page.items.length,
              error: '',
            });
          },
          (error) => {
            if (!controller.signal.aborted) {
              libraryCache.clear(client);
              setResult({
                client,
                query,
                items: [],
                total: 0,
                letters: [],
                next: 0,
                error:
                  error instanceof Error
                    ? error.message
                    : 'Could not load the library.',
              });
            }
          },
        );
      },
      query.q ? 250 : 0,
    );
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [client, query, retry, cacheKey]);
  if (!client)
    return (
      <ScreenState
        message="Connect to your Kinosail Server to browse."
        action="Go home"
        onAction={() => router.replace('/')}
      />
    );
  const visible =
    current ?? (query.offset && result?.client === client ? result : null);
  return (
    <LibraryView
      onAlbums={onAlbums}
      onDownloads={
        downloadsAvailable ? () => router.push('/downloads') : undefined
      }
      searchOpen={searchOpen}
      onCloseSearch={() => router.setParams({ search: undefined })}
      items={visible?.items ?? []}
      total={visible?.total ?? 0}
      letters={visible?.letters ?? []}
      query={query}
      busy={!current}
      error={current?.error ?? ''}
      hasMore={Boolean(
        current &&
        !current.error &&
        current.items.length &&
        current.next < current.total,
      )}
      mediaURL={(path) => client.mediaURL(path)}
      headers={client.authorizationHeaders()}
      onChange={setQuery}
      onMore={() => current && setQuery({ ...query, offset: current.next })}
      onPrevious={() =>
        setQuery({ ...query, offset: Math.max(0, (query.offset ?? 0) - 60) })
      }
      onRetry={() => {
        setResult(null);
        setRetry((value) => value + 1);
      }}
      onOpen={(id) =>
        router.push({
          pathname:
            visible?.items.find((item) => item.id === id)?.kind === 'music'
              ? '/watch/[id]'
              : '/item/[id]',
          params: { id },
        })
      }
      onBack={() => router.back()}
    />
  );
}
