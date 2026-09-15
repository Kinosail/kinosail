import { createBrowseCache } from './browse-cache';
import type { KinosailClient } from './server-client';

const client = () => ({}) as KinosailClient;
afterEach(() => jest.restoreAllMocks());

it('returns saved results synchronously and isolates clients and queries', () => {
  const cache = createBrowseCache<string>(2);
  const one = client(),
    two = client();
  cache.write(one, 'movies', 'saved movies');
  expect(cache.read(one, 'movies')).toBe('saved movies');
  expect(cache.read(one, 'books')).toBeNull();
  expect(cache.read(two, 'movies')).toBeNull();
});

it('expires snapshots at thirty minutes without extending them on reads', () => {
  const now = jest.spyOn(Date, 'now').mockReturnValue(0);
  const cache = createBrowseCache<string>(2),
    one = client();
  cache.write(one, 'home', 'saved');
  now.mockReturnValue(30 * 60_000 - 1);
  expect(cache.read(one, 'home')).toBe('saved');
  now.mockReturnValue(30 * 60_000);
  expect(cache.read(one, 'home')).toBeNull();
});

it('bounds retained pages and replaces refreshed results', () => {
  const cache = createBrowseCache<string>(2),
    one = client();
  cache.write(one, 'first', 'old');
  cache.write(one, 'second', 'second');
  cache.write(one, 'first', 'fresh');
  cache.write(one, 'third', 'third');
  expect(cache.read(one, 'first')).toBe('fresh');
  expect(cache.read(one, 'second')).toBeNull();
  expect(cache.read(one, 'third')).toBe('third');
});

it('clears a failed client without affecting another session', () => {
  const cache = createBrowseCache<string>(2),
    one = client(),
    two = client();
  cache.write(one, 'home', 'one');
  cache.write(two, 'home', 'two');
  cache.clear(one);
  expect(cache.read(one, 'home')).toBeNull();
  expect(cache.read(two, 'home')).toBe('two');
});
