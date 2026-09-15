import { playbackFailure, preferLocalPlayback } from './playback-engine';
import { defaultPlaybackPreferences as preferences } from './media-preferences';
import type { MediaItem, PlaybackSource } from './contract';

const item = { kind: 'video', container: 'mp4' } as MediaItem;
const source = {
  contentType: 'application/octet-stream',
  plan: { mode: 'direct' },
} as PlaybackSource;

it.each(['mkv', 'matroska', 'avi', 'flv', 'ogg', 'ogv'])(
  'opens known unsupported Apple container %s locally without a failed platform start',
  (container) => {
    expect(
      preferLocalPlayback({ ...item, container }, source, preferences, true),
    ).toBe(true);
  },
);
it.each(['', 'mp4', 'mov', 'future-format'])(
  'preserves platform playback for %s',
  (container) => {
    expect(
      preferLocalPlayback({ ...item, container }, source, preferences, true),
    ).toBe(false);
  },
);
it.each(['nightMode', 'dialogueBoost', 'volumeBoost'] as const)(
  'selects the audio processor before playback for %s',
  (name) => {
    expect(
      preferLocalPlayback(
        item,
        source,
        { ...preferences, [name]: name === 'volumeBoost' ? 2 : true },
        true,
      ),
    ).toBe(true);
  },
);
it('never sends a Server rendition to the original-file gateway or invents a missing engine', () => {
  const mkv = { ...item, container: 'mkv' };
  expect(preferLocalPlayback(mkv, source, preferences, false)).toBe(false);
  expect(
    preferLocalPlayback(
      mkv,
      { ...source, plan: { ...source.plan, mode: 'remux' } },
      preferences,
      true,
    ),
  ).toBe(false);
  expect(
    preferLocalPlayback(
      mkv,
      { ...source, contentType: 'application/vnd.apple.mpegurl' },
      preferences,
      true,
    ),
  ).toBe(false);
});
it('uses an MP4 download as MP4 even when the library original was MKV', () => {
  expect(
    preferLocalPlayback(
      { ...item, container: 'mkv' },
      { ...source, contentType: 'video/mp4' },
      preferences,
      true,
    ),
  ).toBe(false);
});
it.each([
  'ERROR_CODE_DECODING_FAILED',
  'DecoderInitializationException',
  'Cannot decode this media',
  'Unsupported codec',
])('recognizes explicit decode evidence %s', (message) => {
  expect(playbackFailure({ message }).decode).toBe(true);
});
it.each([
  undefined,
  null,
  {},
  { message: 42 },
  { message: 'x'.repeat(4097) },
  ...[
    'HTTP 403 unsupported codec',
    'Network error',
    'Decoder connection timed out',
    'Not authorized',
    'The operation could not be completed',
    'Unsupported URL',
    'Playback cancelled',
    'Certificate invalid',
  ].map((message) => ({ message })),
])('does not change engine for an unknown or transport failure %#', (error) => {
  expect(playbackFailure(error).decode).toBe(false);
});
