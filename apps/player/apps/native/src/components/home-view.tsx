import { artworkSource } from '@/core/artwork-source';
import { Image } from 'expo-image';
import { LinearGradient } from 'expo-linear-gradient';
import React, { useState } from 'react';
import {
  FlatList,
  Platform,
  ScrollView,
  StyleSheet,
  Text,
  type ImageStyle,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { Home, MediaItem } from '@/core/contract';
import { ArtworkThemeContext, spacing, useKinoTheme } from '@/design/tokens';

import { TVHomeView } from './tv-home-view';
import { ActionButton } from './action-button';
import { WatchProgressIndicator, useWatchProgress } from './watch-progress';
import { BrandMark } from './brand-mark';
import { HomeBackdrop } from './home-backdrop';
import { useHomeLayout } from './use-home-layout';
import { homeViewStyles as styles } from './home-view.styles';
import { MediaCard } from './media-card';
import { FocusGroup } from './focus-group';

export type HomeViewProps = {
  home: Home;
  mediaURL(path: string): string;
  headers?: Record<string, string>;
  onOpen(id: string): void;
  onPlay?: (id: string, fromBeginning?: boolean) => void;
  onBrowse?: (view?: 'movies' | 'shows' | 'list') => void;
  onDownloads?: () => void;
  onRefresh?: () => void;
  onSignOut?: () => void;
  onTheme?: () => void;
  themeLabel?: string;
};

function Shelf({
  title,
  items,
  cardWidth,
  mediaURL,
  headers,
  onOpen,
  landscape = false,
  onSeeAll,
  onHighlight,
}: {
  title: string;
  items: MediaItem[];
  cardWidth: number;
  mediaURL(path: string): string;
  headers?: Record<string, string>;
  onOpen(id: string): void;
  landscape?: boolean;
  onSeeAll?: () => void;
  onHighlight?: (id: string) => void;
}) {
  const theme = useKinoTheme();
  const gap = Platform.isTV ? spacing.three : 12;
  if (items.length === 0) return null;
  return (
    <FocusGroup style={styles.shelf}>
      <View
        style={{
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <Text
          accessibilityRole="header"
          style={[styles.shelfTitle, { color: theme.text, flexShrink: 1 }]}
        >
          {title}
        </Text>
        {onSeeAll ? (
          <ActionButton
            label="See all"
            accessibilityLabel={`See all ${title.toLowerCase()}`}
            quiet
            onPress={onSeeAll}
            style={{
              borderWidth: 0,
              backgroundColor: 'transparent',
              paddingHorizontal: 8,
              minHeight: 44,
            }}
          />
        ) : null}
      </View>
      <FlatList
        accessibilityLabel={`${title} shelf`}
        accessibilityRole="list"
        data={items}
        horizontal
        contentContainerStyle={styles.shelfRow}
        getItemLayout={(_, index) => ({
          index,
          length: cardWidth + gap,
          offset: (cardWidth + gap) * index,
        })}
        initialNumToRender={Platform.isTV ? 6 : 4}
        ItemSeparatorComponent={() => <View style={{ width: gap }} />}
        keyExtractor={(item) => item.id}
        maxToRenderPerBatch={6}
        removeClippedSubviews={!Platform.isTV && Platform.OS !== 'web'}
        renderItem={({ item }) => (
          <View role="listitem">
            <MediaCard
              headers={headers}
              imageURL={mediaURL(
                landscape ? item.backdrop || item.artwork : item.artwork,
              )}
              landscape={landscape}
              compact={!Platform.isTV}
              item={item}
              onPress={() => onOpen(item.id)}
              onHighlight={onHighlight ? () => onHighlight(item.id) : undefined}
              width={cardWidth}
            />
          </View>
        )}
        showsHorizontalScrollIndicator={false}
        updateCellsBatchingPeriod={16}
        windowSize={3}
      />
    </FocusGroup>
  );
}

function StandardHomeView({
  home,
  mediaURL,
  headers,
  onOpen,
  onPlay,
  onBrowse,
  onDownloads,
  onRefresh,
  onSignOut,
  onTheme,
  themeLabel,
}: HomeViewProps) {
  const [highlighted, setHighlighted] = useState('');
  const continuing = home.continueWatching.filter(
    (item) => !item.progress.watched,
  );
  const featured =
    [...continuing, ...home.recent].find((item) => item.id === highlighted) ??
    continuing[0] ??
    home.recent[0];
  const progress = useWatchProgress({
    id: featured?.id ?? '',
    seconds: featured?.progress.seconds,
    watched: featured?.progress.watched,
    mediaURL,
    headers,
  });
  const theme = useKinoTheme();
  const [failedBackdrop, setFailedBackdrop] = useState('');
  const backdrop = featured?.backdrop || featured?.artwork || '';
  const backdropURI = backdrop ? mediaURL(backdrop) : '';
  const hasBackdrop = Boolean(backdropURI && backdropURI !== failedBackdrop);
  const {
    insets,
    headerHeight,
    setHeaderHeight,
    width,
    isTV,
    accessible,
    wide,
    compact,
    short,
    showViewer,
    cardWidth,
    featureHeight,
    backdropHeight,
    contentInset,
  } = useHomeLayout();
  const [accountOpen, setAccountOpen] = useState(false);
  const artwork = (
    <>
      {hasBackdrop ? (
        <Image
          accessibilityIgnoresInvertColors
          accessibilityLabel={`Backdrop for ${featured?.title}`}
          alt={`Backdrop for ${featured?.title}`}
          cachePolicy="memory-disk"
          contentFit="cover"
          contentPosition="center"
          enforceEarlyResizing
          priority="high"
          source={artworkSource(backdropURI, headers)}
          onError={() => setFailedBackdrop(backdropURI)}
          style={[styles.backdrop as ImageStyle, { height: backdropHeight }]}
        />
      ) : null}
      <LinearGradient
        colors={[
          `${theme.background}10`,
          `${theme.background}B8`,
          theme.background,
        ]}
        locations={[0, 0.5, 1]}
        pointerEvents="none"
        style={[StyleSheet.absoluteFill, { height: backdropHeight }]}
      />
      {isTV ? (
        <LinearGradient
          colors={[
            theme.background,
            `${theme.background}B8`,
            `${theme.background}00`,
          ]}
          locations={[0, 0.4, 1]}
          pointerEvents="none"
          start={{ x: 0, y: 0 }}
          end={{ x: 1, y: 0 }}
          style={[StyleSheet.absoluteFill, { height: backdropHeight }]}
        />
      ) : null}
    </>
  );
  return (
    <ArtworkThemeContext.Provider value={theme}>
      <View
        role="main"
        style={[styles.screen, { backgroundColor: theme.background }]}
      >
        <HomeBackdrop />
        {isTV ? artwork : null}
        <SafeAreaView
          edges={isTV ? undefined : ['left', 'right']}
          style={styles.safeArea}
        >
          <View
            pointerEvents="box-none"
            onLayout={(event) =>
              setHeaderHeight(event.nativeEvent.layout.height)
            }
            style={
              !isTV
                ? {
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    right: 0,
                    zIndex: 2,
                    paddingTop: insets.top,
                    backgroundColor: 'transparent',
                  }
                : undefined
            }
          >
            <View
              pointerEvents="box-none"
              style={[
                styles.header,
                accessible ? styles.headerAccessible : null,
                isTV ? { minHeight: 96 } : null,
                { paddingHorizontal: contentInset },
              ]}
            >
              <View
                style={
                  !isTV
                    ? {
                        backgroundColor: theme.surface,
                        borderRadius: 12,
                        padding: 6,
                      }
                    : undefined
                }
              >
                <BrandMark compact={!isTV && width < 360 && !accessible} />
              </View>
              <View
                style={[
                  styles.account,
                  accessible || isTV ? styles.accountAccessible : null,
                ]}
              >
                {wide && !isTV ? (
                  <Text
                    numberOfLines={1}
                    style={[styles.server, { color: theme.muted }]}
                  >
                    {home.server}
                  </Text>
                ) : null}
                {isTV && onBrowse ? (
                  <ActionButton
                    label="Search & browse"
                    onPress={() => onBrowse()}
                    quiet
                    style={styles.quickAction}
                  />
                ) : null}
                {isTV && onDownloads ? (
                  <ActionButton
                    label="Downloads"
                    onPress={onDownloads}
                    quiet
                    style={styles.quickAction}
                  />
                ) : null}
                {showViewer && !compact ? (
                  <Text
                    numberOfLines={1}
                    style={[styles.viewer, { color: theme.text }]}
                  >
                    {home.viewer.name}
                  </Text>
                ) : null}
                {compact ? (
                  <ActionButton
                    label={home.viewer.name}
                    expanded={accountOpen}
                    accessibilityLabel={`Account options for ${home.viewer.name}`}
                    onPress={() => setAccountOpen(!accountOpen)}
                    quiet
                    style={styles.accountButton}
                  />
                ) : null}
                {!compact && onTheme && themeLabel ? (
                  <ActionButton
                    accessibilityLabel={`Theme: ${themeLabel}`}
                    label={themeLabel}
                    onPress={onTheme}
                    quiet
                    style={styles.theme}
                  />
                ) : null}
                {!compact && onSignOut ? (
                  <ActionButton
                    label="Sign out"
                    onPress={onSignOut}
                    quiet
                    style={styles.signOut}
                  />
                ) : null}
              </View>
            </View>
            {compact && accountOpen ? (
              <View
                style={[
                  styles.quickActions,
                  {
                    paddingHorizontal: contentInset,
                    paddingBottom: spacing.two,
                  },
                ]}
              >
                {onTheme && themeLabel ? (
                  <ActionButton
                    label={themeLabel}
                    accessibilityLabel={`Theme: ${themeLabel}`}
                    onPress={onTheme}
                    quiet
                  />
                ) : null}
                {onSignOut ? (
                  <ActionButton label="Sign out" onPress={onSignOut} quiet />
                ) : null}
              </View>
            ) : null}
          </View>
          <ScrollView
            contentContainerStyle={[
              styles.content,
              { paddingHorizontal: contentInset },
              !isTV
                ? {
                    paddingTop: headerHeight,
                    paddingBottom: 160 + insets.bottom,
                  }
                : null,
              compact || isTV ? styles.contentCompact : null,
            ]}
            showsVerticalScrollIndicator={false}
          >
            {!isTV ? artwork : null}
            {featured ? (
              <View
                style={[
                  styles.hero,
                  compact ? styles.heroCompact : null,
                  {
                    minHeight: featureHeight,
                    justifyContent: 'flex-end',
                    paddingBottom: spacing.three,
                  },
                  short ? { minHeight: 0, paddingTop: 12, gap: 8 } : null,
                  !hasBackdrop
                    ? { minHeight: 0, paddingTop: spacing.three }
                    : null,
                ]}
              >
                <Text style={[styles.eyebrow, { color: theme.muted }]}>
                  {!progress?.complete && featured.progress.seconds > 0
                    ? 'PICK UP WHERE YOU LEFT OFF'
                    : 'FROM YOUR LIBRARY'}
                </Text>
                <Text
                  numberOfLines={accessible ? undefined : 2}
                  style={[
                    styles.heroTitle,
                    compact ? styles.heroTitleCompact : null,
                    short ? { fontSize: 28, lineHeight: 32 } : null,
                    { color: theme.text },
                  ]}
                >
                  {featured.title}
                </Text>
                {!short ? (
                  <Text
                    numberOfLines={accessible ? undefined : 2}
                    style={[
                      styles.heroSummary,
                      compact ? styles.heroSummaryCompact : null,
                      { color: theme.muted },
                    ]}
                  >
                    {featured.plot ||
                      [featured.year, featured.genres]
                        .filter(Boolean)
                        .join(' · ') ||
                      'Ready to play.'}
                  </Text>
                ) : null}
                <WatchProgressIndicator progress={progress} />
                <ActionButton
                  preferredFocus
                  label={
                    progress?.complete
                      ? 'Play again'
                      : featured.progress.seconds > 0
                        ? 'Resume'
                        : 'View details'
                  }
                  onPress={() =>
                    (progress?.complete || featured.progress.seconds > 0) &&
                    onPlay
                      ? onPlay(featured.id, progress?.complete)
                      : onOpen(featured.id)
                  }
                  style={StyleSheet.flatten([
                    styles.heroAction,
                    compact ? { alignSelf: 'stretch' } : null,
                  ])}
                />
              </View>
            ) : !featured ? (
              <View style={styles.empty}>
                <Text
                  accessibilityRole="header"
                  style={[styles.heroTitle, { color: theme.text }]}
                >
                  Your library is ready for its first title.
                </Text>
                <Text style={[styles.heroSummary, { color: theme.muted }]}>
                  Add media on Kinosail Server, then refresh this screen.
                </Text>
                {onRefresh ? (
                  <ActionButton
                    label="Refresh library"
                    onPress={onRefresh}
                    style={[
                      styles.heroAction,
                      compact ? { alignSelf: 'stretch' } : null,
                    ]}
                  />
                ) : null}
              </View>
            ) : null}
            <Shelf
              title="Continue watching"
              items={continuing.filter(
                (item) => item.id !== featured?.id || !progress?.complete,
              )}
              cardWidth={isTV ? cardWidth : Math.min(width * 0.72, 360)}
              mediaURL={mediaURL}
              headers={headers}
              landscape={!isTV}
              onOpen={!isTV && onPlay ? onPlay : onOpen}
              onHighlight={setHighlighted}
            />
            <Shelf
              title="Recently added"
              items={home.recent}
              cardWidth={cardWidth}
              mediaURL={mediaURL}
              headers={headers}
              onOpen={onOpen}
              onHighlight={setHighlighted}
            />
            {!isTV ? (
              <>
                <Shelf
                  title="Movies"
                  items={home.recent.filter(
                    (item) => item.kind === 'video' && !item.show,
                  )}
                  cardWidth={cardWidth}
                  mediaURL={mediaURL}
                  headers={headers}
                  onOpen={onOpen}
                  onHighlight={setHighlighted}
                  onSeeAll={onBrowse ? () => onBrowse('movies') : undefined}
                />
                <Shelf
                  title="TV shows"
                  items={home.recent.filter(
                    (item) => item.kind === 'video' && item.show,
                  )}
                  cardWidth={cardWidth}
                  mediaURL={mediaURL}
                  headers={headers}
                  onOpen={onOpen}
                  onHighlight={setHighlighted}
                  onSeeAll={onBrowse ? () => onBrowse('shows') : undefined}
                />
              </>
            ) : null}
          </ScrollView>
        </SafeAreaView>
      </View>
    </ArtworkThemeContext.Provider>
  );
}

export function HomeView(props: HomeViewProps) {
  return Platform.isTV ? (
    <TVHomeView {...props} />
  ) : (
    <StandardHomeView {...props} />
  );
}
