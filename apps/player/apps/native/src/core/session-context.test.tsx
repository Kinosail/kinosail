import { act, renderHook, waitFor } from '@testing-library/react-native';

import { SessionProvider, useSession } from './session-context';
import type { Session } from './session-store';
import { KinosailClient } from './server-client';
import { clearDownloads } from './downloads';

jest.mock('./verified-transfers', () => ({ verifiedTransfers: null }));
jest.mock('./downloads', () => ({
  clearDownloads: jest.fn().mockResolvedValue(undefined),
}));
jest.mock('expo-crypto', () => ({
  CryptoDigestAlgorithm: { SHA256: 'sha256' },
  digestStringAsync: jest.fn().mockResolvedValue('a'.repeat(64)),
}));

const mockLoad = jest.fn<Promise<Session | null>, []>();
const mockSave = jest.fn<Promise<void>, [Session]>();
const mockClear = jest.fn<Promise<void>, []>();

jest.mock('./session-store', () => ({
  createSessionStore: () => ({
    load: () => mockLoad(),
    save: (session: Session) => mockSave(session),
    clear: () => mockClear(),
  }),
}));

const saved: Session = {
  baseURL: 'https://kino.example',
  token: 'viewer-token',
  identity: { serverId: 'server', profileId: 'viewer' },
  downloadScope: 'a'.repeat(64),
};
const mount = () =>
  renderHook(() => useSession(), { wrapper: SessionProvider });

describe('session lifecycle', () => {
  beforeEach(() => {
    jest
      .spyOn(KinosailClient.prototype, 'loadDownloadIdentity')
      .mockResolvedValue(saved.identity!);
    jest.mocked(clearDownloads).mockClear();
    mockLoad.mockReset().mockResolvedValue(null);
    mockSave.mockReset().mockResolvedValue(undefined);
    mockClear.mockReset().mockResolvedValue(undefined);
  });

  it('requires a provider', async () => {
    await expect(renderHook(() => useSession())).rejects.toThrow(
      'SessionProvider is missing.',
    );
  });

  it('restores the saved session and an authorized client', async () => {
    mockLoad.mockResolvedValue(saved);
    const { result } = await mount();
    await waitFor(() => expect(result.current.booting).toBe(false));
    expect(result.current.bootError).toBe('');
    expect(result.current.session).toEqual(saved);
    expect(result.current.client?.baseURL).toBe(saved.baseURL);
    expect(result.current.client?.authorizationHeaders()).toEqual({
      Authorization: 'Bearer viewer-token',
    });
  });

  it('publishes a new session only after persistence completes', async () => {
    let finishSave = () => {};
    mockSave.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          finishSave = resolve;
        }),
    );
    const { result } = await mount();
    await waitFor(() => expect(result.current.booting).toBe(false));
    expect(result.current.client).toBeNull();
    const pending = result.current.connect(saved);
    await waitFor(() => expect(mockSave).toHaveBeenCalledWith(saved));
    expect(result.current.session).toBeNull();
    await act(async () => {
      finishSave();
      await pending;
    });
    expect(result.current.session).toEqual(saved);
    expect(result.current.client).not.toBeNull();
    await act(async () => {
      await result.current.signOut();
    });
    expect(mockClear).toHaveBeenCalledTimes(1);
    expect(result.current.session).toBeNull();
    expect(result.current.client).toBeNull();
  });

  it('preserves current state when saving or clearing fails', async () => {
    mockLoad.mockResolvedValue(saved);
    const { result } = await mount();
    await waitFor(() => expect(result.current.booting).toBe(false));
    const failure = new Error('storage unavailable');
    mockSave.mockRejectedValue(failure);
    mockClear.mockRejectedValue(failure);
    await expect(
      result.current.connect({ ...saved, token: 'replacement' }),
    ).rejects.toBe(failure);
    expect(result.current.session).toEqual(saved);
    await expect(result.current.signOut()).rejects.toBe(failure);
    expect(result.current.session).toEqual(saved);
  });

  it.each([new Error('storage unavailable'), 'unstructured failure'])(
    'offers a fresh boot after storage fails: %s',
    async (failure) => {
      mockLoad.mockRejectedValueOnce(failure);
      const { result } = await mount();
      await waitFor(() => expect(result.current.booting).toBe(false));
      expect(result.current.bootError).toBe(
        failure instanceof Error
          ? failure.message
          : 'The saved player could not open.',
      );
      let finishLoad = (session: Session | null) => {
        void session;
      };
      mockLoad.mockImplementation(
        () =>
          new Promise<Session | null>((resolve) => {
            finishLoad = resolve;
          }),
      );
      await act(() => result.current.retryBoot());
      expect(result.current.booting).toBe(true);
      expect(result.current.bootError).toBe('');
      await act(async () => {
        finishLoad(saved);
      });
      expect(result.current.session).toEqual(saved);
      expect(result.current.booting).toBe(false);
      expect(mockLoad).toHaveBeenCalledTimes(2);
    },
  );

  it.each(['resolve', 'reject'] as const)(
    'ignores late boot %s after unmount',
    async (outcome) => {
      let finishLoad = () => {};
      mockLoad.mockImplementation(
        () =>
          new Promise<Session | null>((resolve, reject) => {
            finishLoad = () =>
              outcome === 'resolve'
                ? resolve(saved)
                : reject(new Error('late failure'));
          }),
      );
      const { result, unmount } = await mount();
      expect(result.current.booting).toBe(true);
      await unmount();
      await act(async () => {
        finishLoad();
      });
      expect(result.current.session).toBeNull();
      expect(result.current.bootError).toBe('');
    },
  );

  it('starts each retry after consecutive boot failures', async () => {
    mockLoad
      .mockRejectedValueOnce(new Error('first failure'))
      .mockRejectedValueOnce(new Error('second failure'))
      .mockResolvedValueOnce(null);
    const { result } = await mount();
    await waitFor(() => expect(result.current.bootError).toBe('first failure'));
    await act(() => result.current.retryBoot());
    await waitFor(() =>
      expect(result.current.bootError).toBe('second failure'),
    );
    await act(() => result.current.retryBoot());
    await waitFor(() => expect(result.current.booting).toBe(false));
    expect(mockLoad).toHaveBeenCalledTimes(3);
    expect(result.current.bootError).toBe('');
  });
});

it('serializes sign-out after an in-flight connection so credentials cannot return', async () => {
  mockLoad.mockResolvedValue(null);
  mockSave.mockResolvedValue(undefined);
  mockClear.mockResolvedValue(undefined);
  let resolveIdentity!: (identity: NonNullable<Session['identity']>) => void;
  jest
    .spyOn(KinosailClient.prototype, 'loadDownloadIdentity')
    .mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveIdentity = resolve;
        }),
    );
  const { result } = await mount();
  await waitFor(() => expect(result.current.booting).toBe(false));
  const connect = result.current.connect(saved);
  await waitFor(() => expect(resolveIdentity).toBeDefined());
  const signOut = result.current.signOut();
  await act(async () => {
    resolveIdentity(saved.identity!);
    await connect;
    await signOut;
  });
  expect(result.current.session).toBeNull();
  expect(mockSave.mock.invocationCallOrder.at(-1)).toBeLessThan(
    mockClear.mock.invocationCallOrder.at(-1)!,
  );
});
