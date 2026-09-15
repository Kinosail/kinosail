import { downloadBatch, selectDownloadEpisodes } from './download-batch';
import { parseShowEpisodes, parseMediaItem, type MediaItem } from './contract';
import { downloadLimit } from './download-policy';
import { downloadMedia, listDownloads } from './downloads';
import { KinosailClient } from './server-client';
import { routeItem } from '@/testing/route-fixtures';

jest.mock('./downloads', () => ({
  downloadMedia: jest.fn(),
  listDownloads: jest.fn(),
}));
const showId = 'a'.repeat(16);
const episode = (id: string, season = 1, number = 1): MediaItem => ({
  ...routeItem,
  id,
  show: 'Example',
  showId,
  season,
  episode: number,
});
const episodes = [episode('three', 2), episode('two', 1, 2), episode('one')];
const client = new KinosailClient('https://kino.example', 'token');
beforeEach(() => {
  jest.clearAllMocks();
  jest.mocked(listDownloads).mockResolvedValue([]);
  jest.mocked(downloadMedia).mockResolvedValue(undefined);
});
it('selects a complete season or show in episode order, including specials', () => {
  expect(
    selectDownloadEpisodes(episodes[2], episodes, 'season').map(
      (item) => item.id,
    ),
  ).toEqual(['one', 'two']);
  expect(
    selectDownloadEpisodes(episodes[2], episodes, 'show').map(
      (item) => item.id,
    ),
  ).toEqual(['one', 'two', 'three']);
  const special = episode('special', 0);
  expect(
    selectDownloadEpisodes(special, [special, ...episodes], 'season'),
  ).toEqual([special]);
  expect(selectDownloadEpisodes(routeItem, [], 'episode')).toEqual([routeItem]);
  expect(() => selectDownloadEpisodes(episodes[2], [], 'season')).toThrow();
  expect(() =>
    selectDownloadEpisodes(episodes[2], episodes, 'invalid' as 'season'),
  ).toThrow();
});
it('uses one quality and skips existing downloads without changing their quality', async () => {
  jest
    .mocked(listDownloads)
    .mockResolvedValue([{ item: episodes[1], quality: 'original' }] as Awaited<
      ReturnType<typeof listDownloads>
    >);
  const progress = jest.fn(),
    change = jest.fn();
  await downloadBatch(client, episodes, '720p', change, progress);
  expect(downloadMedia).toHaveBeenCalledTimes(2);
  expect(downloadMedia).toHaveBeenNthCalledWith(
    1,
    client,
    episodes[0],
    change,
    '720p',
  );
  expect(progress).toHaveBeenLastCalledWith(2, 2);
});
it('reports partial failure and a retry skips the episodes already added', async () => {
  jest
    .mocked(downloadMedia)
    .mockResolvedValueOnce(undefined)
    .mockRejectedValueOnce(new Error('Not enough space'));
  await expect(
    downloadBatch(client, episodes, '1080p', jest.fn(), jest.fn()),
  ).rejects.toThrow('1 of 3 added');
  jest
    .mocked(listDownloads)
    .mockResolvedValue([{ item: episodes[0] }] as Awaited<
      ReturnType<typeof listDownloads>
    >);
  jest.mocked(downloadMedia).mockClear().mockResolvedValue(undefined);
  await downloadBatch(client, episodes, '1080p', jest.fn(), jest.fn());
  expect(downloadMedia).toHaveBeenCalledTimes(2);
});
it('stops adding when cancelled and never starts the remaining episodes', async () => {
  let stopped = false;
  jest.mocked(downloadMedia).mockImplementation(async () => {
    stopped = true;
  });
  await expect(
    downloadBatch(
      client,
      episodes,
      'original',
      jest.fn(),
      jest.fn(),
      () => stopped,
    ),
  ).rejects.toThrow('Stopped adding');
  expect(downloadMedia).toHaveBeenCalledTimes(1);
});
it.each(
  [
    [],
    [episode('')],
    [episode('x'.repeat(129))],
    [episode('bad\n')],
    [episodes[0], episodes[0]],
    [{ ...episode('one'), season: -1 }],
    [{ ...episode('one'), kind: 'unknown' }],
    Array(downloadLimit + 1).fill(episode('one')),
  ].map((items) => [items]),
)('rejects invalid batches before any side effects: %p', async (items) => {
  await expect(
    downloadBatch(client, items, '720p', jest.fn(), jest.fn()),
  ).rejects.toThrow();
  expect(listDownloads).not.toHaveBeenCalled();
  expect(downloadMedia).not.toHaveBeenCalled();
});
it('preflights capacity before adding any title and allows shows over 50 episodes', async () => {
  const items = Array.from({ length: 60 }, (_, index) =>
    episode(`episode-${index}`),
  );
  await downloadBatch(client, items, '720p', jest.fn(), jest.fn());
  expect(downloadMedia).toHaveBeenCalledTimes(60);
  jest.mocked(downloadMedia).mockClear();
  jest.mocked(listDownloads).mockResolvedValue(
    Array.from({ length: downloadLimit }, (_, index) => ({
      item: episode(`saved-${index}`),
    })) as Awaited<ReturnType<typeof listDownloads>>,
  );
  await expect(
    downloadBatch(client, episodes, '720p', jest.fn(), jest.fn()),
  ).rejects.toThrow('room for 0');
  expect(downloadMedia).not.toHaveBeenCalled();
});
it('validates quality before reading or changing downloads', async () => {
  await expect(
    downloadBatch(client, episodes, '4k' as '720p', jest.fn(), jest.fn()),
  ).rejects.toThrow();
  expect(listDownloads).not.toHaveBeenCalled();
});
it('loads and validates the authorized show API without truncating a large show', async () => {
  const fetcher = jest
    .fn()
    .mockResolvedValue(new Response(JSON.stringify({ id: showId, episodes })));
  const viewer = new KinosailClient('https://kino.example', 'token', fetcher);
  await expect(viewer.loadShowEpisodes('../bad')).rejects.toThrow();
  expect(fetcher).not.toHaveBeenCalled();
  expect(
    (await viewer.loadShowEpisodes(showId)).map((item) => item.id),
  ).toEqual(episodes.map((item) => item.id));
  expect(fetcher).toHaveBeenCalledWith(
    `https://kino.example/api/v1/shows/${showId}`,
    expect.objectContaining({
      headers: expect.objectContaining({ Authorization: 'Bearer token' }),
    }),
  );
  const many = Array.from({ length: 201 }, (_, index) =>
    episode(`episode-${index}`),
  );
  expect(
    parseShowEpisodes({ id: showId, episodes: many }, showId),
  ).toHaveLength(201);
});
it.each([
  {},
  { id: 'other', episodes },
  { id: showId, episodes: [] },
  { id: showId, episodes: [episodes[0], episodes[0]] },
  { id: showId, episodes: [{ ...episodes[0], showId: 'b'.repeat(16) }] },
  { id: showId, episodes: Array(downloadLimit + 1).fill(episodes[0]) },
])('rejects missing, conflicting and oversized show responses: %p', (value) => {
  expect(() => parseShowEpisodes(value, showId)).toThrow();
});
it.each([
  { size: -1 },
  { size: 1.2 },
  { size: Number.MAX_SAFE_INTEGER + 1 },
  { showId: 'invalid' },
  { showId: '' },
])('rejects invalid size and show identity %p', (patch) => {
  expect(() => parseMediaItem({ ...routeItem, ...patch })).toThrow();
});
