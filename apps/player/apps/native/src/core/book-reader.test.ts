import { bookReaderURL } from './book-reader';
it('uses the existing reader path without adding a device token', () => {
  const mediaURL = jest.fn((path) => `https://kino.example${path}`);
  expect(bookReaderURL({ mediaURL }, { kind: 'book', id: 'a/b?c' })).toBe(
    'https://kino.example/read/a%2Fb%3Fc',
  );
});
it.each([
  { kind: 'video', id: 'movie' },
  { kind: 'book', id: '.' },
  { kind: 'book', id: '..' },
  { kind: 'book', id: '' },
  { kind: 'book', id: 'x'.repeat(2049) },
  { kind: 'book', id: '\n' },
  { kind: 'book', id: '\ud800' },
])('rejects invalid book identifiers before constructing a URL %#', (item) => {
  const mediaURL = jest.fn();
  expect(() => bookReaderURL({ mediaURL }, item)).toThrow();
  expect(mediaURL).not.toHaveBeenCalled();
});
