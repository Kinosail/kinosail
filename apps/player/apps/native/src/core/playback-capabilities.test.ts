import { playbackCapabilitiesQuery } from './playback-capabilities';

const valid = {
  videoCodecs: ['h264', 'hevc'],
  audioCodecs: ['aac', 'eac3'],
  hdrFormats: ['sdr', 'hdr10'],
  maxAudioChannels: 6,
};
it('serializes only verified codec and current display/audio-route hints', () => {
  expect(playbackCapabilitiesQuery(valid)).toBe(
    'videoCodecs=h264,hevc&audioCodecs=aac,eac3&hdrFormats=sdr,hdr10&maxAudioChannels=6',
  );
});
it.each([
  null,
  [],
  {},
  { ...valid, extra: true },
  { ...valid, videoCodecs: [] },
  { ...valid, videoCodecs: ['H264'] },
  { ...valid, audioCodecs: ['aac', 'aac'] },
  { ...valid, videoCodecs: ['h264&token=secret'] },
  { ...valid, videoCodecs: Array(5).fill('h264') },
  { ...valid, hdrFormats: ['hdr10'] },
  { ...valid, hdrFormats: ['sdr', 'dolby-vision'] },
  { ...valid, audioCodecs: [null] },
  ...[0, 9, 1.5, NaN, Infinity, '2', undefined].map((maxAudioChannels) => ({
    ...valid,
    maxAudioChannels,
  })),
])(
  'rejects missing, unknown, oversized, duplicate or malformed bridge output %#',
  (value) => {
    expect(() => playbackCapabilitiesQuery(value)).toThrow('Invalid device');
  },
);
