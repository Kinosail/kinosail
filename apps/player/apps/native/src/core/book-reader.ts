import type { MediaItem } from './contract';
import type { KinosailClient } from './server-client';

// Keep the device credential out of browser URLs and browser history.
export function bookReaderURL(
  client: Pick<KinosailClient, 'mediaURL'>,
  item: Pick<MediaItem, 'kind' | 'id'>,
): string {
  if (
    item.kind !== 'book' ||
    typeof item.id !== 'string' ||
    !item.id ||
    item.id === '.' ||
    item.id === '..' ||
    item.id.length > 2048 ||
    /[\u0000-\u001f\u007f\ud800-\udfff]/u.test(item.id)
  ) {
    throw new Error('This ebook is not available.');
  }
  return client.mediaURL(`/read/${encodeURIComponent(item.id)}`);
}
