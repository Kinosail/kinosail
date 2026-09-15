import {
  parseDownloadTracks,
  type DownloadTrackSelection,
} from './download-tracks';
import { verifiedTransfers } from './verified-transfers';
import { parseDownloadManifest } from './download-manifest';
import {
  parseDownloadQuality,
  validatePreparedID,
  type DownloadQuality,
  type PreparedDownload,
} from './download-quality';
import { parsePlaybackDetails } from './playback-details';
import { writeExperienceCache } from './experience-cache';
import * as Network from 'expo-network';
import { fetch } from 'expo/fetch';
import * as Crypto from 'expo-crypto';
import {
  Directory,
  File,
  FileMode,
  Paths,
  type FileHandle,
} from 'expo-file-system';
import { AppState, Platform } from 'react-native';
import {
  parseMediaItem,
  parseProgress,
  type InputValue,
  type MediaItem,
  type PlaybackSource,
  type Progress,
} from './contract';
import {
  backgroundTransfers,
  localCompatibilityAvailable,
  prepareOfflineStorage,
} from './protected-media';
import type { KinosailClient } from './server-client';
import {
  downloadLimit,
  maxDownloadBytes,
  downloadSize,
  validateDownloadResponse,
  validateDownloadID,
  downloadModification,
} from './download-policy';
import type { DownloadEntry } from './downloads.types';
export type { DownloadEntry } from './downloads.types';
export const backgroundDownloadsAvailable = Boolean(
  verifiedTransfers || backgroundTransfers,
);
export const downloadsAvailable =
  !Platform.isTV && (Platform.OS !== 'ios' || localCompatibilityAvailable);
export function downloadStorage(): { total: number; free: number } | null {
  try {
    const total = Paths.totalDiskSpace,
      free = Paths.availableDiskSpace;
    return Number.isSafeInteger(total) &&
      Number.isSafeInteger(free) &&
      total > 0 &&
      free >= 0 &&
      free <= total
      ? { total, free }
      : null;
  } catch {
    return null;
  }
}
const root = () => new Directory(Paths.document, 'kinosail-offline');
let active: {
  id: string;
  controller: AbortController;
  done: Promise<void>;
} | null = null;
let activeEntries: { uri: string; entries: DownloadEntry[] } | null = null;
let clearing = false;
let removing = 0;
const invalid = () =>
  new Error(
    'The saved downloads could not be read. Remove downloads and try again.',
  );
const hash = (value: string) =>
  Crypto.digestStringAsync(Crypto.CryptoDigestAlgorithm.SHA256, value);
const folder = async (client: KinosailClient) =>
  new Directory(
    root(),
    client.downloadScope ??
      (await hash(
        `${client.baseURL}\n${client.authorizationHeaders().Authorization}`,
      )),
  );
const mediaFile = async (directory: Directory, id: string) =>
  new File(directory, `${await hash(validateDownloadID(id))}.media`);
