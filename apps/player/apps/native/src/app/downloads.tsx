import { downloadBatch } from '@/core/download-batch';
import { DownloadOptions } from '@/components/download-options';
import type { MediaItem } from '@/core/contract';
import { router, useLocalSearchParams } from 'expo-router';
import React, { useEffect, useRef, useState } from 'react';
import { DownloadsView } from '@/components/downloads-view';
import { ScreenState } from '@/components/screen-state';
import {
  downloadMedia,
  checkDownloadedMedia,
  listDownloads,
  pauseDownload,
  removeDownload,
  downloadsAvailable,
  backgroundDownloadsAvailable,
  type DownloadEntry,
} from '@/core/downloads';
import { downloadProgressTracker } from '@/core/download-progress';
import { loadMediaItem } from '@/core/media-loader';
import { useSession } from '@/core/session-context';

export default function DownloadsScreen() {
  const { client } = useSession();
  const { id } = useLocalSearchParams<{ id?: string }>();
  const [entries, setEntries] = useState<DownloadEntry[]>([]),
    [error, setError] = useState('');
  const [selection, setSelection] = useState<{
    item: MediaItem;
    wifiOnly: boolean;
    episodes?: MediaItem[];
    episodesError?: string;
  } | null>(null);
  const batchCancelled = useRef(false);
  const addingBatch = useRef(false);
  const [batchProgress, setBatchProgress] = useState('');
  const [limitGiB, setLimitGiB] = useState<number>();
  const [progress, setProgress] = useState<Record<string, string>>({});
  useEffect(() => {
    if (!client || !downloadsAvailable) return;
    let active = true;
    setLimitGiB(undefined);
    const preferencesRequest = client.loadMediaPreferences();
    void preferencesRequest
      .then((preferences) => {
        if (active) setLimitGiB(preferences.downloadLimitGiB);
      })
      .catch(() => {});
    const measure = downloadProgressTracker();
    let refreshing = false;
    let nextPreparationPoll = 0;
    let preparationCursor = 0;
    let preparationError = '';
    const refresh = () => {
      if (refreshing) return;
      refreshing = true;
      void listDownloads(client)
        .then(
          async (value) => {
            if (active) {
              setEntries(value);
              setProgress(measure(value));
              const pending = value.filter(
                (entry) => entry.status === 'preparing',
              );
              const preparing = pending[preparationCursor % pending.length];
              if (
                !addingBatch.current &&
                preparing &&
                Date.now() >= nextPreparationPoll
              ) {
                nextPreparationPoll = Date.now() + 3000;
                preparationCursor++;
                try {
                  await downloadMedia(client, preparing.item, () => {});
                  if (active && preparationError) {
                    const previousError = preparationError;
                    setError((current) =>
                      current === previousError ? '' : current,
                    );
                    preparationError = '';
                  }
                } catch (reason) {
                  preparationError =
                    reason instanceof Error
                      ? reason.message
                      : 'Preparation could not be refreshed.';
                  if (active) setError(preparationError);
                }
              }
            }
          },
          () => {
            if (active) setError('Downloads could not be read.');
          },
        )
        .finally(() => {
          refreshing = false;
        });
    };
    const start = async () => {
      if (typeof id === 'string' && id) {
        try {
          const [item, preferences] = await Promise.all([
            loadMediaItem(client, id),
            preferencesRequest,
          ]);
          if (active) {
            setSelection({ item, wifiOnly: preferences.wifiOnly });
            if (item.showId) {
              void client
                .loadShowEpisodes(item.showId)
                .then((episodes) => {
                  if (!episodes.some((episode) => episode.id === item.id))
                    throw new Error('This episode is no longer in the show.');
                  if (active)
                    setSelection((current) =>
                      current?.item.id === item.id
                        ? { ...current, episodes }
                        : current,
                    );
                })
                .catch(() => {
                  if (active)
                    setSelection((current) =>
                      current?.item.id === item.id
                        ? {
                            ...current,
                            episodesError:
                              'Could not load the show. Close and try again to download a season or all episodes.',
                          }
                        : current,
                    );
                });
            }
          }
        } catch (reason) {
          if (active)
            setError(
              reason instanceof Error
                ? reason.message
                : 'The download could not start.',
            );
        }
      }
      refresh();
    };
    refresh();
    void start();
    const timer = setInterval(refresh, 1000);
    return () => {
      active = false;
      batchCancelled.current = true;
      clearInterval(timer);
    };
  }, [client, id]);
  if (!client || !downloadsAvailable)
    return (
      <ScreenState
        message="Downloads are available in the signed-in mobile app."
        action="Back"
        onAction={() => router.back()}
      />
    );
  const refresh = () => {
    void listDownloads(client).then(setEntries, () =>
      setError('Downloads could not be read.'),
    );
  };
  const action = async (operation: () => Promise<void>) => {
    setError('');
    try {
      await operation();
      refresh();
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : 'Download could not be updated.',
      );
      throw reason;
    }
  };
  const closeOptions = () => {
    batchCancelled.current = true;
    setSelection(null);
    setBatchProgress('');
    router.setParams({ id: '' });
  };
  return (
    <>
      <DownloadsView
        entries={entries}
        progress={progress}
        limitGiB={limitGiB}
        error={error}
        background={backgroundDownloadsAvailable}
        onCheck={() =>
          action(async () => {
            for (const entry of entries.filter(
              (value) => value.status === 'complete',
            ))
              await checkDownloadedMedia(client, entry.item.id);
            await refresh();
          })
        }
        onPause={(entry) => action(() => pauseDownload(client, entry.item.id))}
        onResume={(entry) =>
          action(() => downloadMedia(client, entry.item, refresh))
        }
        onRemove={(entry) =>
          action(() => removeDownload(client, entry.item.id))
        }
        onPlay={(entry) =>
          router.push({
            pathname: '/watch/[id]',
            params: { id: entry.item.id, offline: '1' },
          })
        }
        onBrowse={() =>
          router.push({ pathname: '/library', params: { search: '1' } })
        }
      />
      {selection ? (
        <DownloadOptions
          key={selection.item.id}
          item={selection.item}
          wifiOnly={selection.wifiOnly}
          episodes={selection.episodes}
          episodesError={selection.episodesError}
          client={client}
          batchProgress={batchProgress}
          onStop={() => {
            batchCancelled.current = true;
          }}
          onClose={closeOptions}
          onDownload={async (quality, items, tracks) => {
            batchCancelled.current = false;
            addingBatch.current = true;
            try {
              await action(() =>
                items
                  ? downloadBatch(
                      client,
                      items,
                      quality,
                      refresh,
                      (completed, total) =>
                        setBatchProgress(
                          `Added ${completed} of ${total} episodes. Keep Downloads open while episodes are added.`,
                        ),
                      () => batchCancelled.current,
                    )
                  : downloadMedia(
                      client,
                      selection.item,
                      refresh,
                      quality,
                      tracks,
                    ),
              );
              if (!batchCancelled.current) closeOptions();
            } finally {
              addingBatch.current = false;
              setBatchProgress('');
            }
          }}
        />
      ) : null}
    </>
  );
}
