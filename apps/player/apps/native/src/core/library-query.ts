export const libraryViews = [
  'all',
  'movies',
  'shows',
  'unwatched',
  'list',
  'music',
  'audiobooks',
  'books',
  'photos',
  'history',
] as const;
export const librarySorts = ['title', 'added', 'year'] as const;
export type LibraryQuery = {
  q?: string;
  view?: (typeof libraryViews)[number];
  sort?: (typeof librarySorts)[number];
  offset?: number;
};

export function libraryQuery(input: LibraryQuery): string {
  const invalid = () => new Error('The requested library search is invalid.');
  if (
    !input ||
    typeof input !== 'object' ||
    Array.isArray(input) ||
    Object.keys(input).some(
      (key) => !['q', 'view', 'sort', 'offset'].includes(key),
    )
  )
    throw invalid();
  const { q = '', view = 'all', sort = 'title', offset = 0 } = input;
  if (
    typeof q !== 'string' ||
    q.length > 512 ||
    /[\u0000-\u001f\u007f\ud800-\udfff]/u.test(q) ||
    !libraryViews.includes(view) ||
    !librarySorts.includes(sort) ||
    !Number.isSafeInteger(offset) ||
    offset < 0 ||
    offset > 1_000_000
  )
    throw invalid();
  const query = q.trim();
  if (encodeURIComponent(query).replace(/%[0-9A-F]{2}/g, 'x').length > 512)
    throw invalid();
  return `q=${encodeURIComponent(query)}&view=${view}&sort=${sort}&offset=${offset}&limit=60`;
}
