import * as SecureStore from 'expo-secure-store';

import type { KeyValueStorage } from './session-store';

export const platformStorage: KeyValueStorage = {
  get: (key) => SecureStore.getItemAsync(key),
  remove: (key) => SecureStore.deleteItemAsync(key),
  set: (key, value) => SecureStore.setItemAsync(key, value),
};
