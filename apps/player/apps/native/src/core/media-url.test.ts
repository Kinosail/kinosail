import { KinosailClient, type Fetcher } from './server-client';

const invalid = 'Kinosail Server returned an invalid media URL.';
const base = 'https://kino.example';
const item = {
  id: 'arrival',
  kind: 'video',
  title: 'Arrival',
  artwork: '',
  backdrop: '',
};
const reply = (body: object) =>
  new Response(JSON.stringify(body), { status: 200 });

describe('authenticated native media URLs', () => {
  it('accepts a same-origin path at the exact length limit', () => {
    const path = `/${'a'.repeat(2047)}`;
    expect(new KinosailClient(base).mediaURL(path)).toBe(`${base}${path}`);
  });

  it.each([null, undefined, 0, {}, 'x'.repeat(2049)])(
    'rejects malformed or oversized media input %#',
    (value) => {
      const client = new KinosailClient(base);
      expect(() => client.mediaURL(value as string)).toThrow(invalid);
    },
  );
  it.each([
    'https://other.example/media/arrival',
    '//other.example/art/arrival',
    'http://kino.example/media/arrival',
    'https://kino.example:444/media/arrival',
    'https://user:password@kino.example/media/arrival',
    'file:///media/arrival',
    'data:text/plain,media',
    'https://[invalid',
  ])('rejects an invalid destination %# without network calls', (path) => {
    const fetcher = jest.fn<ReturnType<Fetcher>, Parameters<Fetcher>>();
    const client = new KinosailClient(base, 'viewer-token', fetcher);
    expect(() => client.mediaURL(path)).toThrow(invalid);
    expect(fetcher).not.toHaveBeenCalled();
  });

  it.each([
    ['', ''],
    ['/art/arrival', `${base}/art/arrival`],
    [
      'media/arrival?quality=original',
      `${base}/media/arrival?quality=original`,
    ],
    [`${base}/media/arrival`, `${base}/media/arrival`],
    ['https://KINO.example:443/art/arrival', `${base}/art/arrival`],
  ])('preserves valid relative and same-origin URLs %#', (path, expected) => {
    expect(new KinosailClient(base).mediaURL(path)).toBe(expected);
  });

  it.each(['artwork', 'backdrop'])(
    'rejects invalid %s before publishing item or shelf data',
    async (field) => {
      const media = { ...item, [field]: 'https://other.example/image' };
      const fetcher = jest
        .fn<ReturnType<Fetcher>, Parameters<Fetcher>>()
        .mockResolvedValueOnce(reply({ item: media }))
        .mockResolvedValueOnce(reply({ items: [media] }));
      const client = new KinosailClient(base, 'viewer-token', fetcher);
      await expect(client.loadItem('arrival')).rejects.toThrow(invalid);
      await expect(client.loadLibrary('recent')).rejects.toThrow(invalid);
      expect(fetcher).toHaveBeenCalledTimes(2);
      for (const [url] of fetcher.mock.calls)
        expect(new URL(url).origin).toBe(base);
    },
  );

  it('rejects external playback before returning an authenticated player source', async () => {
    const fetcher = jest
      .fn<ReturnType<Fetcher>, Parameters<Fetcher>>()
      .mockResolvedValue(
        reply({
          plan: { allowed: true, mode: 'direct', reason: 'compatible' },
          direct: 'https://other.example/video',
          directType: 'video/mp4',
        }),
      );
    const client = new KinosailClient(base, 'viewer-token', fetcher);
    await expect(client.loadPlayback('arrival')).rejects.toThrow(invalid);
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(new URL(fetcher.mock.calls[0][0]).origin).toBe(base);
  });
});
