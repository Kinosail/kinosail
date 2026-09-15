import React, { useState } from 'react';
import { Platform, Text, View } from 'react-native';

import { useKinoTheme } from '@/design/tokens';

import { ActionButton } from './action-button';
import { setupFlowStyles as styles } from './setup-flow.styles';

export function CopyCodeButton({ code }: { code: string }) {
  const theme = useKinoTheme();
  const [status, setStatus] = useState('');
  const [busy, setBusy] = useState(false);
  if (Platform.isTV) return null;
  const copy = async () => {
    setBusy(true);
    setStatus('');
    try {
      const Clipboard = await import('expo-clipboard');
      const copied = await Clipboard.setStringAsync(code);
      setStatus(
        copied ? 'Code copied.' : 'Could not copy. Select the code to copy it.',
      );
    } catch {
      setStatus('Could not copy. Select the code to copy it.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <View style={styles.form}>
      <ActionButton label="Copy code" quiet busy={busy} onPress={copy} />
      {status ? (
        <Text
          accessibilityLiveRegion="polite"
          style={[styles.waiting, { color: theme.muted }]}
        >
          {status}
        </Text>
      ) : null}
    </View>
  );
}
