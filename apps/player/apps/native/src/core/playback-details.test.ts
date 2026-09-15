import { parsePlayback } from './contract';
import { parsePlaybackDetails } from './playback-details';
import type { InputValue } from './contract';

it('preserves format facts and chapter boundaries without claiming decode support', () => {
  expect(
    parsePlaybackDetails(
      {
        download: '/download/a',
        next: 'episode-2',
        chapters: [{ title: 'Opening', start: 0, end: 20 }],
        media: {
          container: 'matroska',
          fileVersion: 'revision',
          video: {
            Codec: 'hevc',
            HDR: 'dolby-vision',
            BitDepth: 10,
            DolbyVisionProfile: 7,
          },
        },
      },
      60,
    ),
  ).toMatchObject({
    download: '/download/a',
    next: 'episode-2',
    chapters: [{ title: 'Opening', start: 0, end: 20 }],
    media: {
      codec: 'hevc',
      hdr: 'dolby-vision',
      bitDepth: 10,
      dolbyVisionProfile: 7,
    },
  });
});
it.each([
  null,
  [],
  { chapters: {} },
  { chapters: Array(1025).fill({ start: 0, end: 1 }) },
  { chapters: [{ start: -1, end: 2 }] },
  { chapters: [{ start: 4, end: 3 }] },
  { chapters: [{ start: 0, end: 61 }] },
  {
    chapters: [
      { start: 20, end: 21 },
      { start: 10, end: 11 },
    ],
  },
  { download: 'a\nb' },
  { next: 'a'.repeat(2049) },
  { media: { video: { Width: 65537 } } },
  { media: { video: { BitDepth: 1.1 } } },
  { media: { video: { Codec: {} } } },
])('rejects malformed playback details %p', (value) =>
  expect(() => parsePlaybackDetails(value as InputValue, 60)).toThrow(),
);

it.each([null, 'true', 1, {}, []])(
  'rejects an ambiguous direct-play permission %p',
  (directAllowed) => {
    expect(() =>
      parsePlayback({
        directAllowed,
        plan: { allowed: true, mode: 'direct', reason: 'compatible' },
      }),
    ).toThrow();
  },
);
