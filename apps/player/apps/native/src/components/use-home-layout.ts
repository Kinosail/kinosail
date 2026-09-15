import { useState } from 'react';
import { Platform, useWindowDimensions } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

// Shared by Home and its loading state so responsive geometry stays aligned.
export function useHomeLayout() {
  const insets = useSafeAreaInsets();
  const [headerHeight, setHeaderHeight] = useState(76 + insets.top);
  const { fontScale, width, height } = useWindowDimensions();
  const isTV = Platform.isTV;
  const accessible = !isTV && fontScale >= 1.5;
  const wide = width >= 980 && !accessible;
  const compact = !isTV && !wide;
  const short = !isTV && height - insets.top - insets.bottom < 520;
  const showViewer = isTV || (!accessible && width >= 360);
  const cardWidth = isTV
    ? 206
    : (width < 420 ? 96 : 144) * Math.min(Math.max(fontScale, 1), 1.5);
  const featureHeight = isTV
    ? Math.min(440, Math.max(300, height * 0.38))
    : short
      ? 0
      : Math.min(700, Math.max(compact ? 470 : 480, height * 0.62));
  const backdropHeight = featureHeight + (isTV ? 120 : headerHeight + 100);
  const contentInset = isTV ? Math.min(80, width * 0.05) : wide ? 56 : 20;
  return {
    insets,
    headerHeight,
    setHeaderHeight,
    width,
    fontScale,
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
  };
}
