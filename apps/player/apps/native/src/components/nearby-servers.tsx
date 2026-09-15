import React, { useEffect, useState } from 'react';
import { Text, View } from 'react-native';
import {
  discoverServers,
  serverDiscoveryAvailable,
  stopServerDiscovery,
  type NearbyServer,
} from '@/core/server-discovery';
import { useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';
import { setupFlowStyles as styles } from './setup-flow.styles';

export function NearbyServers({
  disabled,
  onSelect,
}: {
  disabled: boolean;
  onSelect(url: string): void;
}) {
  const theme = useKinoTheme();
  const [servers, setServers] = useState<NearbyServer[]>([]);
  const [attempt, setAttempt] = useState(0);
  const [searching, setSearching] = useState(true);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    if (!serverDiscoveryAvailable) return;
    let active = true;
    setSearching(true);
    setFailed(false);
    setServers([]);
    void discoverServers()
      .then((found) => {
        if (active) setServers(found);
      })
      .catch(() => {
        if (active) setFailed(true);
      })
      .finally(() => {
        if (active) setSearching(false);
      });
    return () => {
      active = false;
      void stopServerDiscovery().catch(() => {});
    };
  }, [attempt]);
  if (!serverDiscoveryAvailable) return null;
  return (
    <View style={styles.form}>
      <Text
        accessibilityRole="header"
        style={[styles.label, { color: theme.text }]}
      >
        Servers on your network
      </Text>
      <Text
        accessibilityLiveRegion="polite"
        style={[styles.body, { color: theme.muted }]}
      >
        {searching
          ? 'Looking for Kinosail Servers…'
          : failed
            ? 'Local discovery is unavailable. Try again or enter an address below.'
            : servers.length
              ? 'Select your server to continue.'
              : 'No servers found. Check that your server is running on the same network, or enter its address below.'}
      </Text>
      {servers.map((server) => (
        <View key={server.url}>
          <ActionButton
            label={server.name}
            accessibilityLabel={`Connect to ${server.name}, ${server.url}`}
            quiet
            disabled={disabled}
            onPress={() => onSelect(server.url)}
          />
          <Text style={[styles.body, { color: theme.muted }]}>
            {server.url}
          </Text>
        </View>
      ))}
      {!searching && (
        <ActionButton
          label="Search again"
          quiet
          disabled={disabled}
          onPress={() => setAttempt(attempt + 1)}
        />
      )}
    </View>
  );
}
