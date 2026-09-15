import React, { useEffect, useRef, useState } from 'react';
import { ScrollView, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { randomUUID } from 'expo-crypto';
import type { MediaItem, Progress } from '@/core/contract';
import type {
  CastingClient,
  CastCommand,
  CastStatus,
  TVPlayback,
} from '@/core/casting';
import { createProgressWriter } from '@/core/progress-writer';
import { spacing, useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';
import { PlaybackSeekBar } from './playback-seek-bar';
import { Skeleton } from './skeleton';

export function TVPlaybackView({
  playback,
  client,
  item,
  saveProgress,
  onDone,
}: {
  playback: TVPlayback;
  client: CastingClient;
  item: MediaItem;
  saveProgress(
    progress: Pick<Progress, 'seconds' | 'session' | 'revision'> & {
      playbackToken?: string;
      watched?: boolean;
    },
  ): Promise<Progress>;
  onDone(): void;
}) {
  const theme = useKinoTheme(),
    { session, controller } = playback;
  const [status, setStatus] = useState<CastStatus>({
    state: 'buffering',
    position: session.position,
    duration: session.duration,
  });
  const [error, setError] = useState(''),
    [busy, setBusy] = useState(false);
  const finished = useRef(false),
    working = useRef(false);
  const writer = useRef<ReturnType<typeof createProgressWriter> | null>(null);
  useEffect(() => {
    let active = true,
      polling = false;
    const progress = createProgressWriter(
      {
        session: `cast-${randomUUID()}`,
        revision: item.progress.revision,
        playbackToken: '',
      },
      saveProgress,
    );
    writer.current = progress;
    const poll = async () => {
      if (!active || finished.current || polling) return;
      polling = true;
      try {
        const next = await controller.status();
        if (!active || finished.current) return;
        setStatus(next);
        setError('');
        // Only receiver observations advance progress. Connecting and failed loads do not.
        if (next.state === 'playing' || next.state === 'paused')
          progress.write(next.position);
        else if (
          next.state === 'stopped' &&
          session.duration > 0 &&
          next.position >= session.duration - 2
        )
          progress.write(session.duration, true);
      } catch {
        if (active && !finished.current)
          setError(
            'The TV is not responding. Check its connection or stop casting.',
          );
      } finally {
        polling = false;
      }
    };
    const timer = setInterval(() => void poll(), 2000);
    void poll();
    return () => {
      active = false;
      clearInterval(timer);
      if (!finished.current) {
        void controller
          .command({ action: 'stop' })
          .catch(() => {})
          .finally(() => client.endCast(session.id).catch(() => {}));
      }
    };
  }, [client, controller, item.progress.revision, saveProgress, session]);
  const command = async (value: CastCommand) => {
    if (working.current) return;
    working.current = true;
    setBusy(true);
    setError('');
    try {
      await controller.command(value);
    } catch {
      setError('The TV could not complete that action. Try again.');
    } finally {
      working.current = false;
      setBusy(false);
    }
  };
  const stop = async () => {
    if (working.current) return;
    working.current = true;
    setBusy(true);
    finished.current = true;
    try {
      await controller.command({ action: 'stop' });
    } catch {
      /* An offline receiver must not prevent revoking its media capability. */
    }
    try {
      await client.endCast(session.id);
      await writer.current?.drain();
      onDone();
    } catch {
      finished.current = false;
      working.current = false;
      setBusy(false);
      setError(
        'Could not revoke TV access. Reconnect to Kinosail Server and try again.',
      );
    }
  };
  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: theme.background }}>
      <ScrollView
        contentContainerStyle={{ padding: spacing.three, gap: spacing.three }}
      >
        <Text
          accessibilityRole="header"
          style={{ color: theme.text, fontSize: 28, fontWeight: '700' }}
        >
          {item.title}
        </Text>
        <Text
          accessibilityLiveRegion="polite"
          style={{ color: theme.text, fontSize: 20 }}
        >
          {{
            playing: 'Playing on',
            paused: 'Paused on',
            buffering: 'Loading on',
            stopped: 'Stopped on',
          }[status.state]}{' '}
          {controller.name}
        </Text>
        <Text style={{ color: theme.muted }}>
          {session.protocol === 'google-cast'
            ? 'Google Cast / Chromecast'
            : 'DLNA / UPnP'}{' '}
          · {status.state}
        </Text>
        {status.state === 'buffering' && !error ? (
          <Skeleton variant="inline" label={`Loading on ${controller.name}`} />
        ) : null}
        <PlaybackSeekBar
          seconds={status.position}
          duration={status.duration}
          onSeek={(position) => void command({ action: 'seek', position })}
        />
        <View style={{ gap: spacing.two }}>
          <ActionButton
            label={
              status.state === 'paused' || status.state === 'stopped'
                ? 'Play on TV'
                : 'Pause TV'
            }
            busy={busy}
            onPress={() =>
              void command({
                action:
                  status.state === 'paused' || status.state === 'stopped'
                    ? 'play'
                    : 'pause',
              })
            }
          />
          <ActionButton
            label="Stop casting and return"
            quiet
            busy={busy}
            onPress={() => void stop()}
          />
        </View>
        {error ? (
          <Text accessibilityRole="alert" style={{ color: theme.text }}>
            {error}
          </Text>
        ) : null}
      </ScrollView>
    </SafeAreaView>
  );
}
