import type { StyleProp, ViewStyle } from 'react-native';
export type VLCPlayerTracks = {
  audio: { id: number; name: string; language?: string }[];
  audioIndex: number;
  subtitle: { id: number; name: string; language?: string }[];
  subtitleIndex: number;
};
export type VLCPlayerRef = {
  startPictureInPicture(): void;
  seek(seconds: number): void;
  getTracks(): void;
  selectAudioTrack(index: number): void;
  selectSubtitleTrack(index: number): void;
  setAudioDelay(micros: number): void;
  setSubtitleDelay(micros: number): void;
};
export type LocalVideoProps = {
  source: { uri: string; start: number };
  paused: boolean;
  rate?: number;
  nightMode?: boolean;
  dialogueBoost?: boolean;
  volumeBoost?: number;
  audioOnly?: boolean;
  sleepDeadline?: number;
  sleepPosition?: number;
  style?: StyleProp<ViewStyle>;
  onFirstFrame(): void;
  onBuffering(event: { active: boolean }): void;
  onPictureInPictureReady(): void;
  onPaused(): void;
  onPlaying(): void;
  onProgress(event: { currentTime: number }): void;
  onEnd(): void;
  onError(): void;
  onTracks(event: VLCPlayerTracks): void;
};
