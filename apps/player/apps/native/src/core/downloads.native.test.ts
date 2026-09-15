import type { KinosailClient } from './server-client';
import { routeItem, routeSource } from '@/testing/route-fixtures';

const mockFiles = new Map<string, Uint8Array>();
const mockDirectories = new Set<string>();
const mockFetch = jest.fn();
const mockPrepare = jest.fn().mockResolvedValue(undefined);
const mockMutations = jest.fn();
let mockBackground: (state: string) => void = () => {};

jest.mock('./experience-cache', () => ({
  writeExperienceCache: jest.fn().mockResolvedValue(undefined),
}));
jest.mock('expo-network', () => ({
  NetworkStateType: {
    WIFI: 'wifi',
    ETHERNET: 'ethernet',
    CELLULAR: 'cellular',
  },
  getNetworkStateAsync: jest.fn().mockResolvedValue({ type: 'wifi' }),
  addNetworkStateListener: jest.fn(() => ({ remove: jest.fn() })),
}));

jest.mock('expo/fetch', () => ({
  fetch: (...args: unknown[]) => mockFetch(...args),
}));
jest.mock('./verified-transfers', () => {
  const native = {
    enabled: false,
    enqueuePreparingDownload: jest.fn().mockResolvedValue(undefined),
    enqueueVerifiedDownload: jest.fn().mockResolvedValue(undefined),
    verifiedDownloadSnapshot: jest.fn().mockResolvedValue([]),
    pauseVerifiedDownload: jest.fn().mockResolvedValue(undefined),
    removeVerifiedDownload: jest.fn().mockResolvedValue(undefined),
    clearVerifiedDownloads: jest.fn().mockResolvedValue(undefined),
  };
  return {
    native,
    get verifiedTransfers() {
      return native.enabled ? native : null;
    },
  };
});
jest.mock('./protected-media', () => {
  const native = {
    enabled: false,
    enqueueDownload: jest.fn().mockResolvedValue(undefined),
    downloadSnapshot: jest.fn().mockResolvedValue([]),
    pauseDownload: jest.fn().mockResolvedValue(undefined),
    removeDownload: jest.fn().mockResolvedValue(undefined),
    clearDownloads: jest.fn().mockResolvedValue(undefined),
  };
  return {
    native,
    get backgroundTransfers() {
      return native.enabled ? native : null;
    },
    localCompatibilityAvailable: true,
    prepareOfflineStorage: () => mockPrepare(),
  };
});
jest.mock('expo-crypto', () => ({
  CryptoDigestAlgorithm: { SHA256: 'sha256' },
  digestStringAsync: async (_algorithm: string, value: string) =>
    jest
      .requireActual('node:crypto')
      .createHash('sha256')
      .update(value)
      .digest('hex'),
}));
jest.mock('react-native', () => ({
  Platform: { OS: 'ios', isTV: false },
  AppState: {
    addEventListener: (_event: string, callback: typeof mockBackground) => {
      mockBackground = callback;
      return { remove: jest.fn() };
    },
  },
}));
jest.mock('expo-file-system', () => {
  const path = (parts: (string | { uri: string })[]) =>
    parts.map((part) => (typeof part === 'string' ? part : part.uri)).join('/');
  class Directory {
    uri: string;
    constructor(...parts: (string | { uri: string })[]) {
      this.uri = path(parts);
    }
    get name() {
      return this.uri.split('/').at(-1)!;
    }
    get exists() {
      return (
        mockDirectories.has(this.uri) ||
        [...mockFiles.keys()].some((key) => key.startsWith(this.uri + '/'))
      );
    }
    create() {
      mockMutations('directory', this.uri);
      mockDirectories.add(this.uri);
    }
    delete() {
      mockMutations('remove-directory', this.uri);
      for (const key of mockFiles.keys())
        if (key.startsWith(this.uri + '/')) mockFiles.delete(key);
    }
  }
  class File {
    uri: string;
    constructor(...parts: (string | { uri: string })[]) {
      this.uri = path(parts);
    }
    get exists() {
      return mockFiles.has(this.uri);
    }
    get size() {
      return mockFiles.get(this.uri)?.byteLength ?? 0;
    }
    textSync() {
      return Buffer.from(mockFiles.get(this.uri)!).toString();
    }
    create() {
      mockMutations('file', this.uri);
      mockFiles.set(this.uri, new Uint8Array());
    }
    write(value: string) {
      mockMutations('write', this.uri);
      mockFiles.set(this.uri, Buffer.from(value));
    }
    moveSync(destination: File) {
      mockFiles.set(destination.uri, mockFiles.get(this.uri)!);
      mockFiles.delete(this.uri);
    }
    delete() {
      mockMutations('remove-file', this.uri);
      mockFiles.delete(this.uri);
    }
    open() {
      return {
        writeBytes: (bytes: Uint8Array) => {
          mockMutations('append', this.uri);
          mockFiles.set(
            this.uri,
            Buffer.concat([mockFiles.get(this.uri)!, bytes]),
          );
        },
        close: jest.fn(),
      };
    }
  }
  return {
    Directory,
    File,
    FileMode: { Append: 'wa' },
    Paths: {
      document: 'file:///Documents',
      totalDiskSpace: 256 * 1024 ** 3,
      availableDiskSpace: 30 * 1024 ** 3,
    },
  };
});

