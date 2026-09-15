import type {
  DownloadTrackOptions,
  DownloadTrackSelection,
} from '@/core/download-tracks';
import {
  selectDownloadEpisodes,
  type DownloadScope,
} from '@/core/download-batch';
import { downloadEstimate } from '@/core/download-estimate';
import { loadMediaPlayback } from '@/core/media-loader';
import type { KinosailClient } from '@/core/server-client';
import React, { useEffect, useMemo, useState } from 'react';
import { Text, View } from 'react-native';
import type { MediaItem } from '@/core/contract';
import type { DownloadQuality } from '@/core/download-quality';
import { useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';
import { ModalSheet } from './modal-sheet';
import { Skeleton } from './skeleton';

export function DownloadOptions({
  item,
  wifiOnly,
  episodes,
  episodesError,
  client,
  batchProgress,
  onStop,
  onDownload,
  onClose,
}: {
  item: MediaItem;
  wifiOnly: boolean;
  episodes?: MediaItem[];
  episodesError?: string;
  client?: KinosailClient;
  batchProgress?: string;
  onStop?: () => void;
  onDownload(
    quality: DownloadQuality,
    items?: MediaItem[],
    tracks?: DownloadTrackSelection,
  ): Promise<void>;
  onClose(): void;
}) {
  const theme = useKinoTheme();
  const [quality, setQuality] = useState<DownloadQuality>('original');
  const [busy, setBusy] = useState(false),
    [error, setError] = useState('');
  const options: DownloadQuality[] =
    item.kind === 'video'
      ? ['original', 'compatible', '1080p', '720p']
      : ['original'];
  const [trackOptions, setTrackOptions] = useState<DownloadTrackOptions>();
  const [tracks, setTracks] = useState<DownloadTrackSelection>();
  const [showTracks, setShowTracks] = useState(false);
  const [showDetails, setShowDetails] = useState(false);
  const [trackError, setTrackError] = useState('');
  useEffect(() => {
    if (!client || !showTracks || item.kind !== 'video') return;
    let active = true;
    void client
      .loadDownloadTracks(item.id)
      .then((options) => {
        if (active) {
          setTrackOptions(options);
          setTracks({
            audio: options.audio.map((track) => track.index),
            subtitles: options.subtitles.map((track) => track.index),
          });
          setTrackError('');
        }
      })
      .catch(() => {
        if (active)
          setTrackError(
            'Track choices are unavailable. All tracks will be included.',
          );
      });
    return () => {
      active = false;
    };
  }, [client, item.id, item.kind, showTracks]);
  const [scope, setScope] = useState<DownloadScope>('episode');
  const selected = useMemo(
    () => selectDownloadEpisodes(item, episodes ?? [], scope),
    [item, episodes, scope],
  );
  const [durations, setDurations] = useState<Record<string, number>>({});
  const [estimating, setEstimating] = useState(false);
  useEffect(() => {
    if (!client || quality === 'original') return;
    let active = true;
    setEstimating(true);
    const pending = selected.filter(
      (episode) => durations[episode.id] === undefined,
    );
    let cursor = 0;
    const worker = async () => {
      while (active && cursor < pending.length) {
        const episode = pending[cursor++];
        try {
          const source = await loadMediaPlayback(client, episode.id);
          if (active)
            setDurations((current) => ({
              ...current,
              [episode.id]: source.duration,
            }));
        } catch {
          // A missing estimate must not prevent downloading.
        }
      }
    };
    void Promise.all(
      Array.from({ length: Math.min(3, pending.length) }, worker),
    ).finally(() => {
      if (active) setEstimating(false);
    });
    return () => {
      active = false;
    };
    // The cache is filled by this effect; rerun only when the selection changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, selected, quality]);
  const estimate = downloadEstimate(selected, quality, durations);
  return (
    <ModalSheet
      title={`Download ${scope === 'episode' ? item.title : item.show}`}
      onClose={onClose}
      dismissLabel={busy ? 'Close' : 'Cancel'}
      footer={
        <View style={{ gap: 8 }}>
          <Text accessibilityLiveRegion="polite" style={{ color: theme.text }}>
            {estimate
              ? `${quality === 'original' ? 'Original size' : 'Estimated size'}: ${estimate}`
              : estimating
                ? 'Estimating size…'
                : 'Size unavailable until preparation'}
          </Text>
          <ActionButton
            label={
              scope === 'episode'
                ? 'Start download'
                : `Download ${selected.length} episodes`
            }
            busy={busy}
            onPress={() => {
              setBusy(true);
              setError('');
              void (
                scope === 'episode'
                  ? tracks && quality !== 'original'
                    ? onDownload(quality, undefined, tracks)
                    : onDownload(quality)
                  : onDownload(quality, selected)
              )
                .catch((reason) => {
                  setError(
                    reason instanceof Error
                      ? reason.message
                      : 'The download could not start.',
                  );
                })
                .finally(() => setBusy(false));
            }}
          />
        </View>
      }
    >
      <Text style={{ color: theme.muted }}>
        Network: {wifiOnly ? 'Wi-Fi only' : 'Wi-Fi + cellular'} · Change in
        Settings
      </Text>
      {item.showId ? (
        <>
          <Text style={{ color: theme.text }}>Episodes to download</Text>
          <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
            {(['episode', 'season', 'show'] as const).map((value) => (
              <ActionButton
                key={value}
                label={
                  value === 'episode'
                    ? 'This episode'
                    : value === 'season'
                      ? `Entire season ${item.season}`
                      : 'All episodes'
                }
                selected={scope === value}
                quiet={scope !== value}
                disabled={busy || (value !== 'episode' && !episodes?.length)}
                onPress={() => setScope(value)}
              />
            ))}
          </View>
          {!episodes && !episodesError ? (
            <Skeleton label="Loading episodes" />
          ) : null}
          {episodesError ? (
            <Text style={{ color: theme.muted }}>
              {episodesError || 'Loading episodes…'}
            </Text>
          ) : null}
          <Text style={{ color: theme.muted }}>
            {selected.length} {selected.length === 1 ? 'episode' : 'episodes'} ·
            Existing downloads are skipped
          </Text>
        </>
      ) : null}
      <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
        {options.map((option) => (
          <ActionButton
            key={option}
            label={
              option === 'original'
                ? 'Original'
                : option === 'compatible'
                  ? 'Compatible'
                  : option
            }
            selected={quality === option}
            quiet={quality !== option}
            disabled={busy}
            onPress={() => setQuality(option)}
          />
        ))}
      </View>
      <Text style={{ color: theme.muted, lineHeight: 22 }}>
        {quality === 'original'
          ? 'Keep the original file and its embedded audio and subtitles. No conversion needed; file sizes may be large.'
          : quality === 'compatible'
            ? 'A copy prepared for offline playback on this device. May take longer to prepare than Original.'
            : `${quality === '1080p' ? 'Higher detail' : 'More room for your trip'}. The Server prepares a copy up to ${quality}. It keeps the selected audio and subtitles; all tracks are included by default. Exact size appears after preparation.`}
      </Text>
      {quality !== 'original' && scope === 'episode' && client ? (
        <>
          <ActionButton
            label="Audio and subtitles"
            quiet
            selected={showTracks}
            disabled={busy}
            onPress={() => setShowTracks(!showTracks)}
          />
          {showTracks ? (
            <View style={{ gap: 12 }}>
              {trackError ? (
                <Text accessibilityRole="alert" style={{ color: theme.text }}>
                  {trackError}
                </Text>
              ) : null}
              {!trackOptions && !trackError ? (
                <Text style={{ color: theme.muted }}>Loading tracks…</Text>
              ) : null}
              {trackOptions && tracks
                ? (['audio', 'subtitles'] as const).map((kind) => (
                    <View key={kind} style={{ gap: 8 }}>
                      <Text style={{ color: theme.text }}>
                        {kind === 'audio' ? 'Audio' : 'Subtitles'}
                      </Text>
                      {trackOptions[kind].length ? (
                        trackOptions[kind].map((track) => (
                          <ActionButton
                            key={track.index}
                            label={track.label}
                            quiet
                            selected={tracks[kind].includes(track.index)}
                            disabled={busy}
                            onPress={() =>
                              setTracks({
                                ...tracks,
                                [kind]: tracks[kind].includes(track.index)
                                  ? tracks[kind].filter(
                                      (index) => index !== track.index,
                                    )
                                  : [...tracks[kind], track.index].sort(
                                      (a, b) => a - b,
                                    ),
                              })
                            }
                          />
                        ))
                      ) : (
                        <Text style={{ color: theme.muted }}>
                          No {kind} tracks
                        </Text>
                      )}
                    </View>
                  ))
                : null}
            </View>
          ) : null}
        </>
      ) : null}
      {quality !== 'original' ? (
        <>
          <Text style={{ color: theme.muted }}>
            Size is an estimate. Your Server prepares the copy before transfer.
          </Text>
          <ActionButton
            label="How downloads work"
            quiet
            expanded={showDetails}
            onPress={() => setShowDetails(!showDetails)}
          />
          {showDetails ? (
            <Text style={{ color: theme.muted, lineHeight: 22 }}>
              Compatible keeps supported H.264 video or converts it up to 1080p
              with AAC audio. The app starts the transfer when the copy is
              ready, verifies the file, and checks local playback. Background
              scheduling can affect when it finishes. You can check progress in
              Downloads.
            </Text>
          ) : null}
        </>
      ) : null}
      {error ? (
        <Text accessibilityRole="alert" style={{ color: theme.text }}>
          {error}
        </Text>
      ) : null}
      {batchProgress ? (
        <Text accessibilityLiveRegion="polite" style={{ color: theme.muted }}>
          {batchProgress}
        </Text>
      ) : null}
      {busy && scope !== 'episode' && onStop ? (
        <ActionButton label="Stop adding episodes" quiet onPress={onStop} />
      ) : null}
    </ModalSheet>
  );
}
