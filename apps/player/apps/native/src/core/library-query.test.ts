import { libraryQuery, type LibraryQuery } from './library-query';
import { KinosailClient } from './server-client';
import { parseLibraryPage } from './contract';

it('normalizes search once and encodes delimiters without adding query fields', () => {
  expect(
    libraryQuery({
      q: '  A&B / ?  ',
      view: 'movies',
      sort: 'year',
      offset: 60,
    }),
  ).toBe('q=A%26B%20%2F%20%3F&view=movies&sort=year&offset=60&limit=60');
});
it.each([
  null,
  [],
  { unknown: 1 },
  { q: 1 },
  { q: 'a'.repeat(513) },
  { q: 'é'.repeat(257) },
  { q: '\ud800' },
  { q: 'a\nb' },
  { view: 'secret' },
  { sort: 'size' },
  { offset: -1 },
  { offset: 0.5 },
  { offset: NaN },
  { offset: 1_000_001 },
])('rejects invalid browse input before requesting data: %p', async (value) => {
  const fetcher = jest.fn();
  const client = new KinosailClient('https://kino.example', 'viewer', fetcher);
  await expect(client.browseLibrary(value as LibraryQuery)).rejects.toThrow(
    'invalid',
  );
  expect(fetcher).not.toHaveBeenCalled();
});
it('preserves a cancellation signal and rejects foreign artwork', async () => {
  const fetcher = jest.fn().mockResolvedValue(
    new Response(
      JSON.stringify({
        items: [
          {
            id: 'a',
            kind: 'video',
            title: 'A',
            artwork: 'https://other.example/a',
          },
        ],
        total: 1,
        offset: 0,
        limit: 60,
      }),
    ),
  );
  const signal = new AbortController().signal;
  await expect(
    new KinosailClient('https://kino.example', 'viewer', fetcher).browseLibrary(
      {},
      signal,
    ),
  ).rejects.toThrow('invalid media URL');
  expect(fetcher.mock.calls[0][1].signal).toBe(signal);
});
it.each([
  { items: [], total: 0, offset: 0, limit: 0 },
  { items: [], total: -1, offset: 0, limit: 60 },
  { items: [], total: 0, offset: 1_000_001, limit: 60 },
  { items: [], total: 0, offset: 0, limit: 201 },
  {
    items: [{ id: 'a', kind: 'video', title: 'A' }],
    total: 0,
    offset: 0,
    limit: 60,
  },
])('rejects inconsistent pagination metadata %p', (value) =>
  expect(() => parseLibraryPage(value)).toThrow(),
);

it('preserves whole-library jump offsets beyond the loaded page', () => {
  const letters = [
    { label: 'A', offset: 0, count: 207 },
    { label: 'Z', offset: 207, count: 8 },
  ];
  expect(
    parseLibraryPage({ items: [], total: 215, offset: 0, limit: 60, letters })
      .letters,
  ).toEqual(letters);
  expect(libraryQuery({ view: 'movies', offset: 207 })).toContain('offset=207');
});
it.each([
  'A',
  [null],
  [{ label: '', offset: 0, count: 2 }],
  [{ label: '123', offset: 0, count: 2 }],
  [{ label: '\u0301', offset: 0, count: 2 }],
  [{ label: 'ABCDE', offset: 0, count: 2 }],
  [{ label: 'A', count: 2 }],
  [{ label: 'A', offset: 0 }],
  [{ label: 'A', offset: 0, count: 2, href: '/elsewhere' }],
  [{ label: 'A', offset: -1, count: 2 }],
  [{ label: 'A', offset: 0, count: 0 }],
  [{ label: 'A', offset: 0, count: 3 }],
  [{ label: 'A', offset: 0, count: 1 }],
  [
    { label: 'A', offset: 0, count: 1 },
    { label: 'A', offset: 1, count: 1 },
  ],
  [
    { label: 'A', offset: 0, count: 1 },
    { label: 'B', offset: 0, count: 1 },
  ],
  Array(4097).fill({ label: 'A', offset: 0, count: 2 }),
])('rejects malformed alphabet metadata: %p', (letters) => {
  expect(() =>
    parseLibraryPage({ items: [], total: 2, offset: 0, limit: 60, letters }),
  ).toThrow('invalid');
});
it('accepts numeric and international title groups and servers without an index', () => {
  const letters = [
    { label: '#', offset: 0, count: 1 },
    { label: 'É', offset: 1, count: 1 },
  ];
  expect(
    parseLibraryPage({ items: [], total: 2, offset: 0, limit: 60, letters })
      .letters,
  ).toEqual(letters);
  expect(
    parseLibraryPage({ items: [], total: 0, offset: 0, limit: 60 }).letters,
  ).toEqual([]);
});

it.each(['photos', 'music', 'audiobooks', 'books', 'list', 'history'] as const)(
  'uses the Server library contract for %s',
  (view) => {
    expect(libraryQuery({ view })).toContain(`view=${view}&`);
  },
);