let {
  clearDownloads,
  downloadStorage,
  downloadMedia,
  downloadedPlayback,
  listDownloads,
  removeDownload,
  pauseDownload,
  saveDownloadedProgress,
} =
  jest.requireActual<typeof import('./downloads.native')>('./downloads.native');
function reloadDownloads() {
  jest.isolateModules(() => {
    ({
      clearDownloads,
      downloadStorage,
      downloadMedia,
      downloadedPlayback,
      listDownloads,
      removeDownload,
      pauseDownload,
      saveDownloadedProgress,
    } =
      jest.requireActual<typeof import('./downloads.native')>(
        './downloads.native',
      ));
  });
}
const mockNative = jest.requireMock('./protected-media').native;
const mockVerified = jest.requireMock('./verified-transfers').native;
const modified = 'Mon, 07 Sep 2026 12:00:00 GMT';
const client = (token = 'viewer-a') =>
  ({
    baseURL: 'https://kino.example',
    loadMediaPreferences: jest
      .fn()
      .mockResolvedValue({ wifiOnly: true, downloadLimitGiB: 20 }),
    loadPlaybackPreferences: jest.fn().mockResolvedValue({ playback: {} }),
    authorizationHeaders: () => ({ Authorization: `Bearer ${token}` }),
    loadPlayback: jest.fn().mockResolvedValue({
      ...routeSource,
      duration: 120,
      details: {
        chapters: [],
        next: '',
        download: 'https://kino.example/download/arrival',
      },
    }),
  }) as unknown as KinosailClient;
const head = () => ({
  status: 200,
  headers: new Headers({ 'Content-Length': '8', 'Last-Modified': modified }),
});
const body = (status = 200, value = 'abcdefgh', range?: string) =>
  new Response(value, {
    status,
    headers: {
      'Content-Length': String(value.length),
      'Last-Modified': modified,
      ...(range ? { 'Content-Range': range } : {}),
    },
  });
beforeEach(() => {
  jest
    .requireMock('expo-network')
    .getNetworkStateAsync.mockResolvedValue({ type: 'wifi' });
  Object.assign(jest.requireMock('expo-file-system').Paths, {
    totalDiskSpace: 256 * 1024 ** 3,
    availableDiskSpace: 30 * 1024 ** 3,
  });
  mockVerified.enabled = false;
  mockVerified.verifiedDownloadSnapshot.mockResolvedValue([]);
  mockNative.enabled = false;
  mockNative.downloadSnapshot.mockResolvedValue([]);
  mockFiles.clear();
  mockDirectories.clear();
  jest.clearAllMocks();
  mockFetch.mockReset();
  reloadDownloads();
});

it.each([undefined, 'cellular', 'unknown'])(
  'does not fetch or enqueue downloads without an allowed network: %s',
  async (type) => {
    jest
      .requireMock('expo-network')
      .getNetworkStateAsync.mockResolvedValue({ type });
    await expect(downloadMedia(client(), routeItem, jest.fn())).rejects.toThrow(
      'Connect to Wi-Fi',
    );
    expect(mockFetch).not.toHaveBeenCalled();
    expect(mockNative.enqueueDownload).not.toHaveBeenCalled();
    expect(mockVerified.enqueueVerifiedDownload).not.toHaveBeenCalled();
    expect(
      mockMutations.mock.calls.some(([operation]) => operation === 'append'),
    ).toBe(false);
  },
);

