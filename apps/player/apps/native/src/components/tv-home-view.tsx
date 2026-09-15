import React, { useState } from 'react';
import {
  FlatList,
  ScrollView,
  StyleSheet,
  Text,
  useWindowDimensions,
  View,
} from 'react-native';
import { Image } from 'expo-image';
import { LinearGradient } from 'expo-linear-gradient';
import { SafeAreaView } from 'react-native-safe-area-context';
import { artworkSource } from '@/core/artwork-source';
import type { MediaItem } from '@/core/contract';
import { useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';
import { BrandMark } from './brand-mark';
import { FocusGroup } from './focus-group';
import type { HomeViewProps } from './home-view';
import { MediaCard } from './media-card';
import { ModalSheet } from './modal-sheet';

function TVShelf({
  title,
  items,
  resume,
  onHighlight,
  ...props
}: HomeViewProps & {
  title: string;
  items: MediaItem[];
  resume?: boolean;
  onHighlight(item: MediaItem): void;
}) {
  const theme = useKinoTheme();
  const { width } = useWindowDimensions();
  const cardWidth = Math.round(width * (resume ? 0.21 : 0.135));
  if (!items.length) return null;
  return (
    <FocusGroup horizontal style={styles.shelf}>
      <Text
        accessibilityRole="header"
        style={[styles.shelfTitle, { color: theme.text }]}
      >
        {title}
      </Text>
      <FlatList
        accessibilityLabel={`${title} shelf`}
        horizontal
        data={items}
        keyExtractor={(item) => item.id}
        showsHorizontalScrollIndicator={false}
        removeClippedSubviews={false}
        contentContainerStyle={styles.shelfContent}
        ItemSeparatorComponent={() => <View style={{ width: 24 }} />}
        getItemLayout={(_, index) => ({
          index,
          length: cardWidth + 24,
          offset: index * (cardWidth + 24),
        })}
        renderItem={({ item }) => (
          <MediaCard
            item={item}
            width={cardWidth}
            landscape={resume}
            resume={Boolean(resume && props.onPlay)}
            imageURL={props.mediaURL(
              resume ? item.backdrop || item.artwork : item.artwork,
            )}
            headers={props.headers}
            onHighlight={() => onHighlight(item)}
            onPress={() =>
              resume && props.onPlay
                ? props.onPlay(item.id)
                : props.onOpen(item.id)
            }
          />
        )}
      />
    </FocusGroup>
  );
}

// TV owns composition and remote navigation; media operations stay shared.
export function TVHomeView(props: HomeViewProps) {
  const { home, onBrowse, onOpen, onPlay } = props;
  const theme = useKinoTheme();
  const { height, width } = useWindowDimensions();
  const [highlighted, setHighlighted] = useState<MediaItem | null>(null);
  const [accountOpen, setAccountOpen] = useState(false);
  const [failedArtwork, setFailedArtwork] = useState('');
  const items = [...home.continueWatching, ...home.recent];
  const featured =
    items.find((item) => item.id === highlighted?.id) ?? items[0];
  const backdrop = featured?.backdrop || featured?.artwork;
  const uri = backdrop ? props.mediaURL(backdrop) : '';
  return (
    <View style={[styles.screen, { backgroundColor: theme.background }]}>
      {uri && failedArtwork !== uri ? (
        <Image
          accessibilityLabel={`Backdrop for ${featured?.title}`}
          source={artworkSource(uri, props.headers)}
          contentFit="cover"
          onError={() => setFailedArtwork(uri)}
          style={[styles.artwork, { height: height * 0.8 }]}
        />
      ) : null}
      <LinearGradient
        pointerEvents="none"
        colors={[
          theme.background,
          `${theme.background}C0`,
          `${theme.background}00`,
        ]}
        locations={[0, 0.42, 1]}
        start={{ x: 0, y: 0 }}
        end={{ x: 1, y: 0 }}
        style={StyleSheet.absoluteFill}
      />
      <LinearGradient
        pointerEvents="none"
        colors={[`${theme.background}00`, theme.background]}
        locations={[0.2, 0.8]}
        style={StyleSheet.absoluteFill}
      />
      <SafeAreaView edges={['top', 'bottom']} style={styles.screen}>
        <FocusGroup
          style={[styles.navigation, { paddingHorizontal: width * 0.045 }]}
        >
          <BrandMark />
          <View style={styles.destinations}>
            {onBrowse ? (
              <>
                <ActionButton
                  label="Movies"
                  quiet
                  onPress={() => onBrowse('movies')}
                  style={styles.navButton}
                />
                <ActionButton
                  label="TV shows"
                  quiet
                  onPress={() => onBrowse('shows')}
                  style={styles.navButton}
                />
                <ActionButton
                  label="My List"
                  quiet
                  onPress={() => onBrowse('list')}
                  style={styles.navButton}
                />
                <ActionButton
                  label="Search"
                  quiet
                  onPress={() => onBrowse()}
                  style={styles.navButton}
                />
              </>
            ) : null}
          </View>
          <ActionButton
            label="Account"
            accessibilityLabel={`Account options for ${home.viewer.name}`}
            quiet
            expanded={accountOpen}
            onPress={() => setAccountOpen(true)}
            style={styles.navButton}
          />
        </FocusGroup>
        <ScrollView
          showsVerticalScrollIndicator={false}
          contentContainerStyle={{ paddingBottom: 64 }}
        >
          {featured ? (
            <View
              testID="tv-feature"
              style={[
                styles.feature,
                {
                  height: height >= 900 ? 480 : 280,
                  paddingHorizontal: width * 0.05,
                },
              ]}
            >
              <Text
                accessibilityRole="header"
                numberOfLines={2}
                style={[
                  styles.title,
                  height < 900 ? { fontSize: 48, lineHeight: 56 } : null,
                  { color: theme.text, maxWidth: width * 0.56 },
                ]}
              >
                {featured.title}
              </Text>
              <Text
                numberOfLines={1}
                style={[styles.facts, { color: theme.text }]}
              >
                {[
                  featured.year,
                  featured.rating,
                  featured.show
                    ? `S${featured.season} E${featured.episode}`
                    : featured.genres,
                ]
                  .filter(Boolean)
                  .join(' · ')}
              </Text>
              {height >= 900 ? (
                <Text
                  numberOfLines={2}
                  style={[
                    styles.plot,
                    { color: theme.muted, maxWidth: width * 0.48 },
                  ]}
                >
                  {featured.plot ||
                    'Choose a title and make yourself comfortable.'}
                </Text>
              ) : null}
              <FocusGroup style={styles.actions}>
                <ActionButton
                  preferredFocus
                  style={{ minHeight: 64 }}
                  label={
                    featured.progress.seconds > 0 && onPlay
                      ? 'Resume'
                      : 'View details'
                  }
                  onPress={() =>
                    featured.progress.seconds > 0 && onPlay
                      ? onPlay(featured.id)
                      : onOpen(featured.id)
                  }
                />
                {featured.progress.seconds > 0 && onPlay ? (
                  <ActionButton
                    label="Details"
                    style={{ minHeight: 64 }}
                    quiet
                    onPress={() => onOpen(featured.id)}
                  />
                ) : null}
              </FocusGroup>
            </View>
          ) : (
            <View style={[styles.empty, { paddingHorizontal: width * 0.05 }]}>
              <Text
                accessibilityRole="header"
                style={[styles.title, { color: theme.text }]}
              >
                Your library starts here
              </Text>
              <Text style={[styles.plot, { color: theme.muted }]}>
                Add media on Kinosail Server, then refresh your library.
              </Text>
              {props.onRefresh ? (
                <ActionButton
                  preferredFocus
                  style={{ minHeight: 64 }}
                  label="Refresh library"
                  onPress={props.onRefresh}
                />
              ) : null}
            </View>
          )}
          <TVShelf
            {...props}
            title="Continue watching"
            items={home.continueWatching}
            resume
            onHighlight={setHighlighted}
          />
          <TVShelf
            {...props}
            title="Recently added"
            items={home.recent}
            onHighlight={setHighlighted}
          />
          <TVShelf
            {...props}
            title="Movies"
            items={home.recent.filter(
              (item) => item.kind === 'video' && !item.show,
            )}
            onHighlight={setHighlighted}
          />
          <TVShelf
            {...props}
            title="TV shows"
            items={home.recent.filter(
              (item) => item.kind === 'video' && item.show,
            )}
            onHighlight={setHighlighted}
          />
        </ScrollView>
      </SafeAreaView>
      {accountOpen ? (
        <ModalSheet
          title={home.viewer.name}
          onClose={() => setAccountOpen(false)}
        >
          <Text style={[styles.plot, { color: theme.muted }]}>
            {home.server}
          </Text>
          {props.onRefresh ? (
            <ActionButton
              label="Refresh library"
              quiet
              onPress={() => {
                setAccountOpen(false);
                props.onRefresh?.();
              }}
            />
          ) : null}
          {props.onTheme && props.themeLabel ? (
            <ActionButton
              label={`Theme: ${props.themeLabel}`}
              quiet
              onPress={props.onTheme}
            />
          ) : null}
          {props.onDownloads ? (
            <ActionButton label="Downloads" quiet onPress={props.onDownloads} />
          ) : null}
          {props.onSignOut ? (
            <ActionButton label="Sign out" quiet onPress={props.onSignOut} />
          ) : null}
        </ModalSheet>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1 },
  artwork: { position: 'absolute', top: 0, right: 0, width: '85%' },
  navigation: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 32,
    paddingVertical: 20,
  },
  destinations: { flex: 1, flexDirection: 'row', gap: 16 },
  navButton: {
    minHeight: 64,
    backgroundColor: 'transparent',
    borderColor: 'transparent',
    paddingHorizontal: 20,
  },
  feature: { justifyContent: 'flex-end', gap: 12, paddingVertical: 16 },
  title: {
    fontSize: 56,
    lineHeight: 64,
    fontWeight: '700',
    letterSpacing: -1.5,
  },
  facts: { fontSize: 23, lineHeight: 30 },
  plot: { fontSize: 24, lineHeight: 34 },
  actions: { flexDirection: 'row', gap: 20 },
  shelf: { gap: 8, marginBottom: 24 },
  shelfTitle: { fontSize: 30, fontWeight: '600', marginHorizontal: '5%' },
  shelfContent: { paddingHorizontal: '5%', paddingVertical: 12 },
  empty: {
    minHeight: 460,
    justifyContent: 'center',
    alignItems: 'flex-start',
    gap: 24,
  },
});
