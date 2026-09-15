import { requireOptionalNativeModule } from 'expo';
import { Platform } from 'react-native';
import { normalizeServerURL } from './server-client';

export type NearbyServer = { name: string; url: string };
const native =
  Platform.OS === 'ios' || Platform.OS === 'android'
    ? requireOptionalNativeModule<{
        scan(): Promise<unknown>;
        stop(): Promise<void>;
      }>('ServerDiscovery')
    : null;
export const serverDiscoveryAvailable = typeof native?.scan === 'function';

export function validateDiscoveredServers(value: unknown): NearbyServer[] {
  if (!Array.isArray(value) || value.length > 32) return [];
  const seen = new Set<string>();
  return value.flatMap((entry): NearbyServer[] => {
    if (
      !entry ||
      typeof entry !== 'object' ||
      Array.isArray(entry) ||
      Object.keys(entry).some((key) => key !== 'name' && key !== 'url') ||
      typeof entry.name !== 'string' ||
      !entry.name.trim() ||
      entry.name.length > 63 ||
      /[\u0000-\u001f\u007f-\u009f]/.test(entry.name) ||
      typeof entry.url !== 'string' ||
      entry.url.length > 240 ||
      /[\u0000-\u0020\u007f\\?#]/.test(entry.url)
    )
      return [];
    try {
      const url = normalizeServerURL(entry.url);
      const host = new URL(url).hostname;
      if (
        host === 'localhost' ||
        host === '[::1]' ||
        host.startsWith('127.') ||
        host === '0.0.0.0' ||
        host === '[::]' ||
        seen.has(url)
      )
        return [];
      seen.add(url);
      return [{ name: entry.name.trim(), url }];
    } catch {
      return [];
    }
  });
}

export async function discoverServers(): Promise<NearbyServer[]> {
  return validateDiscoveredServers(await native?.scan());
}
export async function stopServerDiscovery(): Promise<void> {
  await native?.stop();
}
