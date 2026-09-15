import { KinosailClient } from './server-client';
import {
  parseAlbums,
  parseAlbum,
  parseMusicQueue,
  musicID,
} from './music-catalog';
import { routeItem } from '@/testing/route-fixtures';
import { fetchMock, response } from '@/testing/server-client-test-helpers';
const track = {
  ...routeItem,
  id: 'song-1',
  kind: 'audio',
  title: 'First song',
};
const album = {
  id: 'album-1',
  title: 'First album',
  artist: 'Artist',
  artwork: '/art/song-1',
};
it('preserves album identity and server track order', () => {
  expect(parseAlbums({ albums: [album] })).toEqual([album]);
  const result = parseAlbum(
    {
      id: album.id,
      title: album.title,
      artist: album.artist,
      tracks: [track, { ...track, id: 'song-2' }],
    },
    album.id,
  );
  expect(result.tracks.map((item) => item.id)).toEqual(['song-1', 'song-2']);
});
it.each([
  null,
  {},
  { albums: null },
  { albums: [], extra: true },
  { albums: [album, album] },
  { albums: [{ ...album, id: '../album' }] },
  { albums: [{ ...album, title: '' }] },
  { albums: [{ ...album, title: 'x'.repeat(513) }] },
  { albums: [{ ...album, artist: '\n' }] },
  { albums: Array(10001).fill(album) },
  { albums: [{ ...album, unsupported: true }] },
])('rejects malformed album data %#', (value) => {
  expect(() => parseAlbums(value)).toThrow();
});
it.each([
  undefined,
  null,
  '',
  '.',
  '..',
  '../song',
  'a/b',
  'a?b',
  'a\n',
  'x'.repeat(129),
  ['song'],
])('rejects an invalid music ID before fetching %#', async (id) => {
  expect(() => musicID(id)).toThrow();
  const fetcher = fetchMock();
  const client = new KinosailClient('https://kino.example', '', fetcher);
  await expect(client.loadAlbum(id as string)).rejects.toThrow();
  await expect(client.loadMusicQueue(id as string)).rejects.toThrow();
  expect(fetcher).not.toHaveBeenCalled();
});
it.each(
  [
    [],
    [track, track],
    [{ ...track, kind: 'video' }],
    [{ ...track, id: 'other' }],
    Array(10001).fill(track),
  ].map((items) => [items]),
)('rejects an invalid playback queue %#', (items) => {
  expect(() => parseMusicQueue({ items }, track.id)).toThrow();
});
it('rejects an album response for another album', () => {
  expect(() =>
    parseAlbum({ id: 'other', title: 'Other', tracks: [track] }, album.id),
  ).toThrow();
});
it('uses the authenticated catalog and validates artwork before returning it', async () => {
  const fetcher = fetchMock().mockResolvedValue(
    response(200, { albums: [album] }),
  );
  const client = new KinosailClient(
    'https://kino.example',
    'viewer-token',
    fetcher,
  );
  await expect(client.loadAlbums()).resolves.toEqual([album]);
  expect(fetcher).toHaveBeenCalledWith(
    'https://kino.example/api/v1/albums',
    expect.objectContaining({
      headers: expect.objectContaining({
        Authorization: 'Bearer viewer-token',
      }),
    }),
  );
  fetcher.mockResolvedValue(
    response(200, {
      albums: [{ ...album, artwork: 'https://other.example/art' }],
    }),
  );
  await expect(client.loadAlbums()).rejects.toThrow('invalid media URL');
});

it('normalizes the server audio kind for the native music player', () => {
  expect(parseMusicQueue({ items: [track] }, track.id)[0].kind).toBe('music');
});
