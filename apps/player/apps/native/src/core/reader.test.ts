import {
  parseReader,
  readerResource,
  parseReaderProgress,
  readerPositionEvent,
} from './reader';
describe('reader boundary', () => {
  const book = {
    id: 'book1',
    title: 'Book',
    type: 'epub',
    pages: [
      {
        title: 'Chapter',
        number: 1,
        url: '/read/book1/asset/OPS/chapter.xhtml',
      },
    ],
  };
  it('accepts same-book resources and numbered chapters', () => {
    expect(parseReader(book, 'book1')).toEqual(book);
    expect(parseReaderProgress({ page: 1, total: 2 })).toEqual({
      page: 1,
      total: 2,
    });
  });
  it.each([
    'https://evil.test/read/book1/file',
    '//evil.test/read/book1/file',
    '/read/book2/file',
    '/read/book1/asset/../../secret',
    '/read/book1/file?token=x',
    '/read/book1/file#x',
    '/read/book1/file\n',
    '/api/v1/me',
  ])('rejects resource %s', (path) =>
    expect(() => readerResource(path, 'book1')).toThrow(),
  );
  it.each([
    { pages: [] },
    { type: 'html' },
    { id: 'other' },
    { pages: [{ number: 2, title: 'a', url: '/read/book1/file' }] },
    { unknown: 1 },
  ])('rejects invalid manifest %p', (patch) =>
    expect(() => parseReader({ ...book, ...patch }, 'book1')).toThrow(),
  );
  it.each([
    { page: 0, total: 1 },
    { page: 2, total: 1 },
    { page: 1.2, total: 2 },
    { page: 1, total: 10001 },
    { page: 1, total: 2, extra: true },
  ])('rejects invalid progress %p', (value) =>
    expect(() => parseReaderProgress(value)).toThrow(),
  );
});

it.each([0, 0.625, 1])(
  'preserves a saved within-chapter position %s',
  (offset) => {
    expect(parseReaderProgress({ page: 2, total: 3, offset })).toEqual({
      page: 2,
      total: 3,
      offset,
    });
  },
);
it.each([null, '0.5', -0.01, 1.01, NaN, Infinity, {}, []])(
  'rejects an invalid reading offset %p',
  (offset) => {
    expect(() => parseReaderProgress({ page: 1, total: 2, offset })).toThrow();
  },
);

it.each([
  null,
  {},
  [],
  { nativeEvent: null },
  { nativeEvent: { path: 'chapter' } },
  { nativeEvent: { path: 'chapter', offset: '0.5' } },
  { nativeEvent: { path: 'other', offset: 0.5 } },
])('ignores malformed or stale native position %p', (event) => {
  expect(readerPositionEvent(event, 'chapter')).toBeNull();
});
it('accepts a current native reading position', () => {
  expect(
    readerPositionEvent(
      { nativeEvent: { path: 'chapter', offset: 0.75 } },
      'chapter',
    ),
  ).toBe(0.75);
});
