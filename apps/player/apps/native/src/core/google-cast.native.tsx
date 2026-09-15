import React, { useEffect } from 'react';
import { useKinoTheme } from '@/design/tokens';
import { NativeModules, Platform, StyleSheet, Text, View } from 'react-native';
import type { CastSession, CastController } from './casting';
import { parseCastStatus, validateCastCommand } from './casting';

// TV builds and web must never load the mobile Cast SDK.
export const googleCastAvailable =
  !Platform.isTV && Boolean(NativeModules.RNGCCastContext);
const sdk = () => {
  if (!googleCastAvailable)
    throw new Error('Google Cast is unavailable in this build.');
  return require('react-native-google-cast') as typeof import('react-native-google-cast');
};
export function GoogleCastPicker({
  onConnected,
}: {
  onConnected?: () => void;
}) {
  const theme = useKinoTheme();
  useEffect(() => {
    if (!googleCastAvailable || !onConnected) return;
    let active = true;
    const subscription = sdk()
      .default.getSessionManager()
      .onSessionStarted(() => {
        if (active) onConnected();
      });
    return () => {
      active = false;
      subscription.remove();
    };
  }, [onConnected]);
  if (!googleCastAvailable) return null;
  const CastButton = sdk().CastButton;
  return (
    <View
      style={[
        styles.picker,
        { backgroundColor: theme.raised, borderColor: theme.line },
      ]}
    >
      <Text
        accessible={false}
        pointerEvents="none"
        style={{ color: theme.text, padding: 16 }}
      >
        Choose a TV · Google Cast
      </Text>
      <CastButton
        accessibilityLabel="Choose a Google Cast or Chromecast TV and play"
        tintColor="transparent"
        style={StyleSheet.absoluteFill}
      />
    </View>
  );
}
const current = async () => {
  const session = await sdk()
    .default.getSessionManager()
    .getCurrentCastSession();
  if (!session)
    throw new Error('Choose a TV with the Google Cast button first.');
  return session;
};
export async function connectedGoogleTV(): Promise<string> {
  const session = await current(),
    device = await session.getCastDevice();
  const name = device?.friendlyName;
  return typeof name === 'string' &&
    name.trim() &&
    name.length <= 128 &&
    !/[\u0000-\u001f\u007f]/.test(name)
    ? name
    : 'Google Cast TV';
}
export async function connectGoogleCast(
  media: CastSession,
): Promise<CastController> {
  const session = await current(),
    name = await connectedGoogleTV(),
    client = session.client;
  await client.loadMedia({
    autoplay: true,
    startTime: media.position,
    mediaInfo: {
      contentUrl: media.url,
      contentType: media.contentType,
      contentId: `kinosail:${media.id}`,
      streamDuration: media.duration,
      streamType:
        'buffered' as import('react-native-google-cast').MediaStreamType,
      metadata: { type: 'generic', title: media.title },
      mediaTracks: media.tracks.map((track) => ({
        id: track.id,
        contentId: track.url,
        contentType: 'text/vtt',
        language: track.language,
        name: track.label,
        type: 'text',
        subtype: 'subtitles',
      })),
      ...(media.contentType === 'application/vnd.apple.mpegurl'
        ? {
            hlsSegmentFormat:
              'FMP4' as import('react-native-google-cast').MediaHlsSegmentFormat,
            hlsVideoSegmentFormat:
              'FMP4' as import('react-native-google-cast').MediaHlsVideoSegmentFormat,
          }
        : {}),
    },
  });
  const enabled = media.tracks
    .filter((track) => track.default)
    .map((track) => track.id);
  try {
    if (enabled.length) await client.setActiveTrackIds(enabled);
  } catch (error) {
    await client.stop().catch(() => {});
    throw error;
  }
  const status = async () => {
    const active = await current();
    if (active.id !== session.id) throw new Error('The TV connection changed.');
    const value = await client.getMediaStatus();
    if (!value || value.mediaInfo?.contentId !== `kinosail:${media.id}`)
      throw new Error('The TV is no longer playing this title.');
    if (value.playerState === 'idle' && value.idleReason === 'error')
      throw new Error('The TV could not play this title.');
    return parseCastStatus({
      state:
        value.playerState === 'idle'
          ? 'stopped'
          : value.playerState === 'loading'
            ? 'buffering'
            : value.playerState,
      position: value.streamPosition,
      duration: media.duration,
    });
  };
  return {
    name,
    status,
    command: async (command) => {
      validateCastCommand(command);
      await status(); // Never control another app or a replacement casting session.
      if (command.action === 'seek') {
        if (command.position > media.duration)
          throw new Error('Seek is outside this title.');
        await client.seek({ position: command.position });
      } else if (command.action === 'stop') await client.stop();
      else if (command.action === 'pause') await client.pause();
      else await client.play();
    },
  };
}
const styles = StyleSheet.create({
  picker: {
    minHeight: 56,
    borderRadius: 12,
    borderWidth: 1,
    justifyContent: 'center',
  },
});
