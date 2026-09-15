import { exactObject } from './media-preferences';
import { validatePreparedID, type PreparedDownload } from './download-quality';

export const verificationChunkSize = 8 * 1024 * 1024;
export const maximumVerificationChunks = 16384;
export type DownloadManifest = {
  version: 1;
  id: string;
  size: number;
  sha256: string;
  chunkSize: number;
  chunks: string[];
};
export function parseDownloadManifest(
  raw: unknown,
  job?: PreparedDownload,
): DownloadManifest {
  const value = exactObject(raw, [
    'version',
    'id',
    'size',
    'sha256',
    'chunkSize',
    'chunks',
  ]);
  validatePreparedID(value.id as string);
  if (
    value.version !== 1 ||
    !Number.isSafeInteger(value.size) ||
    (value.size as number) <= 0 ||
    (value.size as number) >
      verificationChunkSize * maximumVerificationChunks ||
    value.chunkSize !== verificationChunkSize ||
    typeof value.sha256 !== 'string' ||
    !/^[a-f0-9]{64}$/.test(value.sha256) ||
    !Array.isArray(value.chunks) ||
    value.chunks.length !==
      Math.ceil((value.size as number) / verificationChunkSize) ||
    value.chunks.some(
      (chunk) => typeof chunk !== 'string' || !/^[a-f0-9]{64}$/.test(chunk),
    ) ||
    (job &&
      (job.state !== 'ready' ||
        job.id !== value.id ||
        job.size !== value.size ||
        job.sha256 !== value.sha256))
  ) {
    throw new Error('The Server returned an invalid download manifest.');
  }
  return value as DownloadManifest;
}
export type DownloadIdentity = { serverId: string; profileId: string };
export function parseDownloadIdentity(raw: unknown): DownloadIdentity {
  const value = exactObject(raw, ['serverId', 'profileId']);
  for (const key of ['serverId', 'profileId']) {
    if (
      typeof value[key] !== 'string' ||
      !/^[A-Za-z0-9_-]{1,256}$/.test(value[key] as string)
    )
      throw new Error('The Server returned an invalid download identity.');
  }
  return value as DownloadIdentity;
}