it('downloads an authorized file and plays it offline within the same viewer scope', async () => {
  const viewer = client();
  mockFetch.mockResolvedValueOnce(head()).mockResolvedValueOnce(body());
  await downloadMedia(viewer, routeItem, jest.fn());
  const entries = await listDownloads(viewer);
  expect(entries).toHaveLength(1);
  expect(entries[0]).toMatchObject({ status: 'complete', bytes: 8, total: 8 });
  expect(mockPrepare).toHaveBeenCalledTimes(1);
  expect(mockFetch.mock.calls[1][1]).toMatchObject({
    redirect: 'error',
    headers: {
      Authorization: 'Bearer viewer-a',
      'If-Unmodified-Since': modified,
      'Cache-Control': 'no-cache, no-store',
    },
  });
  mockFetch.mockClear();
  expect(await downloadedPlayback(viewer, routeItem.id)).toMatchObject({
    source: {
      uri: expect.stringMatching(
        /^file:\/\/\/Documents\/kinosail-offline\/[^/]+\/[^/]+\.media$/,
      ),
      headers: { Authorization: '' },
    },
  });
  expect(await downloadedPlayback(client('viewer-b'), routeItem.id)).toBeNull();
  expect(mockFetch).not.toHaveBeenCalled();
  expect([...mockFiles.keys()].join(' ')).not.toContain('viewer-a');
  await clearDownloads();
  expect(mockFiles.size).toBe(0);
});

it.each([
  '',
  'https://other.example/download/arrival',
  'https://kino.example/admin',
  'https://user:password@kino.example/download/arrival',
  'https://kino.example/download/arrival?redirect=other',
  'https://kino.example/download/arrival#fragment',
])(
  'rejects an unauthorized download destination before native requests or writes',
  async (download) => {
    const viewer = client();
    (viewer.loadPlayback as jest.Mock).mockResolvedValue({
      ...routeSource,
      details: { download },
    });
    await expect(downloadMedia(viewer, routeItem, jest.fn())).rejects.toThrow();
    expect(mockFetch).not.toHaveBeenCalled();
    expect(mockMutations).not.toHaveBeenCalled();
  },
);

it('rejects oversized downloads before creating files', async () => {
  mockFetch.mockResolvedValue({
    status: 200,
    headers: new Headers({
      'Content-Length': String(21 * 1024 ** 3),
      'Last-Modified': modified,
    }),
  });
  await expect(downloadMedia(client(), routeItem, jest.fn())).rejects.toThrow(
    'space',
  );
  expect(mockMutations).not.toHaveBeenCalled();
  expect(mockFetch).toHaveBeenCalledTimes(1);
});

it('preserves partial bytes when a server ignores the resume range, then resumes correctly', async () => {
  const viewer = client();
  const read = jest
    .fn()
    .mockResolvedValueOnce({ done: false, value: Buffer.from('abcd') })
    .mockRejectedValueOnce(new Error('connection lost'));
  mockFetch.mockResolvedValueOnce(head()).mockResolvedValueOnce({
    ...head(),
    body: {
      getReader: () => ({
        read,
        cancel: jest.fn().mockResolvedValue(undefined),
      }),
    },
  });
  await downloadMedia(viewer, routeItem, jest.fn());
  expect((await listDownloads(viewer))[0]).toMatchObject({
    status: 'paused',
    bytes: 4,
  });
  mockFetch.mockResolvedValueOnce(head()).mockResolvedValueOnce(body());
  await downloadMedia(viewer, routeItem, jest.fn());
  expect((await listDownloads(viewer))[0]).toMatchObject({
    status: 'paused',
    bytes: 4,
  });
  mockFetch
    .mockResolvedValueOnce(head())
    .mockResolvedValueOnce(body(206, 'efgh', 'bytes 4-7/8'));
  await downloadMedia(viewer, routeItem, jest.fn());
  expect((await listDownloads(viewer))[0]).toMatchObject({
    status: 'complete',
    bytes: 8,
  });
  expect(
    Buffer.from(
      [...mockFiles.entries()].find(([key]) => key.endsWith('.media'))![1],
    ).toString(),
  ).toBe('abcdefgh');
  expect(viewer.loadPlayback).toHaveBeenCalledTimes(3);
});

