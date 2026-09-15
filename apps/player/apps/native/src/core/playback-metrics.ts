export type PlaybackSample = {
  engine: 'platform' | 'vlc';
  outcome: 'playing' | 'ended' | 'error' | 'closed';
  firstFrameMs: number | null;
  firstProgressMs: number | null;
  seekProgressMs: number[];
  seeks: number;
  incompleteSeeks: number;
  stalls: number;
  stallMs: number;
  elapsedMs: number;
};
const samples: PlaybackSample[] = [];
// Session-local, bounded, and deliberately free of titles, URLs, identifiers, and credentials.
export const playbackMetrics = () =>
  samples.map((sample) => ({
    ...sample,
    seekProgressMs: [...sample.seekProgressMs],
  }));
export const clearPlaybackMetrics = () => {
  samples.length = 0;
};

export function measurePlayback(
  engine: PlaybackSample['engine'],
  start: number,
  now = () => performance.now(),
) {
  if (
    !['platform', 'vlc'].includes(engine) ||
    !Number.isFinite(start) ||
    start < 0
  )
    throw new Error('Invalid playback measurement.');
  const sample: PlaybackSample = {
    engine,
    outcome: 'playing',
    firstFrameMs: null,
    firstProgressMs: null,
    seekProgressMs: [],
    seeks: 0,
    incompleteSeeks: 0,
    stalls: 0,
    stallMs: 0,
    elapsedMs: 0,
  };
  let stalledAt: number | null = null;
  let closed = false;
  let previousPosition: number | null = null;
  let seeking: { target: number; at: number; reached: number | null } | null =
    null;
  const elapsed = () => Math.max(0, now() - start);
  const stopStall = () => {
    if (stalledAt !== null)
      sample.stallMs += Math.max(0, elapsed() - stalledAt);
    stalledAt = null;
  };
  return {
    seek(target: number) {
      if (!Number.isFinite(target) || target < 0 || target > 31_536_000)
        throw new Error('Invalid playback seek measurement.');
      if (closed) return;
      stopStall();
      if (seeking) sample.incompleteSeeks++;
      sample.seeks++;
      seeking = { target, at: elapsed(), reached: null };
    },
    // Timeline advancement is observable on both engines, but is deliberately
    // distinct from a rendered frame. Never call this seek-to-picture latency.
    position(seconds: number) {
      if (
        closed ||
        !Number.isFinite(seconds) ||
        seconds < 0 ||
        seconds > 31_536_000
      )
        return;
      if (seeking) {
        if (Math.abs(seconds - seeking.target) <= 1 && seeking.reached === null)
          seeking.reached = seconds;
        else if (
          seeking.reached !== null &&
          seconds > seeking.reached &&
          seconds <= seeking.target + 5
        ) {
          sample.seekProgressMs.push(Math.max(0, elapsed() - seeking.at));
          if (sample.seekProgressMs.length > 128) sample.seekProgressMs.shift();
          seeking = null;
        }
      } else if (
        previousPosition !== null &&
        seconds > previousPosition &&
        seconds - previousPosition <= 5
      ) {
        if (sample.firstProgressMs === null) sample.firstProgressMs = elapsed();
        stopStall();
      }
      previousPosition = seconds;
    },
    firstFrame() {
      if (!closed && sample.firstFrameMs === null)
        sample.firstFrameMs = elapsed();
    },
    buffering(active: boolean) {
      if (
        closed ||
        seeking ||
        (sample.firstFrameMs === null && sample.firstProgressMs === null)
      )
        return;
      if (!active) stopStall();
      else if (stalledAt === null) {
        stalledAt = elapsed();
        sample.stalls++;
      }
    },
    finish(outcome: Exclude<PlaybackSample['outcome'], 'playing'>) {
      if (!['ended', 'error', 'closed'].includes(outcome))
        throw new Error('Invalid playback outcome.');
      if (closed) return;
      stopStall();
      if (seeking) sample.incompleteSeeks++;
      closed = true;
      sample.elapsedMs = elapsed();
      sample.outcome = outcome;
      samples.push({ ...sample });
      if (samples.length > 100) samples.shift();
    },
  };
}
