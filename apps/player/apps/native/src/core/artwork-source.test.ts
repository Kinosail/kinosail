import { artworkSource } from './artwork-source';
it('isolates protected artwork between sessions without embedding credentials', () => {
  const first = { Authorization: 'Bearer first-private-token' },
    second = { Authorization: 'Bearer second-private-token' };
  const one = artworkSource('/art/a', first),
    same = artworkSource('/art/a', first),
    other = artworkSource('/art/a', second);
  expect(one.cacheKey).toBe(same.cacheKey);
  expect(one.cacheKey).not.toBe(other.cacheKey);
  expect(one.cacheKey).not.toContain('private-token');
  expect(one.headers).toBe(first);
});
