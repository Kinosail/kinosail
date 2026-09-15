import { artworkSource } from '@/core/artwork-source';
import { Image } from 'expo-image';
import { LinearGradient } from 'expo-linear-gradient';
import React, { useState } from 'react';
import {
  Platform,
  ScrollView,
  StyleSheet,
  Text,
  type ImageStyle,
  useWindowDimensions,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { MediaItem } from '@/core/contract';
import {
  ArtworkThemeContext,
  radius,
  spacing,
  useKinoTheme,
} from '@/design/tokens';

import { ActionButton } from './action-button';
import { WatchProgressIndicator, useWatchProgress } from './watch-progress';
import { BottomActions } from './bottom-actions';
import { ModalSheet } from './modal-sheet';

type Props = {
  item: MediaItem;
  mediaURL(path: string): string;
  headers: Record<string, string>;
  onBack(): void;
  onPlay(): void;
  onPlayOnTV?: () => void;
  onPlayFromBeginning?: () => void;
  onDownload?: () => void;
  onShow?: () => void;
};

const facts = (item: MediaItem): string =>
  [
    item.year,
    item.rating,
    item.show ? `S${item.season} E${item.episode}` : '',
    item.container.toUpperCase(),
  ]
    .filter(Boolean)
    .join(' · ');

export function DetailView({
  item,
  mediaURL,
  headers,
  onBack,
  onPlay,
  onPlayOnTV,
  onPlayFromBeginning,
  onDownload,
  onShow,
}: Props) {
  const theme = useKinoTheme();
  const progress = useWatchProgress({
    id: item.id,
    seconds: item.progress.seconds,
    watched: item.progress.watched,
    mediaURL,
    headers,
  });
  const [more, setMore] = useState(false);
  const bottomControls = !Platform.isTV;
  const posterURI = item.artwork ? mediaURL(item.artwork) : '';
  const [failedPoster, setFailedPoster] = useState('');
  const backdropURI = item.backdrop ? mediaURL(item.backdrop) : '';
  const [failedBackdrop, setFailedBackdrop] = useState('');
  const showBackdrop = Boolean(backdropURI && failedBackdrop !== backdropURI);
  const { fontScale, width } = useWindowDimensions();
  const wide = Platform.isTV || (width >= 800 && fontScale < 1.5);
  const back = (
    <ActionButton label="Back" onPress={onBack} quiet style={styles.back} />
  );
  const play = (
    <ActionButton
      label={
        item.kind === 'book'
          ? 'Read ebook'
          : progress?.complete
            ? 'Play again'
            : item.progress.seconds > 0
              ? 'Resume'
              : 'Play'
      }
      preferredFocus
      onPress={
        progress?.complete && onPlayFromBeginning ? onPlayFromBeginning : onPlay
      }
      style={bottomControls ? { flexGrow: 1, minHeight: 48 } : styles.play}
    />
  );
  return (
    <ArtworkThemeContext.Provider value={theme}>
      <View
        role="main"
        style={[styles.screen, { backgroundColor: theme.background }]}
      >
        {showBackdrop ? (
          <Image
            accessibilityIgnoresInvertColors
            accessibilityLabel={`Backdrop for ${item.title}`}
            alt={`Backdrop for ${item.title}`}
            cachePolicy="memory-disk"
            contentFit="cover"
            enforceEarlyResizing
            source={artworkSource(backdropURI, headers)}
            onError={() => setFailedBackdrop(backdropURI)}
            style={[
              styles.backdrop as ImageStyle,
              !wide
                ? { height: Math.min(width * 1.15, 520), opacity: 0.85 }
                : null,
            ]}
          />
        ) : null}
        <LinearGradient
          colors={[`${theme.background}10`, theme.background]}
          locations={[0, 0.8]}
          pointerEvents="none"
          style={
            wide
              ? StyleSheet.absoluteFill
              : {
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  right: 0,
                  height: Math.min(width * 1.15, 520),
                }
          }
        />
        {Platform.isTV ? (
          <LinearGradient
            pointerEvents="none"
            colors={[
              theme.background,
              `${theme.background}D9`,
              `${theme.background}00`,
            ]}
            locations={[0, 0.4, 1]}
            start={{ x: 0, y: 0 }}
            end={{ x: 1, y: 0 }}
            style={StyleSheet.absoluteFill}
          />
        ) : null}
        <SafeAreaView
          edges={Platform.isTV ? ['top', 'bottom'] : undefined}
          style={styles.safeArea}
        >
          <ScrollView
            contentContainerStyle={[
              styles.scroll,
              Platform.isTV
                ? { paddingHorizontal: Math.max(64, width * 0.05) }
                : null,
            ]}
          >
            {!bottomControls ? back : null}
            <View
              style={[
                styles.content,
                {
                  flexDirection: wide && !showBackdrop ? 'row' : 'column',
                  alignItems: Platform.isTV ? 'flex-start' : 'center',
                },
                wide && showBackdrop
                  ? {
                      minHeight: Platform.isTV ? 580 : 480,
                      justifyContent: 'flex-end',
                    }
                  : null,
                Platform.isTV
                  ? {
                      minHeight: 0,
                      justifyContent: 'flex-start',
                      paddingTop: 80,
                    }
                  : null,
                !wide
                  ? {
                      justifyContent: 'flex-start',
                      paddingTop: showBackdrop
                        ? Math.min(width * 0.7, 320)
                        : 24,
                      alignItems: 'stretch',
                    }
                  : null,
              ]}
            >
              {!showBackdrop && posterURI && failedPoster !== posterURI ? (
                <Image
                  accessibilityLabel={`Poster for ${item.title}`}
                  alt={`Poster for ${item.title}`}
                  cachePolicy="memory-disk"
                  contentFit={item.show ? 'contain' : 'cover'}
                  onError={() => setFailedPoster(posterURI)}
                  enforceEarlyResizing
                  priority="high"
                  source={artworkSource(posterURI, headers)}
                  style={[
                    styles.poster as ImageStyle,
                    {
                      width: wide ? 260 : Math.min(width * 0.48, 220),
                      alignSelf: 'center',
                    },
                  ]}
                />
              ) : null}
              <View
                style={[
                  styles.copy,
                  !wide ? { alignSelf: 'center', width: '100%' } : null,
                ]}
              >
                {wide && item.tagline ? (
                  <Text style={[styles.eyebrow, { color: theme.signal }]}>
                    {item.tagline.toUpperCase()}
                  </Text>
                ) : null}
                <Text
                  accessibilityRole="header"
                  style={[
                    styles.title,
                    !wide
                      ? { fontSize: 42, lineHeight: 47, letterSpacing: -1.2 }
                      : null,
                    { color: theme.text },
                  ]}
                >
                  {item.title}
                </Text>
                {!bottomControls && onDownload ? (
                  <ActionButton label="Download" quiet onPress={onDownload} />
                ) : null}
                <Text style={[styles.facts, { color: theme.muted }]}>
                  {facts(item)}
                </Text>
                {Platform.isTV ? (
                  <Text style={[styles.plot, { color: theme.muted }]}>
                    {item.plot || 'No description is available for this title.'}
                  </Text>
                ) : null}
                <WatchProgressIndicator progress={progress} />
                {!bottomControls ? (
                  <View
                    style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 16 }}
                  >
                    {play}
                    {!bottomControls &&
                    item.kind !== 'book' &&
                    item.progress.seconds > 0 &&
                    onPlayFromBeginning ? (
                      <ActionButton
                        label="Play from beginning"
                        quiet
                        onPress={onPlayFromBeginning}
                        style={
                          !wide
                            ? {
                                borderRadius: 28,
                                minHeight: 48,
                                alignSelf: 'stretch',
                              }
                            : styles.play
                        }
                      />
                    ) : null}
                  </View>
                ) : null}
                {!bottomControls && !Platform.isTV && item.kind !== 'book' ? (
                  <ActionButton
                    label="Play on TV"
                    quiet
                    onPress={onPlayOnTV ?? onPlay}
                    style={{ minHeight: 56 }}
                  />
                ) : null}
                {item.genres ? (
                  <Text style={[styles.genres, { color: theme.text }]}>
                    {item.genres}
                  </Text>
                ) : null}
                {!Platform.isTV ? (
                  <Text style={[styles.plot, { color: theme.muted }]}>
                    {item.plot || 'No description is available for this title.'}
                  </Text>
                ) : null}
                {item.showId && onShow ? (
                  <ActionButton
                    label="All seasons & episodes"
                    accessibilityLabel={`All seasons and episodes of ${item.show}`}
                    quiet
                    onPress={onShow}
                  />
                ) : null}
                {item.director ? (
                  <Text style={[styles.credit, { color: theme.muted }]}>
                    Directed by{' '}
                    <Text style={{ color: theme.text }}>{item.director}</Text>
                  </Text>
                ) : null}
                {item.kind === 'book' ? (
                  <Text style={[styles.direct, { color: theme.muted }]}>
                    Opens your server’s reader in your browser. You may need to
                    sign in.
                  </Text>
                ) : null}
              </View>
            </View>
          </ScrollView>
          {bottomControls ? (
            <BottomActions onBack={onBack}>
              {play}
              {onDownload || item.kind !== 'book' ? (
                <ActionButton
                  label="More"
                  accessibilityLabel="More title actions"
                  quiet
                  onPress={() => setMore(true)}
                />
              ) : null}
            </BottomActions>
          ) : null}
          {bottomControls && more ? (
            <ModalSheet title="Title actions" onClose={() => setMore(false)}>
              {onDownload ? (
                <ActionButton label="Download" quiet onPress={onDownload} />
              ) : null}
              {item.kind !== 'book' &&
              item.progress.seconds > 0 &&
              onPlayFromBeginning ? (
                <ActionButton
                  label="Play from beginning"
                  quiet
                  onPress={onPlayFromBeginning}
                />
              ) : null}
              {item.kind !== 'book' ? (
                <ActionButton
                  label="Play on TV"
                  quiet
                  onPress={onPlayOnTV ?? onPlay}
                  style={{ minHeight: 56 }}
                />
              ) : null}
            </ModalSheet>
          ) : null}
        </SafeAreaView>
      </View>
    </ArtworkThemeContext.Provider>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1 },
  safeArea: { flex: 1 },
  backdrop: {
    height: '78%',
    opacity: 0.85,
    position: 'absolute',
    right: 0,
    top: 0,
    width: '100%',
  },
  scroll: { flexGrow: 1, padding: Platform.isTV ? spacing.six : spacing.two },
  back: { alignSelf: 'flex-start', minWidth: 96 },
  content: {
    alignItems: 'center',
    flex: 1,
    gap: Platform.isTV ? spacing.six : spacing.four,
    justifyContent: 'center',
    paddingVertical: spacing.four,
  },
  poster: { aspectRatio: 2 / 3, borderRadius: radius.card },
  copy: { flexShrink: 1, gap: spacing.two, maxWidth: 720 },
  eyebrow: {
    fontSize: Platform.isTV ? 20 : 11,
    fontWeight: '900',
    letterSpacing: 2,
  },
  title: {
    fontSize: Platform.isTV ? 64 : 42,
    fontWeight: '900',
    letterSpacing: -1.5,
    lineHeight: Platform.isTV ? 72 : 47,
  },
  facts: {
    fontSize: Platform.isTV ? 24 : 14,
    fontWeight: '800',
    letterSpacing: 0.5,
  },
  genres: { fontSize: Platform.isTV ? 24 : 15, fontWeight: '700' },
  plot: {
    fontSize: Platform.isTV ? 24 : 16,
    lineHeight: Platform.isTV ? 36 : 25,
  },
  credit: { fontSize: Platform.isTV ? 22 : 14 },
  play: { alignSelf: 'flex-start', minWidth: 160, marginTop: spacing.one },
  direct: {
    fontSize: 11,
    fontWeight: '800',
    letterSpacing: 1.2,
    textTransform: 'uppercase',
  },
});
