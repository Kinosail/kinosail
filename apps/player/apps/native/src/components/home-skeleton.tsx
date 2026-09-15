import React from 'react';
import { Platform, ScrollView, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { LinearGradient } from 'expo-linear-gradient';
import { radius, spacing, useKinoTheme } from '@/design/tokens';
import { BrandMark } from './brand-mark';
import { ScreenState } from './screen-state';
import { homeViewStyles as styles } from './home-view.styles';
import { useHomeLayout } from './use-home-layout';

export function HomeSkeleton({
  message = 'Loading your library…',
}: {
  message?: string;
}) {
  return Platform.isTV ? (
    <ScreenState loading message={message} />
  ) : (
    <MobileHomeSkeleton message={message} />
  );
}

function MobileHomeSkeleton({ message }: { message: string }) {
  const theme = useKinoTheme();
  const {
    insets,
    width,
    fontScale,
    headerHeight,
    setHeaderHeight,
    accessible,
    compact,
    short,
    cardWidth,
    featureHeight,
    backdropHeight,
    contentInset,
  } = useHomeLayout();
  const block = { backgroundColor: theme.line, borderRadius: 4 };
  return (
    <View
      accessibilityRole="progressbar"
      accessibilityLabel={message}
      accessibilityState={{ busy: true }}
      accessible
      pointerEvents="none"
      style={[styles.screen, { backgroundColor: theme.background }]}
    >
      <SafeAreaView edges={['left', 'right']} style={styles.safeArea}>
        <View
          onLayout={(event) => setHeaderHeight(event.nativeEvent.layout.height)}
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            right: 0,
            zIndex: 2,
            paddingTop: insets.top,
          }}
        >
          <View
            style={[
              styles.header,
              accessible ? styles.headerAccessible : null,
              { paddingHorizontal: contentInset },
            ]}
          >
            <View
              style={{
                backgroundColor: theme.surface,
                borderRadius: 12,
                padding: 6,
              }}
            >
              <BrandMark compact={width < 360 && !accessible} />
            </View>
            <View
              style={[block, { width: 96, height: 44, borderRadius: 24 }]}
            />
          </View>
        </View>
        <ScrollView
          scrollEnabled={false}
          showsVerticalScrollIndicator={false}
          contentContainerStyle={[
            styles.content,
            compact ? styles.contentCompact : null,
            {
              paddingHorizontal: contentInset,
              paddingTop: headerHeight,
              paddingBottom: 160 + insets.bottom,
            },
          ]}
        >
          <LinearGradient
            colors={[theme.raised, theme.background]}
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              right: 0,
              height: backdropHeight,
            }}
          />
          <View
            testID="home-skeleton-hero"
            style={[
              styles.hero,
              compact ? styles.heroCompact : null,
              {
                minHeight: featureHeight,
                justifyContent: 'flex-end',
                paddingBottom: spacing.three,
              },
              short ? { minHeight: 0, paddingTop: 12, gap: 8 } : null,
            ]}
          >
            <Text style={[styles.eyebrow, { color: theme.muted }]}>
              {message}
            </Text>
            <View
              style={[
                block,
                {
                  width: '75%',
                  height: (short ? 32 : compact ? 47 : 46) * fontScale,
                },
              ]}
            />
            {!short ? (
              <View
                style={[
                  block,
                  { width: '90%', height: (compact ? 22 : 24) * fontScale },
                ]}
              />
            ) : null}
            <View
              style={[
                block,
                styles.heroAction,
                { height: 48, borderRadius: radius.control },
                compact ? { alignSelf: 'stretch' } : { width: 148 },
              ]}
            />
          </View>
          {['continue', 'recent', 'movies', 'shows'].map((shelf) => {
            const landscape = shelf === 'continue';
            const tileWidth = landscape
              ? Math.min(width * 0.72, 360)
              : cardWidth;
            return (
              <View key={shelf} style={styles.shelf}>
                <View
                  style={{
                    height:
                      shelf === 'movies' || shelf === 'shows'
                        ? Math.max(44, 26 * fontScale)
                        : 26 * fontScale,
                    justifyContent: 'center',
                  }}
                >
                  <View
                    style={[block, { width: 170, height: 21 * fontScale }]}
                  />
                </View>
                <View
                  style={[
                    styles.shelfRow,
                    { flexDirection: 'row', gap: 12, overflow: 'hidden' },
                  ]}
                >
                  {Array.from(
                    { length: Math.ceil(width / (tileWidth + 12)) },
                    (_, index) => (
                      <View
                        key={index}
                        style={{
                          width: tileWidth,
                          flexShrink: 0,
                          gap: spacing.fine,
                        }}
                      >
                        <View
                          testID={
                            landscape
                              ? 'home-skeleton-resume'
                              : 'home-skeleton-poster'
                          }
                          style={[
                            block,
                            {
                              width: tileWidth,
                              aspectRatio: landscape ? 16 / 9 : 2 / 3,
                              borderRadius: radius.card,
                            },
                          ]}
                        />
                        <View
                          style={[
                            block,
                            {
                              width: '75%',
                              height: 19 * Math.min(fontScale, 2),
                              marginTop: 4,
                            },
                          ]}
                        />
                        <View
                          style={[
                            block,
                            {
                              width: '45%',
                              height: 14 * Math.min(fontScale, 2),
                            },
                          ]}
                        />
                      </View>
                    ),
                  )}
                </View>
              </View>
            );
          })}
        </ScrollView>
      </SafeAreaView>
    </View>
  );
}
