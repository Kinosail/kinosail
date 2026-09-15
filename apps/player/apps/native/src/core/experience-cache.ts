import * as Crypto from 'expo-crypto';
import { platformStorage } from './platform-storage';
import type { KinosailClient } from './server-client';

export async function experienceCacheKey(
  client: KinosailClient,
  scope: string,
) {
  const digest = await Crypto.digestStringAsync(
    Crypto.CryptoDigestAlgorithm.SHA256,
    `${client.baseURL}\n${client.authorizationHeaders().Authorization}\n${scope}`,
  );
  return `kinosail.media.${digest}`;
}
export async function readExperienceCache<T>(
  client: KinosailClient,
  scope: string,
  parse: (value: unknown) => T,
): Promise<T | null> {
  const raw = await platformStorage.get(
    await experienceCacheKey(client, scope),
  );
  if (raw === null) return null;
  if (raw.length > 16384)
    throw new Error('Saved media preferences could not be read.');
  return parse(JSON.parse(raw));
}
export async function writeExperienceCache(
  client: KinosailClient,
  scope: string,
  value: unknown,
) {
  const raw = JSON.stringify(value);
  if (raw.length > 16384)
    throw new Error('Saved media preferences are too large.');
  await platformStorage.set(await experienceCacheKey(client, scope), raw);
}
