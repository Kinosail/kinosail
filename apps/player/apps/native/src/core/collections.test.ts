import {
  collectionName,
  parseCollection,
  parseCollections,
} from './collections';
import { KinosailClient } from './server-client';

it.each([
  undefined,
  null,
  [],
  7,
  '',
  ' ',
  '.',
  '..',
  'a/b',
  'a\\b',
  'a\nb',
  '\ud800',
  'x'.repeat(65),
  'é'.repeat(33),
])('rejects invalid collection names before any request: %p', async (name) => {
  const fetcher = jest.fn();
  const client = new KinosailClient('https://kino.example', 'viewer', fetcher);
  await expect(client.loadCollection(name as string)).rejects.toThrow(
    'invalid',
  );
  expect(fetcher).not.toHaveBeenCalled();
});
it('preserves names and safely encodes URL delimiters', async () => {
  const name = 'Family & friends?#';
  expect(collectionName(name)).toBe(name);
  const fetcher = jest
    .fn()
    .mockResolvedValue(new Response(JSON.stringify({ name, items: [] })));
  const signal = new AbortController().signal;
  expect(
    await new KinosailClient(
      'https://kino.example',
      'viewer',
      fetcher,
    ).loadCollection(name, signal),
  ).toEqual([]);
  expect(fetcher.mock.calls[0][0]).toBe(
    `https://kino.example/api/v1/collections/${encodeURIComponent(name)}`,
  );
  expect(fetcher.mock.calls[0][1].signal).toBe(signal);
});
it.each([
  null,
  [],
  {},
  { collections: 'A' },
  { collections: ['A', 'A'] },
  { collections: Array(10001).fill('A') },
])('rejects malformed collection lists: %p', (value) =>
  expect(() => parseCollections(value)).toThrow(),
);
it('accepts empty collections and collections larger than a library page', () => {
  expect(parseCollections({ collections: [] })).toEqual([]);
  expect(parseCollection({ name: 'A', items: [] }, 'A')).toEqual([]);
  const items = Array.from({ length: 201 }, (_, id) => ({
    id: String(id),
    kind: 'photo',
    title: 'Photo',
  }));
  expect(parseCollection({ name: 'A', items }, 'A')).toHaveLength(201);
});
it.each([
  { name: 'B', items: [] },
  { name: 'A', items: null },
  { name: 'A', items: [null] },
  { name: 'A', items: Array(10001).fill({}) },
  { name: 'A', items: Array(2).fill({ id: 'a', kind: 'photo', title: 'A' }) },
])('rejects inconsistent collection responses: %p', (value) => {
  expect(() => parseCollection(value, 'A')).toThrow();
});
it('rejects foreign collection artwork before displaying it', async () => {
  const fetcher = jest.fn().mockResolvedValue(
    new Response(
      JSON.stringify({
        name: 'A',
        items: [
          {
            id: 'a',
            kind: 'photo',
            title: 'A',
            artwork: 'https://other.example/a',
          },
        ],
      }),
    ),
  );
  await expect(
    new KinosailClient(
      'https://kino.example',
      'viewer',
      fetcher,
    ).loadCollection('A'),
  ).rejects.toThrow('invalid media URL');
});
