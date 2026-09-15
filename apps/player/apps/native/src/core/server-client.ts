import {
  musicID,
  parseAlbums,
  parseAlbum,
  parseMusicQueue,
} from './music-catalog';
import {
  approvalCode,
  parseDeviceApproval,
  parsePendingTVs,
} from './device-approval';
import {
  parseDownloadTracks,
  parseDownloadTrackOptions,
  type DownloadTrackSelection,
} from './download-tracks';
import {
  parseDownloadManifest,
  parseDownloadIdentity,
  type DownloadIdentity,
} from './download-manifest';
import {
  castID,
  validateCastStart,
  validateCastCommand,
  parseCastSession,
  parseCastDevices,
  parseCastStatus,
  type CastStart,
  type CastCommand,
} from './casting';
import {
  collectionName,
  parseCollections,
  parseCollection,
} from './collections';
import {
  parseDownloadQuality,
  parsePreparedDownload,
  validatePreparedID,
  type DownloadQuality,
  type PreparedDownload,
} from './download-quality';
import { validateDownloadID } from './download-policy';
import { parseReader, parseReaderProgress } from './reader';
import { validateSyncProgress } from './progress-validation';
import {
  parsePlaybackPreferences,
  parseMediaPreferences,
  parseItemPreferences,
  parseBookmarks,
  validateBookmark,
  type PlaybackPreferences,
  type MediaPreferences,
  type Bookmark,
} from './media-preferences';
import {
  parseChallenge,
  parseShowEpisodes,
  parseItem,
  parseLibrary,
  parseLibraryPage,
  parseMe,
  parsePending,
  parsePlayback,
  parseProgress,
  parseQuickConnectToken,
  type Home,
  type InputValue,
  type MediaItem,
  type PlaybackSource,
  type Progress,
} from './contract';
import { libraryQuery, type LibraryQuery } from './library-query';
import { getPlaybackQuery } from './playback-query';

const MAX_RESPONSE_BYTES = 2 * 1024 * 1024;
const MAX_SERVER_URL_BYTES = 2048;

export type Fetcher = (input: string, init?: RequestInit) => Promise<Response>;

const isPrivateIPv4 = (hostname: string): boolean => {
  // URL parsing already normalizes and bounds numeric IP addresses.
  const parts = hostname.split('.').map(Number);
  if (parts.some((part) => !Number.isInteger(part))) return false;
  return (
    parts[0] === 10 ||
    parts[0] === 127 ||
    (parts[0] === 169 && parts[1] === 254) ||
    (parts[0] === 172 && parts[1] >= 16 && parts[1] <= 31) ||
    (parts[0] === 192 && parts[1] === 168)
  );
};

