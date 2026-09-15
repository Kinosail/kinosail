import {
  createSessionStore,
  type KeyValueStorage,
  type Session,
} from './session-store';

describe('session store', () => {
  it('does not rewrite storage when no session exists', async () => {
    const storage: KeyValueStorage = {
      get: jest.fn().mockResolvedValue(null),
      remove: jest.fn(),
      set: jest.fn(),
    };
    await expect(createSessionStore(storage).load()).resolves.toBeNull();
    expect(storage.remove).not.toHaveBeenCalled();
    expect(storage.set).not.toHaveBeenCalled();
  });

  it.each([false, true])(
    'clears the timeout after storage settles: %s',
    async (fails) => {
      jest.useFakeTimers();
      try {
        const removal = fails
          ? Promise.reject(new Error('private storage failure'))
          : Promise.resolve();
        void removal.catch(() => {});
        const storage: KeyValueStorage = {
          get: jest.fn(),
          remove: jest.fn().mockReturnValue(removal),
          set: jest.fn(),
        };
        const result = createSessionStore(storage).clear();
        const settled = jest.fn();
        void result.then(
          () => settled('removed'),
          (error: Error) => settled(error.message),
        );
        expect(jest.getTimerCount()).toBe(1);
        await jest.advanceTimersByTimeAsync(0);
        expect(settled).toHaveBeenCalledWith(
          fails
            ? 'Secure session storage did not respond. Try again.'
            : 'removed',
        );
        expect(jest.getTimerCount()).toBe(0);
        expect(storage.remove).toHaveBeenCalledWith(
          'kinosail.player.session.v1',
        );
      } finally {
        jest.useRealTimers();
      }
    },
  );

  it.each([
    null,
    false,
    true,
    0,
    5,
    '',
    'session',
    [],
    {},
    { baseURL: 5, token: 'viewer-token' },
    { baseURL: 'https://kino.example', token: null },
    { baseURL: 'https://kino.example', token: true },
    { baseURL: 'https://kino.example', token: 5 },
    { baseURL: 'https://kino.example', token: [] },
    { baseURL: 'https://kino.example', token: {} },
    { baseURL: 'https://kino.example', token: '' },
    { baseURL: 'https://kino.example', token: 'x'.repeat(2049) },
    { baseURL: 'https://kino.example', token: 'viewer-token', extra: true },
  ])('rejects invalid session shape %# before saving', async (input) => {
    const storage: KeyValueStorage = {
      get: jest.fn(),
      remove: jest.fn(),
      set: jest.fn(),
    };
    await expect(
      createSessionStore(storage).save(input as Session),
    ).rejects.toThrow('invalid session');
    expect(storage.set).not.toHaveBeenCalled();
    expect(storage.remove).not.toHaveBeenCalled();
    expect(storage.get).not.toHaveBeenCalled();
  });

  it('accepts the token size boundary and normalizes the server once', async () => {
    const storage: KeyValueStorage = {
      get: jest.fn(),
      remove: jest.fn(),
      set: jest.fn().mockResolvedValue(undefined),
    };
    const token = 'x'.repeat(2048);
    await createSessionStore(storage).save({
      baseURL: 'https://kino.example/',
      token,
    });
    expect(storage.set).toHaveBeenCalledWith(
      'kinosail.player.session.v1',
      JSON.stringify({ baseURL: 'https://kino.example', token }),
    );
  });

  it.each(['{', 'null', '[]', '{"baseURL":"https://kino.example"}'])(
    'clears malformed stored state: %s',
    async (raw) => {
      const storage: KeyValueStorage = {
        get: jest.fn().mockResolvedValue(raw),
        remove: jest.fn().mockResolvedValue(undefined),
        set: jest.fn(),
      };
      await expect(createSessionStore(storage).load()).resolves.toBeNull();
      expect(storage.remove).toHaveBeenCalledWith('kinosail.player.session.v1');
      expect(storage.set).not.toHaveBeenCalled();
    },
  );
  it('round trips a valid session', async () => {
    const values = new Map<string, string>();
    const storage: KeyValueStorage = {
      get: async (key) => values.get(key) ?? null,
      remove: async (key) => void values.delete(key),
      set: async (key, value) => void values.set(key, value),
    };
    const sessions = createSessionStore(storage);

    await sessions.save({
      baseURL: 'https://kino.example',
      token: 'viewer-token',
    });

    await expect(sessions.load()).resolves.toEqual({
      baseURL: 'https://kino.example',
      token: 'viewer-token',
    });
    await sessions.clear();
    await expect(sessions.load()).resolves.toBeNull();
  });

  it('removes invalid persisted state', async () => {
    let value: string | null =
      '{"baseURL":"http://public.example","token":"x"}';
    const storage: KeyValueStorage = {
      get: async () => value,
      remove: async () => {
        value = null;
      },
      set: async (_key, next) => {
        value = next;
      },
    };

    await expect(createSessionStore(storage).load()).resolves.toBeNull();
    expect(value).toBeNull();
  });

  it('bounds stalled secure storage operations', async () => {
    const stalled = new Promise<never>(() => undefined);
    const storage: KeyValueStorage = {
      get: () => stalled,
      remove: () => stalled,
      set: () => stalled,
    };
    const sessions = createSessionStore(storage, 1);

    await expect(sessions.load()).rejects.toThrow(
      'The saved player could not open. Try again.',
    );
    await expect(
      sessions.save({ baseURL: 'https://kino.example', token: 'viewer-token' }),
    ).rejects.toThrow('Secure session storage did not respond. Try again.');
  });

  it('bounds cleanup of invalid persisted state', async () => {
    const stalled = new Promise<never>(() => undefined);
    const storage: KeyValueStorage = {
      get: async () => '{"baseURL":"invalid","token":"viewer-token"}',
      remove: () => stalled,
      set: async () => undefined,
    };

    await expect(createSessionStore(storage, 1).load()).rejects.toThrow(
      'The saved player could not open. Try again.',
    );
  });

  it('does not expose secure storage internals', async () => {
    const failure = Promise.reject(new Error('private native exception'));
    void failure.catch(() => {});
    const storage: KeyValueStorage = {
      get: () => failure,
      remove: async () => undefined,
      set: async () => undefined,
    };

    await expect(createSessionStore(storage).load()).rejects.toThrow(
      'The saved player could not open. Try again.',
    );
  });
});
