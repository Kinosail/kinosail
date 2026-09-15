import { useTVEventHandler } from 'react-native';
export function usePlaybackRemote(toggle: () => void, onActivity?: () => void) {
  useTVEventHandler((event) => {
    if (
      event.eventKeyAction === 1 &&
      ['up', 'down', 'left', 'right', 'playPause'].includes(event.eventType)
    )
      onActivity?.();
    if (event.eventType === 'playPause' && event.eventKeyAction === 1) toggle();
  });
}
