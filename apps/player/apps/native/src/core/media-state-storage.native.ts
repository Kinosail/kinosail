import { Directory, File, Paths } from 'expo-file-system';
import type { KinosailClient } from './server-client';
import { experienceCacheKey } from './experience-cache';
import { prepareOfflineStorage } from './protected-media';
const fileFor = async (client: KinosailClient, name: string) =>
  new File(
    new Directory(Paths.document, 'kinosail-offline', 'state'),
    `${await experienceCacheKey(client, name)}.json`,
  );
export async function readMediaState(
  client: KinosailClient,
  name: string,
): Promise<string | null> {
  const file = await fileFor(client, name);
  if (!file.exists) return null;
  if (file.size > 262144) throw new Error('Saved media state is too large.');
  return file.textSync();
}
export async function writeMediaState(
  client: KinosailClient,
  name: string,
  raw: string,
) {
  if (raw.length > 262144) throw new Error('Saved media state is too large.');
  await prepareOfflineStorage();
  const directory = new Directory(Paths.document, 'kinosail-offline', 'state');
  directory.create({ intermediates: true, idempotent: true });
  const file = await fileFor(client, name),
    pending = new File(directory, `${file.name}.next`);
  pending.write(raw);
  pending.moveSync(file, { overwrite: true });
}
