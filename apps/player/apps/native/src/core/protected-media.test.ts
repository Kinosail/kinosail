import type { PlaybackSource } from './contract';
jest.mock('expo', () => ({
  requireOptionalNativeModule: jest.fn(() => ({
    open: jest.fn().mockResolvedValue('http://127.0.0.1:1234/capability'),
    close: jest.fn(),
    prepareOfflineStorage: jest.fn(),
  })),
}));
import { requireOptionalNativeModule } from 'expo';
jest.mock('react-native', () => ({ Platform: { OS: 'ios', isTV: false } }));
import {
  openProtectedMedia,
  closeProtectedMedia,
  nextProtectedMediaID,
} from './protected-media';
const loader = requireOptionalNativeModule as jest.Mock;
const mockTransport = loader.mock.results[
  loader.mock.calls.findIndex(([name]) => name === 'ProtectedMedia')
].value;
const source = (uri: string, authorization = 'Bearer viewer-token') =>
  ({ uri, headers: { Authorization: authorization } }) as PlaybackSource;
beforeEach(() => jest.clearAllMocks());
it('keeps credentials in the native transport and identifies cleanup by attempt', async () => {
  const first = nextProtectedMediaID(),
    second = nextProtectedMediaID();
  expect(second).toBeGreaterThan(first);
  await openProtectedMedia(
    source('https://kino.example/media/movie-1'),
    second,
  );
  expect(mockTransport.open).toHaveBeenCalledWith(
    'https://kino.example/media/movie-1',
    'Bearer viewer-token',
    second,
  );
  closeProtectedMedia(first);
  expect(mockTransport.close).toHaveBeenCalledWith(first);
});
it.each([
  ['https://user:password@kino.example/media/a', 'Bearer viewer'],
  ['https://kino.example/media/a?token=secret', 'Bearer viewer'],
  ['https://kino.example/media/a#fragment', 'Bearer viewer'],
  ['https://kino.example/media/../admin', 'Bearer viewer'],
  ['http://public.example/media/a', 'Bearer viewer'],
  ['https://kino.example/media/a', 'Bearer viewer\r\nCookie: stolen'],
  ['https://kino.example/media/a', ''],
  ['https://kino.example/media/a', `Bearer ${'x'.repeat(2049)}`],
  ['file:///etc/passwd', ''],
  ['file:///Documents/kinosail-offline/a/b.media', ''],
])(
  'rejects unsafe sources before starting a connection: %s',
  async (uri, authorization) => {
    await expect(
      openProtectedMedia(source(uri, authorization), 1),
    ).rejects.toThrow();
    expect(mockTransport.open).not.toHaveBeenCalled();
  },
);
it.each([NaN, Infinity, -1, 0, 0.5])(
  'rejects invalid attempt IDs before native effects',
  async (id) => {
    await expect(
      openProtectedMedia(source('https://kino.example/media/a'), id),
    ).rejects.toThrow();
    expect(mockTransport.open).not.toHaveBeenCalled();
  },
);
it('accepts a local store path only without credentials', async () => {
  const uri = `file:///Documents/kinosail-offline/${'a'.repeat(64)}/${'b'.repeat(64)}.media`;
  await openProtectedMedia(source(uri, ''), 1);
  expect(mockTransport.open).toHaveBeenCalledWith(uri, '', 1);
});
