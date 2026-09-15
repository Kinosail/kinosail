import type { InputValue } from './contract';

export type CastProtocol = 'google-cast' | 'dlna';
export type CastDevice = { id: string; name: string; protocol: 'dlna' };
export type CastSession = {
  id: string;
  url: string;
  contentType: string;
  title: string;
  position: number;
  duration: number;
  expiresAt: string;
  protocol: CastProtocol;
  tracks: {
    id: number;
    url: string;
    label: string;
    language: string;
    default: boolean;
  }[];
  deviceId?: string;
  deviceName?: string;
};
export type CastStatus = {
  state: 'playing' | 'paused' | 'buffering' | 'stopped';
  position: number;
  duration: number;
};
export type CastCommand =
  { action: 'play' | 'pause' | 'stop' } | { action: 'seek'; position: number };
export type CastStart = {
  protocol: CastProtocol;
  deviceId?: string;
  position: number;
  playbackToken?: string;
};
export type CastController = {
  name: string;
  status(): Promise<CastStatus>;
  command(command: CastCommand): Promise<void>;
};
export type TVPlayback = { session: CastSession; controller: CastController };
export type CastingClient = {
  startCast(id: string, input: CastStart): Promise<CastSession>;
  endCast(id: string): Promise<void>;
  scanCastDevices(): Promise<CastDevice[]>;
  castStatus(id: string): Promise<CastStatus>;
  castCommand(id: string, command: CastCommand): Promise<void>;
};

const invalid = () => new Error('The TV playback response is invalid.');
const record = (value: InputValue): Record<string, InputValue> => {
  if (!value || typeof value !== 'object' || Array.isArray(value))
    throw invalid();
  return value;
};
const text = (value: InputValue, max: number) => {
  if (
    typeof value !== 'string' ||
    !value.trim() ||
    value.length > max ||
    /[\u0000-\u001f\u007f]/.test(value)
  )
    throw invalid();
  return value;
};
export const castPosition = (value: InputValue): number => {
  if (
    typeof value !== 'number' ||
    !Number.isFinite(value) ||
    value < 0 ||
    value > 31_536_000
  )
    throw invalid();
  return value;
};
export const castID = (value: InputValue): string => {
  if (typeof value !== 'string' || !/^[a-f0-9]{32}$/.test(value))
    throw invalid();
  return value;
};
export function validateCastStart(value: CastStart) {
  if (
    !value ||
    !['google-cast', 'dlna'].includes(value.protocol) ||
    Object.keys(value).some(
      (key) =>
        !['protocol', 'position', 'deviceId', 'playbackToken'].includes(key),
    )
  )
    throw invalid();
  castPosition(value.position);
  if (value.protocol === 'dlna') castID(value.deviceId);
  else if (value.deviceId !== undefined) throw invalid();
  if (
    value.playbackToken !== undefined &&
    (typeof value.playbackToken !== 'string' ||
      value.playbackToken.length > 8192 ||
      /[\u0000-\u001f\u007f]/.test(value.playbackToken))
  )
    throw invalid();
}
export function validateCastCommand(value: CastCommand) {
  if (
    !value ||
    !['play', 'pause', 'stop', 'seek'].includes(value.action) ||
    Object.keys(value).some(
      (key) =>
        key !== 'action' && !(value.action === 'seek' && key === 'position'),
    )
  )
    throw invalid();
  if (value.action === 'seek') castPosition(value.position);
}
export function parseCastSession(
  value: InputValue,
  baseURL: string,
): CastSession {
  const input = record(value),
    id = castID(input.id);
  const url = new URL(text(input.url, 4096));
  const protocol = input.protocol;
  if (protocol !== 'google-cast' && protocol !== 'dlna') throw invalid();
  if (
    url.origin !== new URL(baseURL).origin ||
    url.username ||
    url.password ||
    url.hash ||
    ![`/cast/${id}/media`, `/cast/${id}/hls/index.m3u8`].includes(
      url.pathname,
    ) ||
    [...url.searchParams.keys()].length !== 1 ||
    !/^[a-f0-9]{64}$/.test(url.searchParams.get('ticket') ?? '')
  )
    throw invalid();
  const contentType = text(input.contentType, 128);
  if (
    !/^(?:video\/[a-z0-9.+-]+|audio\/[a-z0-9.+-]+|application\/vnd\.apple\.mpegurl)$/.test(
      contentType,
    )
  )
    throw invalid();
  const duration = castPosition(input.duration),
    position = castPosition(input.position);
  if (duration > 0 && position > duration) throw invalid();
  const expiresAt = text(input.expiresAt, 64),
    expiry = Date.parse(expiresAt);
  if (
    !Number.isFinite(expiry) ||
    expiry <= Date.now() ||
    expiry > Date.now() + 25 * 3600_000
  )
    throw invalid();
  if (!Array.isArray(input.tracks) || input.tracks.length > 64) throw invalid();
  const tracks = input.tracks.map((value, index) => {
    const track = record(value),
      trackURL = new URL(text(track.url, 4096));
    if (
      track.id !== index + 1 ||
      typeof track.default !== 'boolean' ||
      trackURL.origin !== url.origin ||
      trackURL.pathname !== `/cast/${id}/subtitles/${index + 1}` ||
      trackURL.search !== url.search ||
      trackURL.hash ||
      trackURL.username ||
      trackURL.password
    )
      throw invalid();
    return {
      id: index + 1,
      url: trackURL.href,
      label: text(track.label, 512),
      language: track.language === '' ? 'und' : text(track.language, 32),
      default: track.default,
    };
  });
  return {
    tracks,
    id,
    url: url.href,
    contentType,
    title: text(input.title, 1024),
    position,
    duration,
    expiresAt,
    protocol,
    ...(protocol === 'dlna'
      ? {
          deviceId: castID(input.deviceId),
          deviceName: text(input.deviceName, 128),
        }
      : {}),
  };
}
export function parseCastDevices(value: InputValue): CastDevice[] {
  const devices = record(value).devices;
  if (!Array.isArray(devices) || devices.length > 64) throw invalid();
  const seen = new Set<string>();
  return devices.map((value) => {
    const input = record(value),
      id = castID(input.id);
    if (input.protocol !== 'dlna' || seen.has(id)) throw invalid();
    seen.add(id);
    return { id, name: text(input.name, 128), protocol: 'dlna' };
  });
}
export function parseCastStatus(value: InputValue): CastStatus {
  const input = record(value);
  if (
    !['playing', 'paused', 'buffering', 'stopped'].includes(
      input.state as string,
    )
  )
    throw invalid();
  const position = castPosition(input.position),
    duration = castPosition(input.duration);
  if (duration > 0 && position > duration + 2) throw invalid();
  return { state: input.state as CastStatus['state'], position, duration };
}
