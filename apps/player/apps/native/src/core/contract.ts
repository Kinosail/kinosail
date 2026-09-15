import { downloadLimit } from './download-policy';
import { parsePlaybackDetails, type PlaybackDetails } from './playback-details';

export const MAX_COLLECTION_ITEMS = 200;

export type Viewer = {
  id: string;
  name: string;
  owner: boolean;
};

export type Progress = {
  seconds: number;
  watched: boolean;
  session: string;
  revision: number;
};

export type MediaItem = {
  id: string;
  kind: string;
  title: string;
  year: string;
  plot: string;
  rating: string;
  tagline: string;
  genres: string;
  director: string;
  studio: string;
  artist: string;
  album: string;
  show: string;
  showId?: string;
  size?: number;
  season: number;
  episode: number;
  artwork: string;
  backdrop: string;
  container: string;
  progress: Progress;
};

export type Home = {
  server: string;
  viewer: Viewer;
  continueWatching: MediaItem[];
  recent: MediaItem[];
};

export type PlaybackRecovery = {
  uri: string;
  plan: PlaybackSource['plan'];
  duration: number;
  progressToken: string;
  omitted: { start: number; end: number }[];
};

export type PlaybackSource = {
  details?: PlaybackDetails;
  compatible?: PlaybackRecovery;
  uri: string;
  headers: { Authorization: string };
  contentType: string;
  duration: number;
  start: number;
  progressToken: string;
  plan: { allowed: boolean; mode: string; reason: string };
};

export type InputValue =
  | boolean
  | number
  | string
  | null
  | undefined
  | InputValue[]
  | { [key: string]: InputValue };

type RecordValue = Record<string, InputValue>;

const invalid = () =>
  new Error('Kinosail Server returned an invalid response.');

const record = (value: InputValue): RecordValue => {
  if (!value || typeof value !== 'object' || Array.isArray(value))
    throw invalid();
  return value as RecordValue;
};

const text = (value: InputValue, maximum: number, required = false): string => {
  if (typeof value !== 'string' || value.length > maximum) throw invalid();
  if (required && value.length === 0) throw invalid();
  return value;
};

const optionalText = (value: InputValue, maximum = 2048): string =>
  value === undefined ? '' : text(value, maximum);

const finite = (value: InputValue, fallback = 0): number => {
  if (value === undefined) return fallback;
  const parsed = value as number;
  if (!Number.isFinite(parsed) || parsed < 0) throw invalid();
  return parsed;
};

const integer = (value: InputValue, fallback = 0): number => {
  const parsed = finite(value, fallback);
  if (!Number.isSafeInteger(parsed)) throw invalid();
  return parsed;
};

export const parseChallenge = (value: InputValue) => {
  const input = record(value);
  const code = text(input.code, 6);
  const secret = text(input.secret, 512, true);
  if (code.length !== 6 || /\D/.test(code)) throw invalid();
  return { code, secret };
};

export const parseQuickConnectToken = (value: InputValue): string => {
  return text(record(value).token, 512, true);
};

export const parsePending = (value: InputValue): void => {
  if (record(value).status !== 'pending') throw invalid();
};

export const parseMe = (
  value: InputValue,
): { server: string; viewer: Viewer } => {
  const input = record(value);
  const viewer = record(input.viewer);
  if (typeof viewer.owner !== 'boolean') throw invalid();
  return {
    server: text(input.server, 120, true),
    viewer: {
      id: text(viewer.id, 128, true),
      name: text(viewer.name, 120, true),
      owner: viewer.owner,
    },
  };
};

export const parseProgress = (value: InputValue): Progress => {
  const input = record(value);
  if (input.watched !== undefined && typeof input.watched !== 'boolean')
    throw invalid();
  return {
    seconds: finite(input.seconds),
    watched: input.watched ?? false,
    session: optionalText(input.session, 128),
    revision: integer(input.revision),
  };
};

export const parseMediaItem = (value: InputValue): MediaItem => {
  const input = record(value);
  if (
    input.showId !== undefined &&
    (typeof input.showId !== 'string' || !/^[a-f0-9]{16}$/.test(input.showId))
  )
    throw invalid();
  return {
    id: text(input.id, 128, true),
    kind: input.kind === 'audio' ? 'music' : text(input.kind, 32, true),
    title: text(input.title, 512, true),
    year: optionalText(input.year, 16),
    plot: optionalText(input.plot, 10_000),
    rating: optionalText(input.rating, 32),
    tagline: optionalText(input.tagline, 512),
    genres: optionalText(input.genres, 512),
    director: optionalText(input.director, 256),
    studio: optionalText(input.studio, 256),
    artist: optionalText(input.artist, 256),
    album: optionalText(input.album, 256),
    show: optionalText(input.show, 256),
    ...(input.showId === undefined
      ? {}
      : { showId: text(input.showId, 16, true) }),
    ...(input.size === undefined ? {} : { size: integer(input.size) }),
    season: integer(input.season),
    episode: integer(input.episode),
    artwork: optionalText(input.artwork),
    backdrop: optionalText(input.backdrop),
    container: optionalText(input.container, 32),
    progress: parseProgress(input.progress ?? {}),
  };
};

