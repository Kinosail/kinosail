import React, { useEffect, useRef, useState } from 'react';
import {
  KeyboardAvoidingView,
  Linking,
  Platform,
  ScrollView,
  Text,
  TextInput,
  useWindowDimensions,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { normalizeServerURL, type KinosailClient } from '@/core/server-client';
import type { Session } from '@/core/session-store';
import { useKinoTheme } from '@/design/tokens';

import { ActionButton } from './action-button';
import { BrandMark } from './brand-mark';
import { NearbyServers } from './nearby-servers';
import { ApprovalQR } from './approval-qr';
import { approvalURL } from '@/core/approval-link';
import { CopyCodeButton } from './copy-code-button';
import { setupFlowStyles as styles } from './setup-flow.styles';

type ConnectClient = Pick<
  KinosailClient,
  'startQuickConnect' | 'pollQuickConnect' | 'cancelQuickConnect'
>;

type Props = {
  createClient(baseURL: string): ConnectClient;
  onConnected(session: Session): void | Promise<void>;
};

const pollInterval = 1000;

export function SetupFlow({ createClient, onConnected }: Props) {
  const theme = useKinoTheme();
  const { fontScale, width } = useWindowDimensions();
  const accessible = !Platform.isTV && fontScale >= 1.5;
  const compact = (!Platform.isTV && width < 420) || accessible;
  const wide = width >= (Platform.isTV ? 1100 : 820) && !compact;
  const [server, setServer] = useState('');
  const [pending, setPending] = useState(false);
  const connectionPending = useRef(false);
  const [challenge, setChallenge] = useState<{
    baseURL: string;
    code: string;
    secret: string;
  } | null>(null);
  const [error, setError] = useState('');
  useEffect(() => {
    if (!challenge) return;
    const client = createClient(challenge.baseURL);
    let active = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const poll = async () => {
      try {
        const token = await client.pollQuickConnect(challenge.secret);
        if (active && token) {
          await onConnected({ baseURL: challenge.baseURL, token });
        } else if (active) {
          timer = setTimeout(poll, pollInterval);
        }
      } catch (reason) {
        if (active) {
          setError(
            reason instanceof Error ? reason.message : 'Connection failed.',
          );
        }
      }
    };
    void poll();
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [challenge, createClient, onConnected]);

  const challengeServer = challenge?.baseURL,
    challengeSecret = challenge?.secret;
  useEffect(() => {
    if (!challengeServer || !challengeSecret) return;
    const client = createClient(challengeServer);
    return () => {
      void client.cancelQuickConnect(challengeSecret).catch(() => {});
    };
  }, [challengeServer, challengeSecret, createClient]);

  const connectTo = async (address: string) => {
    if (connectionPending.current) return;
    connectionPending.current = true;
    setError('');
    setPending(true);
    try {
      const baseURL = normalizeServerURL(address);
      const next = await createClient(baseURL).startQuickConnect(
        Platform.isTV ? 'Kinosail TV' : 'Kinosail Player',
      );
      setChallenge({ baseURL, ...next });
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Connection failed.');
    } finally {
      connectionPending.current = false;
      setPending(false);
    }
  };

  const connect = () => connectTo(server);

  return (
    <SafeAreaView
      style={[styles.screen, { backgroundColor: theme.background }]}
    >
      <KeyboardAvoidingView
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        role="main"
        style={styles.screen}
      >
        {/* Remeasure native text after a live Dynamic Type change. */}
        <ScrollView
          key={fontScale}
          contentContainerStyle={[
            styles.scroll,
            compact ? styles.scrollCompact : null,
            { flexDirection: wide ? 'row' : 'column' },
            Platform.isTV && challenge ? { padding: 32, gap: 48 } : null,
          ]}
          keyboardShouldPersistTaps="handled"
        >
          <View
            style={[
              styles.intro,
              compact ? styles.introCompact : null,
              {
                flexShrink: wide ? 1 : 0,
                flexBasis: wide
                  ? Platform.isTV && challenge
                    ? '42%'
                    : '48%'
                  : 'auto',
              },
            ]}
          >
            <BrandMark compact={accessible} />
            {challenge && Platform.isTV ? (
              <>
                <ApprovalQR
                  server={challenge.baseURL}
                  code={challenge.code}
                  size={320}
                />
                <Text style={[styles.body, { color: theme.muted }]}>
                  Scan with your phone camera
                </Text>
              </>
            ) : !accessible && !(compact && challenge) ? (
              <>
                <Text
                  style={[
                    styles.eyebrow,
                    compact ? styles.eyebrowCompact : null,
                    { color: theme.signal },
                  ]}
                >
                  YOUR MEDIA · DIRECT
                </Text>
                <Text
                  style={[
                    styles.title,
                    compact ? styles.titleCompact : null,
                    { color: theme.text },
                  ]}
                >
                  The shortest path to play.
                </Text>
                <Text style={[styles.summary, { color: theme.muted }]}>
                  Connect to your Kinosail Server. Your library and viewing
                  history stay under your control.
                </Text>
                {!compact ? (
                  <>
                    <View
                      style={[styles.rule, { backgroundColor: theme.line }]}
                    />
                    <Text style={[styles.note, { color: theme.muted }]}>
                      Your server, your library. Pick up where you left off on
                      your phone, tablet, or TV.
                    </Text>
                  </>
                ) : null}
              </>
            ) : null}
          </View>

          <View
            style={[
              styles.panel,
              compact ? styles.panelCompact : null,
              wide ? { flexShrink: 1 } : null,
              { backgroundColor: theme.surface, borderColor: theme.line },
              Platform.isTV && challenge
                ? { flexBasis: 600, flexShrink: 1, padding: 32 }
                : null,
            ]}
          >
            <Text style={[styles.step, { color: theme.muted }]}>
              {challenge ? 'STEP 2 OF 2' : 'STEP 1 OF 2'}
            </Text>
            {challenge ? (
              <View style={styles.challenge}>
                <Text
                  accessibilityRole="header"
                  style={[styles.panelTitle, { color: theme.text }]}
                >
                  Sign in with your phone
                </Text>
                {error ? (
                  <Text
                    accessibilityLiveRegion="assertive"
                    style={[styles.error, { color: theme.danger }]}
                  >
                    {error}
                  </Text>
                ) : null}
                <Text style={[styles.body, { color: theme.muted }]}>
                  {Platform.isTV
                    ? 'Open Player on your phone and approve the TV sign-in request. Or scan the QR code.'
                    : 'Approve from a signed-in Player app or browser.'}
                </Text>
                {!Platform.isTV && Platform.OS === 'web' && width >= 820 ? (
                  <ApprovalQR
                    server={challenge.baseURL}
                    code={challenge.code}
                  />
                ) : null}
                <Text style={[styles.body, { color: theme.muted }]}>
                  Or use this code in Player Settings → Connect a TV:
                </Text>
                <Text
                  selectable
                  style={[
                    styles.code,
                    compact && width < 360 ? styles.codeCompact : null,
                    { color: theme.signal },
                  ]}
                >{`${challenge.code.slice(0, 3)} ${challenge.code.slice(3)}`}</Text>
                {!Platform.isTV ? (
                  <ActionButton
                    label="Open approval page"
                    onPress={() => {
                      void Linking.openURL(
                        approvalURL(challenge.baseURL, challenge.code),
                      ).catch(() =>
                        setError(
                          'Could not open the approval page. Use the code in a signed-in Player browser.',
                        ),
                      );
                    }}
                  />
                ) : null}
                {!Platform.isTV ? (
                  <CopyCodeButton
                    key={challenge.secret}
                    code={challenge.code}
                  />
                ) : null}
                <Text
                  accessibilityLiveRegion="polite"
                  style={[styles.waiting, { color: theme.muted }]}
                >
                  {error ? 'Approval check stopped.' : 'Waiting for approval…'}
                </Text>
                {error ? (
                  <ActionButton
                    label="Retry approval"
                    onPress={() => {
                      setError('');
                      setChallenge({ ...challenge });
                    }}
                  />
                ) : null}
                {error ? (
                  <ActionButton
                    label="Get a new code"
                    quiet
                    onPress={() => {
                      void connectTo(challenge.baseURL);
                    }}
                  />
                ) : null}
                <ActionButton
                  label="Use another server"
                  quiet
                  onPress={() => {
                    setChallenge(null);
                    setError('');
                  }}
                />
              </View>
            ) : (
              <View style={styles.form}>
                <Text
                  accessibilityRole="header"
                  style={[styles.panelTitle, { color: theme.text }]}
                >
                  Find your server
                </Text>
                <NearbyServers
                  disabled={pending}
                  onSelect={(url) => {
                    setServer(url);
                    void connectTo(url);
                  }}
                />
                <Text style={[styles.body, { color: theme.muted }]}>
                  Use the trusted HTTPS address, or a private local network
                  address.
                </Text>
                <Text style={[styles.label, { color: theme.text }]}>
                  Kinosail Server URL
                </Text>
                <TextInput
                  accessibilityLabel="Kinosail Server URL"
                  autoCapitalize="none"
                  autoCorrect={false}
                  editable={!pending}
                  keyboardType="url"
                  onChangeText={setServer}
                  onSubmitEditing={connect}
                  placeholder="https://player.example.com"
                  placeholderTextColor={theme.muted}
                  returnKeyType="go"
                  style={[
                    styles.input,
                    {
                      backgroundColor: theme.background,
                      borderColor: error ? theme.danger : theme.line,
                      color: theme.text,
                    },
                  ]}
                  value={server}
                />
                {error ? (
                  <Text
                    accessibilityLiveRegion="assertive"
                    style={[styles.error, { color: theme.danger }]}
                  >
                    {error}
                  </Text>
                ) : null}
                <ActionButton
                  label={pending ? 'Connecting…' : 'Connect'}
                  busy={pending}
                  onPress={connect}
                />
              </View>
            )}
          </View>
        </ScrollView>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
