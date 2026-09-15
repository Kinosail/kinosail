import { requireOptionalNativeModule } from 'expo';
import { Platform } from 'react-native';
import { normalizeServerURL } from './server-client';
import type { DownloadEntry } from './downloads.types';
import type { PlaybackSource } from './contract';

const transport =
  Platform.OS === 'ios'
    ? requireOptionalNativeModule<{
        open(uri: string, authorization: string, id: number): Promise<string>;
        close(id: number): void;
        prepareOfflineStorage(): Promise<void>;
        enqueueDownload(raw: string): Promise<void>;
        downloadSnapshot(
          scope: string,
        ): Promise<
          (Pick<DownloadEntry, 'bytes' | 'total' | 'status' | 'error'> & {
            key: string;
          })[]
        >;
        pauseDownload(scope: string, key: string): Promise<void>;
        removeDownload(scope: string, key: string): Promise<void>;
        clearDownloads(): Promise<void>;
      }>('ProtectedMedia')
    : null;
export const backgroundTransfers =
  !Platform.isTV && transport?.enqueueDownload ? transport : null;
let generation = 0;
export const nextProtectedMediaID = () => ++generation;
export const localCompatibilityAvailable = Boolean(transport);
export async function openProtectedMedia(
  source: PlaybackSource,
  id: number,
): Promise<string> {
  validateProtectedSource(source, id);
  if (!transport)
    throw new Error(
      'Local compatibility playback is unavailable in this build.',
    );
  return transport.open(source.uri, source.headers.Authorization, id);
}
export function closeProtectedMedia(id: number) {
  transport?.close(id);
}

export async function prepareOfflineStorage() {
  if (Platform.OS === 'ios') {
    if (!transport)
      throw new Error('Downloads require the native Player build.');
    await transport.prepareOfflineStorage();
  }
}

function validateProtectedSource(source: PlaybackSource, id: number) {
  const invalid = () => new Error('The local playback source is invalid.');
  if (
    !Number.isSafeInteger(id) ||
    id <= 0 ||
    !source ||
    typeof source.uri !== 'string' ||
    source.uri.length > 2048 ||
    typeof source.headers?.Authorization !== 'string'
  )
    throw invalid();
  let url: URL;
  try {
    url = new URL(source.uri);
  } catch {
    throw invalid();
  }
  if (url.username || url.password || url.search || url.hash) throw invalid();
  if (url.protocol === 'file:') {
    if (
      url.host ||
      source.headers.Authorization ||
      !/\/kinosail-offline\/[a-f0-9]{64}\/[a-f0-9]{64}\.media$/.test(
        url.pathname,
      )
    )
      throw invalid();
    return; // The native adapter additionally resolves symlinks and enforces the app's Documents root.
  }
  normalizeServerURL(url.origin);
  if (
    !/^\/media\/[A-Za-z0-9_-]{1,128}$/.test(url.pathname) ||
    !/^Bearer [^\u0000-\u0020\u007f]{1,2048}$/.test(
      source.headers.Authorization,
    )
  )
    throw invalid();
}