function read(directory: Directory): DownloadEntry[] {
  const file = new File(directory, 'index.json');
  if (!file.exists) return [];
  if (file.size > 32 * 1024 * 1024) throw invalid();
  const values = JSON.parse(file.textSync()) as unknown;
  if (!Array.isArray(values) || values.length > downloadLimit) throw invalid();
  const seen = new Set<string>();
  return values.map((value) => {
    if (
      !value ||
      typeof value !== 'object' ||
      Object.keys(value).some(
        (key) =>
          ![
            'nativePlan',
            'tracks',
            'manifest',
            'quality',
            'jobID',
            'item',
            'baseline',
            'chapters',
            'bytes',
            'total',
            'duration',
            'contentType',
            'version',
            'status',
            'error',
          ].includes(key),
      )
    )
      throw invalid();
    const item = parseMediaItem(value.item as InputValue);
    if (value.nativePlan !== undefined && typeof value.nativePlan !== 'boolean')
      throw invalid();
    if (value.tracks !== undefined) parseDownloadTracks(value.tracks);
    if (value.manifest !== undefined) {
      const manifest = parseDownloadManifest(value.manifest);
      if (
        manifest.size !== value.total ||
        manifest.id !== value.jobID ||
        value.version !== manifest.sha256
      )
        throw invalid();
    }
    if (value.quality !== undefined) parseDownloadQuality(value.quality);
    if (
      value.quality &&
      value.quality !== 'original' &&
      (!value.jobID || item.kind !== 'video')
    )
      throw invalid();
    if (value.jobID !== undefined) {
      validatePreparedID(value.jobID);
      if (!value.quality) throw invalid();
    }
    const baseline =
      value.baseline === undefined
        ? item.progress
        : parseProgress(value.baseline as InputValue);
    if (seen.has(item.id)) throw invalid();
    seen.add(item.id);
    if (
      !Number.isSafeInteger(value.bytes) ||
      value.bytes < 0 ||
      !Number.isSafeInteger(value.total) ||
      value.total < 0 ||
      (value.total === 0 &&
        (!value.jobID || !['preparing', 'paused'].includes(value.status))) ||
      (value.status === 'preparing' && value.total !== 0) ||
      value.bytes > value.total ||
      value.total > maxDownloadBytes ||
      !Number.isFinite(value.duration) ||
      value.duration < 0 ||
      value.duration > 31_536_000 ||
      typeof value.contentType !== 'string' ||
      value.contentType.length > 128 ||
      /[\r\n]/.test(value.contentType) ||
      typeof value.version !== 'string' ||
      value.version.length > 256 ||
      ![
        'preparing',
        'queued',
        'waiting',
        'pausing',
        'paused',
        'downloading',
        'verifying',
        'complete',
      ].includes(value.status) ||
      typeof value.error !== 'string' ||
      value.error.length > 256 ||
      (value.status === 'complete' && value.bytes !== value.total)
    )
      throw invalid();
    return {
      ...value,
      baseline,
      chapters: parsePlaybackDetails(
        { chapters: value.chapters ?? [] },
        value.duration,
      ).chapters,
      item,
      status:
        value.status === 'downloading' &&
        !active &&
        !backgroundTransfers &&
        !verifiedTransfers
          ? 'paused'
          : value.status,
    };
  });
}
function write(directory: Directory, entries: DownloadEntry[]) {
  const raw = JSON.stringify(entries);
  if (raw.length > 32 * 1024 * 1024) throw invalid();
  const pending = new File(directory, 'index.next');
  pending.write(raw);
  pending.moveSync(new File(directory, 'index.json'), { overwrite: true });
}
export async function listDownloads(
  client: KinosailClient,
): Promise<DownloadEntry[]> {
  const directory = await folder(client);
  const entries = read(directory);
  const originals = new Map(
    entries.map((entry) => [entry.item.id, JSON.stringify(entry)]),
  );
  if (verifiedTransfers || backgroundTransfers) {
    const snapshot = backgroundTransfers
      ? await backgroundTransfers.downloadSnapshot(directory.name)
      : [];
    const verified = verifiedTransfers
      ? await verifiedTransfers.verifiedDownloadSnapshot(directory.name)
      : [];
    for (const entry of entries) {
      const key = await hash(entry.item.id);
      const transfer =
        verified.find((value) => value.key === key) ??
        snapshot.find((value) => value.key === key);
      if (transfer && 'manifest' in transfer && transfer.manifest) {
        const manifest = parseDownloadManifest(transfer.manifest);
        if (
          manifest.id !== entry.jobID ||
          manifest.size !== transfer.total ||
          (entry.manifest && entry.manifest.sha256 !== manifest.sha256)
        )
          throw invalid();
        entry.manifest = manifest;
        entry.total = manifest.size;
        entry.version = manifest.sha256;
      }
      if (
        !transfer &&
        ['queued', 'downloading', 'waiting', 'pausing'].includes(entry.status)
      ) {
        entry.status = 'paused';
        entry.error = 'Download stopped. Resume to continue.';
      }
      if (
        transfer &&
        transfer.total === entry.total &&
        Number.isSafeInteger(transfer.bytes) &&
        transfer.bytes >= 0 &&
        transfer.bytes <= entry.total &&
        [
          'preparing',
          'queued',
          'waiting',
          'pausing',
          'paused',
          'downloading',
          'verifying',
          'complete',
        ].includes(transfer.status) &&
        (transfer.status !== 'complete' || transfer.bytes === transfer.total) &&
        typeof transfer.error === 'string' &&
        transfer.error.length <= 256
      ) {
        Object.assign(entry, {
          bytes: transfer.bytes,
          status: transfer.status,
          error: transfer.error,
        });
      }
    }
  }
  if (verifiedTransfers)
    for (const entry of entries) {
      if (entry.status === 'complete' && !entry.manifest) {
        entry.status = 'paused';
        entry.error = 'Resume to verify this download for offline playback.';
      }
    }
  if (
    verifiedTransfers &&
    entries.some((entry) => entry.nativePlan || entry.manifest)
  ) {
    // Snapshot retrieval yields to enqueue, removal and playback-progress writes.
    // Commit only entries that are still the version observed before that await.
    const current = read(directory).map((entry) => {
      if (JSON.stringify(entry) !== originals.get(entry.item.id)) return entry;
      return (
        entries.find((snapshot) => snapshot.item.id === entry.item.id) ?? entry
      );
    });
    if (directory.exists) write(directory, current);
    return current;
  }
  return entries;
}
export async function pauseDownload(client?: KinosailClient, id?: string) {
  if (client && id) {
    validateDownloadID(id);
    if (active?.id === id) {
      active.controller.abort();
      await active.done;
    }
    const directory = await folder(client),
      entries = read(directory);
    const entry = entries.find((value) => value.item.id === id);
    if (entry && entry.total === 0) {
      await verifiedTransfers?.pauseVerifiedDownload(
        directory.name,
        await hash(id),
      );
      entry.status = 'paused';
      write(directory, entries);
      return;
    }
  }
  if (verifiedTransfers && client && id) {
    const directory = await folder(client);
    await verifiedTransfers.pauseVerifiedDownload(
      directory.name,
      await hash(id),
    );
  }
  if (backgroundTransfers && client && id) {
    await backgroundTransfers.pauseDownload(
      (await folder(client)).name,
      await hash(id),
    );
    return;
  }
  active?.controller.abort();
}
export async function clearDownloads() {
  clearing = true;
  try {
    await pauseDownload();
    await active?.done;
    await verifiedTransfers?.clearVerifiedDownloads();
    await backgroundTransfers?.clearDownloads();
    const directory = root();
    if (directory.exists) directory.delete();
  } finally {
    clearing = false;
  }
}
export async function removeDownload(
  client: KinosailClient,
  id: string,
  canRemove: () => boolean = () => true,
) {
  validateDownloadID(id);
  removing++;
  try {
    if (active) {
      await pauseDownload();
      await active.done;
    }
    const directory = await folder(client);
    const file = await mediaFile(directory, id);
    if (!canRemove()) return;
    await verifiedTransfers?.removeVerifiedDownload(
      directory.name,
      await hash(id),
    );
    await backgroundTransfers?.removeDownload(directory.name, await hash(id));
    const entries = read(directory);
    if (!canRemove()) return;
    if (file.exists) file.delete();
    if (directory.exists)
      write(
        directory,
        entries.filter((entry) => entry.item.id !== id),
      );
  } finally {
    removing--;
  }
}
export async function downloadMedia(
  client: KinosailClient,
  input: MediaItem,
  change: () => void,
  requestedQuality?: DownloadQuality,
  requestedTracks?: DownloadTrackSelection,
): Promise<void> {
  if (requestedQuality !== undefined) parseDownloadQuality(requestedQuality);
  if (!downloadsAvailable || clearing || removing > 0)
    throw new Error('Downloads are not available here.');
  if (active) {
    if (!backgroundDownloadsAvailable)
      throw new Error('Pause the current download before starting another.');
    await active.done;
    return downloadMedia(
      client,
      input,
      change,
      requestedQuality,
      requestedTracks,
    );
  }
  const item = parseMediaItem(input as unknown as InputValue);
  const controller = new AbortController();
  const run = async () => {
    const directory = await folder(client);
    const entries = await listDownloads(client);
    activeEntries = { uri: directory.uri, entries };
    let entry = entries.find((entry) => entry.item.id === item.id);
    const quality = requestedQuality ?? entry?.quality ?? 'original';
    const tracks = requestedTracks
      ? parseDownloadTracks(requestedTracks)
      : entry?.tracks;
    if (tracks && quality === 'original')
      throw new Error('Original downloads keep all embedded tracks.');
    if (
      entry &&
      requestedTracks &&
      JSON.stringify(requestedTracks) !== JSON.stringify(entry.tracks)
    )
      throw new Error('Remove the existing download before changing tracks.');
    if (quality !== 'original' && item.kind !== 'video')
      throw new Error(
        'Video quality choices are available for movies and episodes.',
      );
    if (entry && quality !== (entry.quality ?? 'original'))
      throw new Error(
        'Remove the existing download before choosing a different quality.',
      );
    if (
      (entry?.nativePlan && entry.status === 'preparing') ||
      entry?.status === 'complete' ||
      (backgroundDownloadsAvailable &&
        entry &&
        ['queued', 'waiting', 'downloading', 'pausing'].includes(entry.status))
    )
      return;
    let prepared: PreparedDownload | undefined;
    if (entry?.status === 'preparing' && entry.jobID) {
      prepared = await client.loadPreparedDownload(
        entry.jobID,
        controller.signal,
      );
      if (prepared.itemId !== item.id || prepared.quality !== quality)
        throw invalid();
      if (controller.signal.aborted) return;
      if (prepared.state !== 'ready') {
        if (prepared.state === 'failed') {
          entry.status = 'paused';
          entry.error =
            'The Server could not prepare this download. Resume to try again.';
          write(directory, entries);
          change();
        }
        return;
      }
    }
    const preferences = await client.loadMediaPreferences();
    const playbackPreferences = await client.loadPlaybackPreferences(input.id);
    await writeExperienceCache(
      client,
      `playback:${input.id}`,
      playbackPreferences.playback,
    );
    const network = await Network.getNetworkStateAsync();
    if (
      !backgroundDownloadsAvailable &&
      preferences.wifiOnly &&
      ![
        Network.NetworkStateType.WIFI,
        Network.NetworkStateType.ETHERNET,
      ].includes(network.type ?? Network.NetworkStateType.UNKNOWN)
    )
      throw new Error(
        'Connect to Wi-Fi to download, or change Download settings.',
      );
    if (!entry && entries.length >= downloadLimit)
      throw new Error('Remove a downloaded title before adding another.');
    // A fresh authorized API response is required for every start or resume.
    const source = await client.loadPlayback(item.id);
    let uri = source.details?.download;
    if (
      !uri ||
      new URL(uri).origin !== client.baseURL ||
      new URL(uri).pathname !== `/download/${encodeURIComponent(item.id)}` ||
      uri.length > 2048 ||
      new URL(uri).username ||
      new URL(uri).password ||
      new URL(uri).search ||
      new URL(uri).hash
    )
      throw new Error('Downloads are not permitted for this title.');
    const newEntry = (
      total: number,
      version: string,
      bytes = 0,
    ): DownloadEntry => ({
      baseline: item.progress,
      chapters: source.details?.chapters ?? [],
      item: {
        ...item,
        artwork: '',
        backdrop: '',
        progress: { ...item.progress, seconds: source.start },
      },
      bytes,
      total,
      duration: source.duration,
      contentType: quality === 'original' ? source.contentType : 'video/mp4',
      version,
      quality,
      ...(tracks ? { tracks } : {}),
      status: 'paused',
      error: '',
    });
    let etag: string | undefined;
    if (quality !== 'original' || verifiedTransfers) {
      prepared ??= await client.prepareDownload(
        item.id,
        quality,
        controller.signal,
        tracks,
      );
      if (controller.signal.aborted) return;
      if (
        prepared.itemId !== item.id ||
        prepared.quality !== quality ||
        (entry?.jobID && prepared.id !== entry.jobID)
      )
        throw invalid();
      if (!entry) {
        entry = { ...newEntry(0, ''), jobID: prepared.id, status: 'preparing' };
        await prepareOfflineStorage();
        if (controller.signal.aborted) return;
        directory.create({ intermediates: true, idempotent: true });
        entries.push(entry);
        write(directory, entries);
        change();
      }
      if (prepared.extension === '.mkv') entry.contentType = 'video/x-matroska';
      if (prepared.state !== 'ready') {
        if (verifiedTransfers && prepared.state === 'preparing') {
          await verifiedTransfers.enqueuePreparingDownload(
            JSON.stringify({
              scope: directory.name,
              key: await hash(item.id),
              uri: `${client.baseURL}/api/v1/downloads/${prepared.id}/file`,
              kind: item.kind,
              authorization: client.authorizationHeaders().Authorization,
              wifiOnly: preferences.wifiOnly,
              quota: preferences.downloadLimitGiB * 1024 ** 3,
            }),
          );
          entry.nativePlan = true;
        }
        entry.status = prepared.state === 'failed' ? 'paused' : 'preparing';
        entry.error =
          prepared.state === 'failed'
            ? 'The Server could not prepare this download. Resume to try again.'
            : '';
        write(directory, entries);
        change();
        return;
      }
      uri = `${client.baseURL}/api/v1/downloads/${prepared.id}/file`;
      etag = `"${prepared.sha256}"`;
    }
    const headers = {
      ...client.authorizationHeaders(),
      'Cache-Control': 'no-cache, no-store',
    };
    if (verifiedTransfers && prepared) {
      const manifest = await client.loadDownloadManifest(
        prepared,
        controller.signal,
      );
      const used = entries.reduce(
        (sum, current) => sum + (current === entry ? 0 : current.total),
        0,
      );
      downloadSize(
        String(manifest.size),
        used,
        Paths.availableDiskSpace + (entry?.bytes ?? 0),
        preferences.downloadLimitGiB * 1024 ** 3,
      );
      if (!entry) {
        entry = newEntry(manifest.size, manifest.sha256);
        entries.push(entry);
      }
      if (entry.manifest && entry.manifest.sha256 !== manifest.sha256)
        throw new Error(
          'This title changed. Remove the download before choosing a new version.',
        );
      await prepareOfflineStorage();
      directory.create({ intermediates: true, idempotent: true });
      if (prepared.extension === '.mkv') entry.contentType = 'video/x-matroska';
      entry.manifest = manifest;
      entry.jobID = prepared.id;
      entry.total = manifest.size;
      entry.version = manifest.sha256;
      entry.status = 'queued';
      entry.error = '';
      write(directory, entries);
      // Stop a legacy OS request before adopting the same destination file.
      await backgroundTransfers?.pauseDownload(
        directory.name,
        await hash(item.id),
      );
      try {
        await verifiedTransfers.enqueueVerifiedDownload(
          JSON.stringify({
            scope: directory.name,
            key: await hash(item.id),
            uri,
            manifest,
            kind: item.kind,
            authorization: headers.Authorization,
            wifiOnly: preferences.wifiOnly,
            quota: preferences.downloadLimitGiB * 1024 ** 3,
          }),
        );
      } catch (reason) {
        entry.status = 'paused';
        entry.error =
          reason instanceof Error
            ? reason.message.slice(0, 256)
            : 'Download could not start.';
        write(directory, entries);
        throw reason;
      }
      change();
      return;
    }
    const head = await fetch(uri, {
      method: 'HEAD',
      headers,
      redirect: 'error',
      credentials: 'omit',
      signal: controller.signal,
    });
    if (head.status !== 200)
      throw new Error('The download is no longer available.');
    if (
      prepared &&
      (head.headers.get('etag') !== etag ||
        head.headers.get('content-length') !== String(prepared.size))
    )
      throw new Error(
        'The prepared file changed. Remove this download and start again.',
      );
    const modified = downloadModification(head.headers.get('last-modified'));
    const version = await hash(
      `${etag ?? source.details?.media?.version ?? ''}\n${modified}`,
    );
    const used = entries.reduce(
      (total, value) => total + (value === entry ? 0 : value.total),
      0,
    );
    const total = downloadSize(
      head.headers.get('content-length'),
      used,
      Paths.availableDiskSpace + (entry?.bytes ?? 0),
      preferences.downloadLimitGiB * 1024 ** 3,
    );
    if (
      entry &&
      entry.total > 0 &&
      (entry.total !== total || entry.version !== version)
    )
      throw new Error(
        'The original file changed. Remove this download and start again.',
      );
    if (controller.signal.aborted) return;
    const file = await mediaFile(directory, item.id);
    const offset = file.exists ? file.size : 0;
    if (offset > total) throw invalid();
    if (!entry) {
      entry = newEntry(total, version, offset);
      entries.push(entry);
    }
    await prepareOfflineStorage();
    if (controller.signal.aborted) return;
    directory.create({ intermediates: true, idempotent: true });
    entry.bytes = offset;
    entry.total = total;
    entry.version = version;
    if (offset === total) {
      entry.status = 'complete';
      write(directory, entries);
      change();
      return;
    }
    if (backgroundTransfers) {
      entry.status = 'queued';
      entry.error = '';
      write(directory, entries);
      try {
        await backgroundTransfers.enqueueDownload(
          JSON.stringify({
            scope: directory.name,
            key: await hash(item.id),
            uri,
            modified,
            ...(etag ? { etag } : {}),
            total,
            authorization: headers.Authorization,
            wifiOnly: preferences.wifiOnly,
            quota: preferences.downloadLimitGiB * 1024 ** 3,
          }),
        );
      } catch (reason) {
        entry.status = 'paused';
        entry.error =
          reason instanceof Error
            ? reason.message.slice(0, 256)
            : 'Download could not start.';
        write(directory, entries);
        throw reason;
      }
      change();
      return;
    }
    let handle: FileHandle | null = null;
    const connection = Network.addNetworkStateListener((value) => {
      if (
        preferences.wifiOnly &&
        ![
          Network.NetworkStateType.WIFI,
          Network.NetworkStateType.ETHERNET,
        ].includes(value.type ?? Network.NetworkStateType.UNKNOWN)
      )
        controller.abort();
    });
    const background = AppState.addEventListener('change', (state) => {
      if (state !== 'active') controller.abort();
    });
    try {
      const response = await fetch(uri, {
        headers: {
          ...headers,
          'If-Unmodified-Since': modified,
          ...(etag ? { 'If-Match': etag } : {}),
          ...(offset ? { Range: `bytes=${offset}-` } : {}),
        },
        redirect: 'error',
        credentials: 'omit',
        signal: controller.signal,
      });
      if (
        response.headers.get('last-modified') !== modified ||
        (etag && response.headers.get('etag') !== etag)
      )
        throw new Error(
          'The original file changed. Remove this download and start again.',
        );
      validateDownloadResponse(
        response.status,
        response.headers.get('content-length'),
        response.headers.get('content-range'),
        offset,
        total,
      );
      if (!response.body)
        throw new Error('The Server did not provide the download.');
      if (!file.exists) file.create();
      handle = file.open(FileMode.Append);
      entry.status = 'downloading';
      entry.error = '';
      write(directory, entries);
      change();
      const reader = response.body.getReader();
      let lastUpdate = 0;
      try {
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;
          if (controller.signal.aborted) throw new Error('Download paused.');
          if (entry.bytes + value.byteLength > total)
            throw new Error('The download exceeded its declared size.');
          handle.writeBytes(value);
          entry.bytes += value.byteLength;
          if (Date.now() - lastUpdate >= 1000) {
            write(directory, entries);
            change();
            lastUpdate = Date.now();
          }
        }
      } finally {
        await reader.cancel().catch(() => {});
      }
      if (entry.bytes !== total)
        throw new Error('The download was interrupted. Resume to continue.');
      entry.status = 'complete';
    } catch (error) {
      entry.status = 'paused';
      entry.error = controller.signal.aborted
        ? ''
        : 'Download interrupted. Resume to try again.';
    } finally {
      connection.remove();
      background.remove();
      handle?.close();
      write(directory, entries);
      change();
    }
  };
  const done = run();
  active = { id: item.id, controller, done: done.catch(() => {}) };
  try {
    await done;
  } finally {
    active = null;
    activeEntries = null;
  }
}
export async function downloadedPlayback(
  client: KinosailClient,
  id: string,
): Promise<{
  item: MediaItem;
  source: PlaybackSource;
  baseline?: Progress;
} | null> {
  validateDownloadID(id);
  const directory = await folder(client),
    entry = (await listDownloads(client)).find((entry) => entry.item.id === id);
  if (!entry || entry.status !== 'complete') return null;
  const file = await mediaFile(directory, id);
  if (!file.exists || file.size !== entry.total)
    throw new Error(
      'This download is missing or incomplete. Download it again.',
    );
  return {
    item: entry.item,
    baseline: entry.baseline ?? entry.item.progress,
    source: {
      uri: file.uri,
      headers: { Authorization: '' },
      contentType: entry.contentType,
      duration: entry.duration,
      start: entry.item.progress.seconds,
      progressToken: '',
      plan: { allowed: true, mode: 'direct', reason: 'downloaded' },
      details: { chapters: entry.chapters ?? [], download: '', next: '' },
    },
  };
}

