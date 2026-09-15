import type { MusicQueue } from '@/core/use-music-queue';
import { prioritizePlayback } from '@/core/verified-transfers';
import { matchesPlaybackLanguage } from '@/core/media-preferences';
import { randomUUID } from 'expo-crypto';
import type { PlaybackExperience } from '@/core/use-playback-experience';
import { PlaybackExperienceOptions } from './playback-experience-options';
import { VideoView, useVideoPlayer } from 'expo-video';
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Alert,
  StatusBar,
  ScrollView,
  AppState,
  Platform,
  Pressable,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { MediaItem, PlaybackSource, Progress } from '@/core/contract';
import { createProgressWriter } from '@/core/progress-writer';
import { measurePlayback } from '@/core/playback-metrics';
import {
  compatiblePlaybackSource,
  nativeContentType,
} from '@/core/video-source';
import {
  ArtworkThemeContext,
  palette,
  radius,
  spacing,
  useKinoTheme,
} from '@/design/tokens';

import { ActionButton } from './action-button';
import { PlayOnTV } from './play-on-tv';
import { TVPlaybackView } from './tv-playback-view';
import type { CastingClient, TVPlayback } from '@/core/casting';
import { PlaybackOptions } from './playback-options';
import { PlaybackSeekBar } from './playback-seek-bar';
import { usePlaybackRemote } from './tv-playback-events';
import { useTVPlaybackChrome } from './use-tv-playback-chrome';
import { CompatibilityPlayer } from './compatibility-player';
import { localCompatibilityAvailable } from '@/core/protected-media';
import { Skeleton } from './skeleton';
import { ScreenState } from './screen-state';
import { playbackFailure, preferLocalPlayback } from '@/core/playback-engine';

type ProgressInput = Pick<Progress, 'seconds' | 'session' | 'revision'> & {
  playbackToken?: string;
  watched?: boolean;
};

type Props = {
  musicQueue?: MusicQueue;
  castingClient?: CastingClient;
  showTVPicker?: boolean;
  artworkURL?: string;
  artworkHeaders?: Record<string, string>;
  experience: PlaybackExperience;
  progressMessage: string;
  startedAt?: number;
  item: MediaItem;
  source: PlaybackSource;
  onBack(): void;
  onNext?: () => void;
  saveProgress(progress: ProgressInput): Promise<Progress>;
};

export function PlaybackView(props: Props) {
  if (!props.experience.loaded)
    return (
      <ScreenState
        loading
        message="Loading playback preferences…"
        action="Return to details"
        actionQuiet
        onAction={props.onBack}
      />
    );
  return Platform.isTV ? (
    <ArtworkThemeContext.Provider value={palette.dark}>
      <PlaybackReady {...props} />
    </ArtworkThemeContext.Provider>
  ) : (
    <PlaybackReady {...props} />
  );
}

function PlaybackReady(props: Props) {
  useEffect(() => prioritizePlayback(), []);
  const nativeVideo =
    Platform.OS === 'ios' && !Platform.isTV && props.item.kind === 'video';
  const [initialLocal] = useState(() =>
    preferLocalPlayback(
      props.item,
      props.source,
      props.experience.preferences,
      localCompatibilityAvailable && !nativeVideo,
    ),
  );
  const [tv, setTV] = useState<TVPlayback | null>(null);
  const [airPlayRequested, setAirPlayRequested] = useState(false);
  const [recovered, setRecovered] = useState<PlaybackSource | null>(null);
  const [local, setLocal] = useState<PlaybackSource | null>(null);
  const source = recovered ?? props.source;
  const recover = useCallback(
    (position: number) => {
      const next = compatiblePlaybackSource(source, position);
      if (next) {
        setLocal(null);
        setRecovered(next);
      }
    },
    [source],
  );
  const openLocal = useCallback(
    (position: number) => setLocal({ ...source, start: position }),
    [source],
  );
  if (tv && props.castingClient)
    return (
      <TVPlaybackView
        playback={tv}
        client={props.castingClient}
        item={props.item}
        saveProgress={props.saveProgress}
        onDone={props.onBack}
      />
    );
  const needsLocal = initialLocal && !recovered;
  if (local || needsLocal)
    return (
      <CompatibilityPlayer
        {...props}
        onPrepareAirPlay={(position) => {
          setAirPlayRequested(true);
          recover(position);
        }}
        onTVPlayback={setTV}
        source={local ?? source}
        onRecover={recover}
      />
    );
  return (
    <PlaybackSession
      {...props}
      showTVPicker={props.showTVPicker || airPlayRequested}
      onTVPlayback={setTV}
      source={source}
      recover={recover}
      onFailure={
        localCompatibilityAvailable &&
        !nativeVideo &&
        !recovered &&
        source.plan.mode === 'direct' &&
        nativeContentType(source.contentType) === 'progressive'
          ? openLocal
          : undefined
      }
    />
  );
}

