import { KinosailClient, normalizeServerURL } from './server-client';
import { fetchMock, response } from '../testing/server-client-test-helpers';

describe('normalizeServerURL', () => {
  it.each([
    ['https://kino.example/', 'https://kino.example'],
    ['http://192.168.1.40:38127/', 'http://192.168.1.40:38127'],
    ['http://localhost:38127', 'http://localhost:38127'],
  ])('accepts a supported server URL', (input, expected) => {
    expect(normalizeServerURL(input)).toBe(expected);
  });

  it.each([
    '',
    'kino.example',
    'ftp://kino.example',
    'http://kino.example',
    'https://user:secret@kino.example',
    `https://kino.example/${'a'.repeat(2048)}`,
  ])('rejects an unsafe or malformed URL', (input) => {
    expect(() => normalizeServerURL(input)).toThrow(
      'valid Kinosail Server URL',
    );
  });
});

describe('KinosailClient', () => {
  it.each([
    '',
    'unknown',
    'toString',
    'constructor',
    '__proto__',
    null,
    undefined,
  ])(
    'rejects an unknown library view before requesting it %#',
    async (view) => {
      const fetcher = fetchMock();
      await expect(
        new KinosailClient('https://kino.example', '', fetcher).loadLibrary(
          view as 'recent',
        ),
      ).rejects.toThrow('The requested library view is invalid.');
      expect(fetcher).not.toHaveBeenCalled();
    },
  );

  it('omits unspecified viewing state and normalizes a missing playback token', async () => {
    const fetcher = fetchMock().mockResolvedValue(response(200, {}));
    await new KinosailClient(
      'https://kino.example',
      'viewer-token',
      fetcher,
    ).saveProgress('arrival', { seconds: 0, session: 'play-1', revision: 1 });
    expect(fetcher.mock.calls[0]).toEqual([
      'https://kino.example/api/v1/items/arrival/progress',
      {
        method: 'PUT',
        body: JSON.stringify({
          seconds: 0,
          session: 'play-1',
          revision: 1,
          playbackToken: '',
        }),
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
          Authorization: 'Bearer viewer-token',
        },
      },
    ]);
  });

  it('calls browser fetch with its required global receiver', async () => {
    const fetcher = jest
      .spyOn(globalThis, 'fetch')
      .mockImplementation(function (this: typeof globalThis) {
        expect(this).toBe(globalThis);
        return Promise.resolve(response(200, { items: [] }));
      });
    try {
      await expect(
        new KinosailClient('https://kino.example', 'viewer-token').loadLibrary(
          'recent',
        ),
      ).resolves.toEqual([]);
    } finally {
      fetcher.mockRestore();
    }
  });

  it('does not expose native network internals', async () => {
    const fetcher = fetchMock().mockRejectedValue(
      new Error(
        'fetch failed: UnexpectedException at ExpoModulesCore/Promise.swift:56',
      ),
    );

    await expect(
      new KinosailClient(
        'https://kino.example',
        'viewer-token',
        fetcher,
      ).loadLibrary('recent'),
    ).rejects.toThrow(
      'Could not reach Kinosail Server. Check the address and network, then try again.',
    );
  });

  it('loads bounded home data with bearer authentication', async () => {
    const fetcher = fetchMock()
      .mockResolvedValueOnce(
        response(200, {
          server: 'Den Server',
          viewer: { id: 'viewer-1', name: 'Mike', owner: false },
        }),
      )
      .mockResolvedValueOnce(
        response(200, {
          items: [
            {
              id: 'movie-1',
              kind: 'video',
              title: 'Arrival',
              year: '2016',
              artwork: '/art/movie-1',
              backdrop: '/backdrop/movie-1',
              progress: { seconds: 1800, revision: 2 },
            },
          ],
          total: 1,
        }),
      )
      .mockResolvedValueOnce(
        response(200, {
          items: [
            { id: 'movie-2', kind: 'video', title: 'Moon', progress: {} },
          ],
          total: 1,
        }),
      );
    const client = new KinosailClient(
      'https://kino.example',
      'viewer-token',
      fetcher,
    );

    const home = await client.loadHome();

    expect(home.server).toBe('Den Server');
    expect(home.viewer.name).toBe('Mike');
    expect(home.continueWatching[0]?.title).toBe('Arrival');
    expect(home.recent[0]?.title).toBe('Moon');
    expect(fetcher.mock.calls.map(([url]) => url)).toEqual([
      'https://kino.example/api/v1/me',
      'https://kino.example/api/v1/library?view=history&limit=24',
      'https://kino.example/api/v1/library?sort=added&limit=36',
    ]);
    expect(fetcher.mock.calls[0]?.[1]).toEqual(
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: 'Bearer viewer-token',
        }),
      }),
    );
  });

  it('rejects malformed and oversized response collections', async () => {
    const malformed = fetchMock().mockResolvedValue(
      response(200, {
        items: [{ id: '', kind: 'video', title: 'Missing ID', progress: {} }],
      }),
    );
    const oversized = fetchMock().mockResolvedValue(
      response(200, {
        items: Array.from({ length: 201 }, (_, index) => ({
          id: `item-${index}`,
          kind: 'video',
          title: `Title ${index}`,
          progress: {},
        })),
      }),
    );

    await expect(
      new KinosailClient(
        'https://kino.example',
        'token',
        malformed,
      ).loadLibrary('recent'),
    ).rejects.toThrow('invalid response');
    await expect(
      new KinosailClient(
        'https://kino.example',
        'token',
        oversized,
      ).loadLibrary('recent'),
    ).rejects.toThrow('invalid response');
  });

  it('returns a direct playback source and saves ordered progress', async () => {
    const fetcher = fetchMock()
      .mockResolvedValueOnce(
        response(200, {
          plan: { allowed: true, mode: 'direct', reason: 'compatible' },
          direct: '/media/movie-1',
          directType: 'video/mp4',
          duration: 7200,
          start: 180,
          progressToken: 'timeline-token',
        }),
      )
      .mockResolvedValueOnce(
        response(200, { seconds: 240, session: 'play-1', revision: 3 }),
      );
    const client = new KinosailClient(
      'https://kino.example',
      'viewer-token',
      fetcher,
    );

    await expect(client.loadPlayback('movie-1')).resolves.toEqual(
      expect.objectContaining({
        uri: 'https://kino.example/media/movie-1',
        headers: { Authorization: 'Bearer viewer-token' },
      }),
    );
    await expect(
      client.saveProgress('movie-1', {
        seconds: 240,
        session: 'play-1',
        revision: 2,
        playbackToken: 'timeline-token',
      }),
    ).resolves.toEqual(expect.objectContaining({ seconds: 240, revision: 3 }));
    expect(fetcher.mock.calls).toEqual([
      [
        'https://kino.example/api/v1/items/movie-1/playback?videoCodecs=h264&audioCodecs=aac',
        {
          headers: {
            Accept: 'application/json',
            Authorization: 'Bearer viewer-token',
          },
        },
      ],
      [
        'https://kino.example/api/v1/items/movie-1/progress',
        {
          method: 'PUT',
          body: JSON.stringify({
            seconds: 240,
            session: 'play-1',
            revision: 2,
            playbackToken: 'timeline-token',
          }),
          headers: {
            Accept: 'application/json',
            'Content-Type': 'application/json',
            Authorization: 'Bearer viewer-token',
          },
        },
      ],
    ]);
  });

  it('does not substitute a compatibility stream for missing direct media', async () => {
    const fetcher = fetchMock().mockResolvedValue(
      response(200, {
        plan: { allowed: true, mode: 'transcode', reason: 'video-codec' },
        compatible: '/hls/movie-1/index.m3u8',
        compatiblePlan: { allowed: true, mode: 'transcode', reason: 'video-codec' },
      }),
    );

    await expect(
      new KinosailClient(
        'https://kino.example',
        'viewer-token',
        fetcher,
      ).loadPlayback('movie-1'),
    ).rejects.toThrow('Direct playback is not available');
  });
  it('separates permission to open the original from permission to convert', async () => {
    const fetcher = fetchMock().mockResolvedValue(
      response(200, {
        directAllowed: true,
        direct: '/media/movie-1',
        directType: 'video/x-matroska',
        plan: {
          allowed: false,
          mode: 'denied',
          reason: 'transcoding-not-allowed',
        },
      }),
    );
    const client = new KinosailClient(
      'https://kino.example',
      'viewer-token',
      fetcher,
    );
    await expect(client.loadPlayback('movie-1')).resolves.toMatchObject({
      uri: 'https://kino.example/media/movie-1',
    });
    expect(fetcher).toHaveBeenCalledTimes(1);
    fetcher.mockResolvedValue(
      response(200, {
        directAllowed: false,
        direct: '/media/movie-1',
        plan: { allowed: true, mode: 'direct', reason: 'compatible' },
      }),
    );
    await expect(client.loadPlayback('movie-1')).rejects.toThrow(
      'Direct playback is not available',
    );
  });
});