const isLocalHost = (hostname: string): boolean => {
  const host = hostname.toLowerCase();
  return (
    host === 'localhost' ||
    host.endsWith('.local') ||
    isPrivateIPv4(host) ||
    host === '[::1]' ||
    /\[f[cd][0-9a-f]{2}:/.test(host) ||
    host.startsWith('[fe80:')
  );
};

export const normalizeServerURL = (input: string): string => {
  const message = 'Enter a valid Kinosail Server URL.';
  const raw = input.trim();
  if (!raw || raw.length > MAX_SERVER_URL_BYTES) throw new Error(message);
  let parsed: URL;
  try {
    parsed = new URL(raw);
  } catch {
    throw new Error(message);
  }
  if (
    (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') ||
    (parsed.protocol === 'http:' && !isLocalHost(parsed.hostname)) ||
    parsed.username ||
    parsed.password ||
    parsed.pathname !== '/' ||
    parsed.search ||
    parsed.hash
  ) {
    throw new Error(message);
  }
  return parsed.origin;
};

const validSecret = (value: string): string => {
  if (
    typeof value !== 'string' ||
    !value ||
    value.length > 2048 ||
    /[\r\n]/.test(value)
  ) {
    throw new Error('Kinosail session data is invalid.');
  }
  return value;
};

const validDevice = (value: string): string => {
  const device = value.trim();
  if (!device || device.length > 80 || /[\u0000-\u001f\u007f]/.test(device)) {
    throw new Error('Enter a valid device name.');
  }
  return device;
};

const errorMessage = (status: number): string => {
  if (status === 401) return 'This device session has expired.';
  if (status === 403) return 'This profile cannot use that feature.';
  if (status === 404) return 'The requested media is no longer available.';
  if (status === 409) return 'The server state changed. Refresh and try again.';
  if (status === 429) return 'Too many requests. Wait a minute and try again.';
  return 'Kinosail Server could not complete the request.';
};

export class KinosailClient {
  readonly baseURL: string;
  private readonly token: string;
  private readonly fetcher: Fetcher;
  private readonly mediaHeaders: { Authorization: string };

  constructor(
    baseURL: string,
    token = '',
    fetcher?: Fetcher,
    readonly downloadScope?: string,
  ) {
    if (downloadScope !== undefined && !/^[a-f0-9]{64}$/.test(downloadScope))
      throw new Error('Invalid download storage identity.');
    this.baseURL = normalizeServerURL(baseURL);
    this.token = token ? validSecret(token) : '';
    this.mediaHeaders = Object.freeze({
      Authorization: `Bearer ${this.token}`,
    });
    this.fetcher = fetcher ?? ((input, init) => globalThis.fetch(input, init));
  }

  private async request(
    path: string,
    init: RequestInit = {},
    expected: readonly number[] = [200],
  ): Promise<{ status: number; body: InputValue }> {
    const headers: Record<string, string> = {
      Accept: 'application/json',
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...(this.token ? { Authorization: `Bearer ${this.token}` } : {}),
    };
    const fetcher = this.fetcher;
    let response: Response;
    try {
      response = await fetcher(`${this.baseURL}${path}`, {
        ...init,
        headers: { ...headers, ...(init.headers as Record<string, string>) },
      });
    } catch {
      throw new Error(
        'Could not reach Kinosail Server. Check the address and network, then try again.',
      );
    }
    if (!expected.includes(response.status))
      throw new Error(errorMessage(response.status));
    const raw = await response.text();
    if (raw.length > MAX_RESPONSE_BYTES) {
      throw new Error('Kinosail Server returned an invalid response.');
    }
    try {
      return { status: response.status, body: raw ? JSON.parse(raw) : {} };
    } catch {
      throw new Error('Kinosail Server returned an invalid response.');
    }
  }

  async startQuickConnect(device: string) {
    const result = await this.request(
      '/api/v1/quick-connect',
      { method: 'POST', body: JSON.stringify({ device: validDevice(device) }) },
      [201],
    );
    return parseChallenge(result.body);
  }

  async pollQuickConnect(secret: string): Promise<string | null> {
    const result = await this.request(
      '/api/v1/quick-connect/token',
      { method: 'POST', body: JSON.stringify({ secret: validSecret(secret) }) },
      [201, 202],
    );
    if (result.status === 202) {
      parsePending(result.body);
      return null;
    }
    return parseQuickConnectToken(result.body);
  }

  async cancelQuickConnect(secret: string): Promise<void> {
    await this.request(
      '/api/v1/quick-connect/cancel',
      { method: 'POST', body: JSON.stringify({ secret: validSecret(secret) }) },
      [204],
    );
  }

  async loadViewer() {
    return parseMe((await this.request('/api/v1/me')).body);
  }

  async loadPendingTVs() {
    return parsePendingTVs(
      (await this.request('/api/v1/quick-connect/pending')).body,
    );
  }

  async previewDevice(code: string) {
    const expected = approvalCode(code);
    const pending = parseDeviceApproval(
      (await this.request(`/api/v1/quick-connect/${expected}`)).body,
    );
    if (pending.code !== expected)
      throw new Error('The device request does not match your code.');
    return pending;
  }

  async approveDevice(code: string): Promise<void> {
    await this.request(
      `/api/v1/quick-connect/${approvalCode(code)}`,
      { method: 'POST' },
      [204],
    );
  }

  async loadLibrary(view: 'history' | 'recent'): Promise<MediaItem[]> {
    if (view !== 'history' && view !== 'recent')
      throw new Error('The requested library view is invalid.');
    const path = {
      history: '/api/v1/library?view=history&limit=24',
      recent: '/api/v1/library?sort=added&limit=36',
    }[view];
    return parseLibrary((await this.request(path)).body).map((item) =>
      this.validateItem(item),
    );
  }

  async loadHome(): Promise<Home> {
    const [meResponse, continueWatching, recent] = await Promise.all([
      this.request('/api/v1/me'),
      this.loadLibrary('history'),
      this.loadLibrary('recent'),
    ]);
    const me = parseMe(meResponse.body);
    return { ...me, continueWatching, recent };
  }

  async browseLibrary(query: LibraryQuery = {}, signal?: AbortSignal) {
    const path = `/api/v1/library?${libraryQuery(query)}`;
    const page = parseLibraryPage((await this.request(path, { signal })).body);
    return {
      ...page,
      items: page.items.map((item) => this.validateItem(item)),
    };
  }

  async loadCollections(signal?: AbortSignal): Promise<string[]> {
    return parseCollections(
      (await this.request('/api/v1/collections', { signal })).body,
    );
  }

  async loadCollection(
    name: string,
    signal?: AbortSignal,
  ): Promise<MediaItem[]> {
    const valid = collectionName(name);
    return parseCollection(
      (
        await this.request(`/api/v1/collections/${encodeURIComponent(valid)}`, {
          signal,
        })
      ).body,
      valid,
    ).map((item) => this.validateItem(item));
  }

  async loadAlbums(signal?: AbortSignal) {
    return parseAlbums(
      (await this.request('/api/v1/albums', { signal })).body,
    ).map((album) => {
      this.mediaURL(album.artwork);
      return album;
    });
  }

  async loadAlbum(id: string, signal?: AbortSignal) {
    const album = parseAlbum(
      (await this.request(`/api/v1/albums/${musicID(id)}`, { signal })).body,
      id,
    );
    return {
      ...album,
      tracks: album.tracks.map((item) => this.validateItem(item)),
    };
  }

  async loadMusicQueue(id: string, signal?: AbortSignal) {
    return parseMusicQueue(
      (await this.request(`/api/v1/audio/${musicID(id)}/queue`, { signal }))
        .body,
      id,
    ).map((item) => this.validateItem(item));
  }

  async loadItem(id: string): Promise<MediaItem> {
    return this.validateItem(
      parseItem(
        (
          await this.request(
            `/api/v1/items/${encodeURIComponent(validSecret(id))}`,
          )
        ).body,
      ),
    );
  }

  async loadPlayback(id: string): Promise<PlaybackSource> {
    const itemID = encodeURIComponent(validSecret(id));
    const path = `/api/v1/items/${itemID}/playback?${await getPlaybackQuery()}`;
    const playback = parsePlayback((await this.request(path)).body);
    if (!playback.directAllowed || !playback.direct) {
      throw new Error('Direct playback is not available for this title.');
    }
    return {
      details: {
        ...playback.details,
        download: this.mediaURL(playback.details.download),
      },
      compatible: playback.compatible
        ? {
            ...playback.compatible,
            uri: this.mediaURL(playback.compatible.uri),
          }
        : undefined,
      uri: this.mediaURL(playback.direct),
      headers: { Authorization: `Bearer ${this.token}` },
      contentType: playback.directType,
      duration: playback.duration,
      start: playback.start,
      progressToken: playback.progressToken,
      // This method selected the authorized original URL above. A Server's
      // preferred compatibility plan must not mislabel those original bytes.
      plan:
        playback.plan.mode === 'direct'
          ? playback.plan
          : { allowed: true, mode: 'direct', reason: 'direct-preferred' },
    };
  }

  async startCast(id: string, input: CastStart) {
    validateDownloadID(id);
    validateCastStart(input);
    const result = await this.request(
      `/api/v1/items/${encodeURIComponent(id)}/cast`,
      { method: 'POST', body: JSON.stringify(input) },
      [201],
    );
    return parseCastSession(result.body, this.baseURL);
  }

  async endCast(id: string) {
    await this.request(
      `/api/v1/cast/sessions/${castID(id)}`,
      { method: 'DELETE' },
      [204, 404],
    );
  }

  async scanCastDevices() {
    const result = await this.request('/api/v1/cast/devices/scan', {
      method: 'POST',
      body: '{}',
    });
    return parseCastDevices(result.body);
  }

  async castStatus(id: string) {
    const result = await this.request(`/api/v1/cast/sessions/${castID(id)}`);
    return parseCastStatus(result.body);
  }

  async castCommand(id: string, command: CastCommand) {
    castID(id);
    validateCastCommand(command);
    await this.request(
      `/api/v1/cast/sessions/${id}/commands`,
      { method: 'POST', body: JSON.stringify(command) },
      [204],
    );
  }

  async saveProgress(
    id: string,
    progress: Pick<Progress, 'seconds' | 'session' | 'revision'> & {
      playbackToken?: string;
      watched?: boolean;
    },
  ): Promise<Progress> {
    if (!Number.isFinite(progress.seconds) || progress.seconds < 0) {
      throw new Error('Playback progress is invalid.');
    }
    const result = await this.request(
      `/api/v1/items/${encodeURIComponent(validSecret(id))}/progress`,
      {
        method: 'PUT',
        body: JSON.stringify({
          seconds: progress.seconds,
          session: validSecret(progress.session),
          revision: progress.revision,
          playbackToken: progress.playbackToken || '',
          watched: progress.watched,
        }),
      },
    );
    return parseProgress(result.body);
  }

  async syncProgress(
    id: string,
    progress: Progress,
    expected: Progress,
    playbackToken = '',
  ) {
    const update = validateSyncProgress(progress, true),
      baseline = validateSyncProgress(expected, false);
    if (typeof playbackToken !== 'string' || playbackToken.length > 8192)
      throw new Error('Playback progress is invalid.');
    const response = await this.request(
      `/api/v1/items/${encodeURIComponent(validSecret(id))}/progress/sync`,
      {
        method: 'PUT',
        body: JSON.stringify({
          progress: update,
          expected: baseline,
          playbackToken,
        }),
        signal: AbortSignal.timeout(10000),
      },
      [200, 409],
    );
    const body = response.body as Record<string, InputValue>;
    return {
      conflict: response.status === 409,
      progress: parseProgress(
        response.status === 409 ? body.progress : response.body,
      ),
    };
  }

  async loadReader(id: string) {
    return parseReader(
      (
        await this.request(
          `/api/v1/books/${encodeURIComponent(validSecret(id))}/reader`,
        )
      ).body,
      id,
    );
  }
  async loadReaderProgress(id: string) {
    return parseReaderProgress(
      (
        await this.request(
          `/api/v1/books/${encodeURIComponent(validSecret(id))}/reader/progress?includeOffset=true`,
        )
      ).body,
    );
  }
  async saveReaderProgress(id: string, page: number, offset = 0) {
    if (
      !Number.isInteger(page) ||
      page < 1 ||
      page > 10000 ||
      !Number.isFinite(offset) ||
      offset < 0 ||
      offset > 1
    )
      throw new Error('Invalid reading position.');
    return parseReaderProgress(
      (
        await this.request(
          `/api/v1/books/${encodeURIComponent(validSecret(id))}/reader/progress?includeOffset=true`,
          { method: 'PUT', body: JSON.stringify({ page, offset }) },
        )
      ).body,
    );
  }
  async loadShowEpisodes(id: string): Promise<MediaItem[]> {
    if (typeof id !== 'string' || !/^[a-f0-9]{16}$/.test(id))
      throw new Error('The requested show is invalid.');
    return parseShowEpisodes(
      (await this.request(`/api/v1/shows/${id}`)).body,
      id,
    ).map((item) => this.validateItem(item));
  }

  async prepareDownload(
    id: string,
    quality: DownloadQuality,
    signal?: AbortSignal,
    tracks?: DownloadTrackSelection,
  ) {
    validateDownloadID(id);
    parseDownloadQuality(quality);
    if (tracks) {
      parseDownloadTracks(tracks);
      if (quality === 'original')
        throw new Error('Original downloads keep all embedded tracks.');
    }
    const result = await this.request(
      `/api/v1/items/${encodeURIComponent(id)}/downloads`,
      {
        method: 'POST',
        body: JSON.stringify({ quality, ...(tracks ? { tracks } : {}) }),
        signal,
        redirect: 'error',
      },
      [202],
    );
    const job = parsePreparedDownload(result.body);
    if (job.itemId !== id || job.quality !== quality)
      throw new Error('The Server prepared a different download.');
    return job;
  }

  async loadPreparedDownload(id: string, signal?: AbortSignal) {
    validatePreparedID(id);
    const result = await this.request(`/api/v1/downloads/${id}`, {
      signal,
      redirect: 'error',
    });
    const job = parsePreparedDownload(result.body);
    if (job.id !== id)
      throw new Error('The Server returned a different download.');
    return job;
  }

  async loadDownloadTracks(id: string) {
    validateDownloadID(id);
    const result = await this.request(
      `/api/v1/items/${encodeURIComponent(id)}/download-tracks`,
      { redirect: 'error' },
    );
    return parseDownloadTrackOptions(result.body);
  }

  async loadDownloadManifest(job: PreparedDownload, signal?: AbortSignal) {
    validatePreparedID(job.id);
    const result = await this.request(`/api/v1/downloads/${job.id}/manifest`, {
      signal,
      redirect: 'error',
    });
    return parseDownloadManifest(result.body, job);
  }

  async loadDownloadIdentity(): Promise<DownloadIdentity> {
    const result = await this.request('/api/v1/me', { redirect: 'error' });
    const value = result.body as {
      serverId?: string;
      viewer?: { id?: string };
    };
    return parseDownloadIdentity({
      serverId: value?.serverId,
      profileId: value?.viewer?.id,
    });
  }

  async loadMediaPreferences(): Promise<MediaPreferences> {
    return parseMediaPreferences(
      (await this.request('/api/v1/me/media-preferences')).body,
    );
  }
  async saveMediaPreferences(
    value: MediaPreferences,
  ): Promise<MediaPreferences> {
    const valid = parseMediaPreferences(value);
    return parseMediaPreferences(
      (
        await this.request('/api/v1/me/media-preferences', {
          method: 'PUT',
          body: JSON.stringify(valid),
        })
      ).body,
    );
  }
  async loadPlaybackPreferences(id: string) {
    return parseItemPreferences(
      (
        await this.request(
          `/api/v1/items/${encodeURIComponent(validSecret(id))}/playback-preferences`,
        )
      ).body,
    );
  }
  async savePlaybackPreferences(id: string, value: PlaybackPreferences) {
    const valid = parsePlaybackPreferences(value);
    return parseItemPreferences(
      (
        await this.request(
          `/api/v1/items/${encodeURIComponent(validSecret(id))}/playback-preferences`,
          { method: 'PUT', body: JSON.stringify(valid) },
        )
      ).body,
    );
  }
  async resetPlaybackPreferences(id: string) {
    return parseItemPreferences(
      (
        await this.request(
          `/api/v1/items/${encodeURIComponent(validSecret(id))}/playback-preferences`,
          { method: 'DELETE' },
        )
      ).body,
    );
  }
  async loadBookmarks(id: string): Promise<Bookmark[]> {
    return parseBookmarks(
      (
        await this.request(
          `/api/v1/items/${encodeURIComponent(validSecret(id))}/bookmarks?includeOffset=true`,
        )
      ).body,
    );
  }
  async addBookmark(
    id: string,
    value: Omit<Bookmark, 'id'>,
  ): Promise<Bookmark[]> {
    const valid = validateBookmark(value);
    return parseBookmarks(
      (
        await this.request(
          `/api/v1/items/${encodeURIComponent(validSecret(id))}/bookmarks?includeOffset=true`,
          { method: 'POST', body: JSON.stringify(valid) },
          [201],
        )
      ).body,
    );
  }
  async removeBookmark(id: string, bookmark: string): Promise<Bookmark[]> {
    if (!/^[a-f0-9]{64}$/.test(bookmark))
      throw new Error('The bookmark is invalid.');
    return parseBookmarks(
      (
        await this.request(
          `/api/v1/items/${encodeURIComponent(validSecret(id))}/bookmarks/${bookmark}?includeOffset=true`,
          { method: 'DELETE' },
        )
      ).body,
    );
  }

  mediaURL(path: string): string {
    const message = 'Kinosail Server returned an invalid media URL.';
    if (typeof path !== 'string' || path.length > MAX_SERVER_URL_BYTES)
      throw new Error(message);
    if (!path) return '';
    let url: URL;
    try {
      url = new URL(path, this.baseURL);
    } catch {
      throw new Error(message);
    }
    if (url.origin !== this.baseURL || url.username || url.password)
      throw new Error(message);
    return url.toString();
  }

  private validateItem(item: MediaItem): MediaItem {
    this.mediaURL(item.artwork);
    this.mediaURL(item.backdrop);
    if (item.show && ['.', '..'].includes(item.id))
      throw new Error('Kinosail Server returned an invalid media ID.');
    // The item API exposes landscape episode stills. Our portrait surfaces use
    // the canonical series poster, including when the episode has no still.
    return item.show
      ? { ...item, artwork: `/art/${encodeURIComponent(item.id)}` }
      : item;
  }

  authorizationHeaders(): { Authorization: string } {
    return this.mediaHeaders;
  }
}
