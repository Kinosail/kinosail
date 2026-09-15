import React, { useEffect, useState } from 'react';
import { Platform, StyleSheet, Text, View } from 'react-native';

import { spacing, useKinoTheme } from '@/design/tokens';

type Progress = { seconds: number; duration: number };
const validSeconds = (value: unknown): value is number =>
  typeof value === 'number' &&
  Number.isFinite(value) &&
  value >= 0 &&
  value <= 315360000;
export function parseWatchProgress(value: unknown): Progress | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  const input = value as Record<string, unknown>;
  if (
    Object.keys(input).length !== 2 ||
    !validSeconds(input.seconds) ||
    !validSeconds(input.duration) ||
    (input.duration > 0 && input.seconds > input.duration)
  )
    return null;
  return { seconds: input.seconds, duration: input.duration };
}

type Props = {
  id: string;
  seconds?: number;
  watched?: boolean;
  mediaURL: (path: string) => string;
  headers?: Record<string, string>;
};
type WatchState = Progress & { complete: boolean };

export function useWatchProgress({
  id,
  seconds = 0,
  watched = false,
  mediaURL,
  headers,
}: Props): WatchState | null {
  const url = /^[a-z0-9_-]{1,128}$/i.test(id)
    ? mediaURL(`/api/v1/items/${id}/watch-progress`)
    : '';
  const [result, setResult] = useState<{
    url: string;
    headers: typeof headers;
    savedSeconds: number;
    progress: Progress;
  } | null>(null);
  useEffect(() => {
    if (!url) return;
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 10000);
    let active = true;
    void (async () => {
      try {
        const response = await fetch(url, {
          headers,
          signal: controller.signal,
          redirect: 'error',
        });
        if (
          !response.ok ||
          Number(response.headers.get('content-length')) > 256
        )
          return;
        const body = await response.text();
        if (body.length > 256) return;
        const progress = parseWatchProgress(JSON.parse(body));
        if (active && !controller.signal.aborted && progress)
          setResult({ url, headers, savedSeconds: seconds, progress });
      } catch {
        // Saved elapsed time remains useful when runtime is unavailable.
      } finally {
        clearTimeout(timeout);
      }
    })();
    return () => {
      active = false;
      clearTimeout(timeout);
      controller.abort();
    };
  }, [url, headers, seconds]);
  const progress =
    result?.url === url &&
    result.headers === headers &&
    result.savedSeconds === seconds
      ? result.progress
      : { seconds: validSeconds(seconds) ? seconds : 0, duration: 0 };
  if (!url) return null;
  return {
    ...progress,
    complete:
      watched ||
      (progress.duration > 0 && progress.seconds >= progress.duration),
  };
}

export function WatchProgress(props: Props) {
  return <WatchProgressIndicator progress={useWatchProgress(props)} />;
}

export function WatchProgressIndicator({
  progress,
}: {
  progress: WatchState | null;
}) {
  const theme = useKinoTheme();
  if (
    !progress ||
    (!progress.complete && progress.duration === 0 && progress.seconds === 0)
  )
    return null;
  const percent =
    progress.duration > 0
      ? Math.round((progress.seconds / progress.duration) * 100)
      : 0;
  const label = progress.complete
    ? 'Watched'
    : progress.duration > 0
      ? `${Math.ceil((progress.duration - progress.seconds) / 60)} min left`
      : `${Math.floor(progress.seconds / 60)} min watched`;
  return (
    <View style={styles.container}>
      <Text style={[styles.label, { color: theme.muted }]}>{label}</Text>
      {progress.duration > 0 ? (
        <View
          accessible
          accessibilityRole="progressbar"
          accessibilityLabel="Watch progress"
          accessibilityValue={{ min: 0, max: 100, now: percent, text: label }}
          style={[styles.track, { backgroundColor: theme.line }]}
        >
          <View
            style={[
              styles.fill,
              { width: `${percent}%`, backgroundColor: theme.signal },
            ]}
          />
        </View>
      ) : null}
    </View>
  );
}
const styles = StyleSheet.create({
  container: { alignSelf: 'stretch', gap: spacing.one },
  label: {
    fontSize: Platform.isTV ? 20 : 14,
    textAlign: Platform.isTV ? 'left' : 'right',
  },
  track: { height: 5, borderRadius: 3, overflow: 'hidden' },
  fill: { height: '100%', borderRadius: 3 },
});
