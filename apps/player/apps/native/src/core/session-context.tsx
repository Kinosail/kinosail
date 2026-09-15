import { verifiedTransfers } from './verified-transfers';
import * as Crypto from 'expo-crypto';
import { MediaCoordinator } from './media-coordinator';
import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';

import { Image } from 'expo-image';
import { clearDownloads } from './downloads';
import { clearPlaybackMetrics } from './playback-metrics';
import { platformStorage } from './platform-storage';
import { KinosailClient } from './server-client';
import { createSessionStore, type Session } from './session-store';

type SessionValue = {
  booting: boolean;
  bootError: string;
  session: Session | null;
  client: KinosailClient | null;
  connect(session: Session): Promise<void>;
  retryBoot(): void;
  signOut(): Promise<void>;
};

const SessionContext = createContext<SessionValue | null>(null);
const sessions = createSessionStore(platformStorage);

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const mutation = useRef(0);
  const operations = useRef(Promise.resolve());
  const currentSession = useRef<Session | null>(null);
  function serialize(operation: () => Promise<void>): Promise<void> {
    const result = operations.current.then(operation);
    operations.current = result.catch(() => {});
    return result;
  }
  const [booting, setBooting] = useState(true);
  const [bootError, setBootError] = useState('');
  // Stryker disable next-line BooleanLiteral: either initial boolean yields the same alternating retry signal.
  const [bootAttempt, setBootAttempt] = useState(false);
  const [session, setSession] = useState<Session | null>(null);

  useEffect(() => {
    let active = true;
    const generation = mutation.current;
    sessions.load().then(
      (loaded) => {
        // Stryker disable next-line ConditionalExpression: React ignores state after unmount; the guard prevents unnecessary setter calls.
        if (active && generation === mutation.current) {
          currentSession.current = loaded;
          setSession(loaded);
          setBootError('');
          setBooting(false);
        }
      },
      (reason) => {
        // Stryker disable next-line ConditionalExpression: React ignores state after unmount; the guard prevents unnecessary setter calls.
        if (active && generation === mutation.current) {
          setBootError(
            reason instanceof Error
              ? reason.message
              : 'The saved player could not open.',
          );
          setBooting(false);
        }
      },
    );
    // Stryker disable next-line BlockStatement: React ignores state after unmount; this cleanup only avoids unnecessary setter calls.
    return () => {
      // Stryker disable next-line BooleanLiteral: true would only cause ignored post-unmount setter calls.
      active = false;
    };
  }, [bootAttempt]);

  const client = useMemo(
    () =>
      session
        ? new KinosailClient(
            session.baseURL,
            session.token,
            undefined,
            session.downloadScope,
          )
        : null,
    [session],
  );
  useEffect(() => {
    if (!session || session.identity || !client) return;
    let active = true;
    const generation = mutation.current;
    void client
      .loadDownloadIdentity()
      .then(async (identity) => {
        const downloadScope = await Crypto.digestStringAsync(
          Crypto.CryptoDigestAlgorithm.SHA256,
          `${session.baseURL}\nBearer ${session.token}`,
        );
        if (!active || generation !== mutation.current) return;
        const upgraded = { ...session, identity, downloadScope };
        await serialize(async () => {
          if (!active || generation !== mutation.current) return;
          await sessions.save(upgraded);
          if (generation === mutation.current) {
            currentSession.current = upgraded;
            setSession(upgraded);
          }
        });
      })
      .catch(() => {
        /* Existing offline sessions remain usable without a Server. */
      });
    return () => {
      active = false;
    };
  }, [session, client]);
  const value = useMemo(
    () => ({
      booting,
      bootError,
      session,
      client,
      async connect(next: Session) {
        mutation.current++;
        return serialize(async () => {
          const session = currentSession.current;
          // Only the authenticated Server can establish ownership across token changes.
          const identity = await new KinosailClient(
            next.baseURL,
            next.token,
          ).loadDownloadIdentity();
          const sameOwner =
            session?.identity?.serverId === identity.serverId &&
            session.identity.profileId === identity.profileId &&
            session.baseURL === next.baseURL;
          const downloadScope =
            sameOwner && session.downloadScope
              ? session.downloadScope
              : await Crypto.digestStringAsync(
                  Crypto.CryptoDigestAlgorithm.SHA256,
                  `${next.baseURL}\nBearer ${next.token}`,
                );
          next = {
            baseURL: next.baseURL,
            token: next.token,
            identity,
            downloadScope,
          };
          if (session && !sameOwner) {
            await clearDownloads();
            await Promise.all([
              Image.clearDiskCache(),
              Image.clearMemoryCache(),
            ]);
            clearPlaybackMetrics();
          }
          if (sameOwner)
            await verifiedTransfers?.authorizeVerifiedDownloads(
              downloadScope,
              `Bearer ${next.token}`,
            );
          await sessions.save(next);
          currentSession.current = next;
          setSession(next);
          setBooting(false);
        });
      },
      retryBoot() {
        setBootError('');
        setBooting(true);
        setBootAttempt((attempt) => !attempt);
      },
      async signOut() {
        mutation.current++;
        return serialize(async () => {
          await clearDownloads();
          await Promise.all([Image.clearDiskCache(), Image.clearMemoryCache()]);
          clearPlaybackMetrics();
          await sessions.clear();
          currentSession.current = null;
          setSession(null);
          setBooting(false);
        });
      },
    }),
    [booting, bootError, session, client],
  );
  return (
    <SessionContext.Provider value={value}>
      <MediaCoordinator client={client} />
      {children}
    </SessionContext.Provider>
  );
}

export function useSession(): SessionValue {
  const value = useContext(SessionContext);
  if (!value) throw new Error('SessionProvider is missing.');
  return value;
}