describe('series posters on portrait surfaces', () => {
  it.each(['/art/episode?variant=episode', ''])(
    'uses series artwork when episode artwork is %p',
    async (artwork) => {
      const item = {
        id: 'episode',
        kind: 'video',
        title: 'Pilot',
        show: 'Series',
        artwork,
        progress: {},
      };
      const fetcher = fetchMock().mockImplementation(async () =>
        response(200, { items: [item], total: 1, offset: 0, limit: 60 }),
      );
      const client = new KinosailClient(
        'https://kino.example',
        'viewer',
        fetcher,
      );
      expect((await client.loadLibrary('recent'))[0].artwork).toBe(
        '/art/episode',
      );
      expect(
        (await client.browseLibrary({ view: 'shows' })).items[0].artwork,
      ).toBe('/art/episode');
      fetcher.mockResolvedValue(response(200, { item }));
      expect((await client.loadItem('episode')).artwork).toBe('/art/episode');
    },
  );
  it('encodes an episode identifier as a single artwork path component', async () => {
    const fetcher = fetchMock().mockResolvedValue(
      response(200, {
        item: {
          id: 'part/one?x#y',
          kind: 'video',
          title: 'Pilot',
          show: 'Series',
        },
      }),
    );
    const client = new KinosailClient(
      'https://kino.example',
      'viewer',
      fetcher,
    );
    expect((await client.loadItem('episode')).artwork).toBe(
      '/art/part%2Fone%3Fx%23y',
    );
  });
  it.each([
    'https://other.example/art/episode',
    'https://user:password@kino.example/art/episode',
    'x'.repeat(2049),
  ])(
    'rejects unsafe supplied artwork before replacing it: %p',
    async (artwork) => {
      const fetcher = fetchMock().mockResolvedValue(
        response(200, {
          item: {
            id: 'episode',
            kind: 'video',
            title: 'Pilot',
            show: 'Series',
            artwork,
          },
        }),
      );
      await expect(
        new KinosailClient('https://kino.example', 'viewer', fetcher).loadItem(
          'episode',
        ),
      ).rejects.toThrow();
      expect(fetcher).toHaveBeenCalledTimes(1);
    },
  );
});

it.each(['.', '..'])(
  'rejects ambiguous TV poster identifier %p without an artwork request',
  async (id) => {
    const fetcher = fetchMock().mockResolvedValue(
      response(200, {
        item: { id, kind: 'video', title: 'Pilot', show: 'Series' },
      }),
    );
    await expect(
      new KinosailClient('https://kino.example', 'viewer', fetcher).loadItem(
        'episode',
      ),
    ).rejects.toThrow('invalid media ID');
    expect(fetcher).toHaveBeenCalledTimes(1);
  },
);
