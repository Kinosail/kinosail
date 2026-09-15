import { maintainDownloads } from './smart-downloads';
import { useEffect } from 'react';
import { AppState } from 'react-native';
import * as Network from 'expo-network';
import type { KinosailClient } from './server-client';
import { flushProgress } from './progress-sync';
export function MediaCoordinator({
  client,
}: {
  client: KinosailClient | null;
}) {
  useEffect(() => {
    if (!client) return;
    let active = true,
      running = false;
    const refresh = async () => {
      if (!active || running) return;
      running = true;
      try {
        await flushProgress(client);
        await maintainDownloads(client);
      } catch {
        /* Settings exposes unreadable or pending state. */
      } finally {
        running = false;
      }
    };
    void refresh();
    const state = AppState.addEventListener('change', (value) => {
      if (value === 'active') void refresh();
    });
    const network = Network.addNetworkStateListener((value) => {
      if (value.isConnected) void refresh();
    });
    const timer = setInterval(() => {
      if (AppState.currentState === 'active') void refresh();
    }, 60000);
    return () => {
      active = false;
      state.remove();
      network.remove();
      clearInterval(timer);
    };
  }, [client]);
  return null;
}
