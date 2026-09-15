import React, { useEffect, useRef, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import type { DeviceApproval } from '@/core/device-approval';
import type { KinosailClient } from '@/core/server-client';
import { useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';

type ApprovalClient = Pick<
  KinosailClient,
  'previewDevice' | 'approveDevice' | 'loadViewer'
>;

export function DeviceApprovalPanel({
  client,
  code,
  onClose,
  onApproved,
}: {
  client: ApprovalClient;
  code: string;
  onClose(): void;
  onApproved?(): void;
}) {
  const theme = useKinoTheme();
  const [loaded, setLoaded] = useState<{
    client: ApprovalClient;
    code: string;
    request: DeviceApproval;
    viewer: string;
  } | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [approved, setApproved] = useState(false);
  const [retry, setRetry] = useState(0);
  const current =
    loaded?.client === client && loaded.code === code ? loaded : null;
  const sending = useRef(false);
  const generation = useRef(0);
  useEffect(() => {
    const attempt = ++generation.current;
    setLoaded(null);
    setError('');
    setApproved(false);
    setBusy(false);
    sending.current = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    void Promise.all([client.previewDevice(code), client.loadViewer()])
      .then(([request, me]) => {
        if (generation.current !== attempt) return;
        const remaining = Date.parse(request.expiresAt) - Date.now();
        if (remaining <= 0)
          throw new Error('This code expired. Request a new code on your TV.');
        setLoaded({ client, code, request, viewer: me.viewer.name });
        timer = setTimeout(
          () => {
            setLoaded(null);
            setError('This code expired. Request a new code on your TV.');
          },
          Math.min(remaining, 2147483647),
        );
      })
      .catch(() => {
        if (generation.current === attempt)
          setError(
            'This request is unavailable. It may have expired or been approved already. Check your TV, or try again.',
          );
      });
    return () => {
      generation.current++;
      clearTimeout(timer);
    };
  }, [client, code, retry]);
  const approve = async () => {
    if (
      !current ||
      sending.current ||
      Date.parse(current.request.expiresAt) <= Date.now()
    )
      return;
    const attempt = generation.current;
    sending.current = true;
    setBusy(true);
    setError('');
    try {
      await client.approveDevice(code);
      if (generation.current === attempt) {
        setApproved(true);
        onApproved?.();
      }
    } catch {
      if (generation.current === attempt)
        setError(
          'Could not approve this device. Check your TV and connection, then try again.',
        );
    } finally {
      if (generation.current === attempt) {
        sending.current = false;
        setBusy(false);
      }
    }
  };
  return (
    <View style={styles.panel}>
      <Text
        accessibilityRole="header"
        style={[styles.title, { color: theme.text }]}
      >
        {approved ? 'TV approved' : 'Sign in on your TV?'}
      </Text>
      {approved ? (
        <Text
          accessibilityLiveRegion="polite"
          style={[styles.body, { color: theme.muted }]}
        >
          Your TV can now finish signing in. You can keep using your phone.
        </Text>
      ) : current ? (
        <>
          <Text style={[styles.body, { color: theme.text }]}>
            {current.request.device}
          </Text>
          <Text style={[styles.body, { color: theme.muted }]}>
            Only approve if this code matches the device you’re signing in:
          </Text>
          <Text
            accessibilityLabel={`Code ${code.split('').join(' ')}`}
            style={[styles.code, { color: theme.signal }]}
          >
            {code.slice(0, 3)} {code.slice(3)}
          </Text>
          <Text style={[styles.body, { color: theme.muted }]}>
            The TV will use {current.viewer}’s access and content settings.
          </Text>
        </>
      ) : !error ? (
        <Text style={{ color: theme.muted }}>Checking this request…</Text>
      ) : null}
      {!approved && error ? (
        <Text
          accessibilityLiveRegion="assertive"
          style={{ color: theme.danger }}
        >
          {error}
        </Text>
      ) : null}
      {approved ? (
        <ActionButton label="Done" onPress={onClose} />
      ) : (
        <>
          {current ? (
            <ActionButton
              label={`Approve as ${current.viewer}`}
              busy={busy}
              onPress={() => {
                void approve();
              }}
            />
          ) : error ? (
            <ActionButton
              label="Try again"
              onPress={() => setRetry(retry + 1)}
            />
          ) : null}
          <ActionButton
            label="Not now"
            quiet
            disabled={busy}
            onPress={onClose}
          />
        </>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  panel: { gap: 20, width: '100%' },
  title: { fontSize: 28, fontWeight: '800' },
  body: { fontSize: 16, lineHeight: 24 },
  code: {
    fontSize: 40,
    fontWeight: '800',
    fontVariant: ['tabular-nums'],
    letterSpacing: 4,
  },
});
