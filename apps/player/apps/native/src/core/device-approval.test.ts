import {
  approvalCode,
  parseDeviceApproval,
  parsePendingTVs,
} from './device-approval';
import { approvalURL, readApprovalLink } from './approval-link';
import { KinosailClient } from './server-client';
import { fetchMock, response } from '../testing/server-client-test-helpers';

const pending = {
  code: '123456',
  device: 'Kinosail TV',
  expiresAt: '2026-09-08T22:00:00Z',
};

it('only builds QR links from a validated server and displayed code', () => {
  expect(approvalURL('https://kino.example/', '123456')).toBe(
    'https://kino.example/connect?code=123456',
  );
  expect(
    readApprovalLink(
      { server: 'https://kino.example/', code: '123456' },
      'https://kino.example',
    ),
  ).toBe('123456');
  for (const params of [
    {},
    { code: '123456' },
    { server: 'https://other.example', code: '123456' },
    { server: ['https://kino.example'], code: '123456' },
    { server: 'https://kino.example', code: ['123456', '654321'] },
    { server: 'https://kino.example', code: '123456', token: 'not-allowed' },
    { server: 'https://kino.example/path', code: '123456' },
    { server: 'https://user:password@kino.example', code: '123456' },
    { server: 'javascript:alert(1)', code: '123456' },
    { server: 'x'.repeat(2049), code: '123456' },
  ])
    expect(() => readApprovalLink(params, 'https://kino.example')).toThrow();
});

it('rejects invalid codes before any request is sent', async () => {
  const fetcher = fetchMock(),
    client = new KinosailClient(
      'https://kino.example',
      'viewer-token',
      fetcher,
    );
  for (const code of [
    '',
    '12345',
    '1234567',
    '12345a',
    ' 123456',
    '123456\n',
    '１２３４５６',
    '1'.repeat(5000),
  ]) {
    expect(() => approvalCode(code)).toThrow();
    await expect(client.previewDevice(code)).rejects.toThrow();
    await expect(client.approveDevice(code)).rejects.toThrow();
  }
  expect(fetcher).not.toHaveBeenCalled();
});

it('rejects unknown, oversized, ambiguous, or malformed preview responses', () => {
  expect(parseDeviceApproval(pending)).toEqual(pending);
  for (const value of [
    null,
    [],
    {},
    { ...pending, code: '12345' },
    { ...pending, secret: 'not-allowed' },
    { ...pending, device: 'x'.repeat(81) },
    { ...pending, device: 'TV\nspoof' },
    { ...pending, expiresAt: 'tomorrow' },
    { ...pending, expiresAt: '2026-02-30T00:00:00Z' },
    { ...pending, expiresAt: '2026-09-08T25:00:00Z' },
    { ...pending, expiresAt: '2026-09-08T00:00:00+00:00' },
  ])
    expect(() => parseDeviceApproval(value)).toThrow();
  expect(() => parsePendingTVs([pending, pending])).toThrow();
  expect(() => parsePendingTVs(Array(9).fill(pending))).toThrow();
  expect(() => parsePendingTVs({ pending: [] })).toThrow();
  expect(parsePendingTVs([])).toEqual([]);
});

it('uses the current session for preview and explicit approval', async () => {
  const fetcher = fetchMock()
    .mockResolvedValueOnce(response(200, [pending]))
    .mockResolvedValueOnce(response(200, pending))
    .mockResolvedValueOnce(response(204, {}));
  const client = new KinosailClient(
    'https://kino.example',
    'viewer-token',
    fetcher,
  );
  expect(await client.loadPendingTVs()).toEqual([pending]);
  expect(await client.previewDevice('123456')).toEqual(pending);
  expect(fetcher).toHaveBeenCalledTimes(2);
  await client.approveDevice('123456');
  expect(fetcher).toHaveBeenLastCalledWith(
    'https://kino.example/api/v1/quick-connect/123456',
    expect.objectContaining({
      method: 'POST',
      headers: expect.objectContaining({
        Authorization: 'Bearer viewer-token',
      }),
    }),
  );
});

it('withdraws a request using its polling secret, never its displayed code', async () => {
  const fetcher = fetchMock().mockResolvedValue(response(204, {}));
  await new KinosailClient(
    'https://kino.example',
    undefined,
    fetcher,
  ).cancelQuickConnect('device-held-secret');
  expect(fetcher).toHaveBeenCalledWith(
    'https://kino.example/api/v1/quick-connect/cancel',
    expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ secret: 'device-held-secret' }),
    }),
  );
});

it('rejects a preview for a different request code', async () => {
  const fetcher = fetchMock().mockResolvedValue(
    response(200, { ...pending, code: '654321' }),
  );
  const client = new KinosailClient(
    'https://kino.example',
    'viewer-token',
    fetcher,
  );
  await expect(client.previewDevice('123456')).rejects.toThrow(
    'does not match',
  );
  expect(fetcher).toHaveBeenCalledTimes(1);
});
