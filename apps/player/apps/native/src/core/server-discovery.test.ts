jest.mock('expo', () => ({ requireOptionalNativeModule: jest.fn(() => null) }));
import { validateDiscoveredServers } from './server-discovery';

describe('discovered server validation', () => {
  it('normalizes and deduplicates without trusting names as identity', () => {
    expect(
      validateDiscoveredServers([
        { name: ' Living room ', url: 'http://192.168.1.8:38127/' },
        { name: 'Duplicate', url: 'http://192.168.1.8:38127' },
        { name: 'Living room', url: 'https://player.example.com' },
      ]),
    ).toEqual([
      { name: 'Living room', url: 'http://192.168.1.8:38127' },
      { name: 'Living room', url: 'https://player.example.com' },
    ]);
  });
  it.each([
    null,
    {},
    'servers',
    Array(33).fill({ name: 'Player', url: 'https://example.com' }),
  ])('rejects malformed or oversized batches: %p', (value) => {
    expect(validateDiscoveredServers(value)).toEqual([]);
  });
  it.each([
    null,
    [],
    {},
    { name: 1, url: 'https://example.com' },
    { name: '', url: 'https://example.com' },
    { name: 'x'.repeat(64), url: 'https://example.com' },
    { name: 'bad\nname', url: 'https://example.com' },
    { name: 'Player', url: 3 },
    { name: 'Player', url: 'https://' + 'x'.repeat(241) },
    { name: 'Player', url: 'https://example.com', token: 'untrusted' },
    ...[
      '',
      'ftp://server.local',
      'http://public.example.com',
      'https://user:pass@server.local',
      'https://server.local/path',
      'https://server.local?q=1',
      'https://server.local?',
      'https://server.local#',
      'https://server.local\n',
      'https://server.local#fragment',
      'http://localhost:38127',
      'http://127.0.0.1:38127',
      'https://[::1]',
      'https://0.0.0.0',
      'https://[::]',
      'http://server.local:65536',
    ].map((url) => ({ name: 'Player', url })),
  ])('rejects an untrusted record: %p', (entry) => {
    expect(validateDiscoveredServers([entry])).toEqual([]);
  });
});
