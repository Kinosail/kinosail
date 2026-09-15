import { parseMediaItem, type InputValue, type MediaItem } from './contract';

export type Album = {
  id: string;
  title: string;
  artist: string;
  artwork: string;
};
const invalid = () => new Error('Kinosail Server returned invalid music data.');
export function musicID(value: unknown): string {
  if (typeof value !== 'string' || !/^[a-zA-Z0-9_-]{1,128}$/.test(value))
    throw invalid();
  return value;
}
function object(value: InputValue, keys: string[]) {
  if (
    !value ||
    typeof value !== 'object' ||
    Array.isArray(value) ||
    Object.keys(value).some((key) => !keys.includes(key))
  )
    throw invalid();
  return value;
}
function text(value: InputValue, limit: number, required = false): string {
  if (value === undefined && !required) return '';
  if (
    typeof value !== 'string' ||
    value.length > limit ||
    (required && !value.trim()) ||
    /[\u0000-\u001f\u007f\ud800-\udfff]/u.test(value)
  )
    throw invalid();
  return value;
}
function unique<T extends { id: string }>(items: T[]) {
  if (new Set(items.map((item) => item.id)).size !== items.length)
    throw invalid();
  return items;
}
export function parseAlbums(value: InputValue): Album[] {
  const input = object(value, ['albums']);
  if (!Array.isArray(input.albums) || input.albums.length > 10000)
    throw invalid();
  return unique(
    input.albums.map((value) => {
      const album = object(value, ['id', 'title', 'artist', 'artwork']);
      return {
        id: musicID(album.id),
        title: text(album.title, 512, true),
        artist: text(album.artist, 256),
        artwork: text(album.artwork, 2048),
      };
    }),
  );
}
export function parseMusicTracks(value: InputValue): MediaItem[] {
  if (!Array.isArray(value) || value.length > 10000) throw invalid();
  return unique(
    value.map((entry) => {
      const item = parseMediaItem(entry);
      musicID(item.id);
      if (item.kind !== 'music') throw invalid();
      return item;
    }),
  );
}
export function parseAlbum(value: InputValue, id: string) {
  const input = object(value, ['id', 'title', 'artist', 'tracks']);
  if (musicID(input.id) !== musicID(id)) throw invalid();
  return {
    id,
    title: text(input.title, 512, true),
    artist: text(input.artist, 256),
    tracks: parseMusicTracks(input.tracks),
  };
}
export function parseMusicQueue(value: InputValue, id: string) {
  const items = parseMusicTracks(object(value, ['items']).items);
  if (!items.length || items[0].id !== musicID(id)) throw invalid();
  return items;
}
