import * as SecureStore from 'expo-secure-store';

import { platformStorage as nativeStorage } from './platform-storage.native';
import { platformStorage as browserStorage } from './platform-storage.ts';

jest.mock('expo-secure-store', () => ({
  getItemAsync: jest.fn(),
  deleteItemAsync: jest.fn(),
  setItemAsync: jest.fn(),
}));

describe('platform session storage', () => {
  const original = Object.getOwnPropertyDescriptor(
    globalThis,
    'sessionStorage',
  );

  afterEach(() => {
    if (original) Object.defineProperty(globalThis, 'sessionStorage', original);
    else Reflect.deleteProperty(globalThis, 'sessionStorage');
    jest.resetAllMocks();
  });

  it('keeps sessions in memory when browser storage is unavailable', async () => {
    Object.defineProperty(globalThis, 'sessionStorage', {
      configurable: true,
      value: undefined,
    });
    await expect(browserStorage.get('memory-session')).resolves.toBeNull();
    await browserStorage.set('memory-session', 'saved-session');
    await expect(browserStorage.get('memory-session')).resolves.toBe(
      'saved-session',
    );
    await browserStorage.remove('memory-session');
    await expect(browserStorage.get('memory-session')).resolves.toBeNull();
  });

  it('uses browser session storage and clears a previous memory fallback', async () => {
    Object.defineProperty(globalThis, 'sessionStorage', {
      configurable: true,
      value: undefined,
    });
    await browserStorage.set('browser-session', 'old-session');
    const values = new Map<string, string>();
    const storage = {
      getItem: jest.fn((key: string) => values.get(key) ?? null),
      removeItem: jest.fn((key: string) => {
        values.delete(key);
      }),
      setItem: jest.fn((key: string, value: string) => {
        values.set(key, value);
      }),
    };
    Object.defineProperty(globalThis, 'sessionStorage', { value: storage });
    await expect(browserStorage.get('browser-session')).resolves.toBeNull();
    await browserStorage.set('browser-session', 'current-session');
    await expect(browserStorage.get('browser-session')).resolves.toBe(
      'current-session',
    );
    expect(storage.setItem).toHaveBeenCalledWith(
      'browser-session',
      'current-session',
    );
    await browserStorage.remove('browser-session');
    expect(storage.removeItem).toHaveBeenCalledWith('browser-session');
    await expect(browserStorage.get('browser-session')).resolves.toBeNull();
    Object.defineProperty(globalThis, 'sessionStorage', { value: undefined });
    await expect(browserStorage.get('browser-session')).resolves.toBeNull();
  });

  it('propagates browser persistence failures to the session operation', async () => {
    const failure = new Error('storage unavailable');
    const fail = () => {
      throw failure;
    };
    Object.defineProperty(globalThis, 'sessionStorage', {
      configurable: true,
      value: { getItem: fail, setItem: fail, removeItem: fail },
    });
    await expect(browserStorage.get('session')).rejects.toBe(failure);
    await expect(browserStorage.set('session', 'value')).rejects.toBe(failure);
    await expect(browserStorage.remove('session')).rejects.toBe(failure);
  });

  it('round trips native sessions through secure storage', async () => {
    jest
      .mocked(SecureStore.getItemAsync)
      .mockResolvedValueOnce('saved-session')
      .mockResolvedValueOnce(null);
    jest.mocked(SecureStore.setItemAsync).mockResolvedValue(undefined);
    jest.mocked(SecureStore.deleteItemAsync).mockResolvedValue(undefined);
    await nativeStorage.set('session', 'saved-session');
    expect(SecureStore.setItemAsync).toHaveBeenCalledWith(
      'session',
      'saved-session',
    );
    await expect(nativeStorage.get('session')).resolves.toBe('saved-session');
    expect(SecureStore.getItemAsync).toHaveBeenCalledWith('session');
    await nativeStorage.remove('session');
    expect(SecureStore.deleteItemAsync).toHaveBeenCalledWith('session');
    await expect(nativeStorage.get('session')).resolves.toBeNull();
  });

  it('propagates secure storage failures without silently falling back', async () => {
    const failure = new Error('secure storage unavailable');
    jest.mocked(SecureStore.getItemAsync).mockRejectedValue(failure);
    jest.mocked(SecureStore.setItemAsync).mockRejectedValue(failure);
    jest.mocked(SecureStore.deleteItemAsync).mockRejectedValue(failure);
    await expect(nativeStorage.get('session')).rejects.toBe(failure);
    await expect(nativeStorage.set('session', 'value')).rejects.toBe(failure);
    await expect(nativeStorage.remove('session')).rejects.toBe(failure);
  });
});
