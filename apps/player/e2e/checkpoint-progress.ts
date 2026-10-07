import {randomUUID} from "node:crypto";
import type {Page} from '@playwright/test';

// PlaybackState omits zero seconds. Present values must stay numeric; coercion
// would let malformed public responses satisfy the persistence oracle.
export function checkpointSeconds(state: unknown): number {
  if (!state || typeof state !== 'object' || Array.isArray(state)) throw new TypeError('Invalid public progress state');
  const record = state as Record<string, unknown>;
  const fields = ['seconds', 'readerOffset', 'readerPage', 'watched', 'dismissed', 'updated', 'session', 'revision'];
  if (Object.keys(state).length > fields.length || Object.keys(state).some(key => !fields.includes(key))) throw new TypeError('Invalid public progress fields');
  if (JSON.stringify(state).length > 8192) throw new TypeError('Oversized public progress state');
  for (const key of ['watched', 'dismissed']) if (Object.hasOwn(state, key) && typeof record[key] !== 'boolean') throw new TypeError('Invalid public progress flag');
  for (const key of ['readerOffset', 'readerPage', 'revision']) if (Object.hasOwn(state, key) &&
    (typeof record[key] !== 'number' || !Number.isFinite(record[key]) || record[key] < 0 || record[key] > Number.MAX_SAFE_INTEGER || key !== 'readerOffset' && !Number.isInteger(record[key]))) throw new TypeError('Invalid public progress number');
  for (const key of ['session', 'updated']) if (Object.hasOwn(state, key) && (typeof record[key] !== 'string' || record[key].length > 64)) throw new TypeError('Invalid public progress metadata');
  const seconds = Object.hasOwn(state, 'seconds') ? (state as {seconds: unknown}).seconds : 0;
  if (typeof seconds !== 'number' || !Number.isFinite(seconds) || seconds < 0 || seconds > 31536000) throw new TypeError('Invalid public progress seconds');
  return seconds;
}

// Only independent saved-position cases prepare this baseline. The repeated
// Library durability journey retains the previous iteration's public state.
export async function prepareSavedPositionBaseline(page: Page, watch: string) {
  if (typeof watch !== 'string' || !/^\/watch\/[a-f0-9]{16}$/.test(watch)) throw new TypeError('Invalid fixture watch path');
  const csrf = await page.locator('meta[name="kinosail-csrf"]').getAttribute('content');
  if (typeof csrf !== 'string' || !/^[A-Za-z0-9_-]{1,512}$/.test(csrf)) throw new TypeError('Invalid fixture CSRF');
  const path = `/api/v1/items/${watch.slice(7)}`;
  const before = await page.request.get(path);
  if (before.status() !== 200) throw new Error('Fixture progress read rejected');
  const previous = (await before.json()).item.progress;
  const beforeSeconds = checkpointSeconds(previous);
  const session = randomUUID();
  const saved = await page.request.put(path + '/progress', {
    headers: {Origin: new URL(page.url()).origin, 'X-Kinosail-CSRF': csrf},
    data: {seconds: 0, watched: false, session, revision: 1},
  });
  if (saved.status() !== 200) throw new Error('Fixture progress preparation rejected');
  const after = await page.request.get(path);
  if (after.status() !== 200) throw new Error('Fixture prepared progress read rejected');
  const accepted = (await after.json()).item.progress, afterSeconds = checkpointSeconds(accepted);
  if (afterSeconds !== 0 || accepted.watched === true || accepted.session !== session || accepted.revision !== 1) throw new Error('Fixture baseline was not accepted');
  return {beforeSeconds, afterSeconds};
}