function PlaybackSession({
  castingClient,
  showTVPicker,
  onTVPlayback,
  experience,
  progressMessage,
  item,
  source,
  onBack,
  onNext,
  saveProgress,
  recover,
  startedAt,
  onFailure,
}: Props & {
  onTVPlayback(playback: TVPlayback): void;
  recover(position: number): void;
  onFailure?: (position: number) => void;
}) {
  const theme = useKinoTheme();
  const nativeOnly =
    Platform.OS === 'ios' &&
    !Platform.isTV &&
    item.kind === 'video' &&
    !showTVPicker;
  const transferred = useRef(false);
  const recovering = useRef(false);
  const firstFrame = useRef(false);
  const progressWriter = useRef<ReturnType<typeof createProgressWriter> | null>(
    null,
  );
  const [error, setError] = useState('');
  const [localRecovery, setLocalRecovery] = useState(false);
  const [options, setOptions] = useState(false);
  const [ended, setEnded] = useState(false);
  const [ready, setReady] = useState(false);
  const [buffering, setBuffering] = useState(false);
  const [paused, setPaused] = useState(false);
  const chrome = useTVPlaybackChrome(
    Platform.isTV,
    paused || ended || options || !ready || buffering || Boolean(error),
    options,
    () => setOptions(false),
  );
  const videoView = useRef<VideoView>(null);
  const presented = useRef(false);
  const [fullscreen, setFullscreen] = useState(false);
  const pictureInPicture = useRef(false);
  const [airPlay, setAirPlay] = useState(false);
  const position = useRef(source.start);
  const [displayPosition, setDisplayPosition] = useState(source.start);
  const [sleep, setSleep] = useState<{
    deadline: number;
    position: number;
    label: string;
  }>({ deadline: 0, position: 0, label: '' });
  const repeat = useRef<{
    until: number;
    started: boolean;
    track: typeof player.subtitleTrack;
  } | null>(null);
  const completion = useRef(false);
  const pendingSeek = useRef<number | null>(null);
  const measurement = useRef<ReturnType<typeof measurePlayback> | null>(null);
  const session = useMemo(() => `native-${randomUUID()}`, [source.uri]);
  const seek = (seconds: number) => {
    if (!Number.isFinite(seconds) || seconds < 0 || seconds > 31_536_000)
      return;
    const target = Math.min(source.duration || 31_536_000, seconds);
    measurement.current?.seek(target);
    pendingSeek.current = target;
    setDisplayPosition(target);
    player.currentTime = target;
  };
  const contentType = nativeContentType(source.contentType);
  const player = useVideoPlayer(
    {
      uri: source.uri,
      headers: source.headers,
      contentType,
      metadata: { title: item.title },
      // The disk cache keys by URL, not by the authorized viewer.
      useCaching: false,
    },
    (instance) => {
      instance.currentTime = source.start;
      instance.timeUpdateEventInterval = 1;
      instance.playbackRate = experience.preferences.rate;
      instance.staysActiveInBackground = true;
      if (Platform.OS === 'ios') instance.allowsExternalPlayback = true;
      instance.play();
    },
  );

  // Keep AVKit's presented view alive while replacing a failed source. A new
  // fullscreen view cannot present over the old controller during recovery.
  useEffect(() => {
    recovering.current = false;
    firstFrame.current = false;
    position.current = source.start;
    pendingSeek.current = null;
    completion.current = false;
    repeat.current = null;
    setDisplayPosition(source.start);
    setError('');
    setLocalRecovery(false);
    setReady(false);
    setBuffering(false);
    setEnded(false);
  }, [player, source.uri, source.start]);

  useEffect(() => {
    if (
      item.kind !== 'video' ||
      !source.compatible?.plan.allowed ||
      !['remux', 'audio-transcode', 'transcode'].includes(
        source.compatible.plan.mode,
      )
    )
      return;
    let previous = player.currentTime;
    let withoutPicture = 0;
    const timer = setInterval(() => {
      const current = player.currentTime;
      // Audio can advance successfully even when the video cannot render.
      // Background playback, remote playback and PiP need no inline picture.
      const advancing = Number.isFinite(current) && current > previous;
      previous = current;
      if (firstFrame.current || recovering.current) return;
      if (
        !advancing ||
        !player.playing ||
        AppState.currentState !== 'active' ||
        player.isExternalPlaybackActive ||
        pictureInPicture.current
      ) {
        withoutPicture = 0;
        return;
      }
      if (++withoutPicture < 8) return;
      recovering.current = true;
      recover(pendingSeek.current ?? current);
    }, 1000);
    return () => clearInterval(timer);
  }, [item.kind, player, source.compatible, recover]);

  const toggleTVPlayback = () => {
    if (ended) {
      player.currentTime = 0;
      setEnded(false);
      player.play();
    } else if (player.playing) player.pause();
    else player.play();
  };
  usePlaybackRemote(
    toggleTVPlayback,
    Platform.isTV ? chrome.reveal : undefined,
  );
  useEffect(() => {
    if (!Platform.isTV) return;
    const listener = player.addListener('playingChange', ({ isPlaying }) =>
      setPaused(!isPlaying),
    );
    return () => listener.remove();
  }, [player]);

  useEffect(() => {
    if (Platform.OS !== 'ios' || Platform.isTV) return;
    setAirPlay(player.isExternalPlaybackActive);
    const listener = player.addListener(
      'isExternalPlaybackActiveChange',
      (event) => {
        setAirPlay(event.isExternalPlaybackActive);
      },
    );
    return () => listener.remove();
  }, [player]);

  useEffect(() => {
    const prefs = experience.preferences;
    if (
      localCompatibilityAvailable &&
      (prefs.nightMode || prefs.dialogueBoost || prefs.volumeBoost > 1)
    )
      onFailure?.(position.current);
  }, [
    experience.preferences.nightMode,
    experience.preferences.dialogueBoost,
    experience.preferences.volumeBoost,
  ]);
  useEffect(() => {
    player.playbackRate = experience.preferences.rate;
  }, [player, experience.preferences.rate]);
  useEffect(() => {
    const apply = () => {
      const prefs = experience.preferences;
      const audio = player.availableAudioTracks.find(
        (track) =>
          (prefs.audioTrack && track.label === prefs.audioTrack) ||
          matchesPlaybackLanguage(prefs.audioLanguage, track.language),
      );
      const subtitle = player.availableSubtitleTracks.find(
        (track) =>
          (prefs.subtitleTrack && track.label === prefs.subtitleTrack) ||
          matchesPlaybackLanguage(prefs.subtitleLanguage, track.language),
      );
      if (audio) player.audioTrack = audio;
      if (prefs.subtitleLanguage === 'off') player.subtitleTrack = null;
      else if (subtitle) player.subtitleTrack = subtitle;
    };
    apply();
    const listener = player.addListener('sourceLoad', apply);
    return () => listener.remove();
  }, [
    player,
    experience.preferences.audioTrack,
    experience.preferences.subtitleTrack,
    experience.preferences.audioLanguage,
    experience.preferences.subtitleLanguage,
  ]);
  useEffect(() => {
    const timer = setInterval(() => {
      if (
        (sleep.deadline && Date.now() >= sleep.deadline) ||
        (sleep.position && player.currentTime >= sleep.position)
      ) {
        player.pause();
        setSleep({ deadline: 0, position: 0, label: '' });
      }
    }, 500);
    return () => clearInterval(timer);
  }, [player, sleep]);
  useEffect(() => {
    const metrics = measurePlayback('platform', startedAt ?? performance.now());
    measurement.current = metrics;
    const progress = createProgressWriter(
      {
        session,
        revision: item.progress.revision,
        playbackToken: source.progressToken,
      },
      saveProgress,
    );

    progressWriter.current = progress;
    let playable = player.status === 'readyToPlay';
    let hasProgress = false;
    const time = player.addListener('timeUpdate', ({ currentTime }) => {
      if (
        transferred.current ||
        !playable ||
        !Number.isFinite(currentTime) ||
        currentTime < 0
      )
        return;
      pendingSeek.current = null;
      metrics.position(currentTime);
      hasProgress = true;
      if (repeat.current && currentTime < repeat.current.until - 0.5)
        repeat.current.started = true;
      if (repeat.current?.started && currentTime >= repeat.current.until) {
        player.subtitleTrack = repeat.current.track;
        repeat.current = null;
      }
      if (completion.current && currentTime < source.duration - 2) {
        completion.current = false;
        setEnded(false);
        progress.write(currentTime, false);
      }
      setDisplayPosition(currentTime);
      position.current = currentTime;
      progress.write(currentTime);
    });
    const end = player.addListener('playToEnd', () => {
      hasProgress = true;
      metrics.finish('ended');
      completion.current = true;
      setEnded(true);
      position.current = source.duration;
      progress.write(source.duration, true);
    });
    const handleStatus = (
      event: { status: string; error?: unknown },
      allowRecovery = true,
    ) => {
      playable = event.status === 'readyToPlay';
      setBuffering(event.status === 'loading');
      if (event.status === 'error') {
        if (recovering.current) return;
        metrics.finish('error');
        const failure = playbackFailure(event.error);
        const compatible = source.compatible;
        const resume = pendingSeek.current ?? position.current;
        // Try each offered rendition once; compatiblePlaybackSource consumes
        // the fallback, including video conversion when needed.
        if (failure.decode && onFailure) {
          recovering.current = true;
          onFailure(resume);
          return;
        }
        if (
          allowRecovery &&
          failure.local &&
          compatible?.plan.allowed &&
          ['remux', 'audio-transcode', 'transcode'].includes(
            compatible.plan.mode,
          )
        ) {
          recovering.current = true;
          recover(resume);
          return;
        }
        setError(failure.message);
        setLocalRecovery(failure.local);
        if (Platform.OS === 'ios' && !Platform.isTV && !nativeOnly)
          void videoView.current?.exitFullscreen().catch(() => {});
      }
      // Native playback becomes non-playing while it waits for bytes. Gating
      // on player.playing loses precisely the stalls this measures.
      metrics.buffering(event.status === 'loading');
    };
    const status = player.addListener('statusChange', handleStatus);
    // Loading starts in the native constructor, before effects subscribe.
    // A failure in that interval must still recover or show a return action.
    // The status snapshot has no error details to justify automatic recovery.
    handleStatus({ status: player.status }, false);
    const appState = AppState.addEventListener('change', (state) => {
      if (state !== 'active' && hasProgress && !transferred.current)
        progress.write(position.current);
    });
    return () => {
      metrics.finish('closed');
      time.remove();
      end.remove();
      status.remove();
      appState.remove();
      if (hasProgress && !transferred.current) progress.write(position.current);
    };
  }, [
    item.progress.revision,
    player,
    saveProgress,
    session,
    source.duration,
    source.progressToken,
    source.compatible,
    recover,
    startedAt,
    onFailure,
    nativeOnly,
  ]);

  useEffect(() => {
    if (!nativeOnly || !error) return;
    Alert.alert('Playback stopped', error, [
      ...(source.compatible && localRecovery
        ? [
            {
              text:
                source.compatible.plan.mode === 'transcode'
                  ? 'Convert video and play'
                  : 'Try compatible playback',
              onPress: () => recover(position.current),
            },
          ]
        : []),
      { text: 'Return to details', onPress: onBack },
    ]);
  }, [nativeOnly, error, localRecovery, source.compatible, recover, onBack]);

  const back = (
    <ActionButton
      label="Back"
      onPress={onBack}
      quiet
      style={
        !Platform.isTV
          ? { ...styles.back, borderRadius: 24, minHeight: 44 }
          : styles.back
      }
    />
  );
  const navigation = (
    <>
      {!error && (!Platform.isTV || chrome.visible) ? (
        <SafeAreaView
          edges={
            Platform.isTV
              ? ['top', 'left', 'right']
              : ['bottom', 'left', 'right']
          }
          pointerEvents="box-none"
          style={[
            styles.navigation,
            Platform.isTV && {
              position: 'absolute',
              bottom: 40,
              left: 64,
              right: 64,
              zIndex: 2,
              maxHeight: '85%',
              backgroundColor: '#000000E8',
            },
            !Platform.isTV && { flexDirection: 'column-reverse' },
          ]}
        >
          {Platform.isTV ? (
            <>
              <Text
                numberOfLines={1}
                style={{ color: theme.text, fontSize: 28 }}
              >
                {item.title}
              </Text>
              <PlaybackSeekBar
                seconds={displayPosition}
                duration={source.duration}
                onSeek={seek}
              />
            </>
          ) : null}
          <View
            onFocus={Platform.isTV ? chrome.reveal : undefined}
            style={styles.controls}
          >
            {Platform.isTV ? (
              <>
                <ActionButton
                  label="Back 10 seconds"
                  quiet
                  disabled={!ready}
                  onPress={() =>
                    seek(
                      Math.max(
                        0,
                        (pendingSeek.current ?? position.current) - 10,
                      ),
                    )
                  }
                />
                <ActionButton
                  preferredFocus={ready}
                  label={ended ? 'Replay' : paused ? 'Play' : 'Pause'}
                  disabled={!ready}
                  onPress={toggleTVPlayback}
                />
                <ActionButton
                  label="Forward 10 seconds"
                  quiet
                  disabled={!ready}
                  onPress={() =>
                    seek(
                      Math.min(
                        source.duration || 31_536_000,
                        (pendingSeek.current ?? position.current) + 10,
                      ),
                    )
                  }
                />
              </>
            ) : null}
            {Platform.isTV ? back : null}
            {!Platform.isTV ? (
              <PlayOnTV
                client={castingClient}
                itemId={item.id}
                video={item.kind === 'video'}
                initiallyOpen={showTVPicker}
                getPosition={() => position.current}
                playbackToken={source.progressToken}
                onConnected={async (playback) => {
                  transferred.current = true;
                  player.pause();
                  progressWriter.current?.write(position.current);
                  await progressWriter.current?.drain();
                  onTVPlayback(playback);
                }}
              />
            ) : null}
            {true ? (
              <ActionButton
                label={options ? 'Close playback options' : 'Playback options'}
                quiet
                onPress={() => setOptions(!options)}
                style={
                  !Platform.isTV
                    ? { borderRadius: 24, minHeight: 44 }
                    : undefined
                }
              />
            ) : null}
            {!Platform.isTV ? back : null}
          </View>
          {airPlay ? (
            <Text
              accessibilityLiveRegion="polite"
              style={{ color: theme.text }}
            >
              Playing via AirPlay
            </Text>
          ) : null}
          {options ? (
            <ScrollView
              style={{ maxHeight: 420, backgroundColor: theme.surface }}
            >
              <PlaybackExperienceOptions
                experience={experience}
                position={displayPosition}
                onSeek={(seconds) => {
                  seek(seconds);
                }}
                audioAvailable={localCompatibilityAvailable}
                sleep={sleep.label}
                onSleep={(minutes) =>
                  setSleep({
                    deadline: minutes > 0 ? Date.now() + minutes * 60_000 : 0,
                    position:
                      minutes === -1
                        ? (source.details?.chapters.find(
                            (chapter) => chapter.start > displayPosition + 0.5,
                          )?.start ?? source.duration)
                        : 0,
                    label:
                      minutes === -1
                        ? 'End of chapter'
                        : minutes > 0
                          ? `${minutes} minutes`
                          : '',
                  })
                }
                chapterEnd={source.duration > displayPosition}
                progressMessage={progressMessage}
              />
              <ActionButton
                label="What did they say?"
                quiet
                onPress={() => {
                  repeat.current = {
                    until: player.currentTime,
                    started: player.currentTime < 0.5,
                    track: repeat.current?.track ?? player.subtitleTrack,
                  };
                  if (!player.subtitleTrack)
                    player.subtitleTrack =
                      player.availableSubtitleTracks[0] ?? null;
                  seek(Math.max(0, player.currentTime - 10));
                }}
              />
              {player.availableAudioTracks.map((track) => (
                <ActionButton
                  key={track.id}
                  label={track.label || track.language || 'Audio'}
                  quiet
                  onPress={() => {
                    player.audioTrack = track;
                    experience.change({
                      ...experience.preferences,
                      audioTrack: track.label,
                      audioLanguage: track.language || 'auto',
                    });
                  }}
                />
              ))}
              <ActionButton
                label="Subtitles off"
                quiet
                onPress={() => {
                  repeat.current = null;
                  player.subtitleTrack = null;
                  experience.change({
                    ...experience.preferences,
                    subtitleTrack: '',
                    subtitleLanguage: 'off',
                  });
                }}
              />
              {player.availableSubtitleTracks.map((track) => (
                <ActionButton
                  key={track.id}
                  label={track.label || track.language || 'Subtitles'}
                  quiet
                  onPress={() => {
                    repeat.current = null;
                    player.subtitleTrack = track;
                    experience.change({
                      ...experience.preferences,
                      subtitleTrack: track.label,
                      subtitleLanguage: track.language || 'auto',
                    });
                  }}
                />
              ))}
              <PlaybackOptions
                source={source}
                onSeek={(seconds) => {
                  seek(seconds);
                  setOptions(false);
                }}
                onNext={onNext}
              />
            </ScrollView>
          ) : null}
          {ended && onNext ? (
            <ActionButton label="Next episode" onPress={onNext} />
          ) : null}
        </SafeAreaView>
      ) : null}
    </>
  );
  return (
    <View role="main" style={styles.screen}>
      {nativeOnly ? <StatusBar hidden /> : null}
      {Platform.isTV ? navigation : null}
      <VideoView
        ref={videoView}
        fullscreenOptions={{ enable: true }}
        onLayout={() => {
          if (
            Platform.OS !== 'ios' ||
            Platform.isTV ||
            presented.current ||
            (showTVPicker && !nativeOnly)
          )
            return;
          presented.current = true;
          void videoView.current?.enterFullscreen().catch(() => {
            if (nativeOnly)
              Alert.alert(
                'Unable to open player',
                'Please open this title again.',
                [{ text: 'Return to details', onPress: onBack }],
              );
          });
        }}
        onPictureInPictureStart={() => {
          pictureInPicture.current = true;
        }}
        onPictureInPictureStop={() => {
          pictureInPicture.current = false;
        }}
        onFullscreenEnter={nativeOnly ? () => setFullscreen(true) : undefined}
        onFullscreenExit={
          nativeOnly
            ? () => {
                setFullscreen(false);
                if (!pictureInPicture.current && !recovering.current) onBack();
              }
            : undefined
        }
        accessibilityLabel={`Playing ${item.title}`}
        allowsPictureInPicture
        startsPictureInPictureAutomatically
        contentFit="contain"
        nativeControls={!Platform.isTV && (!nativeOnly || fullscreen)}
        {...(nativeOnly ? {
          showsLoadingIndicator: !error && !airPlay && (!ready || buffering),
        } : {})}
        onFirstFrameRender={() => {
          firstFrame.current = true;
          measurement.current?.firstFrame();
          setReady(true);
        }}
        player={player}
        style={styles.video}
      />
      {nativeOnly && !fullscreen ? (
        <SafeAreaView pointerEvents="box-none" style={styles.overlay}>
          <ActionButton
            label="✕"
            accessibilityLabel="Close player"
            quiet
            onPress={onBack}
            style={styles.close}
          />
        </SafeAreaView>
      ) : null}
      {!nativeOnly ? (
        <SafeAreaView pointerEvents="box-none" style={styles.overlay}>
          {!ready &&
          !error &&
          !airPlay &&
          (Platform.OS !== 'ios' || Platform.isTV) ? (
            <View
              accessibilityLabel="Starting playback"
              accessibilityRole="progressbar"
              pointerEvents="none"
              style={styles.starting}
            >
              <Skeleton variant="media" />
              <Text style={[styles.startingText, { color: theme.muted }]}>
                Starting playback…
              </Text>
            </View>
          ) : null}
          {ready && buffering && !error && !airPlay ? (
            <View pointerEvents="none" style={styles.starting}>
              <Skeleton variant="inline" label="Buffering video" />
              <Text
                accessibilityLiveRegion="polite"
                style={[styles.startingText, { color: theme.muted }]}
              >
                Buffering…
              </Text>
            </View>
          ) : null}
          {error ? (
            <View
              style={[
                styles.errorPanel,
                { backgroundColor: theme.surface, borderColor: theme.line },
              ]}
            >
              <Text
                accessibilityLiveRegion="assertive"
                style={[styles.errorTitle, { color: theme.text }]}
              >
                Playback stopped
              </Text>
              <Text style={[styles.errorBody, { color: theme.muted }]}>
                {error}
              </Text>
              {onFailure && localRecovery ? (
                <ActionButton
                  label="Try local playback"
                  onPress={() => onFailure(position.current)}
                />
              ) : null}
              {source.compatible && localRecovery ? (
                <>
                  <Text style={[styles.errorBody, { color: theme.muted }]}>
                    {source.compatible.plan.mode === 'transcode'
                      ? 'Convert this video on your Server to play it here.'
                      : 'Try the Server’s compatible stream.'}
                  </Text>
                  <ActionButton
                    label={
                      source.compatible.plan.mode === 'transcode'
                        ? 'Convert video and play'
                        : 'Try compatible playback'
                    }
                    onPress={() => recover(position.current)}
                  />
                </>
              ) : null}
              <ActionButton
                label="Return to details"
                onPress={onBack}
                quiet={Boolean(
                  localRecovery && (onFailure || source.compatible),
                )}
              />
            </View>
          ) : null}
        </SafeAreaView>
      ) : null}
      {Platform.isTV && !chrome.visible ? (
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Show playback controls"
          focusable
          hasTVPreferredFocus
          onPress={chrome.reveal}
          style={StyleSheet.absoluteFill}
        />
      ) : null}
      {!Platform.isTV && !nativeOnly ? navigation : null}
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { backgroundColor: '#000000', flex: 1 },
  video: { flex: 1 },
  navigation: { padding: spacing.two },
  controls: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    gap: 8,
    flexWrap: 'wrap',
  },
  overlay: {
    bottom: 0,
    left: 0,
    padding: spacing.two,
    position: 'absolute',
    right: 0,
    top: 0,
  },
  close: { alignSelf: 'flex-start', width: 48, paddingHorizontal: 0 },
  back: { alignSelf: 'flex-start', minWidth: 96 },
  starting: {
    alignItems: 'center',
    alignSelf: 'center',
    gap: spacing.two,
    margin: 'auto',
  },
  startingText: { fontSize: 14, fontWeight: '700' },
  errorPanel: {
    alignSelf: 'center',
    borderRadius: radius.panel,
    borderWidth: 1,
    gap: spacing.two,
    margin: 'auto',
    maxWidth: 520,
    padding: spacing.four,
  },
  errorTitle: { fontSize: 26, fontWeight: '800' },
  errorBody: { fontSize: 16, lineHeight: 24 },
});
