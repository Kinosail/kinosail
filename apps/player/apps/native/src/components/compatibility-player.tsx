import { queueNeighbor, type MusicQueue } from '@/core/use-music-queue';
import { MusicQueueTools } from './music-queue-tools';
import { PlayOnTV } from './play-on-tv';
import type { CastingClient, TVPlayback } from '@/core/casting';
import { BottomActions } from './bottom-actions';
import { AudioArtwork, AudioButton } from './audio-player-controls';
import { AudiobookProgress, AudiobookTools } from './audiobook-tools';
import { matchesPlaybackLanguage } from '@/core/media-preferences';
import { randomUUID } from 'expo-crypto';
import type { PlaybackExperience } from '@/core/use-playback-experience';
import { PlaybackExperienceOptions } from './playback-experience-options';
import Video, { type VLCPlayerRef, type VLCPlayerTracks } from './local-video';
import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  AppState,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import type { MediaItem, PlaybackSource, Progress } from '@/core/contract';
import {
  openProtectedMedia,
  closeProtectedMedia,
  nextProtectedMediaID,
} from '@/core/protected-media';
import { parseLocalTracks } from '@/core/local-tracks';
import { measurePlayback } from '@/core/playback-metrics';
import { createProgressWriter } from '@/core/progress-writer';
import { spacing, useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';
import { PlaybackSeekBar } from './playback-seek-bar';
import { PlaybackOptions } from './playback-options';
import { FocusGroup } from './focus-group';
import { usePlaybackRemote } from './tv-playback-events';
import { Skeleton } from './skeleton';
import { useTVPlaybackChrome } from './use-tv-playback-chrome';

type Props = {
  musicQueue?: MusicQueue;
  castingClient?: CastingClient;
  showTVPicker?: boolean;
  onPrepareAirPlay?: (position: number) => void;
  onTVPlayback?: (playback: TVPlayback) => void;
  artworkURL?: string;
  artworkHeaders?: Record<string, string>;
  experience: PlaybackExperience;
  progressMessage: string;
  startedAt?: number;
  item: MediaItem;
  source: PlaybackSource;
  onBack(): void;
  onNext?: () => void;
  onRecover(position: number): void;
  saveProgress(
    progress: Pick<Progress, 'seconds' | 'session' | 'revision'> & {
      playbackToken?: string;
      watched?: boolean;
    },
  ): Promise<Progress>;
};
export function CompatibilityPlayer({
  musicQueue,
  castingClient,
  showTVPicker,
  onPrepareAirPlay,
  onTVPlayback,
  artworkURL,
  artworkHeaders,
  experience,
  progressMessage,
  startedAt,
  item,
  source,
  onBack,
  onNext,
  onRecover,
  saveProgress,
}: Props) {
  const transferred = useRef(false);
  const theme = useKinoTheme();
  const music = item.kind === 'music';
  const audiobook = item.kind === 'audiobook';
  const audio = music || audiobook;
  const preferences = experience.preferences;
  const [sleepDeadline, setSleepDeadline] = useState(0),
    [sleepPosition, setSleepPosition] = useState(0);
  const [sleepLabel, setSleepLabel] = useState('');
  const repeat = useRef<{
    until: number;
    started: boolean;
    subtitle: number;
  } | null>(null);
  const appliedTracks = useRef(false);
  const [effectStart, setEffectStart] = useState(source.start);
  const effects = `${preferences.nightMode}-${preferences.dialogueBoost}-${preferences.volumeBoost}`;
  const previousEffects = useRef(effects);
  const [uri, setURI] = useState('');
  const [error, setError] = useState('');
  const [paused, setPaused] = useState(false);
  const [ended, setEnded] = useState(false);
  const [replay, setReplay] = useState(0);
  const replayStartedAt = useRef(0);
  const [pipReady, setPipReady] = useState(false);
  const [ready, setReady] = useState(false);
  const [buffering, setBuffering] = useState(false);
  const [options, setOptions] = useState(false);
  const televisionVideo = Platform.isTV && !audio;
  const chrome = useTVPlaybackChrome(
    televisionVideo,
    paused || ended || options || !ready || buffering || Boolean(error),
    options,
    () => setOptions(false),
  );
  const [tracks, setTracks] = useState<VLCPlayerTracks | null>(null);
  const [subtitleDelay, setSubtitleDelay] = useState(0);
  const [audioDelay, setAudioDelay] = useState(0);
  const player = useRef<VLCPlayerRef>(null);
  const position = useRef(source.start);
  const pendingSeek = useRef<number | null>(null);
  const hasProgress = useRef(false);
  const [displayPosition, setDisplayPosition] = useState(source.start);
  const resumed = useRef(false);
  const restartPlayback = useCallback(() => {
    replayStartedAt.current = performance.now();
    resumed.current = false;
    appliedTracks.current = false;
    setReady(false);
    setBuffering(false);
    setReplay((value) => value + 1);
  }, []);
  useEffect(() => {
    if (previousEffects.current !== effects) {
      previousEffects.current = effects;
      setEffectStart(position.current);
      restartPlayback();
    }
  }, [effects, restartPlayback]);
  const chapters = source.details?.chapters ?? [];
  const chapterIndex = chapters.reduce(
    (current, chapter, index) =>
      chapter.start <= displayPosition ? index : current,
    -1,
  );
  const chapterEnd = audiobook
    ? (chapters[chapterIndex]?.end ?? source.duration)
    : (source.details?.chapters.find(
        (chapter) => chapter.start > displayPosition + 0.5,
      )?.start ?? source.duration);
  const setSleep = (minutes: number) => {
    setSleepDeadline(minutes > 0 ? Date.now() + minutes * 60_000 : 0);
    setSleepPosition(minutes === -1 ? chapterEnd : 0);
    setSleepLabel(
      minutes === -1
        ? 'End of chapter'
        : minutes > 0
          ? `${minutes} minutes`
          : '',
    );
  };
  const repeatLine = () => {
    if (!tracks) return;
    repeat.current = {
      until: position.current,
      started: position.current < 0.5,
      subtitle: repeat.current?.subtitle ?? tracks.subtitleIndex,
    };
    if (tracks.subtitleIndex < 0 && tracks.subtitle[0])
      player.current?.selectSubtitleTrack(tracks.subtitle[0].id);
    seek((pendingSeek.current ?? position.current) - 10);
  };
  const measurement = useRef<ReturnType<typeof measurePlayback> | null>(null);
  const lastSaved = useRef(0);
  const progress = useRef<ReturnType<typeof createProgressWriter> | null>(null);
  const session = useRef(`native-vlc-${randomUUID()}`).current;
  const togglePlayback = () => {
    if (ended) {
      progress.current?.write(0, false);
      setEffectStart(0);
      restartPlayback();
      setEnded(false);
      position.current = 0;
      setDisplayPosition(0);
      setPaused(false);
    } else setPaused((value) => !value);
  };
  usePlaybackRemote(
    togglePlayback,
    televisionVideo ? chrome.reveal : undefined,
  );
  useEffect(() => {
    const metrics = measurePlayback(
      'vlc',
      replay ? replayStartedAt.current : (startedAt ?? performance.now()),
    );
    measurement.current = metrics;
    return () => metrics.finish('closed');
  }, [source, session, startedAt, replay]);
  useEffect(() => {
    let active = true;
    const transportID = nextProtectedMediaID();
    openProtectedMedia(source, transportID).then(
      (local) => {
        if (active) setURI(local);
      },
      () => {
        if (active) {
          measurement.current?.finish('error');
          setError('The original file could not be opened on this device.');
        }
      },
    );
    return () => {
      active = false;
      closeProtectedMedia(transportID);
    };
  }, [source, session]);
  useEffect(() => {
    const writer = createProgressWriter(
      {
        session,
        revision: item.progress.revision,
        playbackToken: source.progressToken,
      },
      saveProgress,
    );
    progress.current = writer;
    const state = AppState.addEventListener('change', (value) => {
      if (value !== 'active' && hasProgress.current && !transferred.current)
        writer.write(position.current);
    });
    return () => {
      state.remove();
      if (hasProgress.current && !transferred.current)
        writer.write(position.current);
    };
  }, [item.progress.revision, saveProgress, session, source.progressToken]);
  useEffect(() => {
    if (!uri || ready || error || paused) return;
    const timer = setTimeout(() => {
      measurement.current?.finish('error');
      setError(
        audio
          ? 'Audio playback did not start. Try again or return to details.'
          : 'Playback did not produce a picture. Try a compatible stream or return to details.',
      );
    }, 30_000);
    return () => clearTimeout(timer);
  }, [uri, ready, error, paused, audio]);
  const seek = (seconds: number) => {
    if (!Number.isFinite(seconds) || (audiobook && !ready)) return;
    const next = Math.max(
      0,
      Math.min(source.duration || 31_536_000, 31_536_000, seconds),
    );
    if (audiobook && ended && next >= source.duration) return;
    pendingSeek.current = next;
    setDisplayPosition(next);
    measurement.current?.seek(next);
    if (audiobook && ended) {
      setEffectStart(next);
      restartPlayback();
      setEnded(false);
    } else player.current?.seek(next);
    if (audiobook) {
      position.current = next;
      hasProgress.current = true;
      setDisplayPosition(next);
      progress.current?.write(next, false);
    }
  };
  const delay = (kind: 'audio' | 'subtitle', amount: number) => {
    const next = Math.max(
      -30,
      Math.min(30, (kind === 'audio' ? audioDelay : subtitleDelay) + amount),
    );
    if (kind === 'audio') {
      player.current?.setAudioDelay(next * 1_000_000);
      setAudioDelay(next);
    } else {
      player.current?.setSubtitleDelay(next * 1_000_000);
      setSubtitleDelay(next);
    }
  };
  const changingTrack = useRef(false);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const changeTrack = async (id: string) => {
    if (
      !musicQueue ||
      changingTrack.current ||
      !musicQueue.items.some((track) => track.id === id)
    )
      return;
    if (id === item.id) {
      position.current = 0;
      setDisplayPosition(0);
      setEffectStart(0);
      setEnded(false);
      setPaused(false);
      restartPlayback();
      progress.current?.write(0, false);
      return;
    }
    changingTrack.current = true;
    setPaused(true);
    if (hasProgress.current) progress.current?.write(position.current);
    await progress.current?.drain();
    if (mounted.current) {
      hasProgress.current = false;
      musicQueue.select(id);
    }
  };
  const nextTrack = musicQueue ? queueNeighbor(musicQueue, 1) : undefined;
  const previousTrack = musicQueue ? queueNeighbor(musicQueue, -1) : undefined;
  const content = (
    <>
      <View
        style={[
          styles.top,
          televisionVideo
            ? {
                position: 'absolute',
                top: 40,
                left: 64,
                right: 64,
                zIndex: 2,
                display: chrome.visible ? 'flex' : 'none',
                backgroundColor: '#000000CC',
              }
            : null,
        ]}
      >
        {Platform.isTV && audio ? (
          <AudioButton label="Back" icon="down" onPress={onBack} />
        ) : Platform.isTV ? (
          <ActionButton label="Back" onPress={onBack} quiet />
        ) : null}
        {!Platform.isTV && onTVPlayback ? (
          <PlayOnTV
            airPlayCompatible={audio}
            onPrepareAirPlay={
              source.compatible
                ? () => (onPrepareAirPlay ?? onRecover)(position.current)
                : undefined
            }
            client={castingClient}
            itemId={item.id}
            video={item.kind === 'video'}
            initiallyOpen={showTVPicker}
            getPosition={() => position.current}
            playbackToken={source.progressToken}
            onConnected={async (playback) => {
              transferred.current = true;
              setPaused(true);
              progress.current?.write(position.current);
              await progress.current?.drain();
              onTVPlayback(playback);
            }}
          />
        ) : null}
        <Text numberOfLines={1} style={[styles.title, { color: theme.text }]}>
          {audiobook ? 'Audiobook' : music ? 'Now playing' : item.title}
        </Text>
        {Platform.isTV && audiobook ? (
          <AudioButton
            label="Bookmark this moment"
            icon="bookmark"
            disabled={!ready || !experience.loaded || experience.saving}
            onPress={() => experience.bookmark(position.current)}
          />
        ) : Platform.isTV && music ? (
          <AudioButton
            label={options ? 'Close options' : 'Playback options'}
            icon="more"
            onPress={() => setOptions(!options)}
          />
        ) : null}
      </View>
      <View style={styles.video}>
        {item.kind !== 'video' && !audio && ready ? (
          <View style={styles.center}>
            <Text
              style={{ color: theme.text, fontSize: 28, textAlign: 'center' }}
            >
              {item.title}
            </Text>
            <Text style={{ color: theme.muted }}>
              {item.kind === 'music' ? 'Music' : 'Audiobook'} ·{' '}
              {preferences.rate}×
            </Text>
          </View>
        ) : null}
        {uri && !error ? (
          <Video
            key={`${uri}-${replay}`}
            ref={player}
            source={{ uri, start: effectStart }}
            rate={preferences.rate}
            nightMode={preferences.nightMode}
            dialogueBoost={preferences.dialogueBoost}
            volumeBoost={preferences.volumeBoost}
            audioOnly={item.kind !== 'video'}
            sleepDeadline={sleepDeadline}
            sleepPosition={sleepPosition}
            style={audio ? styles.audioEngine : StyleSheet.absoluteFill}
            paused={paused}
            onFirstFrame={() => {
              measurement.current?.firstFrame();
              setReady(true);
            }}
            onBuffering={({ active }) => {
              if (typeof active === 'boolean') {
                setBuffering(active);
                measurement.current?.buffering(active && !paused);
              }
            }}
            onPictureInPictureReady={() => setPipReady(true)}
            onPaused={() => {
              measurement.current?.buffering(false);
              setPaused(true);
              if (hasProgress.current && !transferred.current)
                progress.current?.write(position.current);
              if (
                (sleepDeadline && Date.now() >= sleepDeadline) ||
                (sleepPosition && position.current >= sleepPosition)
              ) {
                setSleep(0);
              }
            }}
            onPlaying={() => {
              setBuffering(false);
              setPaused(false);
              if (!resumed.current) {
                resumed.current = true;
                if (item.kind !== 'video') setReady(true);
                player.current?.getTracks();
              }
            }}
            onProgress={({ currentTime }) => {
              if (
                !transferred.current &&
                resumed.current &&
                Number.isFinite(currentTime) &&
                currentTime >= 0
              ) {
                if (repeat.current && currentTime < repeat.current.until - 0.5)
                  repeat.current.started = true;
                if (
                  repeat.current?.started &&
                  currentTime >= repeat.current.until
                ) {
                  player.current?.selectSubtitleTrack(repeat.current.subtitle);
                  repeat.current = null;
                }
                if (
                  audiobook &&
                  sleepPosition > 0 &&
                  currentTime >= sleepPosition
                )
                  setSleep(0);
                hasProgress.current = true;
                measurement.current?.position(currentTime);
                pendingSeek.current = null;
                position.current = currentTime;
                setDisplayPosition(currentTime);
                if (Date.now() - lastSaved.current >= 5000) {
                  progress.current?.write(currentTime);
                  lastSaved.current = Date.now();
                }
              }
            }}
            onEnd={() => {
              if (transferred.current) return;
              hasProgress.current = true;
              measurement.current?.finish('ended');
              setEnded(true);
              setPaused(true);
              setDisplayPosition(source.duration);
              position.current = source.duration;
              progress.current?.write(source.duration, true);
              if (music && musicQueue?.repeat === 'one')
                void changeTrack(item.id);
              else if (music && nextTrack) void changeTrack(nextTrack);
            }}
            onError={() => {
              measurement.current?.finish('error');
              setError(
                'This file could not play with the local compatibility player.',
              );
            }}
            onTracks={(value) => {
              const parsed = parseLocalTracks(value);
              setTracks(parsed);
              if (parsed && !appliedTracks.current) {
                appliedTracks.current = true;
                const audio = parsed.audio.find(
                  (track) =>
                    (preferences.audioTrack &&
                      track.name === preferences.audioTrack) ||
                    matchesPlaybackLanguage(
                      preferences.audioLanguage,
                      track.language,
                    ),
                );
                const subtitle = parsed.subtitle.find(
                  (track) =>
                    (preferences.subtitleTrack &&
                      track.name === preferences.subtitleTrack) ||
                    matchesPlaybackLanguage(
                      preferences.subtitleLanguage,
                      track.language,
                    ),
                );
                if (audio) player.current?.selectAudioTrack(audio.id);
                if (preferences.subtitleLanguage === 'off')
                  player.current?.selectSubtitleTrack(-1);
                else if (subtitle)
                  player.current?.selectSubtitleTrack(subtitle.id);
              }
            }}
          />
        ) : null}
        {audio && !error ? (
          <AudioArtwork item={item} uri={artworkURL} headers={artworkHeaders} />
        ) : null}
        {ready && buffering && !paused && !ended && !error ? (
          <View
            pointerEvents="none"
            style={audio ? styles.audioLoading : styles.center}
          >
            <Skeleton variant="inline" label="Buffering media" />
            <Text accessibilityLiveRegion="polite" style={{ color: theme.muted }}>
              Buffering…
            </Text>
          </View>
        ) : null}
        {!ready && !error ? (
          <View style={audio ? styles.audioLoading : styles.center}>
            <Skeleton variant="media" />
            <Text style={{ color: theme.muted }}>Opening original file…</Text>
          </View>
        ) : null}
        {error ? (
          <View style={styles.center}>
            <Text accessibilityRole="alert" style={{ color: theme.text }}>
              {error}
            </Text>
            {source.compatible ? (
              <ActionButton
                label={
                  source.compatible.plan.mode === 'transcode'
                    ? audio
                      ? 'Convert audio and play'
                      : 'Convert video and play'
                    : 'Try compatible stream'
                }
                onPress={() => onRecover(position.current)}
              />
            ) : null}
            <ActionButton label="Return to details" onPress={onBack} />
          </View>
        ) : null}
      </View>
      {!error && (!televisionVideo || chrome.visible) ? (
        <View
          onFocus={televisionVideo ? chrome.reveal : undefined}
          style={
            televisionVideo
              ? {
                  position: 'absolute',
                  bottom: 40,
                  left: 64,
                  right: 64,
                  maxHeight: '75%',
                  backgroundColor: '#000000E8',
                  padding: 24,
                  gap: 16,
                }
              : undefined
          }
        >
          {audio ? (
            <View style={styles.audioDetails}>
              <Text
                accessibilityRole="header"
                style={{ color: theme.text, fontSize: 26, fontWeight: '700' }}
              >
                {item.title}
              </Text>
              {item.artist ? (
                <Text style={{ color: theme.muted, fontSize: 18 }}>
                  {item.artist}
                </Text>
              ) : null}
              {music && item.album ? (
                <Text style={{ color: theme.muted, fontSize: 14 }}>
                  {item.album}
                </Text>
              ) : null}
            </View>
          ) : null}
          {audiobook ? (
            <AudiobookProgress
              source={source}
              position={displayPosition}
              chapterIndex={chapterIndex}
              rate={preferences.rate}
            />
          ) : null}
          <PlaybackSeekBar
            seconds={displayPosition}
            duration={source.duration}
            onSeek={seek}
          />
          {!audio && onNext && (!Platform.isTV || ended) ? (
            <ActionButton label="Next episode" quiet onPress={onNext} />
          ) : null}
          {item.kind === 'video' && (!Platform.isTV || options) ? (
            <ActionButton
              label="What did they say?"
              quiet
              onPress={repeatLine}
              disabled={!ready}
            />
          ) : null}
          {audio ? (
            <View
              style={[
                styles.audioTransport,
                audiobook && { paddingVertical: 8 },
              ]}
            >
              <AudioButton
                label={
                  audiobook
                    ? 'Back 30 seconds'
                    : previousTrack && displayPosition <= 3
                      ? 'Previous track'
                      : 'Restart track'
                }
                icon={audiobook ? 'back30' : 'restart'}
                disabled={!ready}
                onPress={() =>
                  audiobook
                    ? seek(position.current - 30)
                    : previousTrack && position.current <= 3
                      ? void changeTrack(previousTrack)
                      : ended
                        ? togglePlayback()
                        : seek(0)
                }
              />
              <AudioButton
                label={ended ? 'Replay' : paused ? 'Play' : 'Pause'}
                icon={ended || paused ? 'play' : 'pause'}
                primary
                onPress={togglePlayback}
              />
              <AudioButton
                label={audiobook ? 'Forward 30 seconds' : 'Next track'}
                icon={audiobook ? 'forward30' : 'next'}
                onPress={() =>
                  audiobook
                    ? seek(position.current + 30)
                    : nextTrack
                      ? void changeTrack(nextTrack)
                      : !musicQueue
                        ? onNext?.()
                        : undefined
                }
                disabled={
                  audiobook ? !ready : musicQueue ? !nextTrack : !onNext
                }
              />
            </View>
          ) : (
            <FocusGroup style={styles.controls}>
              {pipReady ? (
                <ActionButton
                  label="Picture in Picture"
                  quiet
                  onPress={() => player.current?.startPictureInPicture()}
                />
              ) : null}
              <ActionButton
                label="Back 10 seconds"
                onPress={() =>
                  seek((pendingSeek.current ?? position.current) - 10)
                }
                quiet
              />
              <ActionButton
                label={ended ? 'Replay' : paused ? 'Play' : 'Pause'}
                preferredFocus={televisionVideo}
                onPress={togglePlayback}
              />
              <ActionButton
                label="Forward 10 seconds"
                onPress={() =>
                  seek((pendingSeek.current ?? position.current) + 10)
                }
                quiet
              />
              <ActionButton
                label={
                  options ? 'Close options' : 'Audio, subtitles & chapters'
                }
                quiet
                onPress={() => {
                  player.current?.getTracks();
                  setOptions(!options);
                }}
              />
            </FocusGroup>
          )}
          {music && musicQueue ? (
            <MusicQueueTools
              queue={musicQueue}
              onSelect={(id) => void changeTrack(id)}
            />
          ) : null}
          {audiobook ? (
            <AudiobookTools
              source={source}
              position={displayPosition}
              chapterIndex={chapterIndex}
              canSeek={ready}
              experience={experience}
              sleep={sleepLabel}
              onSeek={seek}
              onSleep={setSleep}
              chapterEnd={chapterIndex >= 0 && chapterEnd > displayPosition}
              progressMessage={progressMessage}
              onNext={onNext}
            />
          ) : null}
          {!audiobook && options ? (
            <ScrollView
              style={styles.options}
              contentContainerStyle={styles.optionContent}
            >
              <PlaybackExperienceOptions
                experience={experience}
                position={displayPosition}
                onSeek={seek}
                audioAvailable
                sleep={sleepLabel}
                onSleep={setSleep}
                chapterEnd={chapterEnd > displayPosition}
                progressMessage={progressMessage}
              />
              {!music ? (
                <>
                  <Text style={[styles.optionTitle, { color: theme.text }]}>
                    Audio
                  </Text>
                  <FocusGroup style={styles.controls}>
                    {tracks?.audio
                      .filter((track) => track.id >= 0)
                      .map((track) => (
                        <ActionButton
                          key={track.id}
                          label={track.name || `Audio ${track.id}`}
                          quiet={track.id !== tracks.audioIndex}
                          onPress={() => {
                            experience.change({
                              ...preferences,
                              audioTrack: track.name,
                            });
                            player.current?.selectAudioTrack(track.id);
                            player.current?.getTracks();
                          }}
                        />
                      ))}
                  </FocusGroup>
                  <Text style={[styles.optionTitle, { color: theme.text }]}>
                    Subtitles
                  </Text>
                  <FocusGroup style={styles.controls}>
                    <ActionButton
                      label="Off"
                      quiet={tracks?.subtitleIndex !== -1}
                      onPress={() => {
                        repeat.current = null;
                        experience.change({
                          ...preferences,
                          subtitleLanguage: 'off',
                          subtitleTrack: '',
                        });
                        player.current?.selectSubtitleTrack(-1);
                        player.current?.getTracks();
                      }}
                    />
                    {tracks?.subtitle
                      .filter((track) => track.id >= 0)
                      .map((track) => (
                        <ActionButton
                          key={track.id}
                          label={track.name || `Subtitle ${track.id}`}
                          quiet={track.id !== tracks.subtitleIndex}
                          onPress={() => {
                            repeat.current = null;
                            experience.change({
                              ...preferences,
                              subtitleLanguage: 'auto',
                              subtitleTrack: track.name,
                            });
                            player.current?.selectSubtitleTrack(track.id);
                            player.current?.getTracks();
                          }}
                        />
                      ))}
                  </FocusGroup>
                  <Text style={{ color: theme.text }}>
                    Subtitle delay: {subtitleDelay.toFixed(1)} s · Audio delay:{' '}
                    {audioDelay.toFixed(1)} s
                  </Text>
                  <FocusGroup style={styles.controls}>
                    <ActionButton
                      label="Subtitles earlier"
                      quiet
                      onPress={() => delay('subtitle', -0.1)}
                    />
                    <ActionButton
                      label="Subtitles later"
                      quiet
                      onPress={() => delay('subtitle', 0.1)}
                    />
                    <ActionButton
                      label="Audio earlier"
                      quiet
                      onPress={() => delay('audio', -0.1)}
                    />
                    <ActionButton
                      label="Audio later"
                      quiet
                      onPress={() => delay('audio', 0.1)}
                    />
                  </FocusGroup>
                  <PlaybackOptions
                    source={source}
                    onSeek={(seconds) => {
                      seek(seconds);
                      setOptions(false);
                    }}
                    onNext={onNext}
                  />
                </>
              ) : null}
            </ScrollView>
          ) : null}
        </View>
      ) : null}
      {televisionVideo && !chrome.visible ? (
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Show playback controls"
          focusable
          hasTVPreferredFocus
          onPress={chrome.reveal}
          style={StyleSheet.absoluteFill}
        />
      ) : null}
    </>
  );
  return (
    <SafeAreaView
      edges={televisionVideo ? [] : undefined}
      style={[styles.screen, audio && { backgroundColor: theme.background }]}
    >
      {audio ? (
        <ScrollView contentContainerStyle={{ flexGrow: 1 }}>
          {content}
        </ScrollView>
      ) : (
        content
      )}
      {!Platform.isTV ? (
        <BottomActions onBack={onBack}>
          {audiobook ? (
            <ActionButton
              label="Bookmark this moment"
              quiet
              disabled={!ready || !experience.loaded || experience.saving}
              onPress={() => experience.bookmark(position.current)}
            />
          ) : music ? (
            <ActionButton
              label={options ? 'Close options' : 'Playback options'}
              quiet
              onPress={() => setOptions(!options)}
            />
          ) : null}
        </BottomActions>
      ) : null}
    </SafeAreaView>
  );
}
const styles = StyleSheet.create({
  audioEngine: { position: 'absolute', width: 1, height: 1, opacity: 0 },
  audioLoading: { padding: 8, alignItems: 'center', gap: 4 },
  audioDetails: { paddingHorizontal: 24, gap: 4 },
  audioTransport: {
    flexDirection: 'row',
    justifyContent: 'space-evenly',
    alignItems: 'center',
    paddingVertical: 24,
  },
  screen: { flex: 1, backgroundColor: '#000' },
  top: {
    flexDirection: 'row',
    gap: spacing.two,
    alignItems: 'center',
    padding: spacing.two,
  },
  title: { fontSize: Platform.isTV ? 28 : 18, flex: 1 },
  video: { flex: 1 },
  center: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    gap: spacing.two,
    padding: spacing.two,
  },
  controls: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.one,
    padding: spacing.one,
  },
  options: { maxHeight: '45%' },
  optionContent: { padding: spacing.two, gap: spacing.one },
  optionTitle: { fontSize: 22, fontWeight: '700' },
});
