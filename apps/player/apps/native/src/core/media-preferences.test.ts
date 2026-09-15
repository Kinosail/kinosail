import { mediaLanguages } from './media-languages';
import { matchesPlaybackLanguage } from './media-preferences';
import {
  defaultMediaPreferences,
  defaultPlaybackPreferences,
  parseMediaPreferences,
  parsePlaybackPreferences,
  parseBookmarks,
  validateBookmark,
} from './media-preferences';
describe('media preferences', () => {
  it('accepts defaults and a supported set of personal preferences', () => {
    expect(parseMediaPreferences(defaultMediaPreferences)).toEqual(
      defaultMediaPreferences,
    );
    expect(
      parsePlaybackPreferences({
        ...defaultPlaybackPreferences,
        rate: 1.5,
        volumeBoost: 2,
        audioLanguage: 'en-US',
      }).rate,
    ).toBe(1.5);
  });
  it.each([
    { rate: 0 },
    { rate: Infinity },
    { volumeBoost: 2.01 },
    { nightMode: 'true' },
    { audioLanguage: '*' },
    { subtitleLanguage: 'x'.repeat(33) },
    { audioTrack: 'bad\ntrack' },
    { subtitleTrack: 'x'.repeat(257) },
    { unknown: true },
  ])('rejects malformed playback input %p', (patch) => {
    expect(() =>
      parsePlaybackPreferences({ ...defaultPlaybackPreferences, ...patch }),
    ).toThrow();
  });
  it('requires every field', () => {
    for (const key of Object.keys(defaultMediaPreferences)) {
      const value = { ...defaultMediaPreferences } as Record<string, unknown>;
      delete value[key];
      expect(() => parseMediaPreferences(value)).toThrow();
    }
  });
  it.each([
    { autoDownloadNext: 4 },
    { autoDownloadNext: -1 },
    { autoDownloadNext: 0.5 },
    { downloadLimitGiB: -1 },
    { downloadLimitGiB: 8388608 },
    { downloadLimitGiB: 0.5 },
    { downloadLimitGiB: null },
    { downloadLimitGiB: '0' },
    { downloadLimitGiB: Infinity },
    { readerFontSize: 33 },
    { readerTheme: 'blue' },
    { wifiOnly: null },
    { removeWatched: 1 },
  ])('rejects invalid media input %p', (patch) => {
    expect(() =>
      parseMediaPreferences({ ...defaultMediaPreferences, ...patch }),
    ).toThrow();
  });
  it.each([
    { title: '', seconds: 1 },
    { title: 'a', seconds: -1 },
    { title: 'a', seconds: NaN },
    { title: 'a', page: 0 },
    { title: 'a', page: 10001 },
    { title: 'a', page: 1, seconds: 0 },
    { title: 'a' },
    { title: 'x'.repeat(161), seconds: 0 },
    { title: 'a', seconds: 0, unknown: true },
  ])('rejects invalid bookmark %p', (value) => {
    expect(() => validateBookmark(value)).toThrow();
  });
  it('rejects duplicate imported bookmarks', () => {
    const bookmark = { id: 'a'.repeat(64), title: 'One', seconds: 1 };
    expect(() => parseBookmarks({ bookmarks: [bookmark, bookmark] })).toThrow();
  });
});

it('defaults downloads to Wi-Fi and preserves an explicit cellular choice', () => {
  expect(defaultMediaPreferences.wifiOnly).toBe(true);
  expect(
    parseMediaPreferences({ ...defaultMediaPreferences, wifiOnly: false })
      .wifiOnly,
  ).toBe(false);
});

it.each([0, 50, 250, 1000, 8388607])(
  'accepts storage limit %s without changing other preferences',
  (downloadLimitGiB) => {
    const value = { ...defaultMediaPreferences, downloadLimitGiB };
    expect(parseMediaPreferences(value)).toEqual(value);
  },
);

it('preserves the position within a bookmarked chapter', () => {
  const bookmark = {
    id: 'b'.repeat(64),
    title: 'Chapter 2',
    page: 2,
    offset: 0.625,
  };
  expect(validateBookmark(bookmark)).toEqual({
    title: bookmark.title,
    page: 2,
    offset: 0.625,
  });
  expect(parseBookmarks({ bookmarks: [bookmark] })).toEqual([bookmark]);
});
it.each([null, '0.5', NaN, Infinity, -1, 1.1, {}])(
  'rejects malformed bookmark offset %p',
  (offset) => {
    expect(() =>
      validateBookmark({ title: 'Spot', page: 1, offset }),
    ).toThrow();
  },
);
it('rejects chapter offsets attached to time bookmarks', () => {
  expect(() =>
    validateBookmark({ title: 'Spot', seconds: 12, offset: 0.5 }),
  ).toThrow();
});

it.each(mediaLanguages)(
  'accepts bundled language %s through the shared preference validator',
  (tag) => {
    expect(
      parsePlaybackPreferences({
        ...defaultPlaybackPreferences,
        audioLanguage: tag,
        subtitleLanguage: tag,
      }).audioLanguage,
    ).toBe(tag);
  },
);
it.each([
  ['pl', 'pol'],
  ['nl', 'dut'],
  ['zh-Hant', 'chi'],
  ['pt-BR', 'por'],
  ['ko', 'kor'],
])('matches preference %s to track %s', (preference, track) => {
  expect(matchesPlaybackLanguage(track, preference)).toBe(true);
  expect(matchesPlaybackLanguage('eng', preference)).toBe(false);
});
