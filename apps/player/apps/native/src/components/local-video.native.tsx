import { requireNativeView } from 'expo';
import { localCompatibilityAvailable } from '@/core/protected-media';
import React, { useImperativeHandle, useRef } from 'react';
import { Platform, type NativeSyntheticEvent } from 'react-native';
import type {
  LocalVideoProps,
  VLCPlayerRef,
  VLCPlayerTracks,
} from './local-video.types';
export type { VLCPlayerRef, VLCPlayerTracks } from './local-video.types';
type NativeProps = Omit<
  LocalVideoProps,
  'source' | 'onProgress' | 'onTracks' | 'onBuffering'
> & {
  ref?: React.Ref<VLCPlayerRef>;
  uri: string;
  start: number;
  onBuffering(event: NativeSyntheticEvent<{ active: boolean }>): void;
  onProgress(event: NativeSyntheticEvent<{ currentTime: number }>): void;
  onTracks(event: NativeSyntheticEvent<VLCPlayerTracks>): void;
};
const NativeVideo =
  Platform.OS === 'ios' && localCompatibilityAvailable
    ? requireNativeView<NativeProps>('ProtectedMedia')
    : null;
export default React.forwardRef<VLCPlayerRef, LocalVideoProps>(
  function LocalVideo(
    { source, onProgress, onTracks, onBuffering, ...props },
    ref,
  ) {
    const native = useRef<VLCPlayerRef>(null);
    useImperativeHandle(
      ref,
      () => ({
        startPictureInPicture: () => {
          native.current?.startPictureInPicture();
        },
        seek: (value) => {
          native.current?.seek(value);
        },
        getTracks: () => {
          native.current?.getTracks();
        },
        selectAudioTrack: (value) => {
          native.current?.selectAudioTrack(value);
        },
        selectSubtitleTrack: (value) => {
          native.current?.selectSubtitleTrack(value);
        },
        setAudioDelay: (value) => {
          native.current?.setAudioDelay(value);
        },
        setSubtitleDelay: (value) => {
          native.current?.setSubtitleDelay(value);
        },
      }),
      [],
    );
    return NativeVideo ? (
      <NativeVideo
        {...props}
        ref={native}
        uri={source.uri}
        start={source.start}
        onBuffering={(event) => onBuffering(event.nativeEvent)}
        onProgress={(event) => onProgress(event.nativeEvent)}
        onTracks={(event) => onTracks(event.nativeEvent)}
      />
    ) : null;
  },
);
