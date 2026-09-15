import {
  parseDownloadQuality,
  parsePreparedDownload,
} from './download-quality';
import { KinosailClient } from './server-client';

const ready = {
  id: 'a'.repeat(16),
  itemId: 'arrival',
  profileId: 'viewer',
  title: 'Arrival',
  quality: '720p',
  state: 'ready',
  size: 8,
  sha256: 'b'.repeat(64),
  readyOffline: true,
  extension: '.mp4',
  created: '2026-09-08T12:00:00Z',
};
it('accepts only supported quality choices and valid preparation states', () => {
  for (const quality of ['original', '1080p', '720p'])
    expect(parseDownloadQuality(quality)).toBe(quality);
  expect(parsePreparedDownload(ready).size).toBe(8);
  expect(
    parsePreparedDownload({
      ...ready,
      state: 'preparing',
      size: 0,
      sha256: '',
      readyOffline: false,
    }).state,
  ).toBe('preparing');
});
it.each([
  { id: '../file' },
  { itemId: '' },
  { profileId: '' },
  { quality: '4k' },
  { state: 'downloading' },
  { size: -1 },
  { size: Number.MAX_SAFE_INTEGER + 1 },
  { size: 0 },
  { size: 1.2 },
  { sha256: 'bad' },
  { readyOffline: false },
  { url: 'https://other.example' },
  { title: 'x'.repeat(513) },
  { error: 'x'.repeat(1001) },
  { created: 'bad' },
  { extension: '/../mp4' },
  { state: 'preparing' },
  { state: 'failed' },
])(
  'rejects malformed, oversized, unknown and conflicting manifest values %p',
  (patch) => {
    expect(() => parsePreparedDownload({ ...ready, ...patch })).toThrow();
  },
);
it('rejects missing required fields', () => {
  for (const key of [
    'id',
    'itemId',
    'profileId',
    'title',
    'quality',
    'state',
    'created',
    'readyOffline',
  ]) {
    const value: Record<string, unknown> = { ...ready };
    delete value[key];
    expect(() => parsePreparedDownload(value)).toThrow();
  }
});
it('validates requests before sending and checks the returned item and job identity', async () => {
  const fetcher = jest
    .fn()
    .mockResolvedValue(new Response(JSON.stringify(ready), { status: 202 }));
  const client = new KinosailClient('https://kino.example', 'token', fetcher);
  await expect(client.prepareDownload('', '720p')).rejects.toThrow();
  await expect(
    client.prepareDownload('arrival', '4k' as '720p'),
  ).rejects.toThrow();
  await expect(client.loadPreparedDownload('../file')).rejects.toThrow();
  expect(fetcher).not.toHaveBeenCalled();
  await client.prepareDownload('arrival', '720p');
  expect(fetcher).toHaveBeenCalledWith(
    'https://kino.example/api/v1/items/arrival/downloads',
    expect.objectContaining({
      method: 'POST',
      body: '{"quality":"720p"}',
      redirect: 'error',
    }),
  );
  fetcher.mockResolvedValue(
    new Response(JSON.stringify({ ...ready, itemId: 'other' }), {
      status: 202,
    }),
  );
  await expect(client.prepareDownload('arrival', '720p')).rejects.toThrow(
    'different',
  );
  fetcher.mockResolvedValue(
    new Response(JSON.stringify(ready), { status: 200 }),
  );
  await expect(client.loadPreparedDownload('c'.repeat(16))).rejects.toThrow(
    'different',
  );
});

it('accepts prepared files larger than the old 20 GB budget', () => {
  expect(parsePreparedDownload({ ...ready, size: 100 * 1024 ** 3 }).size).toBe(
    100 * 1024 ** 3,
  );
});
