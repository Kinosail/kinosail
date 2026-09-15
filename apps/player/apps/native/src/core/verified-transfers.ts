import { requireOptionalNativeModule } from 'expo';
import { Platform } from 'react-native';
import type { DownloadEntry } from './downloads.types';

type Snapshot = Pick<DownloadEntry, 'bytes' | 'total' | 'status' | 'error'> & {
  key: string;
  manifest?: DownloadEntry['manifest'];
};
type NativeTransfers = {
  enqueuePreparingDownload(raw: string): Promise<void>;
  authorizeVerifiedDownloads(
    scope: string,
    authorization: string,
  ): Promise<void>;
  enqueueVerifiedDownload(raw: string): Promise<void>;
  verifiedDownloadSnapshot(scope: string): Promise<Snapshot[]>;
  pauseVerifiedDownload(scope: string, key: string): Promise<void>;
  removeVerifiedDownload(scope: string, key: string): Promise<void>;
  clearVerifiedDownloads(): Promise<void>;
  checkVerifiedDownload(scope: string, key: string): Promise<void>;
  setDownloadPlaybackActive(active: boolean): Promise<void>;
};
const native =
  Platform.OS !== 'web' && !Platform.isTV
    ? requireOptionalNativeModule<NativeTransfers>('ProtectedMedia')
    : null;
export const verifiedTransfers = native?.enqueueVerifiedDownload
  ? native
  : null;
let playing = 0;
export function prioritizePlayback(): () => void {
  playing++;
  void verifiedTransfers?.setDownloadPlaybackActive(true).catch(() => {});
  let released = false;
  return () => {
    if (released) return;
    released = true;
    playing = Math.max(0, playing - 1);
    if (!playing)
      void verifiedTransfers?.setDownloadPlaybackActive(false).catch(() => {});
  };
}