export async function saveDownloadedProgress(
  client: KinosailClient,
  id: string,
  input: Pick<Progress, 'seconds' | 'session' | 'revision'> & {
    watched?: boolean;
  },
): Promise<Progress> {
  validateDownloadID(id);
  if (
    !input ||
    typeof input !== 'object' ||
    Object.keys(input).some(
      (key) =>
        ![
          'seconds',
          'session',
          'revision',
          'watched',
          'playbackToken',
        ].includes(key),
    ) ||
    !Number.isFinite(input.seconds) ||
    input.seconds < 0 ||
    input.seconds > 31_536_000 ||
    !Number.isSafeInteger(input.revision) ||
    input.revision < 0 ||
    input.revision >= Number.MAX_SAFE_INTEGER ||
    typeof input.session !== 'string' ||
    !input.session ||
    /[\u0000-\u001f\u007f]/.test(input.session) ||
    input.session.length > 128 ||
    (input.watched !== undefined && typeof input.watched !== 'boolean')
  )
    throw invalid();
  const directory = await folder(client),
    entries = await listDownloads(client),
    entry = entries.find((value) => value.item.id === id);
  if (
    !entry ||
    entry.status !== 'complete' ||
    (entry.duration > 0 && input.seconds > entry.duration) ||
    (input.session === entry.item.progress.session &&
      input.revision <= entry.item.progress.revision)
  )
    throw invalid();
  entry.item.progress = {
    seconds: input.seconds,
    session: input.session,
    revision: input.revision,
    watched: input.watched ?? entry.item.progress.watched,
  };
  const downloading =
    activeEntries?.uri === directory.uri
      ? activeEntries.entries.find((value) => value.item.id === id)
      : undefined;
  if (downloading) downloading.item.progress = entry.item.progress;
  write(directory, entries);
  return entry.item.progress;
}

export async function acknowledgeDownloadedProgress(
  client: KinosailClient,
  id: string,
  snapshot: Progress,
) {
  validateDownloadID(id);
  const progress = parseProgress(snapshot as unknown as InputValue);
  const directory = await folder(client);
  if (!directory.exists) return;
  const entries = read(directory),
    entry = entries.find((value) => value.item.id === id);
  if (!entry) return;
  entry.baseline = progress;
  entry.item.progress = progress;
  if (activeEntries?.uri === directory.uri) {
    const current = activeEntries.entries.find((value) => value.item.id === id);
    if (current) {
      current.baseline = progress;
      current.item.progress = progress;
    }
  }
  write(directory, entries);
}

// Recheck on-device bytes and decoding without contacting the Server.
export async function checkDownloadedMedia(client: KinosailClient, id: string) {
  validateDownloadID(id);
  if (!verifiedTransfers)
    throw new Error('Offline checks require the native Player build.');
  const directory = await folder(client);
  const entry = read(directory).find((value) => value.item.id === id);
  if (!entry?.manifest)
    throw new Error('Resume this download to upgrade its integrity checks.');
  await verifiedTransfers.checkVerifiedDownload(directory.name, await hash(id));
}
