import React from 'react';
import { Platform, View, useWindowDimensions } from 'react-native';
import { radius, useKinoTheme } from '@/design/tokens';

// The surrounding screen owns its header and spacing. Only missing results get
// placeholders, with one loading announcement and no focusable fake controls.
export function BrowseSkeleton({
  variant,
  label,
  width,
  columns = 1,
  rowGap = 24,
}: {
  variant: 'poster' | 'square' | 'album' | 'track' | 'row';
  label: string;
  width?: number;
  columns?: number;
  rowGap?: number;
}) {
  const theme = useKinoTheme();
  const { fontScale } = useWindowDimensions();
  const grid =
    variant === 'poster' || variant === 'square' || variant === 'album';
  const album = variant === 'album';
  const track = variant === 'track';
  const block = { backgroundColor: theme.line, borderRadius: 4 };
  return (
    <View
      accessibilityRole="progressbar"
      accessibilityLabel={label}
      accessibilityState={{ busy: true }}
      accessible
      pointerEvents="none"
      style={{ flexDirection: 'row', flexWrap: 'wrap', columnGap: 16, rowGap }}
    >
      {Array.from({ length: grid ? columns * 2 : 6 }, (_, index) =>
        grid ? (
          <View key={index} style={{ width, gap: album ? 8 : 4 }}>
            <View
              testID="browse-skeleton-artwork"
              style={[
                block,
                {
                  width: '100%',
                  aspectRatio: variant === 'poster' ? 2 / 3 : 1,
                  borderRadius: album ? 14 : radius.card,
                },
              ]}
            />
            <View
              style={[
                block,
                {
                  width: '80%',
                  height:
                    (Platform.isTV ? 54 : album ? 20 : 38) *
                    Math.min(fontScale, 2),
                  marginTop: album ? 0 : 8,
                },
              ]}
            />
            <View
              style={[
                block,
                {
                  width: '50%',
                  height: (Platform.isTV ? 20 : 14) * Math.min(fontScale, 2),
                },
              ]}
            />
          </View>
        ) : (
          <View
            key={index}
            testID="browse-skeleton-row"
            style={{
              width: '100%',
              minHeight: Platform.isTV ? 80 : track ? 64 : 48,
              padding: 12,
              borderWidth: 2,
              borderColor: 'transparent',
              borderRadius: radius.control,
              flexDirection: 'row',
              alignItems: 'center',
              gap: 16,
              backgroundColor: track ? undefined : theme.surface,
            }}
          >
            {track ? <View style={[block, { width: 28, height: 24 }]} /> : null}
            <View
              style={{
                flex: 1,
                gap: 4,
                alignItems: track ? 'flex-start' : 'center',
              }}
            >
              <View
                style={[
                  block,
                  {
                    width: '65%',
                    height: (Platform.isTV ? 28 : 17) * fontScale,
                  },
                ]}
              />
              {track ? (
                <View
                  style={[block, { width: '40%', height: 14 * fontScale }]}
                />
              ) : null}
            </View>
            {track ? <View style={[block, { width: 24, height: 24 }]} /> : null}
          </View>
        ),
      )}
    </View>
  );
}
