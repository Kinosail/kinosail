import React, { useEffect, useState } from 'react';
import { Text, View } from 'react-native';
import type { KinosailClient } from '@/core/server-client';
import {
  pendingProgress,
  flushProgress,
  type PendingProgress,
} from '@/core/progress-sync';
import { formatPosition } from '@/core/media-preferences';
import { useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';
export function ProgressSyncPanel({ client }: { client: KinosailClient }) {
  const theme = useKinoTheme(),
    [entries, setEntries] = useState<PendingProgress[]>([]),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    let active = true;
    const refresh = () => {
      void pendingProgress(client).then(
        (value) => active && setEntries(value),
        () => active && setError('Saved progress could not be read.'),
      );
    };
    refresh();
    const timer = setInterval(refresh, 10000);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [client]);
  const run = async (operation: () => Promise<unknown>) => {
    setBusy(true);
    setError('');
    try {
      await operation();
      setEntries(await pendingProgress(client));
    } catch {
      setError(
        'Could not sync progress. Check your Server connection and try again.',
      );
    } finally {
      setBusy(false);
    }
  };
  return (
    <View style={{ gap: 12 }}>
      <Text
        accessibilityRole="header"
        style={{ color: theme.text, fontSize: 24, fontWeight: '600' }}
      >
        Playback progress
      </Text>
      <Text style={{ color: theme.muted }}>
        {entries.length
          ? `${entries.length} title${entries.length === 1 ? '' : 's'} with progress saved on this device.`
          : 'No progress waiting to sync.'}
      </Text>
      {entries.map((entry) => (
        <View
          key={entry.id}
          style={{ gap: 8, padding: 12, backgroundColor: theme.surface }}
        >
          <Text style={{ color: theme.text, fontSize: 18 }}>
            {entry.title || 'Saved title'} ·{' '}
            {formatPosition(entry.progress.seconds)}
          </Text>
          <Text style={{ color: theme.muted }}>
            Waiting for a Server connection. The furthest saved position will be
            kept.
          </Text>
        </View>
      ))}
      <ActionButton
        label="Sync now"
        quiet
        disabled={busy}
        onPress={() => void run(() => flushProgress(client))}
      />
      {error ? (
        <Text accessibilityRole="alert" style={{ color: theme.danger }}>
          {error}
        </Text>
      ) : null}
    </View>
  );
}
