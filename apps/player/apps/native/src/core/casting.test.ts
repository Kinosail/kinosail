import {
  parseCastSession,
  parseCastDevices,
  parseCastStatus,
  type CastStart,
  type CastCommand,
} from './casting';
import { KinosailClient } from './server-client';
import { fetchMock, response } from '../testing/server-client-test-helpers';

const id = 'a'.repeat(32),
  ticket = 'b'.repeat(64);
const base = 'https://kino.example';
const media = () => ({
  id,
  url: `${base}/cast/${id}/media?ticket=${ticket}`,
  title: 'Film',
  contentType: 'video/mp4',
  protocol: 'google-cast',
  position: 10,
  duration: 120,
  expiresAt: new Date(Date.now() + 3600_000).toISOString(),
  tracks: [],
});

it('accepts only the scoped receiver URL and returns no account credential', async () => {
  const fetcher = fetchMock().mockResolvedValue(response(201, media()));
  const result = await new KinosailClient(
    base,
    'account-secret',
    fetcher,
  ).startCast('film', { protocol: 'google-cast', position: 10 });
  expect(result.url).toBe(media().url);
  expect(JSON.stringify(result)).not.toContain('account-secret');
  expect(fetcher.mock.calls[0][1]?.body).toBe(
    JSON.stringify({ protocol: 'google-cast', position: 10 }),
  );
});
it.each([
  {},
  { protocol: 'roku', position: 0 },
  { protocol: 'google-cast', position: -1 },
  { protocol: 'google-cast', position: Number.NaN },
  { protocol: 'google-cast', position: 31_536_001 },
  { protocol: 'google-cast', position: 0, deviceId: id },
  { protocol: 'dlna', position: 0 },
  { protocol: 'dlna', position: 0, deviceId: 'http://192.168.1.1' },
  { protocol: 'google-cast', position: 0, playbackToken: 'x'.repeat(8193) },
  { protocol: 'google-cast', position: 0, unknown: true },
])('rejects invalid cast start before any fetch %#', async (input) => {
  const fetcher = fetchMock();
  await expect(
    new KinosailClient(base, '', fetcher).startCast('film', input as CastStart),
  ).rejects.toThrow();
  expect(fetcher).not.toHaveBeenCalled();
});
it.each([
  {},
  { action: 'launch' },
  { action: 'seek' },
  { action: 'seek', position: -1 },
  { action: 'seek', position: Infinity },
  { action: 'stop', position: 10 },
  { action: 'pause', unknown: true },
])('rejects invalid commands before any fetch %#', async (input) => {
  const fetcher = fetchMock();
  await expect(
    new KinosailClient(base, '', fetcher).castCommand(id, input as CastCommand),
  ).rejects.toThrow();
  expect(fetcher).not.toHaveBeenCalled();
});
it.each([
  { url: `https://other.example/cast/${id}/media?ticket=${ticket}` },
  { url: `${base}/media/film?ticket=${ticket}` },
  { url: `${base}/cast/${id}/media?ticket=${ticket}&ticket=${ticket}` },
  { url: `${base}/cast/${id}/media?ticket=${ticket}&api_key=secret` },
  { url: `https://user:secret@kino.example/cast/${id}/media?ticket=${ticket}` },
  { protocol: 'unknown' },
  { position: 121 },
  { expiresAt: 'invalid' },
  { expiresAt: new Date(0).toISOString() },
  { tracks: Array(65).fill({}) },
  {
    tracks: [
      {
        id: 1,
        url: `${base}/subtitle/film`,
        label: 'English',
        language: 'en',
        default: true,
      },
    ],
  },
])('rejects unsafe or inconsistent receiver responses %#', (change) => {
  expect(() => parseCastSession({ ...media(), ...change }, base)).toThrow();
});
it('validates discovery identities and receiver observations', () => {
  const device = { id, name: 'Living room', protocol: 'dlna' };
  expect(parseCastDevices({ devices: [device] })).toEqual([device]);
  expect(() => parseCastDevices({ devices: [device, device] })).toThrow();
  expect(() =>
    parseCastDevices({ devices: [{ ...device, name: '\nTV' }] }),
  ).toThrow();
  expect(() =>
    parseCastStatus({ state: 'unknown', position: 1, duration: 2 }),
  ).toThrow();
  expect(() =>
    parseCastStatus({ state: 'playing', position: 10, duration: 2 }),
  ).toThrow();
  expect(
    parseCastStatus({ state: 'paused', position: 10, duration: 120 }),
  ).toEqual({ state: 'paused', position: 10, duration: 120 });
});
