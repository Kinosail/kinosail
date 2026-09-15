import React, { useEffect, useRef, useState } from 'react';
import { Linking, Platform, Text, View } from 'react-native';
import type {
  CastingClient,
  CastDevice,
  CastSession,
  TVPlayback,
} from '@/core/casting';
import {
  GoogleCastPicker,
  googleCastAvailable,
  connectedGoogleTV,
  connectGoogleCast,
} from '@/core/google-cast';
import { spacing, useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';
import { AirPlayButton } from './airplay-button';
import { ModalSheet } from './modal-sheet';

export function PlayOnTV({
  client,
  itemId,
  video,
  getPosition,
  playbackToken = '',
  initiallyOpen = false,
  airPlayCompatible = true,
  onPrepareAirPlay,
  onConnected,
}: {
  client?: CastingClient;
  itemId: string;
  video: boolean;
  getPosition(): number;
  playbackToken?: string;
  initiallyOpen?: boolean;
  airPlayCompatible?: boolean;
  onPrepareAirPlay?: () => void;
  onConnected(playback: TVPlayback): Promise<void> | void;
}) {
  const theme = useKinoTheme();
  const [open, setOpen] = useState(initiallyOpen),
    [busy, setBusy] = useState(false);
  const [message, setMessage] = useState(''),
    [devices, setDevices] = useState<CastDevice[]>([]);
  const [scanning, setScanning] = useState(false);
  const [scanError, setScanError] = useState('');
  const [scanVersion, setScanVersion] = useState(0);
  const [googleName, setGoogleName] = useState('');
  const [showHelp, setShowHelp] = useState(false);
  const active = useRef(true),
    working = useRef(false);
  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
    };
  }, []);
  useEffect(() => {
    if (!open || Platform.isTV) return;
    let current = true;
    if (googleCastAvailable)
      void connectedGoogleTV().then(
        (name) => {
          if (current) setGoogleName(name);
        },
        () => {
          if (current) setGoogleName('');
        },
      );
    if (client) {
      setScanning(true);
      setScanError('');
      void client
        .scanCastDevices()
        .then(
          (found) => {
            if (current) setDevices(found);
          },
          () => {
            if (current)
              setScanError(
                'Could not find TVs on the Server network. Try searching again.',
              );
          },
        )
        .finally(() => {
          if (current) setScanning(false);
        });
    }
    return () => {
      current = false;
    };
  }, [open, client, scanVersion]);
  if (Platform.isTV) return null;
  const run = async (work: () => Promise<void>) => {
    if (working.current) return;
    working.current = true;
    setBusy(true);
    setMessage('');
    try {
      await work();
    } catch (error) {
      if (active.current)
        setMessage(
          error instanceof Error
            ? error.message
            : 'Could not connect to the TV.',
        );
    } finally {
      working.current = false;
      if (active.current) setBusy(false);
    }
  };
  const start = (device?: CastDevice) =>
    run(async () => {
      if (!open || !active.current) return;
      if (!client)
        throw new Error('Connect to Kinosail Server to play on a TV.');
      if (!device) {
        const name = await connectedGoogleTV();
        if (active.current) setGoogleName(name);
      }
      let session: CastSession | undefined;
      let connected: TVPlayback | undefined;
      try {
        session = await client.startCast(itemId, {
          protocol: device ? 'dlna' : 'google-cast',
          ...(device ? { deviceId: device.id } : {}),
          position: getPosition(),
          playbackToken,
        });
        const selected = session;
        const controller = device
          ? {
              name: device.name,
              status: () => client.castStatus(selected.id),
              command: (command: import('@/core/casting').CastCommand) =>
                client.castCommand(selected.id, command),
            }
          : await connectGoogleCast(session);
        connected = { session, controller };
        if (!active.current) {
          await controller.command({ action: 'stop' }).catch(() => {});
          await client.endCast(session.id);
          return;
        }
        await onConnected(connected);
        if (active.current) setOpen(false);
      } catch (error) {
        if (connected)
          await connected.controller
            .command({ action: 'stop' })
            .catch(() => {});
        if (session) await client.endCast(session.id).catch(() => {});
        throw error;
      }
    });
  const help = { color: theme.muted, fontSize: 15, lineHeight: 22 };
  return (
    <>
      <ActionButton
        label="Play on TV"
        quiet
        onPress={() => setOpen(true)}
        style={{ minHeight: 56 }}
      />
      <ModalSheet
        visible={open}
        title="Play on TV"
        dismissDisabled={busy}
        onClose={() => {
          if (!working.current) setOpen(false);
        }}
      >
        <Text style={help}>Choose a TV to continue watching there.</Text>
        {devices.map((device) => (
          <ActionButton
            key={device.id}
            label={`${device.name} · DLNA`}
            quiet
            busy={busy}
            onPress={() => void start(device)}
          />
        ))}
        {googleName ? (
          <ActionButton
            label={`${googleName} · Google Cast`}
            busy={busy}
            disabled={!client}
            onPress={() => void start()}
          />
        ) : null}
        {Platform.OS === 'ios' && !busy ? (
          airPlayCompatible ? (
            <AirPlayButton video={video} />
          ) : onPrepareAirPlay ? (
            <ActionButton
              label="Prepare for AirPlay"
              quiet
              onPress={onPrepareAirPlay}
            />
          ) : null
        ) : null}
        {googleCastAvailable && open && !busy && client ? (
          <GoogleCastPicker onConnected={() => void start()} />
        ) : null}
        {busy ? (
          <Text accessibilityLiveRegion="polite" style={help}>
            Connecting to your TV…
          </Text>
        ) : null}
        {scanning ? (
          <Text accessibilityLiveRegion="polite" style={help}>
            Finding nearby TVs…
          </Text>
        ) : null}
        {scanError ? (
          <Text accessibilityRole="alert" style={help}>
            {scanError}
          </Text>
        ) : null}
        {!scanning && !devices.length ? (
          <Text style={help}>
            No DLNA TVs found. AirPlay and Google Cast list their devices when
            you open their pickers.
          </Text>
        ) : null}
        {message ? (
          <Text accessibilityRole="alert" style={{ color: theme.text }}>
            {message}
          </Text>
        ) : null}
        <ActionButton
          label="Search again"
          quiet
          busy={scanning}
          disabled={!client || busy}
          onPress={() => setScanVersion((value) => value + 1)}
        />
        <ActionButton
          label="Can’t find your TV?"
          quiet
          expanded={showHelp}
          onPress={() => setShowHelp(!showHelp)}
        />
        {showHelp ? (
          <View style={{ gap: spacing.two }}>
            <Text style={help}>
              Keep the TV, phone, and Kinosail Server on the same network. Turn
              on your TV and allow local network access for Kinosail.
            </Text>
            <Text style={help}>
              AirPlay supports Apple TV and compatible smart TVs. Google Cast
              starts playback after you select a receiver. DLNA TVs may need
              media sharing or renderer mode enabled.
            </Text>
            <Text style={help}>
              {Platform.OS === 'android'
                ? 'For screen mirroring, use your phone’s Cast or Smart View controls. Availability depends on the phone and TV.'
                : 'For screen mirroring, open Control Center → Screen Mirroring and choose an AirPlay-compatible TV.'}
            </Text>
            {Platform.OS === 'android' ? (
              <ActionButton
                label="Open screen casting settings"
                quiet
                onPress={() =>
                  void run(async () => {
                    try {
                      await Linking.sendIntent(
                        'android.settings.CAST_SETTINGS',
                      );
                    } catch {
                      throw new Error(
                        'Open Cast, Smart View, or Screen mirroring from your phone’s quick settings.',
                      );
                    }
                  })
                }
              />
            ) : null}
          </View>
        ) : null}
      </ModalSheet>
    </>
  );
}
