import { MusicTrackRow } from './music-track-row';
import React, { useState } from 'react';
import {
  FlatList,
  Platform,
  StyleSheet,
  Text,
  TextInput,
  View,
  useWindowDimensions,
} from 'react-native';
import {
  SafeAreaView,
  useSafeAreaInsets,
} from 'react-native-safe-area-context';
import type { MediaItem, LibraryLetter } from '@/core/contract';
import type { LibraryQuery } from '@/core/library-query';
import { spacing, radius, useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';
import { MediaCard } from './media-card';
import { FocusGroup } from './focus-group';
import { AlphabetRail } from './alphabet-rail';
import { ModalSheet } from './modal-sheet';
import { BrowseSkeleton } from './browse-skeleton';

type Props = {
  onAlbums?: () => void;
  onDownloads?: () => void;
  title?: string;
  collection?: boolean;
  searchOpen?: boolean;
  onCloseSearch?: () => void;
  items: MediaItem[];
  total: number;
  letters?: LibraryLetter[];
  query: LibraryQuery;
  busy: boolean;
  error: string;
  hasMore: boolean;
  mediaURL(path: string): string;
  headers: Record<string, string>;
  onChange(query: LibraryQuery): void;
  onOpen(id: string): void;
  onMore(): void;
  onPrevious(): void;
  onRetry(): void;
  onBack(): void;
};
const views = [
  ['all', 'All'],
  ['movies', 'Movies'],
  ['shows', 'TV shows'],
  ['unwatched', 'Unwatched'],
  ['list', 'My List'],
] as const;

export function LibraryView(props: Props) {
  const insets = useSafeAreaInsets();
  const [tvSearchOpen, setTVSearchOpen] = useState(Boolean(props.searchOpen));
  const closeSearch = () => {
    setTVSearchOpen(false);
    props.onCloseSearch?.();
  };
  const [headerHeight, setHeaderHeight] = useState(100 + insets.top);
  const theme = useKinoTheme();
  const { width, fontScale } = useWindowDimensions();
  const inset = Platform.isTV
    ? Math.max(48, width * 0.05, insets.left, insets.right)
    : 20;
  const letters =
    !Platform.isTV &&
    !props.query.q?.trim() &&
    (props.query.sort ?? 'title') === 'title'
      ? (props.letters ?? [])
      : [];
  const railWidth = letters.length > 1 ? 36 : 0;
  const gridWidth = width - inset * 2 - railWidth;
  const minimum = Platform.isTV
    ? 240
    : (railWidth && width < 360 ? 112 : 130) * Math.min(fontScale, 1.5);
  const columns =
    props.query.view === 'music'
      ? 1
      : Math.max(1, Math.floor((gridWidth + 16) / (minimum + 16)));
  const cardWidth = (gridWidth - (columns - 1) * 16) / columns;
  const bookFilters = ['books', 'audiobooks'].includes(props.query.view ?? '')
    ? (['books', 'audiobooks'] as const).map((view) => (
        <ActionButton
          key={view}
          label={view === 'books' ? 'Ebooks' : 'Audiobooks'}
          quiet={props.query.view !== view}
          accessibilityLabel={`${view === 'books' ? 'Ebooks' : 'Audiobooks'}${props.query.view === view ? ', selected' : ''}`}
          onPress={() =>
            props.onChange({ ...props.query, view, offset: 0, q: '' })
          }
        />
      ))
    : null;
  const sortLabels = { title: 'Title', added: 'Recently added', year: 'Year' };
  const filtered = Boolean(
    props.query.q?.trim() || props.query.view === 'unwatched',
  );
  const clearFilters = () =>
    props.onChange({
      ...props.query,
      q: '',
      offset: 0,
      view: props.query.view === 'unwatched' ? 'all' : props.query.view,
    });
  const controls = (
    <>
      <TextInput
        accessibilityLabel="Search your library"
        placeholder="Search titles, people, and more"
        placeholderTextColor={theme.muted}
        value={props.query.q ?? ''}
        onChangeText={(q) => props.onChange({ ...props.query, q, offset: 0 })}
        autoCapitalize="none"
        autoCorrect={false}
        maxLength={512}
        returnKeyType="search"
        onSubmitEditing={closeSearch}
        style={[
          styles.search,
          {
            color: theme.text,
            backgroundColor: theme.surface,
            borderColor: theme.line,
          },
        ]}
      />
      <FocusGroup style={styles.filters}>
        {Platform.isTV ? bookFilters : null}
        {views
          .filter(
            ([view]) =>
              Platform.isTV ||
              (!['movies', 'shows'].includes(view) &&
                ![
                  'movies',
                  'shows',
                  'music',
                  'audiobooks',
                  'books',
                  'photos',
                  'history',
                ].includes(props.query.view ?? '')),
          )
          .map(([view, label]) => (
            <ActionButton
              key={view}
              label={label}
              accessibilityLabel={`${label}${(props.query.view ?? 'all') === view ? ', selected' : ''}`}
              quiet={(props.query.view ?? 'all') !== view}
              onPress={() =>
                props.onChange({ ...props.query, view, offset: 0 })
              }
            />
          ))}
      </FocusGroup>
      <Text accessibilityRole="header" style={{ color: theme.text }}>
        Sort by
      </Text>
      <FocusGroup style={styles.filters}>
        {(['title', 'added', 'year'] as const).map((sort) => (
          <ActionButton
            key={sort}
            label={sortLabels[sort]}
            quiet
            selected={(props.query.sort ?? 'title') === sort}
            onPress={() => props.onChange({ ...props.query, sort, offset: 0 })}
          />
        ))}
      </FocusGroup>
    </>
  );
  return (
    <SafeAreaView
      edges={Platform.isTV ? ['top', 'bottom'] : ['left', 'right']}
      style={[
        styles.screen,
        { backgroundColor: theme.background, paddingHorizontal: inset },
        !Platform.isTV ? { gap: 0 } : null,
      ]}
    >
      <View
        pointerEvents="box-none"
        onLayout={(event) => setHeaderHeight(event.nativeEvent.layout.height)}
        style={
          !Platform.isTV
            ? {
                position: 'absolute',
                top: 0,
                left: 0,
                right: 0,
                zIndex: 2,
                paddingTop: insets.top,
                paddingHorizontal: inset,
                paddingBottom: 16,
                gap: 16,
                backgroundColor: theme.background,
              }
            : { gap: 16 }
        }
      >
        <View style={styles.heading}>
          {props.onAlbums ? (
            <ActionButton
              label="Albums & artists"
              quiet
              onPress={props.onAlbums}
            />
          ) : null}
          {Platform.isTV || props.collection ? (
            <ActionButton label="Back" onPress={props.onBack} quiet />
          ) : null}
          <Text
            accessibilityRole="header"
            style={[styles.title, { color: theme.text }]}
          >
            {props.title ??
              (props.query.view === 'photos'
                ? 'Photos'
                : props.query.view === 'history'
                  ? 'History'
                  : props.query.view === 'list'
                    ? 'My List'
                    : props.query.view === 'movies'
                      ? 'Movies'
                      : props.query.view === 'shows'
                        ? 'TV'
                        : props.query.view === 'music'
                          ? 'Songs'
                          : props.query.view === 'books'
                            ? 'Ebooks'
                            : props.query.view === 'audiobooks'
                              ? 'Audiobooks'
                              : 'Library')}
          </Text>
          {Platform.isTV && !props.collection ? (
            <ActionButton
              label="Search & filters"
              quiet
              onPress={() => setTVSearchOpen(true)}
              style={{ marginLeft: 'auto' }}
            />
          ) : null}
        </View>
        {!Platform.isTV && bookFilters ? (
          <FocusGroup style={styles.filters}>{bookFilters}</FocusGroup>
        ) : null}
        {!props.collection ? (
          <ModalSheet
            visible={Platform.isTV ? tvSearchOpen : Boolean(props.searchOpen)}
            title="Search & browse"
            onClose={closeSearch}
            footer={
              <ActionButton
                label={
                  props.error
                    ? 'Back to library'
                    : props.busy
                      ? 'Show results'
                      : `Show ${props.total} ${props.total === 1 ? 'title' : 'titles'}`
                }
                onPress={closeSearch}
              />
            }
          >
            {controls}
          </ModalSheet>
        ) : null}
        <Text
          accessibilityLiveRegion="polite"
          style={{
            color: theme.muted,
            fontSize: Platform.isTV ? 24 : undefined,
          }}
        >
          {props.error
            ? 'Library unavailable'
            : props.busy
              ? 'Loading titles…'
              : `${props.total} ${props.total === 1 ? 'title' : 'titles'}`}
        </Text>
        {props.query.q?.trim() ||
        (props.query.sort && props.query.sort !== 'title') ? (
          <Text style={{ color: theme.muted }}>
            {[
              props.query.q?.trim() ? `Search: ${props.query.q.trim()}` : '',
              `Sorted by ${sortLabels[props.query.sort ?? 'title']}`,
            ]
              .filter(Boolean)
              .join(' · ')}
          </Text>
        ) : null}
        {filtered ? (
          <ActionButton label="Clear filters" quiet onPress={clearFilters} />
        ) : null}
        {props.error ? (
          <View style={styles.recovery}>
            <Text accessibilityRole="alert" style={{ color: theme.text }}>
              {props.error}
            </Text>
            <ActionButton label="Try again" onPress={props.onRetry} />
            {props.onDownloads ? (
              <ActionButton
                label="Go to Downloads"
                quiet
                onPress={props.onDownloads}
              />
            ) : null}
          </View>
        ) : null}
      </View>
      <FocusGroup
        style={[styles.results, railWidth ? { marginRight: -inset } : null]}
      >
        <FlatList
          key={`${columns}:${props.query.offset ?? 0}:${props.query.view}:${props.query.sort}:${props.query.q}`}
          style={{ marginRight: railWidth ? railWidth + inset : 0 }}
          accessibilityLabel="Library results"
          data={props.items}
          numColumns={columns}
          keyExtractor={(item) => item.id}
          keyboardShouldPersistTaps="handled"
          contentContainerStyle={[
            styles.grid,
            !Platform.isTV
              ? {
                  paddingTop: headerHeight + 8,
                  paddingBottom: 160 + insets.bottom,
                }
              : null,
          ]}
          columnWrapperStyle={columns > 1 ? styles.row : undefined}
          initialNumToRender={columns * 2}
          maxToRenderPerBatch={columns * 2}
          windowSize={5}
          removeClippedSubviews={!Platform.isTV && Platform.OS !== 'web'}
          renderItem={({ item, index }) =>
            props.query.view === 'music' ? (
              <MusicTrackRow
                item={item}
                onPress={() => props.onOpen(item.id)}
              />
            ) : (
              <MediaCard
                item={item}
                preferredFocus={Platform.isTV && index === 0 && !tvSearchOpen}
                square={
                  props.query.view === 'audiobooks' || item.kind === 'photo'
                }
                imageURL={props.mediaURL(item.artwork)}
                headers={props.headers}
                width={cardWidth}
                onPress={() => props.onOpen(item.id)}
              />
            )
          }
          ListEmptyComponent={
            props.busy ? (
              <BrowseSkeleton
                variant={
                  props.query.view === 'music'
                    ? 'track'
                    : props.query.view === 'audiobooks' ||
                        props.query.view === 'photos'
                      ? 'square'
                      : 'poster'
                }
                label="Loading titles…"
                width={cardWidth}
                columns={columns}
                rowGap={spacing.three}
              />
            ) : props.error ? null : (
              <Text
                style={{
                  color: theme.muted,
                  fontSize: Platform.isTV ? 24 : undefined,
                }}
              >
                {props.collection
                  ? 'This collection has no items.'
                  : filtered
                    ? 'No matching titles. Clear filters to browse again.'
                    : props.query.view === 'list'
                      ? 'No titles saved to My List yet.'
                      : 'No titles in this library yet.'}
              </Text>
            )
          }
          ListFooterComponent={
            <View style={styles.filters}>
              {(props.query.offset ?? 0) > 0 ? (
                <ActionButton
                  label="Previous page"
                  disabled={props.busy}
                  onPress={props.onPrevious}
                  quiet
                />
              ) : null}
              {props.hasMore ? (
                <ActionButton
                  label="Next page"
                  busy={props.busy}
                  onPress={props.onMore}
                  quiet
                />
              ) : null}
            </View>
          }
        />
        {railWidth ? (
          <View
            pointerEvents="box-none"
            style={{
              position: 'absolute',
              top: headerHeight,
              right: 0,
              bottom: 50 + insets.bottom,
              width: railWidth,
            }}
          >
            <AlphabetRail
              letters={letters}
              offset={props.query.offset ?? 0}
              onJump={(offset) => props.onChange({ ...props.query, offset })}
            />
          </View>
        ) : null}
      </FocusGroup>
    </SafeAreaView>
  );
}
const styles = StyleSheet.create({
  screen: { flex: 1, gap: spacing.two },
  heading: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.two,
    paddingTop: spacing.two,
  },
  title: {
    fontSize: Platform.isTV ? 40 : 28,
    fontWeight: '700',
    flexShrink: 1,
  },
  search: {
    minHeight: Platform.isTV ? 64 : 52,
    borderWidth: 2,
    borderRadius: radius.control,
    paddingHorizontal: spacing.two,
    fontSize: Platform.isTV ? 24 : 17,
  },
  filters: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.one },
  results: { flex: 1 },
  grid: {
    gap: spacing.three,
    paddingTop: spacing.one,
    paddingBottom: 100,
  },
  row: { gap: spacing.two },
  recovery: { gap: spacing.one },
});
