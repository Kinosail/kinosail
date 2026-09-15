import React, { useEffect, useRef, useState } from 'react';
import { AppState, Modal, Platform, ScrollView, View } from 'react-native';
import { usePathname } from 'expo-router';
import { useSession } from '@/core/session-context';
import type { KinosailClient } from '@/core/server-client';
import type { DeviceApproval } from '@/core/device-approval';
import { radius, useKinoTheme } from '@/design/tokens';
import { DeviceApprovalPanel } from './device-approval-panel';

export function TVApprovalPrompt() {
  const { client } = useSession(),
    theme = useKinoTheme(),
    path = usePathname();
  const [foreground, setForeground] = useState(
    AppState.currentState === 'active',
  );
  const [offered, setOffered] = useState<{
    client: KinosailClient;
    request: DeviceApproval;
    finished?: boolean;
  } | null>(null);
  const request = offered?.client === client ? offered.request : null;
  const dismissed = useRef(new Map<string, number>());
  const quietUntil = useRef(0);
  const allowed =
    !Platform.isTV &&
    Platform.OS !== 'web' &&
    foreground &&
    path !== '/approve' &&
    !path.startsWith('/watch/') &&
    !path.startsWith('/read/');
  useEffect(() => {
    const subscription = AppState.addEventListener('change', (state) =>
      setForeground(state === 'active'),
    );
    return () => subscription.remove();
  }, []);
  useEffect(() => {
    dismissed.current.clear();
    quietUntil.current = 0;
    setOffered(null);
  }, [client]);
  useEffect(() => {
    if (!client || !allowed) {
      setOffered(null);
      return;
    }
    let active = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const poll = async () => {
      let delay = 5000;
      try {
        const pending = await client.loadPendingTVs();
        if (!active) return;
        for (const [key, expires] of dismissed.current)
          if (expires <= Date.now()) dismissed.current.delete(key);
        const next = pending.find(
          (entry) =>
            Date.now() >= quietUntil.current &&
            Date.parse(entry.expiresAt) > Date.now() &&
            !dismissed.current.has(`${entry.code}:${entry.expiresAt}`),
        );
        setOffered((current) => {
          if (
            current?.client === client &&
            (current.finished ||
              pending.some(
                (entry) =>
                  entry.code === current.request.code &&
                  entry.expiresAt === current.request.expiresAt,
              ))
          )
            return current;
          return next ? { client, request: next } : null;
        });
      } catch {
        delay = 30000; // QR and manual approval remain available offline.
      }
      if (active) timer = setTimeout(poll, delay);
    };
    void poll();
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [client, allowed]);
  const close = () => {
    if (request) {
      if (!offered?.finished) quietUntil.current = Date.now() + 60000;
      if (dismissed.current.size >= 64)
        dismissed.current.delete(dismissed.current.keys().next().value!);
      dismissed.current.set(
        `${request.code}:${request.expiresAt}`,
        Date.parse(request.expiresAt),
      );
    }
    setOffered(null);
  };
  if (!client || !request || !allowed) return null;
  return (
    <Modal transparent animationType="none" visible onRequestClose={close}>
      <View
        style={{
          flex: 1,
          backgroundColor: theme.background,
          justifyContent: 'center',
          padding: 24,
        }}
      >
        <ScrollView
          contentContainerStyle={{ flexGrow: 1, justifyContent: 'center' }}
        >
          <View
            accessibilityViewIsModal
            style={{
              width: '100%',
              maxWidth: 480,
              alignSelf: 'center',
              padding: 24,
              borderRadius: radius.panel,
              backgroundColor: theme.surface,
            }}
          >
            <DeviceApprovalPanel
              key={`${request.code}:${request.expiresAt}`}
              client={client}
              code={request.code}
              onClose={close}
              onApproved={() =>
                setOffered((current) =>
                  current?.client === client
                    ? { ...current, finished: true }
                    : null,
                )
              }
            />
          </View>
        </ScrollView>
      </View>
    </Modal>
  );
}
