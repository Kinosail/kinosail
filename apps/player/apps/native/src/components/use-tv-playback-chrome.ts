import { useCallback, useEffect, useState } from 'react';
import { AccessibilityInfo, BackHandler } from 'react-native';

// A remote press reveals controls; Back closes options, then controls, then the route.
export function useTVPlaybackChrome(
  enabled: boolean,
  pinned: boolean,
  options: boolean,
  closeOptions: () => void,
) {
  const [visible, setVisible] = useState(true);
  const [activity, setActivity] = useState(0);
  const [screenReader, setScreenReader] = useState(false);
  const reveal = useCallback(() => {
    setVisible(true);
    setActivity((value) => value + 1);
  }, []);
  useEffect(() => {
    if (!enabled) return;
    let active = true;
    void AccessibilityInfo.isScreenReaderEnabled().then((value) => {
      if (active) setScreenReader(value);
    });
    const subscription = AccessibilityInfo.addEventListener(
      'screenReaderChanged',
      setScreenReader,
    );
    return () => {
      active = false;
      subscription.remove();
    };
  }, [enabled]);
  useEffect(() => {
    if (!enabled) return;
    if (pinned || screenReader) {
      setVisible(true);
      return;
    }
    if (!visible) return;
    const timer = setTimeout(() => setVisible(false), 5000);
    return () => clearTimeout(timer);
  }, [activity, enabled, pinned, screenReader, visible]);
  useEffect(() => {
    if (!enabled) return;
    const subscription = BackHandler.addEventListener(
      'hardwareBackPress',
      () => {
        if (options) {
          closeOptions();
          reveal();
          return true;
        }
        if (visible && !pinned && !screenReader) {
          setVisible(false);
          return true;
        }
        return false;
      },
    );
    return () => subscription.remove();
  }, [closeOptions, enabled, options, pinned, reveal, screenReader, visible]);
  return { visible: visible || pinned || screenReader, reveal };
}
