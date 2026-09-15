import type { InputValue } from './contract';

export type PlaybackDetails = {
  chapters: { title: string; start: number; end: number }[];
  download: string;
  next: string;
  downloadNext?: string;
  media?: {
    version: string;
    container: string;
    codec: string;
    profile: string;
    hdr: string;
    pixelFormat: string;
    bitDepth: number;
    width: number;
    height: number;
    dolbyVisionProfile: number;
    dolbyVisionCompatibility: number;
  };
};
const invalid = () =>
  new Error('Kinosail Server returned invalid playback details.');
function record(value: InputValue): Record<string, InputValue> {
  if (!value || typeof value !== 'object' || Array.isArray(value))
    throw invalid();
  return value;
}
function text(value: InputValue, limit: number) {
  if (value === undefined) return '';
  if (
    typeof value !== 'string' ||
    value.length > limit ||
    /[\u0000-\u001f\u007f]/.test(value)
  )
    throw invalid();
  return value;
}
function number(value: InputValue, limit: number, integer = false) {
  if (value === undefined) return 0;
  if (
    typeof value !== 'number' ||
    !Number.isFinite(value) ||
    value < 0 ||
    value > limit ||
    (integer && !Number.isSafeInteger(value))
  )
    throw invalid();
  return value;
}
export function parsePlaybackDetails(
  value: InputValue,
  duration: number,
): PlaybackDetails {
  const input = record(value);
  const chapters = input.chapters ?? [];
  if (!Array.isArray(chapters) || chapters.length > 1024) throw invalid();
  let previous = -1;
  const result: PlaybackDetails = {
    download: text(input.download, 2048),
    next: text(input.next, 2048),
    chapters: chapters.map((value) => {
      const chapter = record(value);
      const start = number(chapter.start, duration);
      const end = number(chapter.end, duration);
      if (start < previous || end < start) throw invalid();
      previous = start;
      return { start, end, title: text(chapter.title, 512) };
    }),
  };
  if (input.downloadNext !== undefined)
    result.downloadNext = text(input.downloadNext, 128);
  if (input.media !== undefined) {
    const media = record(input.media);
    const video = record(media.video);
    result.media = {
      version: text(media.fileVersion, 256),
      container: text(media.container, 64),
      codec: text(video.Codec, 64),
      profile: text(video.Profile, 128),
      hdr: text(video.HDR, 64),
      pixelFormat: text(video.PixelFormat, 64),
      bitDepth: number(video.BitDepth, 64, true),
      width: number(video.Width, 65536, true),
      height: number(video.Height, 65536, true),
      dolbyVisionProfile: number(video.DolbyVisionProfile, 255, true),
      dolbyVisionCompatibility: number(
        video.DolbyVisionCompatibility,
        255,
        true,
      ),
    };
  }
  return result;
}
