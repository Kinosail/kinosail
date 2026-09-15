import { KinosailClient, type Fetcher } from './server-client';

const fetchMock = () => jest.fn<ReturnType<Fetcher>, Parameters<Fetcher>>();
const invalidResponse = 'Kinosail Server returned an invalid response.';

describe('native request boundaries', () => {
  it.each([2 * 1024 * 1024, 2 * 1024 * 1024 + 1])(
    'bounds otherwise valid JSON at %s characters',
    async (length) => {
      const body = JSON.stringify({ items: [], padding: '' });
      const fetcher = fetchMock().mockResolvedValue(
        new Response(
          body.replace(
            '"padding":""',
            `"padding":"${'x'.repeat(length - body.length)}"`,
          ),
          { status: 200 },
        ),
      );
      const result = new KinosailClient(
        'https://kino.example',
        '',
        fetcher,
      ).loadLibrary('recent');
      if (length === 2 * 1024 * 1024) await expect(result).resolves.toEqual([]);
      else await expect(result).rejects.toThrow(invalidResponse);
    },
  );

  it('preserves maximum valid secret and trimmed device names', async () => {
    const secret = 's'.repeat(2048);
    const device = 'd'.repeat(80);
    const fetcher = fetchMock()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ code: '123456', secret: 'challenge' }), {
          status: 201,
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ status: 'pending' }), { status: 202 }),
      );
    const client = new KinosailClient('https://kino.example', secret, fetcher);
    await client.startQuickConnect(` ${device} `);
    await client.pollQuickConnect(secret);
    expect(fetcher.mock.calls).toEqual([
      [
        'https://kino.example/api/v1/quick-connect',
        {
          method: 'POST',
          body: JSON.stringify({ device }),
          headers: {
            Accept: 'application/json',
            'Content-Type': 'application/json',
            Authorization: `Bearer ${secret}`,
          },
        },
      ],
      [
        'https://kino.example/api/v1/quick-connect/token',
        {
          method: 'POST',
          body: JSON.stringify({ secret }),
          headers: {
            Accept: 'application/json',
            'Content-Type': 'application/json',
            Authorization: `Bearer ${secret}`,
          },
        },
      ],
    ]);
  });

  it.each([false, true])(
    'preserves explicit viewing state %s and its playback token',
    async (watched) => {
      const fetcher = fetchMock().mockResolvedValue(
        new Response(
          JSON.stringify({
            seconds: 0,
            watched,
            revision: 1,
            session: 'play-1',
          }),
          { status: 200 },
        ),
      );
      const client = new KinosailClient(
        'https://kino.example',
        'viewer-token',
        fetcher,
      );
      await expect(
        client.saveProgress('arrival', {
          seconds: 0,
          revision: 1,
          session: 'play-1',
          playbackToken: 'playback-token',
          watched,
        }),
      ).resolves.toEqual({
        seconds: 0,
        revision: 1,
        session: 'play-1',
        watched,
      });
      expect(JSON.parse(fetcher.mock.calls[0][1]!.body as string)).toEqual({
        seconds: 0,
        revision: 1,
        session: 'play-1',
        playbackToken: 'playback-token',
        watched,
      });
    },
  );

  it.each([
    [401, 'This device session has expired.'],
    [403, 'This profile cannot use that feature.'],
    [404, 'The requested media is no longer available.'],
    [409, 'The server state changed. Refresh and try again.'],
    [429, 'Too many requests. Wait a minute and try again.'],
    [500, 'Kinosail Server could not complete the request.'],
  ] as const)(
    'translates status %s without exposing its body',
    async (status, message) => {
      const response = new Response('private server exception', { status });
      const read = jest.spyOn(response, 'text');
      const fetcher = fetchMock().mockResolvedValue(response);
      await expect(
        new KinosailClient(
          'https://kino.example',
          'viewer-token',
          fetcher,
        ).loadLibrary('recent'),
      ).rejects.toThrow(message);
      expect(read).not.toHaveBeenCalled();
    },
  );

  it.each([
    '',
    ' ',
    'x'.repeat(81),
    'invalid\u0000device',
    'invalid\u007fdevice',
  ])(
    'rejects an invalid device name %# before requesting a code',
    async (name) => {
      const fetcher = fetchMock();
      await expect(
        new KinosailClient(
          'https://kino.example',
          '',
          fetcher,
        ).startQuickConnect(name),
      ).rejects.toThrow('Enter a valid device name.');
      expect(fetcher).not.toHaveBeenCalled();
    },
  );

  it.each(['', 'x'.repeat(2049), 'invalid\rsecret', 'invalid\nsecret'])(
    'rejects an invalid authorization secret %# before polling',
    async (secret) => {
      const fetcher = fetchMock();
      await expect(
        new KinosailClient(
          'https://kino.example',
          '',
          fetcher,
        ).pollQuickConnect(secret),
      ).rejects.toThrow('Kinosail session data is invalid.');
      expect(fetcher).not.toHaveBeenCalled();
    },
  );

  it('rejects invalid saved credentials before any request', () => {
    const fetcher = fetchMock();
    expect(
      () =>
        new KinosailClient(
          'https://kino.example',
          'invalid\ncredential',
          fetcher,
        ),
    ).toThrow('Kinosail session data is invalid.');
    expect(fetcher).not.toHaveBeenCalled();
  });

  it.each(['', '{', 'x'.repeat(2 * 1024 * 1024 + 1)])(
    'rejects an invalid response body %#',
    async (body) => {
      const fetcher = fetchMock().mockResolvedValue(
        new Response(body, { status: 200 }),
      );
      await expect(
        new KinosailClient(
          'https://kino.example',
          'viewer-token',
          fetcher,
        ).loadLibrary('recent'),
      ).rejects.toThrow(invalidResponse);
      expect(fetcher).toHaveBeenCalledTimes(1);
    },
  );

  it('loads an encoded item identifier with authenticated headers', async () => {
    const fetcher = fetchMock().mockResolvedValue(
      new Response(
        JSON.stringify({
          item: { id: 'item', kind: 'video', title: 'Arrival' },
        }),
        { status: 200 },
      ),
    );
    const client = new KinosailClient(
      'https://kino.example',
      'viewer-token',
      fetcher,
    );
    await expect(client.loadItem('item/part 1')).resolves.toMatchObject({
      id: 'item',
      title: 'Arrival',
    });
    expect(fetcher).toHaveBeenCalledWith(
      'https://kino.example/api/v1/items/item%2Fpart%201',
      expect.objectContaining({
        headers: {
          Accept: 'application/json',
          Authorization: 'Bearer viewer-token',
        },
      }),
    );
    expect(client.mediaURL('')).toBe('');
  });

  it.each([-1, Infinity, NaN])(
    'rejects invalid playback seconds %# before sending progress',
    async (seconds) => {
      const fetcher = fetchMock();
      const client = new KinosailClient(
        'https://kino.example',
        'viewer-token',
        fetcher,
      );
      await expect(
        client.saveProgress('arrival', {
          seconds,
          session: 'play-1',
          revision: 2,
        }),
      ).rejects.toThrow('Playback progress is invalid.');
      expect(fetcher).not.toHaveBeenCalled();
    },
  );
});
