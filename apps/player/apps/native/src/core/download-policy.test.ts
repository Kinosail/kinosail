import {
  downloadSize,
  downloadStorageLimits,
  maxDownloadBytes,
  downloadQuota,
  validateDownloadResponse,
  validateDownloadID,
  downloadModification,
} from './download-policy';

it('reserves free space and bounds the complete collection', () => {
  expect(downloadSize('1024', 0, 1024 ** 3)).toBe(1024);
  expect(() => downloadSize('1024', downloadQuota, 30 * 1024 ** 3)).toThrow(
    'space',
  );
  expect(() => downloadSize('1024', 0, 512 * 1024 ** 2)).toThrow('space');
});
it.each([
  null,
  '',
  '0',
  '-1',
  '1.5',
  '1e3',
  ' 123',
  '123 ',
  '0x100',
  '9'.repeat(17),
])('rejects an ambiguous download size %p', (value) => {
  expect(() => downloadSize(value, 0, 30 * 1024 ** 3)).toThrow();
});
it.each([
  [NaN, 30 * 1024 ** 3],
  [-1, 30 * 1024 ** 3],
  [0, Infinity],
  [0, -1],
])('rejects malformed quota values', (used, free) =>
  expect(() => downloadSize('10', used, free)).toThrow(),
);
it('accepts only the exact requested full file or single byte range', () => {
  expect(() =>
    validateDownloadResponse(200, '100', null, 0, 100),
  ).not.toThrow();
  expect(() =>
    validateDownloadResponse(206, '60', 'bytes 40-99/100', 40, 100),
  ).not.toThrow();
});
it.each([
  [200, '100', null, 40, 100],
  [206, '59', 'bytes 40-99/100', 40, 100],
  [206, '60', 'bytes 41-100/101', 40, 100],
  [200, '100', 'bytes 0-99/100', 0, 100],
  [206, '0', 'bytes 100-99/100', 100, 100],
  [200, '100', null, -1, 100],
  [200, '100', null, 0, Infinity],
  [302, '100', null, 0, 100],
] as const)(
  'rejects changed or inconsistent responses before writing',
  (status, length, range, offset, total) =>
    expect(() =>
      validateDownloadResponse(status, length, range, offset, total),
    ).toThrow(),
);
it.each(['', 'a\nb', 'a'.repeat(2049), null, [], {}])(
  'rejects invalid identifiers %p',
  (id) => expect(() => validateDownloadID(id as string)).toThrow(),
);
it('requires an unambiguous HTTP modification date for resumed downloads', () => {
  expect(downloadModification('Mon, 07 Sep 2026 12:00:00 GMT')).toBe(
    'Mon, 07 Sep 2026 12:00:00 GMT',
  );
  for (const value of [
    null,
    '',
    'yesterday',
    '2026-09-07',
    'Mon, 07 Sep 2026 12:00:00 GMT\r\nX: 1',
  ])
    expect(() => downloadModification(value)).toThrow();
});

describe('personal download quota', () => {
  it('honors a smaller quota before starting a transfer', () => {
    expect(() =>
      downloadSize(String(2 * 1024 ** 3), 0, 10 * 1024 ** 3, 1024 ** 3),
    ).toThrow();
    expect(
      downloadSize(String(512 * 1024 ** 2), 0, 10 * 1024 ** 3, 1024 ** 3),
    ).toBe(512 * 1024 ** 2);
  });
  it.each([-1, 0.5, 1024 ** 3 - 1, NaN, Infinity, Number.MAX_SAFE_INTEGER + 1])(
    'rejects invalid quota %s',
    (quota) => {
      expect(() => downloadSize('1', 0, 10 * 1024 ** 3, quota)).toThrow();
    },
  );
});

it('allows larger limits and unlimited downloads while retaining the free-space reserve', () => {
  const size = 100 * 1024 ** 3,
    used = 30 * 1024 ** 3;
  expect(downloadSize(String(size), used, size + 512 * 1024 ** 2, 0)).toBe(
    size,
  );
  expect(
    downloadSize(String(size), used, size + 1024 ** 3, 250 * 1024 ** 3),
  ).toBe(size);
  expect(() => downloadSize(String(size), used, size, 0)).toThrow('space');
  expect(() => downloadSize('1', maxDownloadBytes, 1024 ** 3, 0)).toThrow();
  expect(() =>
    downloadSize(String(maxDownloadBytes + 1), 0, maxDownloadBytes, 0),
  ).toThrow();
  expect(() =>
    validateDownloadResponse(200, String(size), null, 0, size),
  ).not.toThrow();
});
it('sizes choices to device capacity and preserves a previously selected limit', () => {
  expect(downloadStorageLimits(256 * 1024 ** 3, 20)).toEqual([
    1, 2, 5, 10, 20, 50, 100, 250, 0,
  ]);
  expect(downloadStorageLimits(32 * 1024 ** 3, 100)).toEqual([
    1, 2, 5, 10, 20, 100, 0,
  ]);
  expect(downloadStorageLimits(512 * 1024 ** 2, 0)).toEqual([0]);
  expect(downloadStorageLimits(undefined, 0)).toEqual([1, 2, 5, 10, 20, 0]);
});
