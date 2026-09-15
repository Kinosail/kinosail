import { nativeContentType } from './video-source';

describe('nativeContentType', () => {
  it.each([
    ['video/mp4', 'progressive'],
    ['video/x-matroska', 'progressive'],
    ['application/vnd.apple.mpegurl', 'hls'],
    ['APPLICATION/X-MPEGURL; charset=utf-8', 'hls'],
    [' application/vnd.apple.mpegurl ; charset=utf-8', 'hls'],
    ['application/dash+xml', 'dash'],
    ['application/vnd.ms-sstr+xml', 'smoothStreaming'],
  ])('maps %s to %s', (mime, expected) => {
    expect(nativeContentType(mime)).toBe(expected);
  });

  it.each(['', 'unknown', 'x'.repeat(4096)])(
    'uses progressive delivery for an unknown type',
    (mime) => {
      expect(nativeContentType(mime)).toBe('progressive');
    },
  );
});
