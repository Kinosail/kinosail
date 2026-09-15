import { useEffect, useRef } from 'react';
import { BackHandler, Platform, TVEventControl } from 'react-native';
import { router, usePathname } from 'expo-router';

export function useTVNavigation() {
  const pathname = usePathname();
  const currentPath = useRef(pathname);
  currentPath.current = pathname;
  // Register once: a screen's later Back handler gets first refusal (e.g. playback options).
  useEffect(() => {
    if (!Platform.isTV) return;
    const subscription = BackHandler.addEventListener(
      'hardwareBackPress',
      () => {
        if (currentPath.current === '/') return false;
        if (router.canGoBack()) router.back();
        else router.replace('/');
        return true;
      },
    );
    return () => subscription.remove();
  }, []);
  useEffect(() => {
    if (!Platform.isTV || Platform.OS !== 'ios') return;
    if (pathname === '/') TVEventControl.disableTVMenuKey();
    else TVEventControl.enableTVMenuKey();
    return () => TVEventControl.disableTVMenuKey();
  }, [pathname]);
}
