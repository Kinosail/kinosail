import { downloadEstimate } from './download-estimate';
import { routeItem } from '@/testing/route-fixtures';
it('sums original file sizes and gives rough resolution ranges', () => {
  const items = [
    { ...routeItem, size: 2_000_000_000 },
    { ...routeItem, id: 'second', size: 1_000_000_000 },
  ];
  expect(downloadEstimate(items, 'original', {})).toBe('3.0 GB');
  expect(downloadEstimate([routeItem], '1080p', { arrival: 3600 })).toBe(
    'About 1.4 GB–3.7 GB',
  );
  expect(downloadEstimate([routeItem], '720p', { arrival: 3600 })).toBe(
    'About 747 MB–1.9 GB',
  );
});
it.each([undefined, 0, -1, NaN, Infinity, Number.MAX_SAFE_INTEGER + 1])(
  'does not invent sizes from missing or invalid data %p',
  (size) => {
    expect(
      downloadEstimate([{ ...routeItem, size }], 'original', {}),
    ).toBeNull();
  },
);
it('does not present a partial total as a complete estimate', () => {
  expect(downloadEstimate([routeItem], '720p', {})).toBeNull();
  expect(
    downloadEstimate([routeItem, { ...routeItem, id: 'missing' }], '1080p', {
      arrival: 3600,
    }),
  ).toBeNull();
  expect(
    downloadEstimate([routeItem], '720p', { arrival: Infinity }),
  ).toBeNull();
  expect(downloadEstimate([], 'original', {})).toBeNull();
});
