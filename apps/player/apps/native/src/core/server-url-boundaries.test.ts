import { normalizeServerURL } from './server-client';

describe('native server URL boundaries', () => {
  it('normalizes whitespace before enforcing the server URL limit', () => {
    const url = `https://${'a'.repeat(2040)}`;
    expect(normalizeServerURL(` ${url} `)).toBe(url);
    expect(() => normalizeServerURL(`${url}a`)).toThrow(
      'Enter a valid Kinosail Server URL.',
    );
  });

  it.each([
    'http://168.254.1.1',
    'http://169.253.1.1',
    'http://171.16.1.1',
    'http://172.15.1.1',
    'http://172.32.1.1',
    'http://191.168.1.1',
    'http://192.167.1.1',
    'http://8.8.8.8',
  ])('rejects a public address beside local ranges %#', (url) => {
    expect(() => normalizeServerURL(url)).toThrow(
      'Enter a valid Kinosail Server URL.',
    );
  });

  it.each([
    'http://10.public.example.com',
    'http://127.public.example.com',
    'http://[fc::1]',
    'http://[fd::1]',
    'http://[fda::1]',
  ])('does not mistake a nonprivate host for a local address %#', (value) => {
    expect(() => normalizeServerURL(value)).toThrow(
      'Enter a valid Kinosail Server URL.',
    );
  });

  it.each([
    'http://10.1.2.3',
    'http://127.0.0.1',
    'http://169.254.1.2',
    'http://172.16.0.1',
    'http://172.31.255.254',
    'http://192.168.1.1',
    'http://server.local',
    'http://[::1]',
    'http://[fc00::1]',
    'http://[fdff::1]',
    'http://[fe80::1]',
  ])('preserves a valid local server %#', (value) => {
    expect(normalizeServerURL(value)).toBe(value);
  });

  it.each([
    'https://kino.example/path',
    'https://kino.example?query=1',
    'https://kino.example#fragment',
    'https://:secret@kino.example',
  ])('rejects a server URL with extra components %#', (value) => {
    expect(() => normalizeServerURL(value)).toThrow(
      'Enter a valid Kinosail Server URL.',
    );
  });
});
