import {
  parseDownloadManifest,
  parseDownloadIdentity,
  verificationChunkSize,
} from './download-manifest';
import {
  parseDownloadTracks,
  parseDownloadTrackOptions,
} from './download-tracks';
const manifest = {
  version: 1,
  id: 'a'.repeat(16),
  size: verificationChunkSize + 1,
  chunkSize: verificationChunkSize,
  sha256: 'b'.repeat(64),
  chunks: ['c'.repeat(64), 'd'.repeat(64)],
};
it('binds exact verification blocks to one prepared revision', () => {
  expect(parseDownloadManifest(manifest)).toEqual(manifest);
  expect(() =>
    parseDownloadManifest(manifest, {
      id: manifest.id,
      itemId: 'film',
      quality: 'original',
      state: 'ready',
      size: manifest.size,
      sha256: 'e'.repeat(64),
    }),
  ).toThrow();
});
it.each([
  { version: 2 },
  { id: '../file' },
  { size: 0 },
  { size: -1 },
  { size: 1.5 },
  { size: Number.MAX_SAFE_INTEGER + 1 },
  { size: verificationChunkSize * 16384 + 1 },
  { chunkSize: 1 },
  { chunks: [] },
  { chunks: ['c'.repeat(64)] },
  { chunks: ['invalid', 'd'.repeat(64)] },
  { sha256: '' },
  { unknown: true },
  { chunks: 'invalid' },
])('rejects malformed or conflicting manifest metadata %p', (patch) => {
  expect(() => parseDownloadManifest({ ...manifest, ...patch })).toThrow();
});
it('rejects every missing manifest field', () => {
  for (const field of Object.keys(manifest)) {
    const value: Record<string, unknown> = { ...manifest };
    delete value[field];
    expect(() => parseDownloadManifest(value)).toThrow();
  }
});
it('separates stable ownership from credentials and rejects ambiguous identity', () => {
  expect(
    parseDownloadIdentity({ serverId: 'server', profileId: 'viewer' }),
  ).toEqual({ serverId: 'server', profileId: 'viewer' });
  for (const patch of [
    { serverId: '' },
    { profileId: '../other' },
    { token: 'secret' },
    { profileId: 'a'.repeat(257) },
  ])
    expect(() =>
      parseDownloadIdentity({
        serverId: 'server',
        profileId: 'viewer',
        ...patch,
      }),
    ).toThrow();
});
it('preserves explicit empty track selection and validates every selected index', () => {
  expect(parseDownloadTracks({ audio: [], subtitles: [] })).toEqual({
    audio: [],
    subtitles: [],
  });
  for (const patch of [
    { audio: null },
    { audio: [1, 1] },
    { subtitles: [-1] },
    { subtitles: [4096] },
    { audio: Array.from({ length: 33 }, (_, i) => i) },
    { unknown: true },
  ])
    expect(() =>
      parseDownloadTracks({ audio: [0], subtitles: [], ...patch }),
    ).toThrow();
  expect(() => parseDownloadTracks({ audio: [] })).toThrow();
  expect(() =>
    parseDownloadTrackOptions({
      audio: [
        { index: 0, label: 'English' },
        { index: 0, label: 'Duplicate' },
      ],
      subtitles: [],
    }),
  ).toThrow();
});
