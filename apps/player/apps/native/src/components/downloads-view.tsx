import React, { useState } from 'react';
import { Pressable, ScrollView, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import type { DownloadEntry } from '@/core/downloads.types';
import { useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';
import { ModalSheet } from './modal-sheet';
import { NavigationIcon } from './navigation-icon';

export function downloadBytes(bytes: number) {
  return bytes >= 1024 ** 3
    ? `${(bytes / 1024 ** 3).toFixed(1)} GB`
    : `${Math.ceil(bytes / 1024 ** 2)} MB`;
}
export function downloadStatus(entry: DownloadEntry) {
  if (entry.status === 'verifying') return 'Checking offline playback…';
  if (entry.status === 'complete') return 'Ready offline';
  if (entry.status === 'preparing') return 'Preparing on the Server';
  if (entry.status === 'paused' && entry.total === 0) return 'Download paused';
  if (entry.status === 'queued') return 'Queued · starts automatically';
  if (entry.status === 'waiting') return 'Waiting for a connection';
  if (entry.status === 'pausing') return 'Pausing…';
  return `${entry.status === 'paused' ? 'Paused' : 'Downloading'} · ${Math.floor((entry.bytes / entry.total) * 100)}%`;
}
type Props = {
  entries: DownloadEntry[];
  progress?: Record<string, string>;
  limitGiB?: number;
  background: boolean;
  error: string;
  onCheck?(): Promise<void>;
  onPause(entry: DownloadEntry): Promise<void>;
  onResume(entry: DownloadEntry): Promise<void>;
  onRemove(entry: DownloadEntry): Promise<void>;
  onPlay(entry: DownloadEntry): void;
  onBrowse(): void;
};
export function DownloadsView(props: Props) {
  const theme = useKinoTheme();
  const [selectedID, setSelectedID] = useState('');
  const [busy, setBusy] = useState(false);
  const [confirmRemoval, setConfirmRemoval] = useState(false);
  const [focusedID, setFocusedID] = useState('');
  const selected = props.entries.find((entry) => entry.item.id === selectedID);
  const ready = props.entries.filter((entry) => entry.status === 'complete');
  const queue = props.entries.filter((entry) => entry.status !== 'complete');
  const used = props.entries.reduce((sum, entry) => sum + entry.bytes, 0);
  const run = async (
    action: (entry: DownloadEntry) => Promise<void>,
    close = false,
  ) => {
    if (!selected || busy) return;
    setBusy(true);
    try {
      await action(selected);
      if (close) setSelectedID('');
    } catch {
      // The screen supplies the error; keep the title's actions available.
    } finally {
      setBusy(false);
    }
  };
  return (
    <SafeAreaView
      edges={['top', 'left', 'right']}
      style={{ flex: 1, backgroundColor: theme.background }}
    >
      <ScrollView
        contentContainerStyle={{ padding: 24, paddingBottom: 80, gap: 24 }}
      >
        <View style={{ gap: 8 }}>
          <Text
            accessibilityRole="header"
            style={{ fontSize: 34, fontWeight: '700', color: theme.text }}
          >
            Downloads
          </Text>
          <Text style={{ color: theme.muted, fontSize: 17 }}>
            Your library, wherever you go.
          </Text>
        </View>
        <View
          style={{
            borderRadius: 18,
            padding: 20,
            backgroundColor: theme.surface,
            gap: 12,
          }}
        >
          <Text style={{ color: theme.text, fontSize: 24, fontWeight: '600' }}>
            {ready.length} {ready.length === 1 ? 'title' : 'titles'} ready
            offline
          </Text>
          <Text style={{ color: theme.muted }}>
            {downloadBytes(used)} on this device
            {props.limitGiB === 0
              ? ' · No download storage limit'
              : props.limitGiB
                ? ` · ${props.limitGiB} GB download limit`
                : ''}
          </Text>
          {props.limitGiB ? (
            <View
              style={{
                height: 5,
                borderRadius: 3,
                backgroundColor: theme.line,
                overflow: 'hidden',
              }}
            >
              <View
                style={{
                  height: 5,
                  width: `${Math.min(100, (used / (props.limitGiB * 1024 ** 3)) * 100)}%`,
                  backgroundColor: theme.signal,
                }}
              />
            </View>
          ) : null}
          <Text style={{ color: theme.muted, lineHeight: 21 }}>
            {props.background
              ? 'Keep browsing or lock your phone. Downloads continue in the background. If a transfer stops, resume it here.'
              : 'Keep the app open while downloading on this device. You can pause and resume anytime.'}
          </Text>
        </View>
        {props.onCheck && ready.length ? (
          <ActionButton
            label="Check trip readiness"
            quiet
            onPress={() => {
              void props.onCheck?.();
            }}
          />
        ) : null}
        {props.error ? (
          <Text accessibilityRole="alert" style={{ color: theme.text }}>
            {props.error}
          </Text>
        ) : null}
        {!props.entries.length ? (
          <View style={{ paddingVertical: 32, gap: 16 }}>
            <NavigationIcon name="download" color={theme.signal} />
            <Text
              style={{ color: theme.text, fontSize: 24, fontWeight: '600' }}
            >
              Take something good with you.
            </Text>
            <Text style={{ color: theme.muted, fontSize: 16, lineHeight: 24 }}>
              Open a movie, episode, or audiobook and choose Download. Your
              saved titles will be here, ready without a connection.
            </Text>
            <ActionButton
              label="Find something to download"
              onPress={props.onBrowse}
            />
          </View>
        ) : null}
        {(
          [
            ['In progress', queue],
            ['Ready offline', ready],
          ] as const
        ).map(([title, entries]) =>
          entries.length ? (
            <View key={title} style={{ gap: 12 }}>
              <Text
                accessibilityRole="header"
                style={{ color: theme.text, fontSize: 21, fontWeight: '700' }}
              >
                {title} · {entries.length}
              </Text>
              {entries.map((entry) => (
                <Pressable
                  key={entry.item.id}
                  accessibilityRole="button"
                  accessibilityLabel={`${entry.item.title}. ${downloadStatus(entry)}. Download options`}
                  focusable
                  onPress={() => {
                    setConfirmRemoval(false);
                    setSelectedID(entry.item.id);
                  }}
                  onFocus={() => setFocusedID(entry.item.id)}
                  onBlur={() => setFocusedID('')}
                  style={({ pressed }) => ({
                    padding: 18,
                    borderRadius: 16,
                    borderWidth: 2,
                    borderColor:
                      focusedID === entry.item.id ? theme.focus : theme.line,
                    backgroundColor: pressed ? theme.raised : theme.surface,
                    gap: 10,
                  })}
                >
                  <Text
                    style={{
                      color: theme.text,
                      fontSize: 19,
                      fontWeight: '600',
                    }}
                  >
                    {entry.item.title}
                  </Text>
                  <Text
                    style={{
                      color:
                        entry.status === 'complete'
                          ? theme.signal
                          : theme.muted,
                    }}
                  >
                    {downloadStatus(entry)}
                  </Text>
                  {entry.status === 'downloading' &&
                  props.progress?.[entry.item.id] ? (
                    <Text style={{ color: theme.muted, fontSize: 13 }}>
                      {props.progress[entry.item.id]}
                    </Text>
                  ) : null}
                  {entry.status !== 'complete' && entry.total > 0 ? (
                    <View
                      accessibilityRole="progressbar"
                      accessibilityLabel={`${entry.item.title} download progress`}
                      accessibilityValue={{
                        min: 0,
                        max: 100,
                        now: Math.floor((entry.bytes / entry.total) * 100),
                      }}
                      style={{
                        height: 5,
                        borderRadius: 3,
                        backgroundColor: theme.line,
                        overflow: 'hidden',
                      }}
                    >
                      <View
                        style={{
                          height: 5,
                          width: `${(entry.bytes / entry.total) * 100}%`,
                          backgroundColor: theme.signal,
                        }}
                      />
                    </View>
                  ) : null}
                  <Text style={{ color: theme.muted, fontSize: 13 }}>
                    {entry.quality === '1080p' || entry.quality === '720p'
                      ? `${entry.quality} · `
                      : 'Original · '}
                    {entry.total === 0
                      ? 'Size available after preparation'
                      : entry.status === 'complete'
                        ? downloadBytes(entry.total)
                        : `${downloadBytes(entry.bytes)} of ${downloadBytes(entry.total)}`}{' '}
                    · Tap for options
                  </Text>
                  {entry.total === 0 ? (
                    <Text style={{ color: theme.muted, fontSize: 13 }}>
                      {entry.status === 'preparing'
                        ? 'Keep Downloads open to start the transfer when ready. Preparation is saved if you leave.'
                        : 'Server preparation can continue. Resume here to transfer the file.'}
                    </Text>
                  ) : null}
                  {entry.error ? (
                    <Text style={{ color: theme.text }}>{entry.error}</Text>
                  ) : null}
                </Pressable>
              ))}
            </View>
          ) : null,
        )}
        <Text style={{ color: theme.muted, fontSize: 13, lineHeight: 19 }}>
          Signing out removes downloads. Offline progress stays on this device.
          {props.background
            ? ' If you force-quit the app, reopen it to resume interrupted downloads.'
            : ''}
        </Text>
      </ScrollView>
      <ModalSheet
        visible={Boolean(selected)}
        title={
          confirmRemoval
            ? 'Remove download?'
            : (selected?.item.title ?? 'Download')
        }
        dismissLabel={confirmRemoval ? 'Keep download' : 'Close'}
        dismissDisabled={busy}
        onClose={() =>
          confirmRemoval ? setConfirmRemoval(false) : setSelectedID('')
        }
      >
        {selected ? (
          <>
            <Text style={{ color: theme.muted }}>
              {downloadStatus(selected)}
              {selected.total > 0 ? ` · ${downloadBytes(selected.total)}` : ''}
            </Text>
            {selected.error ? (
              <Text accessibilityRole="alert" style={{ color: theme.text }}>
                {selected.error}
              </Text>
            ) : null}
            {props.error ? (
              <Text accessibilityRole="alert" style={{ color: theme.text }}>
                {props.error}
              </Text>
            ) : null}
            {confirmRemoval ? (
              <>
                <Text style={{ color: theme.text }}>
                  Remove {selected.item.title} from this device? This frees{' '}
                  {downloadBytes(selected.bytes)}. You will need a connection to
                  download it again.
                </Text>
                <ActionButton
                  label="Remove from device"
                  quiet
                  busy={busy}
                  style={{ borderColor: theme.danger }}
                  onPress={() => void run(props.onRemove, true)}
                />
              </>
            ) : selected.status === 'complete' ? (
              <ActionButton
                label={
                  selected.item.kind === 'audiobook' ||
                  selected.item.kind === 'track'
                    ? 'Listen offline'
                    : 'Watch offline'
                }
                onPress={() => {
                  setSelectedID('');
                  props.onPlay(selected);
                }}
              />
            ) : selected.status === 'paused' ? (
              <ActionButton
                label="Resume download"
                busy={busy}
                onPress={() => void run(props.onResume)}
              />
            ) : (
              <ActionButton
                label={
                  selected.status === 'pausing' ? 'Pausing…' : 'Pause download'
                }
                busy={busy}
                disabled={selected.status === 'pausing'}
                onPress={() => void run(props.onPause)}
              />
            )}
            {!confirmRemoval ? (
              <ActionButton
                label="Remove download"
                quiet
                disabled={busy}
                onPress={() => setConfirmRemoval(true)}
              />
            ) : null}
          </>
        ) : null}
      </ModalSheet>
    </SafeAreaView>
  );
}
