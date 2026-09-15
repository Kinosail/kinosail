import { KinosailClient } from './server-client';
import { response, fetchMock } from '../testing/server-client-test-helpers';

describe('native client authentication requests', () => {
  it('does not invent authorization for a new connection', async () => {
    const fetcher = jest
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue(
        response(201, { code: '123456', secret: 'challenge' }),
      );
    try {
      await new KinosailClient('https://kino.example').startQuickConnect(
        'Phone',
      );
      expect(fetcher).toHaveBeenCalledWith(
        'https://kino.example/api/v1/quick-connect',
        {
          method: 'POST',
          body: JSON.stringify({ device: 'Phone' }),
          headers: {
            Accept: 'application/json',
            'Content-Type': 'application/json',
          },
        },
      );
    } finally {
      fetcher.mockRestore();
    }
  });

  it('rejects malformed pending approval responses', async () => {
    const fetcher = fetchMock().mockResolvedValue(
      response(202, { status: 'approved' }),
    );
    await expect(
      new KinosailClient('https://kino.example', '', fetcher).pollQuickConnect(
        'challenge',
      ),
    ).rejects.toThrow('Kinosail Server returned an invalid response.');
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it('starts and completes Quick Connect with strict requests', async () => {
    const fetcher = fetchMock()
      .mockResolvedValueOnce(
        response(201, { code: '381204', secret: 'secret-value' }),
      )
      .mockResolvedValueOnce(response(202, { status: 'pending' }))
      .mockResolvedValueOnce(response(201, { token: 'viewer-token' }));
    const client = new KinosailClient('https://kino.example', '', fetcher);

    await expect(client.startQuickConnect('Living Room TV')).resolves.toEqual({
      code: '381204',
      secret: 'secret-value',
    });
    await expect(client.pollQuickConnect('secret-value')).resolves.toBeNull();
    await expect(client.pollQuickConnect('secret-value')).resolves.toBe(
      'viewer-token',
    );

    expect(fetcher).toHaveBeenNthCalledWith(
      1,
      'https://kino.example/api/v1/quick-connect',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ device: 'Living Room TV' }),
      }),
    );
    expect(fetcher.mock.calls.slice(1)).toEqual([
      [
        'https://kino.example/api/v1/quick-connect/token',
        {
          method: 'POST',
          body: JSON.stringify({ secret: 'secret-value' }),
          headers: {
            Accept: 'application/json',
            'Content-Type': 'application/json',
          },
        },
      ],
      [
        'https://kino.example/api/v1/quick-connect/token',
        {
          method: 'POST',
          body: JSON.stringify({ secret: 'secret-value' }),
          headers: {
            Accept: 'application/json',
            'Content-Type': 'application/json',
          },
        },
      ],
    ]);
  });
});
