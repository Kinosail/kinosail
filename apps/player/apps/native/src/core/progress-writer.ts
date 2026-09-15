import type { Progress } from './contract';

type ProgressInput = Pick<Progress, 'seconds' | 'session' | 'revision'> & {
  playbackToken?: string;
  watched?: boolean;
};
type Position = Pick<Progress, 'seconds'> & { watched?: boolean };
export function createProgressWriter(
  initial: Omit<ProgressInput, 'seconds'>,
  save: (progress: ProgressInput) => Promise<Progress>,
) {
  let revision = Math.max(1, initial.revision + 1),
    pending: Position | null = null,
    writing: Promise<void> | null = null,
    completed = false;
  const flush = async () => {
    while (pending) {
      const position = pending;
      pending = null;
      const event = { ...initial, ...position, revision: revision++ };
      try {
        const saved = await save(event);
        revision = Math.max(revision, saved.revision + 1);
      } catch {
        if (pending) return true;
        pending = position;
        return false;
      }
    }
    return true;
  };
  const start = () => {
    if (writing) return;
    let follow = false;
    writing = flush()
      .then((value) => {
        follow = value;
      })
      .finally(() => {
        writing = null;
        if (follow && pending) start();
      });
  };
  return {
    write(seconds: number, watched?: boolean) {
      if (!Number.isFinite(seconds) || seconds < 0 || seconds > 31_536_000)
        return;
      if (watched !== undefined) completed = watched;
      pending = { seconds, watched: completed };
      start();
    },
    async drain() {
      while (writing) await writing;
    },
    retry() {
      if (pending) start();
    },
  };
}