it('pauses when backgrounded and does not append an arriving chunk after cancellation', async () => {
  const read = jest.fn().mockImplementation(async () => {
    mockBackground('background');
    return { done: false, value: Buffer.from('abcd') };
  });
  mockFetch.mockResolvedValueOnce(head()).mockResolvedValueOnce({
    ...head(),
    body: {
      getReader: () => ({
        read,
        cancel: jest.fn().mockResolvedValue(undefined),
      }),
    },
  });
  const viewer = client();
  await downloadMedia(viewer, routeItem, jest.fn());
  expect((await listDownloads(viewer))[0]).toMatchObject({
    status: 'paused',
    bytes: 0,
    error: '',
  });
  expect(
    mockMutations.mock.calls.some(([operation]) => operation === 'append'),
  ).toBe(false);
});

it('persists offline progress without networking and rejects stale or out-of-range updates', async () => {
  const viewer = client();
  mockFetch.mockResolvedValueOnce(head()).mockResolvedValueOnce(body());
  await downloadMedia(viewer, routeItem, jest.fn());
  mockFetch.mockClear();
  const saved = await saveDownloadedProgress(viewer, routeItem.id, {
    seconds: 30,
    session: 'offline',
    revision: routeItem.progress.revision,
  });
  expect(saved).toMatchObject({
    seconds: 30,
    revision: routeItem.progress.revision,
  });
  for (const seconds of [-1, NaN, 121])
    await expect(
      saveDownloadedProgress(viewer, routeItem.id, {
        seconds,
        session: 'offline',
        revision: saved.revision,
      }),
    ).rejects.toThrow();
  await expect(
    saveDownloadedProgress(viewer, routeItem.id, {
      seconds: 20,
      session: 'offline',
      revision: routeItem.progress.revision,
    }),
  ).rejects.toThrow();
  expect((await listDownloads(viewer))[0].item.progress).toEqual(saved);
  expect(mockFetch).not.toHaveBeenCalled();
});

it.each(['', 'a\nb', 'a'.repeat(2049)])(
  'rejects invalid removal identifiers without side effects',
  async (id) => {
    await expect(removeDownload(client(), id)).rejects.toThrow();
    expect(mockMutations).not.toHaveBeenCalled();
  },
);

it('removes different titles concurrently without resurrecting either index entry', async () => {
  const viewer = client();
  (viewer.loadPlayback as jest.Mock).mockImplementation(async (id: string) => ({
    ...routeSource,
    details: { download: `https://kino.example/download/${id}` },
  }));
  for (const id of ['arrival', 'moon']) {
    mockFetch.mockResolvedValueOnce(head()).mockResolvedValueOnce(body());
    await downloadMedia(viewer, { ...routeItem, id }, jest.fn());
  }
  await Promise.all(
    ['arrival', 'moon'].map((id) => removeDownload(viewer, id)),
  );
  expect(await listDownloads(viewer)).toEqual([]);
  expect([...mockFiles.keys()].some((key) => key.endsWith('.media'))).toBe(
    false,
  );
});

