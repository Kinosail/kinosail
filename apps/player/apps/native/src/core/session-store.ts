import {
  parseDownloadIdentity,
  type DownloadIdentity,
} from './download-manifest';
import type { InputValue } from './contract';
import { normalizeServerURL } from './server-client';

const SESSION_KEY = 'kinosail.player.session.v1';

export type Session = {
  baseURL: string;
  token: string;
  identity?: DownloadIdentity;
  downloadScope?: string;
};

export type KeyValueStorage = {
  get(key: string): Promise<string | null>;
  remove(key: string): Promise<void>;
  set(key: string, value: string): Promise<void>;
};

const storageTimeout = 3000;

const within = <Value>(
  operation: Promise<Value>,
  message: string,
  timeout: number,
): Promise<Value> =>
  new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(message)), timeout);
    operation.then(
      (value) => {
        clearTimeout(timer);
        resolve(value);
      },
      () => {
        clearTimeout(timer);
        reject(new Error(message));
      },
    );
  });

const parseSession = (raw: string): Session => {
  const value: InputValue = JSON.parse(raw);
  if (!value || Array.isArray(value)) {
    throw new Error('invalid session');
  }
  const session = value as Record<string, InputValue>;
  if (
    typeof session.baseURL !== 'string' ||
    typeof session.token !== 'string' ||
    !session.token ||
    session.token.length > 2048 ||
    Object.keys(session).some(
      (key) => !['baseURL', 'token', 'identity', 'downloadScope'].includes(key),
    )
  ) {
    throw new Error('invalid session');
  }
  let identity: DownloadIdentity | undefined;
  if (session.identity !== undefined || session.downloadScope !== undefined) {
    identity = parseDownloadIdentity(session.identity);
    if (
      typeof session.downloadScope !== 'string' ||
      !/^[a-f0-9]{64}$/.test(session.downloadScope)
    )
      throw new Error('invalid session');
  }
  return {
    baseURL: normalizeServerURL(session.baseURL),
    token: session.token,
    ...(identity
      ? { identity, downloadScope: session.downloadScope as string }
      : {}),
  };
};

export const createSessionStore = (
  storage: KeyValueStorage,
  timeout = storageTimeout,
) => ({
  async clear(): Promise<void> {
    await within(
      storage.remove(SESSION_KEY),
      'Secure session storage did not respond. Try again.',
      timeout,
    );
  },
  async load(): Promise<Session | null> {
    const raw = await within(
      storage.get(SESSION_KEY),
      'The saved player could not open. Try again.',
      timeout,
    );
    if (!raw) return null;
    try {
      return parseSession(raw);
    } catch {
      await within(
        storage.remove(SESSION_KEY),
        'The saved player could not open. Try again.',
        timeout,
      );
      return null;
    }
  },
  async save(session: Session): Promise<void> {
    const validated = parseSession(JSON.stringify(session));
    await within(
      storage.set(SESSION_KEY, JSON.stringify(validated)),
      'Secure session storage did not respond. Try again.',
      timeout,
    );
  },
});
