import { URL as NativeURL } from 'expo/src/winter/url';

import { KinosailClient, normalizeServerURL } from './server-client';

describe('native Expo URL runtime', () => {
  const originalURL = globalThis.URL;
  beforeEach(() => {
    globalThis.URL = NativeURL as typeof URL;
  });
  afterEach(() => {
    globalThis.URL = originalURL;
  });

  it.each([
    ['http://127.1', 'http://127.0.0.1'],
    ['http://0x7f000001', 'http://127.0.0.1'],
    ['http://[FC00::1]', 'http://[fc00::1]'],
    ['https://KINO.example:443/', 'https://kino.example'],
  ])(
    'normalizes supported addresses with the shipped parser %#',
    (input, expected) => {
      expect(normalizeServerURL(input)).toBe(expected);
    },
  );

  it.each([
    'http://10.public.example.com',
    'http://[fc::1]',
    'http://999.1.2.3',
    'http://172.32.0.1',
  ])(
    'rejects invalid or nonprivate addresses with the shipped parser %#',
    (input) => {
      expect(() => normalizeServerURL(input)).toThrow(
        'Enter a valid Kinosail Server URL.',
      );
    },
  );

  it('preserves origin checks with the shipped parser', () => {
    const client = new KinosailClient('https://kino.example');
    expect(client.mediaURL('/media/arrival')).toBe(
      'https://kino.example/media/arrival',
    );
    expect(() => client.mediaURL('//other.example/video')).toThrow(
      'Kinosail Server returned an invalid media URL.',
    );
  });
});