describe('OS-owned background queue', () => {
  beforeEach(() => {
    mockNative.enabled = true;
    reloadDownloads();
  });
  it('hands multiple titles to native without streaming their bodies in JavaScript', async () => {
    const viewer = client();
    (viewer.loadPlayback as jest.Mock).mockImplementation(
      async (id: string) => ({
        ...routeSource,
        details: { download: `https://kino.example/download/${id}` },
      }),
    );
    mockFetch.mockResolvedValue(head());
    await Promise.all([
      downloadMedia(viewer, routeItem, jest.fn()),
      downloadMedia(
        viewer,
        { ...routeItem, id: 'moon', title: 'Moon' },
        jest.fn(),
      ),
    ]);
    expect(mockNative.enqueueDownload).toHaveBeenCalledTimes(2);
    expect(
      mockFetch.mock.calls.every((call) => call[1].method === 'HEAD'),
    ).toBe(true);
    const jobs = mockNative.enqueueDownload.mock.calls.map(([raw]: [string]) =>
      JSON.parse(raw),
    );
    expect(jobs[0]).toMatchObject({
      uri: 'https://kino.example/download/arrival',
      authorization: 'Bearer viewer-a',
      total: 8,
      modified,
    });
    expect(jobs[0].scope).toMatch(/^[a-f0-9]{64}$/);
    expect(jobs[0].key).not.toBe(jobs[1].key);
    expect(
      mockMutations.mock.calls.some(([operation]) => operation === 'append'),
    ).toBe(false);
  });
  it.each(['queued', 'waiting', 'downloading', 'pausing', 'complete'] as const)(
    'does not contact the Server again for a native %s download',
    async (status) => {
      const viewer = client();
      mockFetch.mockResolvedValue(head());
      await downloadMedia(viewer, routeItem, jest.fn());
      const job = JSON.parse(mockNative.enqueueDownload.mock.calls[0][0]);
      mockNative.downloadSnapshot.mockResolvedValue([
        {
          key: job.key,
          bytes: status === 'complete' ? 8 : 4,
          total: 8,
          status,
          error: '',
        },
      ]);
      mockFetch.mockClear();
      (viewer.loadMediaPreferences as jest.Mock)
        .mockClear()
        .mockRejectedValue(new Error('offline'));
      await downloadMedia(viewer, routeItem, jest.fn());
      expect(viewer.loadMediaPreferences).not.toHaveBeenCalled();
      expect(mockFetch).not.toHaveBeenCalled();
      expect(mockNative.enqueueDownload).toHaveBeenCalledTimes(1);
    },
  );
  it('queues on cellular while delegating Wi-Fi-only transfer enforcement to iOS', async () => {
    jest
      .requireMock('expo-network')
      .getNetworkStateAsync.mockResolvedValue({ type: 'cellular' });
    const viewer = client();
    mockFetch.mockResolvedValue(head());
    await downloadMedia(viewer, routeItem, jest.fn());
    expect(
      JSON.parse(mockNative.enqueueDownload.mock.calls[0][0]).wifiOnly,
    ).toBe(true);
    expect(
      mockFetch.mock.calls.every((call) => call[1].method === 'HEAD'),
    ).toBe(true);
  });
  it('persists preparation and hands only a finalized, version-pinned copy to iOS', async () => {
    const viewer = client();
    const job = {
      id: 'a'.repeat(16),
      itemId: routeItem.id,
      quality: '720p',
      state: 'preparing',
      size: 0,
      sha256: '',
    };
    viewer.prepareDownload = jest.fn().mockResolvedValue(job);
    viewer.loadPreparedDownload = jest.fn().mockResolvedValue({
      ...job,
      state: 'ready',
      size: 8,
      sha256: 'b'.repeat(64),
    });
    await downloadMedia(viewer, routeItem, jest.fn(), '720p');
    expect((await listDownloads(viewer))[0]).toMatchObject({
      status: 'preparing',
      quality: '720p',
      jobID: job.id,
      total: 0,
    });
    expect(mockNative.enqueueDownload).not.toHaveBeenCalled();
    expect(mockFetch).not.toHaveBeenCalled();
    mockFetch.mockResolvedValue({
      status: 200,
      headers: new Headers({
        'Content-Length': '8',
        'Last-Modified': modified,
        ETag: `"${'b'.repeat(64)}"`,
      }),
    });
    await downloadMedia(viewer, routeItem, jest.fn());
    expect(viewer.loadPreparedDownload).toHaveBeenCalledWith(
      job.id,
      expect.any(AbortSignal),
    );
    expect(
      JSON.parse(mockNative.enqueueDownload.mock.calls[0][0]),
    ).toMatchObject({
      uri: `https://kino.example/api/v1/downloads/${job.id}/file`,
      etag: `"${'b'.repeat(64)}"`,
      total: 8,
    });
    expect((await listDownloads(viewer))[0]).toMatchObject({
      quality: '720p',
      total: 8,
    });
  });
  it('can pause and remove preparation without starting a transfer', async () => {
    const viewer = client();
    viewer.prepareDownload = jest.fn().mockResolvedValue({
      id: 'a'.repeat(16),
      itemId: routeItem.id,
      quality: '1080p',
      state: 'preparing',
      size: 0,
      sha256: '',
    });
    await downloadMedia(viewer, routeItem, jest.fn(), '1080p');
    await pauseDownload(viewer, routeItem.id);
    expect((await listDownloads(viewer))[0]).toMatchObject({
      status: 'paused',
      total: 0,
    });
    expect(mockNative.pauseDownload).not.toHaveBeenCalled();
    await removeDownload(viewer, routeItem.id);
    expect(await listDownloads(viewer)).toEqual([]);
    expect(mockNative.enqueueDownload).not.toHaveBeenCalled();
  });
  it('rejects a changed prepared response before native enqueue or media writes', async () => {
    const viewer = client();
    viewer.prepareDownload = jest.fn().mockResolvedValue({
      id: 'a'.repeat(16),
      itemId: routeItem.id,
      quality: '720p',
      state: 'ready',
      size: 8,
      sha256: 'b'.repeat(64),
    });
    mockFetch.mockResolvedValue(head()); // Missing the pinned ETag.
    await expect(
      downloadMedia(viewer, routeItem, jest.fn(), '720p'),
    ).rejects.toThrow('changed');
    expect(mockNative.enqueueDownload).not.toHaveBeenCalled();
    expect(
      mockMutations.mock.calls.some(([operation]) => operation === 'append'),
    ).toBe(false);
  });
  it('rejects unsupported quality and conflicting existing quality before preparation', async () => {
    const viewer = client();
    viewer.prepareDownload = jest.fn();
    await expect(
      downloadMedia(viewer, routeItem, jest.fn(), '4k' as '720p'),
    ).rejects.toThrow();
    await expect(
      downloadMedia(
        viewer,
        { ...routeItem, kind: 'audiobook' },
        jest.fn(),
        '720p',
      ),
    ).rejects.toThrow();
    expect(mockMutations).not.toHaveBeenCalled();
    mockFetch.mockResolvedValue(head());
    await downloadMedia(viewer, routeItem, jest.fn());
    await expect(
      downloadMedia(viewer, routeItem, jest.fn(), '720p'),
    ).rejects.toThrow('different quality');
    expect(viewer.prepareDownload).not.toHaveBeenCalled();
  });
  it('recovers an interrupted enqueue as a resumable entry', async () => {
    const viewer = client();
    mockFetch.mockResolvedValue(head());
    await downloadMedia(viewer, routeItem, jest.fn());
    expect((await listDownloads(viewer))[0]).toMatchObject({
      status: 'paused',
      error: 'Download stopped. Resume to continue.',
    });
  });
  it('restores OS progress and targets pause/remove at only the requested title', async () => {
    const viewer = client();
    mockFetch.mockResolvedValue(head());
    await downloadMedia(viewer, routeItem, jest.fn());
    const job = JSON.parse(mockNative.enqueueDownload.mock.calls[0][0]);
    mockNative.downloadSnapshot.mockResolvedValue([
      { key: job.key, bytes: 4, total: 8, status: 'downloading', error: '' },
    ]);
    expect((await listDownloads(viewer))[0]).toMatchObject({
      bytes: 4,
      status: 'downloading',
    });
    await pauseDownload(viewer, routeItem.id);
    expect(mockNative.pauseDownload).toHaveBeenCalledWith(job.scope, job.key);
    await removeDownload(viewer, routeItem.id);
    expect(mockNative.removeDownload).toHaveBeenCalledWith(job.scope, job.key);
    expect(await listDownloads(viewer)).toEqual([]);
  });
  it('credits native partial bytes when preflighting a resume with limited free space', async () => {
    const viewer = client();
    const fs = jest.requireMock('expo-file-system');
    const gib = 1024 ** 3;
    mockFetch.mockResolvedValue({
      status: 200,
      headers: new Headers({
        'Content-Length': String(10 * gib),
        'Last-Modified': modified,
      }),
    });
    await downloadMedia(viewer, routeItem, jest.fn());
    const job = JSON.parse(mockNative.enqueueDownload.mock.calls[0][0]);
    mockNative.downloadSnapshot.mockResolvedValue([
      {
        key: job.key,
        bytes: 9 * gib,
        total: 10 * gib,
        status: 'paused',
        error: '',
      },
    ]);
    const originalSpace = fs.Paths.availableDiskSpace;
    try {
      fs.Paths.availableDiskSpace = 2 * gib;
      await downloadMedia(viewer, routeItem, jest.fn());
      expect(mockNative.enqueueDownload).toHaveBeenCalledTimes(2);
    } finally {
      fs.Paths.availableDiskSpace = originalSpace;
    }
  });
  it('does not trust mismatched or oversized native progress', async () => {
    const viewer = client();
    mockFetch.mockResolvedValue(head());
    await downloadMedia(viewer, routeItem, jest.fn());
    const job = JSON.parse(mockNative.enqueueDownload.mock.calls[0][0]);
    for (const transfer of [
      { key: job.key, bytes: 8, total: 9, status: 'complete', error: '' },
      { key: job.key, bytes: 9, total: 8, status: 'complete', error: '' },
      { key: job.key, bytes: 8, total: 8, status: 'unknown', error: '' },
    ]) {
      mockNative.downloadSnapshot.mockResolvedValue([transfer]);
      expect(await downloadedPlayback(viewer, routeItem.id)).toBeNull();
    }
  });
  it('cancels native transfers before removing the offline store on sign out', async () => {
    const viewer = client();
    mockFetch.mockResolvedValue(head());
    await downloadMedia(viewer, routeItem, jest.fn());
    mockNative.clearDownloads.mockImplementationOnce(async () => {
      expect(mockFiles.size).toBeGreaterThan(0);
    });
    await clearDownloads();
    expect(mockNative.clearDownloads).toHaveBeenCalledTimes(1);
    expect(mockFiles.size).toBe(0);
  });
  it.each([
    '',
    'https://other.example/download/arrival',
    'https://kino.example/admin',
    'https://user:password@kino.example/download/arrival',
    'https://kino.example/download/arrival?redirect=other',
    'https://kino.example/download/arrival#fragment',
  ])(
    'rejects invalid destination %s before native side effects',
    async (download) => {
      const viewer = client();
      (viewer.loadPlayback as jest.Mock).mockResolvedValue({
        ...routeSource,
        details: { download },
      });
      await expect(
        downloadMedia(viewer, routeItem, jest.fn()),
      ).rejects.toThrow();
      expect(mockNative.enqueueDownload).not.toHaveBeenCalled();
      expect(mockMutations).not.toHaveBeenCalled();
    },
  );
});

