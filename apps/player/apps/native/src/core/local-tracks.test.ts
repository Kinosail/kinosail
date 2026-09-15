import { parseLocalTracks } from './local-tracks';
const valid = {
  audio: [{ id: 0, name: 'English' }],
  audioIndex: 0,
  subtitle: [],
  subtitleIndex: -1,
};
it('accepts available tracks and disabled subtitles', () => {
  expect(parseLocalTracks(valid)).toEqual(valid);
});
it.each([
  null,
  undefined,
  [],
  {},
  { ...valid, extra: true },
  { ...valid, audioIndex: 2 },
  { ...valid, subtitleIndex: 0 },
  { ...valid, audio: [null] },
  { ...valid, audio: [{ id: -1, name: 'bad' }] },
  { ...valid, audio: [{ id: 128, name: 'bad' }] },
  { ...valid, audio: [{ id: 0, name: 12 }] },
  { ...valid, audio: [{ id: 0, name: 'a'.repeat(513) }] },
  { ...valid, audio: [valid.audio[0], valid.audio[0]] },
  { ...valid, audio: Array(129).fill(valid.audio[0]) },
  { ...valid, audio: [{ ...valid.audio[0], extra: true }] },
])('rejects malformed or conflicting native metadata %#', (value) => {
  expect(parseLocalTracks(value)).toBeNull();
});
