import type { KeyValueStorage } from './session-store';

const memory = new Map<string, string>();
const available = () => typeof globalThis.sessionStorage !== 'undefined';

export const platformStorage: KeyValueStorage = {
  async get(key) {
    return available()
      ? globalThis.sessionStorage.getItem(key)
      : (memory.get(key) ?? null);
  },
  async remove(key) {
    if (available()) globalThis.sessionStorage.removeItem(key);
    memory.delete(key);
  },
  async set(key, value) {
    if (available()) globalThis.sessionStorage.setItem(key, value);
    else memory.set(key, value);
  },
};