it.each([
  { quality: '4k' },
  { quality: '720p' },
  { jobID: '../file' },
  { status: 'preparing', total: 8 },
])(
  'rejects invalid saved download-quality metadata before any transfer: %p',
  async (patch) => {
    const viewer = client();
    mockFetch.mockResolvedValueOnce(head()).mockResolvedValueOnce(body());
    await downloadMedia(viewer, routeItem, jest.fn());
    const index = [...mockFiles.keys()].find((key) =>
      key.endsWith('/index.json'),
    )!;
    const entries = JSON.parse(Buffer.from(mockFiles.get(index)!).toString());
    mockFiles.set(
      index,
      Buffer.from(JSON.stringify([{ ...entries[0], ...patch }])),
    );
    mockMutations.mockClear();
    await expect(listDownloads(viewer)).rejects.toThrow();
    expect(mockMutations).not.toHaveBeenCalled();
  },
);

it.each([0, 250])(
  'queues a file larger than 20 GB with storage preference %s and preserves its index',
  async (downloadLimitGiB) => {
    mockNative.enabled = true;
    reloadDownloads();
    const viewer = client();
    jest
      .mocked(viewer.loadMediaPreferences)
      .mockResolvedValueOnce({ wifiOnly: true, downloadLimitGiB } as Awaited<
        ReturnType<typeof viewer.loadMediaPreferences>
      >);
    const total = 25 * 1024 ** 3;
    mockFetch.mockResolvedValueOnce({
      status: 200,
      headers: new Headers({
        'Content-Length': String(total),
        'Last-Modified': modified,
      }),
    });
    await downloadMedia(viewer, routeItem, jest.fn());
    expect(
      JSON.parse(mockNative.enqueueDownload.mock.calls[0][0]),
    ).toMatchObject({ total, quota: downloadLimitGiB * 1024 ** 3 });
    expect((await listDownloads(viewer))[0].total).toBe(total);
  },
);
it('rejects an unlimited download that would consume the free-space reserve before writing or enqueueing', async () => {
  mockNative.enabled = true;
  reloadDownloads();
  const viewer = client();
  jest
    .mocked(viewer.loadMediaPreferences)
    .mockResolvedValueOnce({ wifiOnly: true, downloadLimitGiB: 0 } as Awaited<
      ReturnType<typeof viewer.loadMediaPreferences>
    >);
  mockFetch.mockResolvedValueOnce({
    status: 200,
    headers: new Headers({
      'Content-Length': String(30 * 1024 ** 3),
      'Last-Modified': modified,
    }),
  });
  await expect(downloadMedia(viewer, routeItem, jest.fn())).rejects.toThrow(
    'space',
  );
  expect(mockMutations).not.toHaveBeenCalled();
  expect(mockPrepare).not.toHaveBeenCalled();
  expect(mockNative.enqueueDownload).not.toHaveBeenCalled();
});
it('reports physical storage and rejects unavailable or malformed disk facts', () => {
  expect(downloadStorage()).toEqual({
    total: 256 * 1024 ** 3,
    free: 30 * 1024 ** 3,
  });
  for (const [totalDiskSpace, availableDiskSpace] of [
    [0, 0],
    [NaN, 1],
    [100, -1],
    [100, 101],
    [100, Infinity],
  ]) {
    Object.assign(jest.requireMock('expo-file-system').Paths, {
      totalDiskSpace,
      availableDiskSpace,
    });
    expect(downloadStorage()).toBeNull();
  }
});

