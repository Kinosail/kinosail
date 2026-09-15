import {
  clearPlaybackMetrics,
  measurePlayback,
  playbackMetrics,
} from './playback-metrics';
afterEach(clearPlaybackMetrics);
it('separates startup from stalls and closes each sample once', () => {
  let time = 100;
  const sample = measurePlayback('platform', 100, () => time);
  sample.buffering(true);
  time = 500;
  sample.firstFrame();
  time = 1000;
  sample.buffering(true);
  sample.buffering(true);
  time = 1500;
  sample.buffering(false);
  time = 2000;
  sample.finish('ended');
  sample.finish('closed');
  expect(playbackMetrics()).toEqual([
    {
      engine: 'platform',
      outcome: 'ended',
      firstFrameMs: 400,
      firstProgressMs: null,
      seekProgressMs: [],
      seeks: 0,
      incompleteSeeks: 0,
      stalls: 1,
      stallMs: 500,
      elapsedMs: 1900,
    },
  ]);
});

it('requires advancement after reaching a seek target and excludes seeking from stalls', () => {
  let time = 0;
  const sample = measurePlayback('platform', 0, () => time);
  sample.firstFrame();
  sample.position(5);
  time = 100;
  sample.seek(60);
  sample.buffering(true);
  sample.position(5);
  time = 200;
  sample.position(60);
  sample.position(60);
  time = 450;
  sample.position(60.5);
  sample.finish('ended');
  expect(playbackMetrics()[0]).toMatchObject({
    seeks: 1,
    seekProgressMs: [350],
    incompleteSeeks: 0,
    stalls: 0,
  });
  const copy = playbackMetrics();
  copy[0].seekProgressMs.push(999);
  expect(playbackMetrics()[0].seekProgressMs).toEqual([350]);
});
it('keeps abandoned and incomplete seeks instead of reporting only successes', () => {
  const sample = measurePlayback('vlc', 0, () => 100);
  sample.seek(10);
  sample.seek(20);
  sample.position(20);
  sample.finish('closed');
  expect(playbackMetrics()[0]).toMatchObject({
    seeks: 2,
    incompleteSeeks: 2,
    seekProgressMs: [],
  });
});
it('does not confuse a seek jump or invalid position with moving playback', () => {
  let time = 0;
  const sample = measurePlayback('platform', 0, () => time);
  sample.position(10);
  sample.position(NaN);
  sample.position(Infinity);
  sample.position(-1);
  sample.position(100);
  time = 500;
  sample.position(101);
  sample.finish('ended');
  expect(playbackMetrics()[0]).toMatchObject({
    firstFrameMs: null,
    firstProgressMs: 500,
  });
});
it.each([-1, NaN, Infinity, 31_536_001])(
  'rejects invalid seek measurement %p without changing history',
  (target) => {
    const sample = measurePlayback('platform', 0, () => 0);
    expect(() => sample.seek(target)).toThrow();
    sample.finish('closed');
    expect(playbackMetrics()[0]).toMatchObject({
      seeks: 0,
      incompleteSeeks: 0,
      seekProgressMs: [],
    });
  },
);
it('bounds per-attempt seek history without losing the total', () => {
  const sample = measurePlayback('vlc', 0, () => 0);
  for (let n = 0; n < 130; n++) {
    sample.seek(5);
    sample.position(5);
    sample.position(5.5);
  }
  sample.finish('ended');
  expect(playbackMetrics()[0].seekProgressMs).toHaveLength(128);
  expect(playbackMetrics()[0].seeks).toBe(130);
});
it('retains failures without inventing a first frame and bounds the history', () => {
  for (let n = 0; n < 101; n++)
    measurePlayback('vlc', 0, () => n).finish('error');
  const values = playbackMetrics();
  expect(values).toHaveLength(100);
  expect(values[0].firstFrameMs).toBeNull();
  values[0].stalls = 100;
  expect(playbackMetrics()[0].stalls).toBe(0);
});
it.each([-1, NaN, Infinity])('rejects an invalid start %p', (start) => {
  expect(() => measurePlayback('platform', start)).toThrow();
  expect(playbackMetrics()).toEqual([]);
});
