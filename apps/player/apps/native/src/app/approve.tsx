import React, { useState } from 'react';
import { router, useLocalSearchParams } from 'expo-router';
import { ScrollView, Text, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useSession } from '@/core/session-context';
import { readApprovalLink } from '@/core/approval-link';
import { approvalCode } from '@/core/device-approval';
import { radius, useKinoTheme } from '@/design/tokens';
import { ActionButton } from '@/components/action-button';
import { DeviceApprovalPanel } from '@/components/device-approval-panel';

export default function ApproveScreen() {
  const { client, session, booting } = useSession(),
    theme = useKinoTheme();
  const params = useLocalSearchParams();
  const [input, setInput] = useState(''),
    [manualCode, setManualCode] = useState(''),
    [error, setError] = useState('');
  const close = () => router.replace('/');
  let code = manualCode,
    linkError = '';
  if (Object.keys(params).length && session) {
    try {
      code = readApprovalLink(params, session.baseURL);
    } catch (reason) {
      linkError =
        reason instanceof Error
          ? reason.message
          : 'This sign-in link is invalid.';
    }
  }
  const review = () => {
    try {
      setManualCode(approvalCode(input));
      setError('');
    } catch (reason) {
      setError(
        reason instanceof Error ? reason.message : 'Enter the code on your TV.',
      );
    }
  };
  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: theme.background }}>
      <ScrollView
        keyboardShouldPersistTaps="handled"
        contentContainerStyle={{
          flexGrow: 1,
          justifyContent: 'center',
          padding: 24,
        }}
      >
        <View
          style={{ width: '100%', maxWidth: 480, alignSelf: 'center', gap: 20 }}
        >
          {booting ? (
            <Text style={{ color: theme.muted }}>Opening Player…</Text>
          ) : !client ? (
            <>
              <Text
                accessibilityRole="header"
                style={{ fontSize: 28, color: theme.text }}
              >
                Sign in to approve your TV
              </Text>
              <Text style={{ color: theme.muted }}>
                Connect this app to your Server first, then scan the TV again.
                You can also return to your camera’s link and choose Use
                browser.
              </Text>
              <ActionButton label="Open Player" onPress={close} />
            </>
          ) : linkError ? (
            <>
              <Text
                accessibilityLiveRegion="assertive"
                style={{ color: theme.danger }}
              >
                {linkError}
              </Text>
              <ActionButton label="Back to Player" onPress={close} />
            </>
          ) : code ? (
            <DeviceApprovalPanel client={client} code={code} onClose={close} />
          ) : (
            <>
              <Text
                accessibilityRole="header"
                style={{ fontSize: 28, fontWeight: '800', color: theme.text }}
              >
                Connect a TV
              </Text>
              <Text style={{ color: theme.muted }}>
                Enter the six-digit code shown on your TV. You’ll review the
                request before approving it.
              </Text>
              <TextInput
                accessibilityLabel="Six-digit TV code"
                value={input}
                onChangeText={(value) =>
                  setInput(value.replace(/\D/g, '').slice(0, 6))
                }
                maxLength={6}
                keyboardType="number-pad"
                autoComplete="one-time-code"
                returnKeyType="go"
                onSubmitEditing={review}
                style={{
                  color: theme.text,
                  backgroundColor: theme.surface,
                  borderRadius: radius.control,
                  borderWidth: 1,
                  borderColor: theme.line,
                  minHeight: 56,
                  padding: 12,
                  fontSize: 28,
                  letterSpacing: 4,
                }}
              />
              {error ? (
                <Text
                  accessibilityLiveRegion="assertive"
                  style={{ color: theme.danger }}
                >
                  {error}
                </Text>
              ) : null}
              <ActionButton
                label="Review device"
                disabled={input.length !== 6}
                onPress={review}
              />
              <ActionButton label="Cancel" quiet onPress={close} />
            </>
          )}
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}
