import type { DownloadEntry } from './downloads.types';

type Sample = { bytes: number; time: number };

// Only observed progress in this viewing session contributes to an estimate.
// Persisted bytes and time spent paused must never look like transfer speed.
export function downloadProgressTracker() {
  const samples = new Map<string, Sample[]>();
  return (entries: DownloadEntry[], now = performance.now()) => {
    const labels: Record<string, string> = {};
    for (const id of samples.keys()) {
      if (!entries.some((entry) => entry.item.id === id)) samples.delete(id);
    }
    for (const entry of entries) {
      const id = entry.item.id;
      if (entry.status !== 'downloading' || entry.bytes >= entry.total) {
        samples.delete(id);
        continue;
      }
      let history = samples.get(id) ?? [];
      const last = history.at(-1);
      if (last && (entry.bytes < last.bytes || now <= last.time || now - last.time > 15000)) history = [];
      history.push({ bytes: entry.bytes, time: now });
      while (history.length > 1 && now - history[0].time > 10000) history.shift();
      samples.set(id, history);
      const elapsed = now - history[0].time;
      const bytes = entry.bytes - history[0].bytes;
      const changed = [...history].reverse().find((sample) => sample.bytes < entry.bytes);
      if (elapsed < 2000 || bytes <= 0 || !changed || now - changed.time > 5000) continue;
      const speed = bytes * 1000 / elapsed;
      const seconds = Math.ceil((entry.total - entry.bytes) / speed);
      const remaining = seconds < 60 ? 'Less than a minute left'
        : seconds < 3600 ? `About ${Math.ceil(seconds / 60)} min left`
          : `About ${Math.ceil(seconds / 3600)} hr left`;
      const rate = speed >= 1024 ** 2 ? `${(speed / 1024 ** 2).toFixed(1)} MB/s`
        : `${Math.max(1, Math.round(speed / 1024))} KB/s`;
      labels[id] = `${rate} · ${remaining}`;
    }
    return labels;
  };
}
