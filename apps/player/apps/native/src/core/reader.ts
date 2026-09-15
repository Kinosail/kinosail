import { exactObject } from './media-preferences';
export type ReaderBook = {
  id: string;
  title: string;
  type: 'epub' | 'pdf' | 'comic';
  pages: { title: string; url: string; number: number }[];
};
export function readerResource(path: string, id: string) {
  if (
    typeof id !== 'string' ||
    !/^[A-Za-z0-9_-]{1,128}$/.test(id) ||
    typeof path !== 'string' ||
    path.length > 2048 ||
    !path.startsWith(`/read/${id}/`) ||
    /[?#\\\u0000-\u0020\u007f]/.test(path)
  )
    throw new Error('Invalid book resource.');
  const url = new URL(path, 'https://reader.invalid');
  if (
    url.origin !== 'https://reader.invalid' ||
    url.pathname !== path ||
    !(path === `/read/${id}/file` || path.startsWith(`/read/${id}/asset/`))
  )
    throw new Error('Invalid book resource.');
  return path;
}
export function parseReader(value: unknown, id: string): ReaderBook {
  const v = exactObject(value, ['id', 'title', 'type', 'pages']);
  if (
    v.id !== id ||
    typeof v.title !== 'string' ||
    v.title.length > 512 ||
    !['epub', 'pdf', 'comic'].includes(v.type as string) ||
    !Array.isArray(v.pages) ||
    !v.pages.length ||
    v.pages.length > 10000
  )
    throw new Error('Invalid book.');
  const pages = v.pages.map((entry, index) => {
    const page = exactObject(entry, ['title', 'url', 'number']);
    if (
      typeof page.title !== 'string' ||
      page.title.length > 512 ||
      page.number !== index + 1 ||
      typeof page.url !== 'string'
    )
      throw new Error('Invalid book page.');
    return {
      title: page.title,
      url: readerResource(page.url, id),
      number: page.number as number,
    };
  });
  return { ...v, pages } as ReaderBook;
}
export function parseReaderProgress(value: unknown) {
  const v = exactObject(value, ['page', 'total', 'offset']);
  if (
    (v.offset !== undefined &&
      (typeof v.offset !== 'number' ||
        !Number.isFinite(v.offset) ||
        v.offset < 0 ||
        v.offset > 1)) ||
    !Number.isInteger(v.total) ||
    Number(v.total) < 1 ||
    Number(v.total) > 10000 ||
    !Number.isInteger(v.page) ||
    Number(v.page) < 1 ||
    Number(v.page) > Number(v.total)
  )
    throw new Error('Invalid reading position.');
  return {
    page: Number(v.page),
    total: Number(v.total),
    ...(v.offset === undefined ? {} : { offset: v.offset as number }),
  };
}

export function readerPositionEvent(
  event: unknown,
  path: string,
): number | null {
  if (!event || typeof event !== 'object' || !('nativeEvent' in event))
    return null;
  const value = event.nativeEvent;
  if (
    !value ||
    typeof value !== 'object' ||
    !('path' in value) ||
    value.path !== path ||
    !('offset' in value) ||
    typeof value.offset !== 'number' ||
    !Number.isFinite(value.offset) ||
    value.offset < 0 ||
    value.offset > 1
  )
    return null;
  return value.offset;
}
