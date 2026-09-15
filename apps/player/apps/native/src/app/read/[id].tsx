import React, { useEffect, useState, useRef } from 'react';
import { router, useLocalSearchParams } from 'expo-router';
import { Platform, View, Text, ScrollView, useColorScheme } from 'react-native';
import { BottomActions } from '@/components/bottom-actions';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useSession } from '@/core/session-context';
import { readerPositionEvent, type ReaderBook } from '@/core/reader';
import {
  defaultMediaPreferences,
  type MediaPreferences,
  type Bookmark,
} from '@/core/media-preferences';
import { ReaderContent } from '@/components/reader-content';
import { ActionButton } from '@/components/action-button';
import { ScreenState } from '@/components/screen-state';
import { useKinoTheme } from '@/design/tokens';
export default function ReadScreen() {
  const { id } = useLocalSearchParams<{ id: string }>(),
    { client } = useSession(),
    theme = useKinoTheme(),
    scheme = useColorScheme();
  const [book, setBook] = useState<ReaderBook | null>(null),
    [page, setPage] = useState(1),
    [offset, setOffset] = useState(0),
    [jump, setJump] = useState(0),
    [saving, setSaving] = useState(false),
    [bookmarkSaving, setBookmarkSaving] = useState(false),
    [positionError, setPositionError] = useState(''),
    [message, setMessage] = useState(''),
    [prefs, setPrefs] = useState<MediaPreferences>(defaultMediaPreferences),
    [bookmarks, setBookmarks] = useState<Bookmark[]>([]),
    [error, setError] = useState(''),
    [options, setOptions] = useState(false);
  const queue = useRef(Promise.resolve());
  const revision = useRef(0);
  const locationKey = `${id}-${page}-${jump}-${options}`;
  const activeLocation = useRef(locationKey);
  activeLocation.current = locationKey;
  const pendingPosition = useRef<{
    page: number;
    offset: number;
    revision: number;
  } | null>(null);
  const positionQueued = useRef(false);
  useEffect(() => {
    let active = true;
    if (!client || typeof id !== 'string') return;
    void Promise.all([
      client.loadReader(id),
      client.loadReaderProgress(id),
      client.loadMediaPreferences(),
      client.loadBookmarks(id),
    ]).then(
      ([loaded, progress, preferences, marks]) => {
        if (active) {
          setBook(loaded);
          setPage(Math.min(progress.page, loaded.pages.length));
          setOffset(
            progress.page <= loaded.pages.length ? (progress.offset ?? 0) : 0,
          );
          setPrefs(preferences);
          setBookmarks(marks);
        }
      },
      () =>
        active &&
        setError(
          'Could not load this book. Connect to your Server and try again.',
        ),
    );
    return () => {
      active = false;
    };
  }, [client, id]);
  const savePosition = (next: number, fraction: number) => {
    if (
      !client ||
      !book ||
      !Number.isInteger(next) ||
      next < 1 ||
      next > book.pages.length ||
      !Number.isFinite(fraction) ||
      fraction < 0 ||
      fraction > 1
    )
      return;
    const current = ++revision.current;
    setSaving(true);
    setPositionError('');
    pendingPosition.current = {
      page: next,
      offset: fraction,
      revision: current,
    };
    if (positionQueued.current) return;
    positionQueued.current = true;
    queue.current = queue.current.then(async () => {
      while (pendingPosition.current) {
        const position = pendingPosition.current;
        pendingPosition.current = null;
        try {
          await client.saveReaderProgress(id, position.page, position.offset);
          if (position.revision === revision.current)
            setMessage('Reading position saved');
        } catch {
          if (position.revision === revision.current)
            setPositionError(
              'Reading position could not sync. Keep the book open and retry.',
            );
        }
      }
      positionQueued.current = false;
      setSaving(false);
    });
  };
  const go = (next: number, fraction = 0) => {
    if (
      !book ||
      !Number.isInteger(next) ||
      next < 1 ||
      next > book.pages.length ||
      !Number.isFinite(fraction) ||
      fraction < 0 ||
      fraction > 1
    )
      return;
    setPage(next);
    setOffset(fraction);
    setJump((value) => value + 1);
    setOptions(false);
    savePosition(next, fraction);
  };
  const preference = (next: MediaPreferences) => {
    if (!client) return;
    setPrefs(next);
    queue.current = queue.current
      .then(async () => {
        await client.saveMediaPreferences(next);
      })
      .catch(() => setError('Reader settings could not save.'));
  };
  const bookmarkSpot = () => {
    if (!client || !book || bookmarkSaving) return;
    setBookmarkSaving(true);
    setError('');
    void client
      .addBookmark(id, {
        title: `${book.type === 'epub' ? 'Chapter' : 'Page'} ${page} · ${Math.round(offset * 100)}%`,
        page,
        offset,
      })
      .then(
        (marks) => {
          setBookmarks(marks);
          setMessage('Bookmark saved');
        },
        () => setError('Bookmark could not save.'),
      )
      .finally(() => setBookmarkSaving(false));
  };
  if (!book || !client)
    return (
      <ScreenState
        loading={!error && Boolean(client)}
        message={error || 'Opening book…'}
        action="Back"
        onAction={() => router.back()}
      />
    );
  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: theme.background }}>
      <View
        style={{
          padding: 12,
          flexDirection: 'row',
          gap: 8,
          alignItems: 'center',
        }}
      >
        {Platform.isTV ? (
          <ActionButton
            label="Back"
            style={{ paddingHorizontal: 12 }}
            quiet
            onPress={() => router.back()}
          />
        ) : null}
        <Text numberOfLines={1} style={{ flex: 1, color: theme.text }}>
          {book.title}
        </Text>
        {Platform.isTV ? (
          <ActionButton
            label={options ? 'Close' : 'Options'}
            accessibilityLabel={
              options ? 'Close reader options' : 'Reader options'
            }
            style={{ paddingHorizontal: 12 }}
            quiet
            onPress={() => setOptions(!options)}
          />
        ) : null}
      </View>
      {options ? (
        <ScrollView
          style={{ flex: 1 }}
          contentContainerStyle={{ padding: 16, paddingBottom: 32, gap: 12 }}
        >
          <View style={{ flexDirection: 'row', gap: 8, flexWrap: 'wrap' }}>
            <ActionButton
              label="Smaller text"
              quiet
              disabled={prefs.readerFontSize <= 16}
              onPress={() =>
                preference({
                  ...prefs,
                  readerFontSize: prefs.readerFontSize - 2,
                })
              }
            />
            <ActionButton
              label="Larger text"
              quiet
              disabled={prefs.readerFontSize >= 32}
              onPress={() =>
                preference({
                  ...prefs,
                  readerFontSize: prefs.readerFontSize + 2,
                })
              }
            />
            {(['auto', 'light', 'dark', 'sepia'] as const).map((value) => (
              <ActionButton
                key={value}
                label={
                  value === 'auto'
                    ? 'App theme'
                    : value[0].toUpperCase() + value.slice(1)
                }
                selected={prefs.readerTheme === value}
                quiet={prefs.readerTheme !== value}
                onPress={() => preference({ ...prefs, readerTheme: value })}
              />
            ))}
          </View>
          <ActionButton
            label="Bookmark my spot"
            quiet
            busy={bookmarkSaving}
            onPress={bookmarkSpot}
          />
          {bookmarks
            .filter((mark) => mark.page !== undefined)
            .map((mark) => (
              <View key={mark.id} style={{ gap: 8 }}>
                <ActionButton
                  label={mark.title}
                  quiet
                  onPress={() => go(mark.page!, mark.offset ?? 0)}
                />
                <ActionButton
                  label={`Remove ${mark.title}`}
                  quiet
                  onPress={() => {
                    void client
                      .removeBookmark(id, mark.id)
                      .then(setBookmarks, () =>
                        setError('Bookmark could not be removed.'),
                      );
                  }}
                />
              </View>
            ))}
          {book.pages.map((value) => (
            <ActionButton
              key={value.number}
              label={value.title || `Page ${value.number}`}
              selected={value.number === page}
              quiet={value.number !== page}
              onPress={() => go(value.number)}
            />
          ))}
        </ScrollView>
      ) : null}
      {error ? (
        <Text
          accessibilityRole="alert"
          style={{ padding: 12, color: theme.text }}
        >
          {error}
        </Text>
      ) : null}
      {!options ? (
        <View style={{ flex: 1, overflow: 'hidden' }}>
          <ReaderContent
            key={`${id}-${page}-${jump}`}
            offset={offset}
            onPosition={(event) => {
              if (activeLocation.current !== locationKey) return;
              const fraction = readerPositionEvent(
                event,
                book.pages[page - 1].url,
              );
              if (fraction === null) return;
              setOffset(fraction);
              savePosition(page, fraction);
            }}
            server={client.baseURL}
            authorization={client.authorizationHeaders().Authorization}
            id={id}
            path={book.pages[page - 1].url}
            type={book.type}
            fontSize={prefs.readerFontSize}
            theme={
              prefs.readerTheme === 'auto'
                ? scheme === 'dark'
                  ? 'dark'
                  : 'light'
                : prefs.readerTheme
            }
            onError={() =>
              setError(
                'This page could not open. Check your Server connection.',
              )
            }
          />
        </View>
      ) : null}
      <View style={{ paddingHorizontal: 16, paddingTop: 8, gap: 8 }}>
        <View
          accessible
          accessibilityRole="progressbar"
          accessibilityLabel="Reading progress"
          accessibilityValue={{
            min: 0,
            max: 100,
            now: Math.round(((page - 1 + offset) / book.pages.length) * 100),
          }}
          style={{ height: 4, backgroundColor: theme.line }}
        >
          <View
            style={{
              height: 4,
              backgroundColor: theme.signal,
              width: `${((page - 1 + offset) / book.pages.length) * 100}%`,
            }}
          />
        </View>
        <Text style={{ color: theme.muted, textAlign: 'center' }}>
          {book.type === 'epub' ? 'Chapter' : 'Page'} {page} of{' '}
          {book.pages.length} ·{' '}
          {Math.round(((page - 1 + offset) / book.pages.length) * 100)}%
        </Text>
        <Text
          accessibilityLiveRegion={
            positionError || message === 'Bookmark saved' ? 'polite' : 'none'
          }
          style={{ color: theme.muted, textAlign: 'center' }}
        >
          {saving
            ? 'Saving position…'
            : positionError
              ? positionError
              : message || 'Position restores automatically'}
        </Text>
        {positionError ? (
          <ActionButton
            label="Retry saving position"
            quiet
            onPress={() => savePosition(page, offset)}
          />
        ) : null}
      </View>
      <View
        style={{
          padding: 12,
          flexDirection: 'row',
          flexWrap: 'wrap',
          gap: 8,
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <ActionButton
          label="Previous"
          style={{ paddingHorizontal: 12 }}
          quiet
          disabled={page <= 1}
          onPress={() => go(page - 1)}
        />

        <ActionButton
          label="Bookmark"
          accessibilityLabel="Bookmark my spot"
          quiet
          busy={bookmarkSaving}
          onPress={bookmarkSpot}
          style={{ paddingHorizontal: 12 }}
        />
        <ActionButton
          label="Next"
          style={{ paddingHorizontal: 12 }}
          quiet
          disabled={page >= book.pages.length}
          onPress={() => go(page + 1)}
        />
      </View>
      {!Platform.isTV ? (
        <BottomActions onBack={() => router.back()}>
          <ActionButton
            label={options ? 'Close' : 'Options'}
            accessibilityLabel={
              options ? 'Close reader options' : 'Reader options'
            }
            style={{ paddingHorizontal: 12 }}
            quiet
            onPress={() => setOptions(!options)}
          />
        </BottomActions>
      ) : null}
    </SafeAreaView>
  );
}