describe('verified native preparation', () => {
  const preparation = {
    id: 'a'.repeat(16),
    itemId: routeItem.id,
    quality: 'original',
    state: 'preparing',
    size: 0,
    sha256: '',
  };
  async function prepare() {
    mockVerified.enabled = true;
    reloadDownloads();
    const viewer = client();
    viewer.prepareDownload = jest.fn().mockResolvedValue(preparation);
    await downloadMedia(viewer, routeItem, jest.fn());
    return viewer;
  }
  it('hands original preparation to the OS and pauses the OS job before returning', async () => {
    const viewer = await prepare();
    expect(mockVerified.enqueuePreparingDownload).toHaveBeenCalledTimes(1);
    expect(mockFetch).not.toHaveBeenCalled();
    await pauseDownload(viewer, routeItem.id);
    expect(mockVerified.pauseVerifiedDownload).toHaveBeenCalledTimes(1);
    expect((await listDownloads(viewer))[0].status).toBe('paused');
  });
  it('does not resurrect a removed title when an older native snapshot returns', async () => {
    const viewer = await prepare();
    let release!: (value: unknown[]) => void;
    mockVerified.verifiedDownloadSnapshot.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          release = resolve;
        }),
    );
    const listing = listDownloads(viewer);
    while (!release) await Promise.resolve();
    await removeDownload(viewer, routeItem.id);
    release([]);
    expect(await listing).toEqual([]);
    expect(await listDownloads(viewer)).toEqual([]);
  });
  it('rejects a manifest for another prepared revision without changing the index', async () => {
    const viewer = await prepare();
    const plan = JSON.parse(
      mockVerified.enqueuePreparingDownload.mock.calls.at(-1)[0],
    );
    mockVerified.verifiedDownloadSnapshot.mockResolvedValue([
      {
        key: plan.key,
        total: 1,
        bytes: 1,
        status: 'complete',
        error: '',
        manifest: {
          version: 1,
          id: 'b'.repeat(16),
          size: 1,
          sha256: 'c'.repeat(64),
          chunkSize: 8388608,
          chunks: ['d'.repeat(64)],
        },
      },
    ]);
    const before = [...mockFiles.entries()].map(([key, value]) => [
      key,
      [...value],
    ]);
    await expect(listDownloads(viewer)).rejects.toThrow();
    expect(
      [...mockFiles.entries()].map(([key, value]) => [key, [...value]]),
    ).toEqual(before);
  });
});
