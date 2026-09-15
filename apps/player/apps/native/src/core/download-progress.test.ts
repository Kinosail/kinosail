import { downloadProgressTracker } from './download-progress';
import { routeItem } from '@/testing/route-fixtures';
import type { DownloadEntry } from './downloads.types';

const entry: DownloadEntry = {
  item: routeItem, bytes: 0, total: 100 * 1024 ** 2, status: 'downloading',
  duration: 120, contentType: 'video/mp4', version: 'one', error: '',
};
const at = (bytes: number, status = entry.status) => [{ ...entry, bytes, status }];
it('estimates from newly observed bytes, not an existing partial file', () => {
  const measure = downloadProgressTracker();
  expect(measure(at(50 * 1024 ** 2), 0)).toEqual({});
  expect(measure(at(52 * 1024 ** 2), 2000)[entry.item.id]).toBe('1.0 MB/s · Less than a minute left');
});
it('hides estimates after a stall and restarts after pause or suspension', () => {
  const measure = downloadProgressTracker();
  measure(at(0), 0);
  measure(at(1024 ** 2), 2000);
  expect(measure(at(1024 ** 2), 8000)).toEqual({});
  expect(measure(at(1024 ** 2, 'paused'), 9000)).toEqual({});
  expect(measure(at(2 * 1024 ** 2), 10000)).toEqual({});
  expect(measure(at(4 * 1024 ** 2), 12000)[entry.item.id]).toContain('1.0 MB/s');
  expect(measure(at(30 * 1024 ** 2), 50000)).toEqual({});
});
it('resets when a transfer restarts, disappears, or completes', () => {
  const measure = downloadProgressTracker();
  measure(at(1024 ** 2), 0);
  expect(measure(at(0), 2000)).toEqual({});
  measure([], 3000);
  expect(measure(at(1024 ** 2), 4000)).toEqual({});
  expect(measure(at(entry.total, 'complete'), 6000)).toEqual({});
});
