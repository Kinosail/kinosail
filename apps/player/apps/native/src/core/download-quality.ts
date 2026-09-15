import { parseDownloadTracks } from './download-tracks';
import { exactObject } from './media-preferences';
import { validateDownloadID } from './download-policy';

export type DownloadQuality = 'original' | 'compatible' | '1080p' | '720p';
export function parseDownloadQuality(value: unknown): DownloadQuality {
  if (
    value !== 'original' &&
    value !== 'compatible' &&
    value !== '1080p' &&
    value !== '720p'
  )
    throw new Error(
      'Choose Original, Compatible, 1080p, or 720p download quality.',
    );
  return value;
}
export function validatePreparedID(id: string) {
  if (typeof id !== 'string' || !/^[a-f0-9]{16}$/.test(id))
    throw new Error('The prepared download is invalid.');
  return id;
}
export type PreparedDownload = {
  id: string;
  itemId: string;
  quality: DownloadQuality;
  state: 'preparing' | 'ready' | 'failed';
  size: number;
  sha256: string;
  extension?: string;
};
export function parsePreparedDownload(raw: unknown): PreparedDownload {
  const value = exactObject(raw, [
    'id',
    'itemId',
    'profileId',
    'title',
    'quality',
    'state',
    'size',
    'sha256',
    'readyOffline',
    'error',
    'extension',
    'created',
    'tracks',
    'sourceVersion',
  ]);
  if (value.tracks !== undefined) parseDownloadTracks(value.tracks);
  if (
    value.sourceVersion !== undefined &&
    (typeof value.sourceVersion !== 'string' ||
      value.sourceVersion.length > 128)
  )
    throw new Error('The Server returned an invalid source revision.');
  const id = validatePreparedID(value.id as string);
  const itemId = validateDownloadID(value.itemId as string);
  const quality = parseDownloadQuality(value.quality);
  const size = value.size ?? 0,
    sha256 = value.sha256 ?? '';
  if (
    typeof value.profileId !== 'string' ||
    !/^[A-Za-z0-9_-]{1,256}$/.test(value.profileId) ||
    typeof value.title !== 'string' ||
    !value.title ||
    value.title.length > 512 ||
    typeof value.created !== 'string' ||
    value.created.length > 64 ||
    !Number.isFinite(Date.parse(value.created)) ||
    (value.extension !== undefined &&
      (typeof value.extension !== 'string' ||
        !/^\.[A-Za-z0-9]{1,15}$/.test(value.extension))) ||
    (value.error !== undefined &&
      (typeof value.error !== 'string' || value.error.length > 1000)) ||
    !Number.isSafeInteger(size) ||
    (size as number) < 0 ||
    typeof sha256 !== 'string' ||
    (sha256 !== '' && !/^[a-f0-9]{64}$/.test(sha256)) ||
    !['preparing', 'ready', 'failed'].includes(value.state as string) ||
    typeof value.readyOffline !== 'boolean' ||
    (value.state === 'ready' &&
      (!value.readyOffline ||
        !sha256 ||
        !(size as number) ||
        Boolean(value.error))) ||
    (value.state === 'preparing' &&
      (value.readyOffline ||
        size !== 0 ||
        sha256 !== '' ||
        Boolean(value.error))) ||
    (value.state === 'failed' && (value.readyOffline || !value.error))
  )
    throw new Error('The Server returned an invalid prepared download.');
  return {
    id,
    itemId,
    quality,
    state: value.state as PreparedDownload['state'],
    size: size as number,
    sha256,
    ...(value.extension ? { extension: value.extension as string } : {}),
  };
}
