import { downloadStorage } from '@/core/downloads';
import { downloadStorageLimits } from '@/core/download-policy';
import { downloadBytes } from '@/components/downloads-view';
import { ProgressSyncPanel } from '@/components/progress-sync-panel';
import React, { useEffect, useState } from 'react';
import { router } from 'expo-router';
import { Platform, ScrollView, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useSession } from '@/core/session-context';
import {
  defaultMediaPreferences,
  type MediaPreferences,
} from '@/core/media-preferences';
import { localCompatibilityAvailable } from '@/core/protected-media';
import { useKinoTheme } from '@/design/tokens';
import { ActionButton } from '@/components/action-button';
import { LanguagePicker } from '@/components/language-picker';
import { Skeleton } from '@/components/skeleton';
import { PlaybackPreferenceControls } from '@/components/playback-preference-controls';
import { BottomActions } from '@/components/bottom-actions';
import { ScreenState } from '@/components/screen-state';
import { writeExperienceCache } from '@/core/experience-cache';

export default function SettingsScreen() {
  const { client } = useSession(),
    theme = useKinoTheme();
  const [value, setValue] = useState<MediaPreferences>(defaultMediaPreferences);
  const [storage] = useState(downloadStorage);
  const [advanced, setAdvanced] = useState(false);
  const [readingOpen, setReadingOpen] = useState(false);
  const [downloadAdvanced, setDownloadAdvanced] = useState(false);
  const [loaded, setLoaded] = useState(false),
    [saving, setSaving] = useState(false),
    [error, setError] = useState(''),
    [saved, setSaved] = useState(false);
  useEffect(() => {
    let active = true;
    if (client)
      void client.loadMediaPreferences().then(
        (prefs) => {
          if (active) {
            setValue(prefs);
            setLoaded(true);
          }
        },
        () => {
          if (active)
            setError(
              'Could not load settings. Connect to your Server and try again.',
            );
        },
      );
    return () => {
      active = false;
    };
  }, [client]);
  const save = async () => {
    if (!client || saving) return;
    setSaving(true);
    setError('');
    setSaved(false);
    try {
      const result = await client.saveMediaPreferences(value);
      await writeExperienceCache(client, 'defaults', result);
      setValue(result);
      setSaved(true);
    } catch {
      setError('Could not save settings. Try again.');
    } finally {
      setSaving(false);
    }
  };
  if (!client)
    return (
      <ScreenState
        message="Connect to your Server to change settings."
        onAction={() => router.replace('/')}
        action="Home"
      />
    );
  return (
    <SafeAreaView
      style={{ flex: 1, backgroundColor: theme.background }}
      edges={['top', 'left', 'right']}
    >
      <ScrollView
        contentContainerStyle={{
          padding: 20,
          paddingBottom: 24,
          gap: 24,
          maxWidth: 800,
          width: '100%',
          alignSelf: 'center',
        }}
      >
        <View
          style={{
            flexDirection: 'row',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <Text
            accessibilityRole="header"
            style={{ fontSize: 32, fontWeight: '700', color: theme.text }}
          >
            Settings
          </Text>
          {Platform.isTV ? (
            <ActionButton label="Back" quiet onPress={() => router.back()} />
          ) : null}
        </View>
        {error ? (
          <Text accessibilityRole="alert" style={{ color: theme.danger }}>
            {error}
          </Text>
        ) : null}
        {!loaded ? (
          !error ? (
            <Skeleton label="Loading settings" />
          ) : (
            <ActionButton
              label="Retry"
              onPress={() => {
                setError('');
                void client.loadMediaPreferences().then(
                  (v) => {
                    setValue(v);
                    setLoaded(true);
                  },
                  () => setError('Could not load settings. Try again.'),
                );
              }}
            />
          )
        ) : (
          <>
            <Text style={{ color: theme.muted }}>
              Defaults for your profile. Choices made while playing are
              remembered for that show, book, or title.
            </Text>
            {(['audioLanguage', 'subtitleLanguage'] as const).map((key) => (
              <LanguagePicker
                key={key}
                value={value.playback[key]}
                subtitles={key === 'subtitleLanguage'}
                disabled={saving}
                onChange={(language) => {
                  setSaved(false);
                  setValue({
                    ...value,
                    playback: {
                      ...value.playback,
                      [key]: language,
                      [key === 'audioLanguage'
                        ? 'audioTrack'
                        : 'subtitleTrack']: '',
                    },
                  });
                }}
              />
            ))}
            <Text
              accessibilityRole="header"
              style={{ fontSize: 24, fontWeight: '600', color: theme.text }}
            >
              Downloads
            </Text>
            <Text style={{ color: theme.text }}>Download over</Text>
            <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
              {[true, false].map((wifiOnly) => (
                <ActionButton
                  key={String(wifiOnly)}
                  label={wifiOnly ? 'Wi-Fi only' : 'Wi-Fi + cellular'}
                  selected={value.wifiOnly === wifiOnly}
                  quiet={value.wifiOnly !== wifiOnly}
                  disabled={saving}
                  onPress={() => {
                    setSaved(false);
                    setValue({ ...value, wifiOnly });
                  }}
                />
              ))}
            </View>
            <Text style={{ color: theme.muted }}>
              Wi-Fi only is the default. Cellular downloads use your mobile data
              plan. Changes apply to new downloads and downloads you pause and
              resume.
            </Text>
            <Text style={{ color: theme.text }}>Storage limit</Text>
            <Text style={{ color: theme.muted }}>
              {storage
                ? `${downloadBytes(storage.free)} free of ${downloadBytes(storage.total)} on this device. `
                : ''}
              No limit uses available storage. Downloads always leave 512 MB
              free.
            </Text>
            <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
              {downloadStorageLimits(
                storage?.total,
                value.downloadLimitGiB,
              ).map((limit) => (
                <ActionButton
                  key={limit}
                  label={limit === 0 ? 'No limit' : `${limit} GB`}
                  selected={value.downloadLimitGiB === limit}
                  quiet={value.downloadLimitGiB !== limit}
                  disabled={saving}
                  onPress={() => {
                    setSaved(false);
                    setValue({ ...value, downloadLimitGiB: limit });
                  }}
                />
              ))}
            </View>
            <ActionButton
              label="Automatic downloads and cleanup"
              quiet
              expanded={downloadAdvanced}
              onPress={() => setDownloadAdvanced(!downloadAdvanced)}
            />
            {downloadAdvanced ? (
              <View style={{ gap: 16 }}>
                <Text style={{ color: theme.muted }}>
                  Automatically queue upcoming episodes from a show you
                  download. Existing downloads are only removed when you enable
                  watched cleanup.
                </Text>
                <View
                  style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}
                >
                  {[0, 1, 2, 3].map((count) => (
                    <ActionButton
                      key={count}
                      label={
                        count ? `${count} upcoming` : 'Automatic downloads off'
                      }
                      selected={value.autoDownloadNext === count}
                      quiet={value.autoDownloadNext !== count}
                      disabled={saving}
                      onPress={() => {
                        setSaved(false);
                        setValue({ ...value, autoDownloadNext: count });
                      }}
                    />
                  ))}
                </View>
                <ActionButton
                  label={`Remove watched downloads: ${value.removeWatched ? 'On' : 'Off'}`}
                  quiet={!value.removeWatched}
                  disabled={saving}
                  onPress={() => {
                    setSaved(false);
                    setValue({ ...value, removeWatched: !value.removeWatched });
                  }}
                />
              </View>
            ) : null}
            <ActionButton
              label="Reading preferences"
              quiet
              expanded={readingOpen}
              onPress={() => setReadingOpen(!readingOpen)}
            />
            {readingOpen ? (
              <View style={{ gap: 16 }}>
                <Text
                  accessibilityRole="header"
                  style={{ fontSize: 24, fontWeight: '600', color: theme.text }}
                >
                  Reading
                </Text>
                <Text style={{ color: theme.text }}>
                  Text size: {value.readerFontSize}
                </Text>
                <View style={{ flexDirection: 'row', gap: 8 }}>
                  <ActionButton
                    label="Smaller text"
                    quiet
                    disabled={saving || value.readerFontSize <= 16}
                    onPress={() => {
                      setSaved(false);
                      setValue({
                        ...value,
                        readerFontSize: value.readerFontSize - 2,
                      });
                    }}
                  />
                  <ActionButton
                    label="Larger text"
                    quiet
                    disabled={saving || value.readerFontSize >= 32}
                    onPress={() => {
                      setSaved(false);
                      setValue({
                        ...value,
                        readerFontSize: value.readerFontSize + 2,
                      });
                    }}
                  />
                </View>
                <View
                  style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}
                >
                  {(['auto', 'light', 'dark', 'sepia'] as const).map(
                    (readerTheme) => (
                      <ActionButton
                        key={readerTheme}
                        label={
                          readerTheme === 'auto'
                            ? 'App theme'
                            : readerTheme[0].toUpperCase() +
                              readerTheme.slice(1)
                        }
                        selected={value.readerTheme === readerTheme}
                        quiet={value.readerTheme !== readerTheme}
                        disabled={saving}
                        onPress={() => {
                          setSaved(false);
                          setValue({ ...value, readerTheme });
                        }}
                      />
                    ),
                  )}
                </View>
              </View>
            ) : null}
            <ActionButton
              label="Advanced playback"
              quiet
              expanded={advanced}
              onPress={() => setAdvanced(!advanced)}
            />
            {advanced ? (
              <View style={{ gap: 16 }}>
                <PlaybackPreferenceControls
                  value={value.playback}
                  audioAvailable={localCompatibilityAvailable}
                  disabled={saving}
                  onChange={(playback) => {
                    setSaved(false);
                    setValue({ ...value, playback });
                  }}
                />
              </View>
            ) : null}
            <ProgressSyncPanel client={client} />
            {!Platform.isTV ? (
              <ActionButton
                label="Connect a TV"
                quiet
                onPress={() => router.push('/approve')}
              />
            ) : null}
            {Platform.isTV ? (
              <ActionButton
                label="Save settings"
                busy={saving}
                onPress={() => void save()}
              />
            ) : null}
            {saved && Platform.isTV ? (
              <Text
                accessibilityLiveRegion="polite"
                style={{ color: theme.text }}
              >
                Settings saved.
              </Text>
            ) : null}
          </>
        )}
      </ScrollView>
      {!Platform.isTV ? (
        <BottomActions
          onBack={router.canGoBack() ? () => router.back() : undefined}
        >
          {saved ? (
            <Text
              accessibilityLiveRegion="polite"
              style={{ color: theme.text, width: '100%' }}
            >
              Settings saved.
            </Text>
          ) : null}
          <ActionButton
            label="Save settings"
            disabled={!loaded}
            busy={saving}
            onPress={() => void save()}
          />
        </BottomActions>
      ) : null}
    </SafeAreaView>
  );
}
