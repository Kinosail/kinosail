import { playbackLanguageBase } from './media-languages';
import { maxDownloadLimitGiB } from './download-policy';
export type PlaybackPreferences = {
  rate: number;
  audioLanguage: string;
  subtitleLanguage: string;
  audioTrack: string;
  subtitleTrack: string;
  nightMode: boolean;
  dialogueBoost: boolean;
  volumeBoost: number;
};
export type MediaPreferences = {
  playback: PlaybackPreferences;
  autoDownloadNext: number;
  removeWatched: boolean;
  downloadLimitGiB: number;
  wifiOnly: boolean;
  readerFontSize: number;
  readerTheme: 'auto' | 'light' | 'dark' | 'sepia';
};
export const defaultPlaybackPreferences: PlaybackPreferences = {
  rate: 1,
  audioLanguage: 'auto',
  subtitleLanguage: 'auto',
  audioTrack: '',
  subtitleTrack: '',
  nightMode: false,
  dialogueBoost: false,
  volumeBoost: 1,
};
export const defaultMediaPreferences: MediaPreferences = {
  playback: defaultPlaybackPreferences,
  autoDownloadNext: 0,
  removeWatched: false,
  downloadLimitGiB: 20,
  wifiOnly: true,
  readerFontSize: 20,
  readerTheme: 'auto',
};
const invalid = () => new Error('The media preferences are invalid.');
export function exactObject(
  value: unknown,
  keys: readonly string[],
): Record<string, unknown> {
  if (
    !value ||
    typeof value !== 'object' ||
    Array.isArray(value) ||
    Object.keys(value).some((key) => !keys.includes(key))
  )
    throw invalid();
  return value as Record<string, unknown>;
}
const bounded = (value: unknown, min: number, max: number) =>
  typeof value === 'number' &&
  Number.isFinite(value) &&
  value >= min &&
  value <= max;
export function parsePlaybackPreferences(value: unknown): PlaybackPreferences {
  const v = exactObject(value, Object.keys(defaultPlaybackPreferences));
  const language = (value: unknown, subtitle: boolean) =>
    typeof value === 'string' &&
    value.length <= 32 &&
    (value === 'auto' ||
      (subtitle && value === 'off') ||
      /^[a-z]{2,3}(?:-[A-Za-z0-9]{2,8}){0,3}$/.test(value));
  if (
    !bounded(v.rate, 0.5, 3) ||
    !bounded(v.volumeBoost, 1, 2) ||
    !language(v.audioLanguage, false) ||
    !language(v.subtitleLanguage, true) ||
    typeof v.nightMode !== 'boolean' ||
    typeof v.dialogueBoost !== 'boolean' ||
    [v.audioTrack, v.subtitleTrack].some(
      (value) =>
        typeof value !== 'string' ||
        value.length > 256 ||
        /[\u0000-\u001f\u007f]/u.test(value),
    )
  )
    throw invalid();
  return { ...v } as PlaybackPreferences;
}
export function parseMediaPreferences(value: unknown): MediaPreferences {
  const v = exactObject(value, Object.keys(defaultMediaPreferences));
  if (
    !Number.isInteger(v.autoDownloadNext) ||
    !bounded(v.autoDownloadNext, 0, 3) ||
    !Number.isInteger(v.downloadLimitGiB) ||
    !bounded(v.downloadLimitGiB, 0, maxDownloadLimitGiB) ||
    !Number.isInteger(v.readerFontSize) ||
    !bounded(v.readerFontSize, 16, 32) ||
    typeof v.removeWatched !== 'boolean' ||
    typeof v.wifiOnly !== 'boolean' ||
    !['auto', 'light', 'dark', 'sepia'].includes(v.readerTheme as string)
  )
    throw invalid();
  return {
    ...v,
    playback: parsePlaybackPreferences(v.playback),
  } as MediaPreferences;
}
export function parseItemPreferences(value: unknown) {
  const v = exactObject(value, ['playback', 'overridden']);
  if (typeof v.overridden !== 'boolean') throw invalid();
  return {
    playback: parsePlaybackPreferences(v.playback),
    overridden: v.overridden,
  };
}
export function needsAudioProcessing(value: PlaybackPreferences) {
  return value.nightMode || value.dialogueBoost || value.volumeBoost > 1;
}

export type Bookmark = {
  id: string;
  title: string;
  seconds?: number;
  page?: number;
  offset?: number;
};
export function parseBookmarks(value: unknown): Bookmark[] {
  const v = exactObject(value, ['bookmarks']);
  if (!Array.isArray(v.bookmarks) || v.bookmarks.length > 100) throw invalid();
  const ids = new Set<string>();
  return v.bookmarks.map((raw) => {
    const b = exactObject(raw, ['id', 'title', 'seconds', 'page', 'offset']);
    if (
      typeof b.id !== 'string' ||
      !/^[a-f0-9]{64}$/.test(b.id) ||
      ids.has(b.id)
    )
      throw invalid();
    validateBookmark(b);
    ids.add(b.id);
    return { ...b } as Bookmark;
  });
}
export function validateBookmark(value: unknown): Omit<Bookmark, 'id'> {
  const b = exactObject(value, ['id', 'title', 'seconds', 'page', 'offset']);
  if (
    (b.offset !== undefined &&
      (b.page === undefined || !bounded(b.offset, 0, 1))) ||
    typeof b.title !== 'string' ||
    !b.title.trim() ||
    b.title.length > 160 ||
    /[\u0000-\u001f\u007f]/u.test(b.title) ||
    (b.seconds === undefined) === (b.page === undefined) ||
    (b.seconds !== undefined && !bounded(b.seconds, 0, 31_536_000)) ||
    (b.page !== undefined &&
      (!Number.isInteger(b.page) || !bounded(b.page, 1, 10_000)))
  )
    throw new Error('The bookmark is invalid.');
  return {
    title: b.title.trim(),
    ...(b.offset === undefined ? {} : { offset: b.offset as number }),
    ...(b.seconds !== undefined
      ? { seconds: b.seconds as number }
      : { page: b.page as number }),
  };
}
export const formatPosition = (seconds: number) => {
  const value = Math.max(0, Math.floor(seconds));
  return `${Math.floor(value / 3600) ? `${Math.floor(value / 3600)}:` : ''}${String(Math.floor(value / 60) % 60).padStart(2, '0')}:${String(value % 60).padStart(2, '0')}`;
};

export function matchesPlaybackLanguage(
  preferred: string,
  available: string | undefined,
) {
  if (!available || preferred === 'auto' || preferred === 'off') return false;
  return playbackLanguageBase(preferred) === playbackLanguageBase(available);
}
