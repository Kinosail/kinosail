import { maintainDownloads, setPlayingDownload } from './smart-downloads';
import type { KinosailClient } from './server-client';
const mockList = jest.fn(),
  mockDownload = jest.fn(),
  mockRemove = jest.fn();
let mockNetwork: { isConnected: boolean; type?: string } = {
  isConnected: true,
  type: 'WIFI',
};
let mockPending: { id: string }[] = [];
jest.mock('expo-network', () => ({
  NetworkStateType: { WIFI: 'WIFI', ETHERNET: 'ETHERNET' },
  getNetworkStateAsync: async () => mockNetwork,
}));
jest.mock('./downloads', () => ({
  downloadsAvailable: true,
  listDownloads: (...args: unknown[]) => mockList(...args),
  downloadMedia: (...args: unknown[]) => mockDownload(...args),
  removeDownload: (...args: unknown[]) => mockRemove(...args),
}));
jest.mock('./progress-sync', () => ({
  withPendingProgress: async (
    _client: unknown,
    run: (entries: typeof mockPending) => Promise<void>,
  ) => run(mockPending),
}));
function client(overrides = {}) {
  return {
    loadMediaPreferences: jest.fn().mockResolvedValue({
      autoDownloadNext: 2,
      removeWatched: false,
      wifiOnly: true,
      ...overrides,
    }),
    loadItem: jest.fn(async (id: string) => ({
      id,
      show: 'Show',
      progress: { watched: false },
    })),
    loadPlayback: jest.fn(async (id: string) => ({
      details: {
        downloadNext: id === 'one' ? 'two' : id === 'two' ? 'three' : null,
      },
    })),
  } as unknown as KinosailClient;
}
beforeEach(() => {
  jest.clearAllMocks();
  mockNetwork = { isConnected: true, type: 'WIFI' };
  mockPending = [];
  setPlayingDownload('');
  mockList.mockResolvedValue([{ item: { id: 'one' }, status: 'complete' }]);
});
it('downloads the bounded next episodes even when autoplay has no next item', async () => {
  await maintainDownloads(client());
  expect(mockDownload.mock.calls.map((call) => call[1].id)).toEqual([
    'two',
    'three',
  ]);
});
it.each([
  { isConnected: false, type: 'WIFI' },
  { isConnected: true, type: 'CELLULAR' },
  { isConnected: true, type: undefined },
])('does no download work on disallowed networks: %p', async (network) => {
  mockNetwork = network;
  await maintainDownloads(client());
  expect(mockList).not.toHaveBeenCalled();
  expect(mockDownload).not.toHaveBeenCalled();
  expect(mockRemove).not.toHaveBeenCalled();
});
it('preserves watched files while playing or awaiting progress sync', async () => {
  const api = client({ removeWatched: true, autoDownloadNext: 0 });
  (api.loadItem as jest.Mock).mockResolvedValue({
    id: 'one',
    progress: { watched: true },
  });
  setPlayingDownload('one');
  await maintainDownloads(api);
  expect(mockRemove).not.toHaveBeenCalled();
  setPlayingDownload('');
  mockPending = [{ id: 'one' }];
  await maintainDownloads(api);
  expect(mockRemove).not.toHaveBeenCalled();
  mockPending = [];
  await maintainDownloads(api);
  expect(mockRemove).toHaveBeenCalledWith(api, 'one', expect.any(Function));
});
it('never follows a next episode into another show', async () => {
  const api = client();
  (api.loadItem as jest.Mock).mockImplementation(async (id) => ({
    id,
    show: id === 'one' ? 'Show' : 'Other',
    progress: { watched: false },
  }));
  await maintainDownloads(api);
  expect(mockDownload).not.toHaveBeenCalled();
});

it('keeps the chosen quality when automatically downloading upcoming episodes', async () => {
  mockList.mockResolvedValue([
    { item: { id: 'one' }, status: 'complete', quality: '720p' },
  ]);
  await maintainDownloads(client());
  expect(mockDownload.mock.calls.map((call) => call[3])).toEqual([
    '720p',
    '720p',
  ]);
});
