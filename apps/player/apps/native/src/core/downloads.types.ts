import type { DownloadTrackSelection } from './download-tracks';
import type { DownloadManifest } from './download-manifest';
import type { DownloadQuality } from './download-quality';
import type { MediaItem, Progress } from './contract';
export type DownloadEntry = {
  nativePlan?: boolean;
  tracks?: DownloadTrackSelection;
  manifest?: DownloadManifest;
  item: MediaItem;
  baseline?: Progress;
  chapters?: { title: string; start: number; end: number }[];
  quality?: DownloadQuality;
  jobID?: string;
  bytes: number;
  total: number;
  duration: number;
  contentType: string;
  version: string;
  status:
    | 'preparing'
    | 'queued'
    | 'waiting'
    | 'pausing'
    | 'paused'
    | 'downloading'
    | 'verifying'
    | 'complete';
  error: string;
};
