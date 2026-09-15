import { acknowledgeDownloadedProgress } from './downloads';
import type { KinosailClient } from './server-client';
import type { Progress } from './contract';
import { validateSyncProgress } from './progress-validation';
import { exactObject } from './media-preferences';
import { readMediaState, writeMediaState } from './media-state-storage';
import { validateDownloadID } from './download-policy';

export type ProgressInput = Pick<
  Progress,
  'seconds' | 'session' | 'revision'
> & { watched?: boolean; playbackToken?: string };
export type PendingProgress = {
  id: string;
  title: string;
  expected: Progress;
  progress: Progress;
  playbackToken: string;
  conflict: boolean;
};
const invalid = () => new Error('Saved playback progress could not be read.');
export function parsePendingProgress(raw: string | null): PendingProgress[] {
  if (raw === null) return [];
  if (raw.length > 262144) throw invalid();
  const values: unknown = JSON.parse(raw);
  if (!Array.isArray(values) || values.length > 50) throw invalid();
  const seen = new Set<string>();
  return values.map((value) => {
    const v = exactObject(value, [
      'id',
      'title',
      'expected',
      'progress',
      'playbackToken',
      'conflict',
    ]);
    if (
      typeof v.id !== 'string' ||
      typeof v.title !== 'string' ||
      v.title.length > 512 ||
      /[\u0000-\u001f\u007f]/u.test(v.title) ||
      typeof v.playbackToken !== 'string' ||
      v.playbackToken.length > 8192 ||
      typeof v.conflict !== 'boolean' ||
      seen.has(v.id)
    )
      throw invalid();
    validateDownloadID(v.id);
    seen.add(v.id);
    return {
      id: v.id,
      title: v.title,
      expected: validateSyncProgress(v.expected, false),
      progress: validateSyncProgress(v.progress, true),
      playbackToken: v.playbackToken,
      conflict: v.conflict,
    };
  });
}
let mutations = Promise.resolve();
const serial = <T>(operation: () => Promise<T>): Promise<T> => {
  const result = mutations.then(operation);
  mutations = result.then(
    () => {},
    () => {},
  );
  return result;
};
const read = (client: KinosailClient) =>
  readMediaState(client, 'progress-outbox').then(parsePendingProgress);
const write = (client: KinosailClient, values: PendingProgress[]) =>
  writeMediaState(client, 'progress-outbox', JSON.stringify(values));
export const pendingProgress = (client: KinosailClient) =>
  serial(() => read(client));

async function send(
  client: KinosailClient,
  entry: PendingProgress,
  values: PendingProgress[],
): Promise<Progress> {
  try {
    let result = await client.syncProgress(
      entry.id,
      entry.progress,
      entry.expected,
      entry.playbackToken,
    );
    // Retry once against the returned baseline; leave further races queued.
    if (result.conflict && entry.progress.seconds > result.progress.seconds) {
      entry.expected = result.progress;
      entry.conflict = false;
      await write(client, values);
      result = await client.syncProgress(
        entry.id,
        entry.progress,
        entry.expected,
        entry.playbackToken,
      );
    }
    if (result.conflict && entry.progress.seconds > result.progress.seconds) {
      entry.expected = result.progress;
      entry.conflict = false;
      await write(client, values);
      return entry.progress;
    }
    await acknowledgeDownloadedProgress(client, entry.id, result.progress);
    await write(
      client,
      values.filter((value) => value.id !== entry.id),
    );
    return result.progress;
  } catch {
    return entry.progress;
  }
}

// Persist before the network attempt. A successful local save is useful while
// offline; conflicts automatically keep the furthest saved position.
export function saveSyncedProgress(
  client: KinosailClient,
  id: string,
  title: string,
  expected: Progress,
  input: ProgressInput,
): Promise<Progress> {
  validateDownloadID(id);
  const progress = validateSyncProgress(
    {
      seconds: input.seconds,
      session: input.session,
      revision: input.revision,
      watched: input.watched ?? false,
    },
    true,
  );
  validateSyncProgress(expected, false);
  if (
    typeof title !== 'string' ||
    title.length > 512 ||
    /[\u0000-\u001f\u007f]/u.test(title) ||
    typeof (input.playbackToken ?? '') !== 'string' ||
    (input.playbackToken ?? '').length > 8192
  )
    throw invalid();
  return serial(async () => {
    const values = await read(client),
      existing = values.find((value) => value.id === id);
    if (
      existing &&
      existing.progress.session === progress.session &&
      existing.progress.revision >= progress.revision
    )
      return existing.progress;
    if (!existing && values.length >= 50)
      throw new Error('Sync saved progress before starting more titles.');
    const entry: PendingProgress = {
      id,
      title,
      expected: existing?.expected ?? expected,
      progress,
      playbackToken: input.playbackToken ?? '',
      conflict: false,
    };
    const next = values.filter((value) => value.id !== id);
    next.push(entry);
    await write(client, next);
    return send(client, entry, next);
  });
}
export function flushProgress(client: KinosailClient, id?: string) {
  if (id !== undefined) validateDownloadID(id);
  return serial(async () => {
    const values = await read(client);
    for (const entry of values) {
      if (id === undefined || entry.id === id) {
        const current = await read(client);
        const pending = current.find((value) => value.id === entry.id);
        if (pending) await send(client, pending, current);
      }
    }
    return read(client);
  });
}
export const withPendingProgress = <T>(
  client: KinosailClient,
  operation: (values: PendingProgress[]) => Promise<T>,
) => serial(async () => operation(await read(client)));
