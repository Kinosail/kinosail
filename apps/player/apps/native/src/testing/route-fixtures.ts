import type { MediaItem, PlaybackSource } from '@/core/contract';
import type { KinosailClient } from '@/core/server-client';
import type { useSession } from '@/core/session-context';

export const routeItem: MediaItem = {
  id: 'arrival',
  kind: 'video',
  title: 'Arrival',
  year: '2016',
  plot: 'Visitors arrive on Earth.',
  rating: 'PG-13',
  tagline: '',
  genres: '',
  director: '',
  studio: '',
  artist: '',
  album: '',
  show: '',
  season: 0,
  episode: 0,
  artwork: '/art/arrival',
  backdrop: '/backdrop/arrival',
  container: 'mp4',
  progress: { seconds: 12, watched: false, session: '', revision: 2 },
};

export const routeSource: PlaybackSource = {
  uri: 'https://kino.example/media/arrival',
  headers: { Authorization: 'Bearer viewer-token' },
  contentType: 'video/mp4',
  duration: 120,
  start: 12,
  progressToken: 'progress-token',
  plan: { allowed: true, mode: 'direct', reason: 'compatible' },
};

export function routeSession(
  client: KinosailClient | null,
): ReturnType<typeof useSession> {
  return {
    booting: false,
    bootError: '',
    client,
    session: client ? { baseURL: client.baseURL, token: 'viewer-token' } : null,
    connect: jest.fn(),
    retryBoot: jest.fn(),
    signOut: jest.fn(),
  };
}

export function deferred<Value>() {
  let resolve: (value: Value) => void = () => {};
  let reject: (reason: Error) => void = () => {};
  const promise = new Promise<Value>((done, fail) => {
    resolve = done;
    reject = fail;
  });
  return { promise, resolve, reject };
}
