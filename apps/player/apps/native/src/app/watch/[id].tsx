import { musicID } from '@/core/music-catalog';
import { useMusicQueue } from '@/core/use-music-queue';
import { setPlayingDownload } from '@/core/smart-downloads';
import { usePlaybackExperience } from '@/core/use-playback-experience';
import {
  saveSyncedProgress,
  pendingProgress,
  flushProgress,
} from '@/core/progress-sync';
import { router, useLocalSearchParams } from 'expo-router';
import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';

import { downloadedPlayback, saveDownloadedProgress } from '@/core/downloads';
import { PlaybackView } from '@/components/playback-view';
import { ScreenState } from '@/components/screen-state';
import type { MediaItem, PlaybackSource, Progress } from '@/core/contract';
import type { KinosailClient } from '@/core/server-client';
import { useSession } from '@/core/session-context';

type Loaded = { item: MediaItem; source: PlaybackSource };

export default function WatchScreen() {
  const params = useLocalSearchParams();
  const [attempt, setAttempt] = useState(0);
  return (
    <WatchSession
      key={`${params.id}:${params.offline ?? ''}:${params.album ?? ''}:${params.shuffle ?? ''}:${attempt}`}
      onRetry={() => setAttempt((value) => value + 1)}
    />
  );
}

function WatchSession({ onRetry }: { onRetry(): void }) {
  const {
    id: rootID,
    offline,
    start,
    cast,
    album,
    shuffle,
  } = useLocalSearchParams<{
    id: string;
    offline?: string;
    start?: string;
    cast?: string;
    album?: string;
    shuffle?: string;
  }>();
  const { client } = useSession();
  const [selectedID, setSelectedID] = useState<string>();
  const id = selectedID ?? rootID;
  const returnToDetails = useCallback(() => {
    if (router.canGoBack()) router.back();
    else if (typeof id === 'string' && id)
      router.replace({ pathname: '/item/[id]', params: { id } });
    else router.replace('/');
  }, [id]);
  const selectTrack = useCallback((next: string) => setSelectedID(next), []);
  let invalidMusic = false;
  try {
    if (album !== undefined) {
      musicID(album);
      musicID(rootID);
    }
    invalidMusic =
      (shuffle !== undefined && shuffle !== '1') ||
      (shuffle !== undefined && album === undefined) ||
      (offline === '1' && (album !== undefined || shuffle !== undefined));
  } catch {
    invalidMusic = true;
  }
  const invalidStart =
    invalidMusic ||
    (start !== undefined && start !== 'beginning') ||
    (cast !== undefined && cast !== '1') ||
    (cast === '1' && offline === '1');
  useEffect(() => {
    setPlayingDownload(!invalidStart && typeof id === 'string' ? id : '');
    return () => setPlayingDownload('');
  }, [id, invalidStart]);
  const experience = usePlaybackExperience(
    invalidStart ? null : client,
    typeof id === 'string' ? id : '',
  );
  const baseline = useRef<Progress>({
    seconds: 0,
    watched: false,
    session: '',
    revision: 0,
  });
  const [progressMessage, setProgressMessage] = useState('');
  const startedAt = useMemo(
    () => performance.now(),
    [client, id, offline, start],
  );
  const [result, setResult] = useState<{
    client: KinosailClient;
    id: string;
    loaded: Loaded | null;
    start?: string;
    error: string;
  } | null>(null);
  const current =
    result?.client === client && result.id === id && result.start === start
      ? result
      : null;
  const loaded = current?.loaded;
  const error = current?.error;
  const music = loaded?.item.kind === 'music';
  const queue = useMusicQueue({
    client: invalidStart ? null : client,
    root: rootID,
    current: id,
    enabled:
      ((music && Boolean(album || loaded?.item.album)) ||
        selectedID !== undefined) &&
      offline !== '1',
    album,
    initialShuffle: shuffle === '1',
    onSelect: selectTrack,
  });

  useEffect(() => {
    let active = true;
    if (!client || typeof id !== 'string' || !id || invalidStart) return;
    setProgressMessage('');
    const activeClient = client;
    const itemID = id;
    const loading =
      offline === '1'
        ? downloadedPlayback(activeClient, itemID).then((value) => {
            if (!value) throw new Error('This title has not been downloaded.');
            return [value.item, value.source, value.baseline] as const;
          })
        : Promise.all([
            activeClient.loadItem(itemID),
            activeClient.loadPlayback(itemID),
          ]);
    Promise.all([loading, pendingProgress(activeClient)])
      .then(async ([[item, source, original], savedProgress]) => {
        if (!active) return;
        if (
          (album !== undefined || shuffle !== undefined) &&
          item.kind !== 'music'
        )
          throw new Error('Album playback requires a music track.');
        let pending = savedProgress.find((value) => value.id === itemID);
        if (pending && offline !== '1') {
          pending = (await flushProgress(activeClient, itemID)).find(
            (value) => value.id === itemID,
          );
          if (!active) return;
          if (!pending)
            [item, source] = await Promise.all([
              activeClient.loadItem(itemID),
              activeClient.loadPlayback(itemID),
            ]);
        }
        if (!active) return;
        baseline.current = original ?? item.progress;
        if (
          pending &&
          pending.progress.seconds >
            Math.max(source.start, item.progress.seconds)
        ) {
          source = { ...source, start: pending.progress.seconds };
          item = { ...item, progress: pending.progress };
          setProgressMessage('Resuming progress saved on this device.');
        }
        if (start === 'beginning' || item.kind === 'music') {
          source = { ...source, start: 0 };
          setProgressMessage('');
        }
        setResult({
          start,
          client: activeClient,
          id: itemID,
          loaded: { item, source },
          error: '',
        });
      })
      .catch(
        (reason) =>
          active &&
          setResult({
            start,
            client: activeClient,
            id: itemID,
            loaded: null,
            error:
              reason instanceof Error
                ? reason.message
                : 'Playback could not start.',
          }),
      );
    return () => {
      active = false;
    };
  }, [client, id, offline, start, album, shuffle, invalidStart]);

  const saveProgress = useMemo(() => {
    if (!client || !id) return null;
    return (
      progress: Pick<Progress, 'seconds' | 'session' | 'revision'> & {
        playbackToken?: string;
        watched?: boolean;
      },
    ) => {
      const save = async () => {
        const saved = await saveSyncedProgress(
          client,
          id,
          loaded?.item.title ?? '',
          baseline.current,
          progress,
        );
        baseline.current = saved;
        const pending = (await pendingProgress(client)).find(
          (value) => value.id === id,
        );
        if (offline === '1' && pending)
          await saveDownloadedProgress(client, id, pending.progress);
        setProgressMessage(
          pending
            ? 'Progress saved on this device. It will sync when connected.'
            : '',
        );
        return saved;
      };
      return save();
    };
  }, [client, id, offline, loaded?.item.title]);

  if (invalidStart)
    return (
      <ScreenState
        action="Return to details"
        message="This playback start option is not valid."
        onAction={returnToDetails}
      />
    );
  if (!saveProgress) {
    return (
      <ScreenState
        action="Return to details"
        message="This media item is not available."
        onAction={returnToDetails}
      />
    );
  }
  if (error)
    return (
      <ScreenState
        action="Try again"
        message={error}
        onAction={onRetry}
        secondaryAction="Return to details"
        onSecondaryAction={returnToDetails}
      />
    );
  if (!loaded || (music && shuffle === '1' && queue.loading))
    return (
      <ScreenState
        loading
        message="Preparing direct playback…"
        action="Return to details"
        actionQuiet
        onAction={returnToDetails}
      />
    );
  return (
    <PlaybackView
      musicQueue={
        music && offline !== '1' && Boolean(album || loaded.item.album)
          ? queue
          : undefined
      }
      castingClient={offline === '1' ? undefined : (client ?? undefined)}
      showTVPicker={cast === '1'}
      key={`${id}-${offline ?? ''}-${start ?? ''}`}
      onNext={
        loaded.source.details?.next
          ? () =>
              router.replace({
                pathname: '/watch/[id]',
                params: { id: loaded.source.details!.next },
              })
          : undefined
      }
      artworkURL={
        loaded.item.artwork && client
          ? client.mediaURL(loaded.item.artwork)
          : ''
      }
      artworkHeaders={client?.authorizationHeaders()}
      startedAt={startedAt}
      item={loaded.item}
      source={loaded.source}
      onBack={returnToDetails}
      saveProgress={saveProgress}
      experience={experience}
      progressMessage={progressMessage}
    />
  );
}
