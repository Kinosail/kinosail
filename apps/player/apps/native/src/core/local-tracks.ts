import type { VLCPlayerTracks } from '@/components/local-video.types';

export function parseLocalTracks(value: unknown): VLCPlayerTracks | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  const fields = value as Record<string, unknown>;
  if (
    Object.keys(fields).some(
      (key) =>
        !['audio', 'audioIndex', 'subtitle', 'subtitleIndex'].includes(key),
    )
  )
    return null;
  for (const kind of ['audio', 'subtitle'] as const) {
    const entries = fields[kind];
    const selected = fields[`${kind}Index`];
    if (
      !Array.isArray(entries) ||
      entries.length > 128 ||
      !Number.isSafeInteger(selected)
    )
      return null;
    const ids = new Set<number>();
    for (const entry of entries) {
      if (
        !entry ||
        typeof entry !== 'object' ||
        Array.isArray(entry) ||
        Object.keys(entry).some(
          (key) => !['id', 'name', 'language'].includes(key),
        ) ||
        !Number.isSafeInteger(entry.id) ||
        entry.id < 0 ||
        entry.id >= 128 ||
        ids.has(entry.id) ||
        typeof entry.name !== 'string' ||
        entry.name.length > 512 ||
        (entry.language !== undefined &&
          (typeof entry.language !== 'string' ||
            entry.language.length > 32 ||
            !/^[-A-Za-z0-9]*$/.test(entry.language)))
      )
        return null;
      ids.add(entry.id);
    }
    if (selected !== -1 && !ids.has(selected as number)) return null;
  }
  return fields as VLCPlayerTracks;
}