export const parseLibrary = (value: InputValue): MediaItem[] => {
  const items = record(value).items;
  if (!Array.isArray(items) || items.length > MAX_COLLECTION_ITEMS)
    throw invalid();
  return items.map(parseMediaItem);
};

export type LibraryLetter = { label: string; offset: number; count: number };

export const parseLibraryPage = (value: InputValue) => {
  const input = record(value);
  const items = parseLibrary(value);
  const total = integer(input.total);
  const offset = integer(input.offset);
  const limit = integer(input.limit);
  if (
    !limit ||
    limit > MAX_COLLECTION_ITEMS ||
    offset > 1_000_000 ||
    items.length > limit ||
    items.length > Math.max(0, total - offset)
  )
    throw invalid();
  const rawLetters = input.letters ?? [];
  if (!Array.isArray(rawLetters) || rawLetters.length > 4096) throw invalid();
  const labels = new Set<string>();
  let end = 0;
  const letters: LibraryLetter[] = rawLetters.map((value) => {
    const entry = record(value);
    const label = entry.label;
    const start = integer(entry.offset);
    const count = integer(entry.count);
    if (
      typeof label !== 'string' ||
      !/^(?:#|\p{L}[\p{L}\p{M}]{0,3})$/u.test(label) ||
      typeof entry.offset !== 'number' ||
      typeof entry.count !== 'number' ||
      Object.keys(entry).some(
        (key) => !['label', 'offset', 'count'].includes(key),
      ) ||
      labels.has(label) ||
      start !== end ||
      start > 1_000_000 ||
      !count ||
      start + count > total
    )
      throw invalid();
    labels.add(label);
    end = start + count;
    return { label, offset: start, count };
  });
  if (letters.length && end !== total) throw invalid();
  return { items, total, offset, limit, letters };
};

export const parseItem = (value: InputValue): MediaItem =>
  parseMediaItem(record(value).item);

export const parsePlayback = (value: InputValue) => {
  const input = record(value);
  const planInput = record(input.plan);
  const mode = text(planInput.mode, 32, true);
  if (
    !['direct', 'remux', 'audio-transcode', 'transcode', 'denied'].includes(
      mode,
    ) ||
    typeof planInput.allowed !== 'boolean' ||
    (input.directAllowed !== undefined &&
      typeof input.directAllowed !== 'boolean')
  )
    throw invalid();
  return {
    directAllowed: input.directAllowed ?? planInput.allowed,
    details: parsePlaybackDetails(input, finite(input.duration)),
    compatible: parsePlaybackRecovery(input),
    direct: optionalText(input.direct),
    directType: optionalText(input.directType, 128),
    duration: finite(input.duration),
    start: finite(input.start),
    progressToken: optionalText(input.progressToken, 2048),
    plan: {
      allowed: planInput.allowed,
      mode,
      reason: text(planInput.reason, 128, true),
    },
  };
};

function parsePlaybackRecovery(input: RecordValue) {
  const uri = optionalText(input.compatible);
  if (!uri) return undefined;
  const plan = record(input.compatiblePlan);
  if (typeof plan.allowed !== 'boolean') throw invalid();
  const mode = text(plan.mode, 32, true);
  if (!['remux', 'audio-transcode', 'transcode', 'denied'].includes(mode)) {
    throw invalid();
  }
  if (!plan.allowed || mode === 'denied') return undefined;
  const timeline = plan.timeline ? record(plan.timeline) : {};
  const ranges = timeline.omitted ?? [];
  if (!Array.isArray(ranges) || ranges.length > 128) throw invalid();
  const duration = finite(input.compatibleDuration ?? input.duration);
  const sourceDuration = finite(timeline.sourceDuration ?? input.duration);
  let previousEnd = 0;
  const omitted = ranges.map((value) => {
    const range = record(value);
    const start = finite(range.start);
    const end = finite(range.end);
    if (start < previousEnd || end <= start || end > sourceDuration) {
      throw invalid();
    }
    previousEnd = end;
    return { start, end };
  });
  const progressToken = optionalText(input.compatibleProgressToken, 8192);
  const removed = omitted.reduce(
    (total, range) => total + range.end - range.start,
    0,
  );
  if (
    omitted.length &&
    (!progressToken || Math.abs(sourceDuration - removed - duration) > 0.001)
  ) {
    throw invalid();
  }
  return {
    uri,
    duration,
    progressToken,
    omitted,
    plan: { allowed: true, mode, reason: text(plan.reason, 128, true) },
  };
}

export const parseShowEpisodes = (
  value: InputValue,
  id: string,
): MediaItem[] => {
  const input = record(value);
  if (
    input.id !== id ||
    !Array.isArray(input.episodes) ||
    !input.episodes.length ||
    input.episodes.length > downloadLimit
  )
    throw invalid();
  const episodes = input.episodes.map(parseMediaItem);
  const seen = new Set<string>();
  for (const episode of episodes) {
    if (
      episode.kind !== 'video' ||
      !episode.show ||
      episode.showId !== id ||
      seen.has(episode.id)
    )
      throw invalid();
    seen.add(episode.id);
  }
  return episodes;
};
