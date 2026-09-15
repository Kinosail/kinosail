import type { KinosailClient } from './server-client';
import { experienceCacheKey } from './experience-cache';
import { platformStorage } from './platform-storage';
export async function readMediaState(
  client: KinosailClient,
  name: string,
): Promise<string | null> {
  return platformStorage.get(await experienceCacheKey(client, name));
}
export async function writeMediaState(
  client: KinosailClient,
  name: string,
  raw: string,
) {
  if (raw.length > 262144) throw new Error('Saved media state is too large.');
  await platformStorage.set(await experienceCacheKey(client, name), raw);
}
