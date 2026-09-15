import { router, useLocalSearchParams } from 'expo-router';
import React, { useEffect, useState } from 'react';

import { downloadsAvailable } from '@/core/downloads';
import { PhotoView } from '@/components/photo-view';
import { DetailView } from '@/components/detail-view';
import { ScreenState } from '@/components/screen-state';
import type { MediaItem } from '@/core/contract';
import {
  loadMediaItem,
  loadMediaPlayback,
  readMediaItem,
} from '@/core/media-loader';
import type { KinosailClient } from '@/core/server-client';
import { useSession } from '@/core/session-context';

const goBack = () => {
  if (router.canGoBack()) router.back();
  else router.replace('/');
};

export default function ItemScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { client } = useSession();
  const [result, setResult] = useState<{
    client: KinosailClient;
    id: string;
    item: MediaItem | null;
    error: string;
  } | null>(null);
  const current = result?.client === client && result.id === id ? result : null;
  const item = current
    ? current.item
    : client
      ? readMediaItem(client, id)
      : null;
  const error = current?.error;
  const [readerError, setReaderError] = useState('');
  const openReader = () => {
    if (!client || !item) return;
    setReaderError('');
    router.push({ pathname: '/read/[id]', params: { id: item.id } });
  };

  useEffect(() => {
    setReaderError('');
    let active = true;
    let preload = 0;
    if (!client || !id) return;
    const activeClient = client;
    const itemID = id;
    loadMediaItem(activeClient, itemID).then(
      (loaded) => {
        if (!active) return;
        setResult({
          client: activeClient,
          id: itemID,
          item: loaded,
          error: '',
        });
        if (!['book', 'photo'].includes(loaded.kind))
          preload = requestAnimationFrame(() => {
            if (active)
              void loadMediaPlayback(activeClient, itemID).catch(() => {});
          });
      },
      (reason) =>
        active &&
        setResult({
          client: activeClient,
          id: itemID,
          item: null,
          error:
            reason instanceof Error
              ? reason.message
              : 'Could not load this title.',
        }),
    );
    return () => {
      active = false;
      if (preload) cancelAnimationFrame(preload);
    };
  }, [client, id]);

  if (!client || !id) {
    return (
      <ScreenState
        action="Go back"
        message="This media item is not available."
        onAction={goBack}
      />
    );
  }
  if (readerError)
    return (
      <ScreenState
        action="Try again"
        message={readerError}
        onAction={openReader}
        secondaryAction="Go back"
        onSecondaryAction={goBack}
      />
    );
  if (error)
    return <ScreenState action="Go back" message={error} onAction={goBack} />;
  if (!item)
    return (
      <ScreenState
        loading
        message="Loading details…"
        action="Go back"
        actionQuiet
        onAction={goBack}
      />
    );
  if (item.kind === 'photo')
    return (
      <PhotoView
        key={`${client.baseURL}:${item.id}`}
        item={item}
        uri={client.mediaURL(`/media/${encodeURIComponent(item.id)}`)}
        headers={client.authorizationHeaders()}
        onBack={goBack}
      />
    );
  return (
    <DetailView
      onPlayOnTV={() =>
        router.push({
          pathname: '/watch/[id]',
          params: { id: item.id, cast: '1' },
        })
      }
      headers={client.authorizationHeaders()}
      item={item}
      mediaURL={(path) => client.mediaURL(path)}
      onDownload={
        downloadsAvailable && item.kind !== 'book'
          ? () => {
              router.push({ pathname: '/downloads', params: { id: item.id } });
            }
          : undefined
      }
      onShow={
        item.showId
          ? () =>
              router.push({
                pathname: '/show/[id]',
                params: { id: item.showId! },
              })
          : undefined
      }
      onBack={goBack}
      onPlayFromBeginning={
        item.kind !== 'book'
          ? () =>
              router.push({
                pathname: '/watch/[id]',
                params: { id: item.id, start: 'beginning' },
              })
          : undefined
      }
      onPlay={() =>
        item.kind === 'book'
          ? openReader()
          : router.push({ pathname: '/watch/[id]', params: { id: item.id } })
      }
    />
  );
}
