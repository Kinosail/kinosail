import { parsePlayback } from './contract';
import type { PlaybackSource } from './contract';
import { compatiblePlaybackSource } from './video-source';
import { KinosailClient } from './server-client';
import { fetchMock, response } from '../testing/server-client-test-helpers';

const playback = {
  plan: { allowed: true, mode: 'direct', reason: 'direct-preferred' },
  direct: '/media/movie',
  directType: 'video/mp4',
  duration: 120,
  start: 10,
  compatible: '/hls/movie/p/t-a0-s0-none-t0-b0/index.m3u8',
  compatibleDuration: 120,
  compatiblePlan: {
    allowed: true,
    mode: 'transcode',
    reason: 'video-codec-unsupported',
  },
};

describe('explicit native compatibility recovery', () => {
  it('rejects an unknown plan rather than relabeling it as original playback', () => {
    expect(() =>
      parsePlayback({
        ...playback,
        plan: { ...playback.plan, mode: 'unknown' },
      }),
    ).toThrow('invalid response');
  });
  it('describes the selected original even when the Server prefers a rendition', async () => {
    const fetcher = fetchMock().mockResolvedValue(
      response(200, {
        ...playback,
        directAllowed: true,
        plan: { allowed: true, mode: 'remux', reason: 'container-unsupported' },
      }),
    );
    const client = new KinosailClient(
      'https://kino.example',
      'viewer-token',
      fetcher,
    );
    const source = await client.loadPlayback('movie');
    expect(source.uri).toBe('https://kino.example/media/movie');
    expect(source.plan.mode).toBe('direct');
    expect(source.compatible?.plan.mode).toBe('transcode');
    expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it('retains a validated compatible source without requesting media', async () => {
    const fetcher = fetchMock().mockResolvedValue(response(200, playback));
    const client = new KinosailClient(
      'https://kino.example',
      'viewer-token',
      fetcher,
    );
    const source = await client.loadPlayback('movie');
    expect(source.uri).toBe('https://kino.example/media/movie');
    expect(source.compatible?.uri).toBe(
      `https://kino.example${playback.compatible}`,
    );
    expect(source.compatible?.plan.mode).toBe('transcode');
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(fetcher.mock.calls[0][0]).toBe(
      'https://kino.example/api/v1/items/movie/playback?videoCodecs=h264&audioCodecs=aac',
    );
  });

  it.each([
    'https://other.example/hls/movie',
    'https://user:password@kino.example/hls/movie',
  ])('rejects an unsafe recovery URL', async (compatible) => {
    const fetcher = fetchMock().mockResolvedValue(
      response(200, { ...playback, compatible }),
    );
    const client = new KinosailClient(
      'https://kino.example',
      'viewer-token',
      fetcher,
    );
    await expect(client.loadPlayback('movie')).rejects.toThrow(
      'invalid media URL',
    );
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it('keeps conversion unavailable when the server denies it', () => {
    expect(
      parsePlayback({
        ...playback,
        compatiblePlan: {
          allowed: false,
          mode: 'denied',
          reason: 'transcoding-not-allowed',
        },
      }).compatible,
    ).toBeUndefined();
  });

  it('preserves a bounded server timeline for progress and resume', () => {
    const source = parsePlayback({
      ...playback,
      compatibleDuration: 110,
      compatibleProgressToken: 'timeline-token',
      compatiblePlan: {
        ...playback.compatiblePlan,
        timeline: {
          sourceDuration: 120,
          duration: 110,
          omitted: [{ start: 0, end: 10 }],
        },
      },
    });
    expect(source.compatible?.omitted).toEqual([{ start: 0, end: 10 }]);
    expect(source.compatible?.duration).toBe(110);
    expect(source.compatible?.progressToken).toBe('timeline-token');
  });

  it.each(
    [
      [{ start: 5, end: 4 }],
      [{ start: -1, end: 4 }],
      [{ start: 0, end: 121 }],
      [
        { start: 0, end: 10 },
        { start: 9, end: 12 },
      ],
      Array.from({ length: 129 }, (_, i) => ({ start: i, end: i + 1 })),
    ].map((omitted) => ({ omitted })),
  )('rejects malformed recovery timelines', ({ omitted }) => {
    expect(() =>
      parsePlayback({
        ...playback,
        compatibleProgressToken: 'token',
        compatiblePlan: {
          ...playback.compatiblePlan,
          timeline: { sourceDuration: 120, duration: 110, omitted },
        },
      }),
    ).toThrow('invalid response');
  });
});

it('maps recovery time and cannot silently retry the recovered stream', () => {
  const source: PlaybackSource = {
    uri: 'https://kino.example/media/movie',
    headers: { Authorization: 'Bearer token' },
    contentType: 'video/mp4',
    duration: 120,
    start: 0,
    progressToken: '',
    plan: playback.plan,
    compatible: {
      uri: 'https://kino.example/hls/movie/index.m3u8',
      duration: 100,
      progressToken: 'timeline',
      plan: playback.compatiblePlan,
      omitted: [
        { start: 0, end: 10 },
        { start: 50, end: 60 },
      ],
    },
  };
  const recovered = compatiblePlaybackSource(source, 70);
  expect(recovered?.start).toBe(50);
  expect(recovered?.headers).toEqual(source.headers);
  expect(recovered?.contentType).toBe('application/vnd.apple.mpegurl');
  expect(recovered?.progressToken).toBe('timeline');
  expect(recovered && compatiblePlaybackSource(recovered, 50)).toBeNull();
  expect(compatiblePlaybackSource(source, Number.NaN)?.start).toBe(0);
  expect(source.compatible).toBeDefined();
});
