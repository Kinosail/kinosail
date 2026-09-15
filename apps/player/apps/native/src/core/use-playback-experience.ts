import { useEffect, useRef, useState } from 'react';
import type { KinosailClient } from './server-client';
import {
  defaultPlaybackPreferences,
  parsePlaybackPreferences,
  type PlaybackPreferences,
  type Bookmark,
  formatPosition,
} from './media-preferences';
import { readExperienceCache, writeExperienceCache } from './experience-cache';

export type PlaybackExperience = {
  preferences: PlaybackPreferences;
  loaded: boolean;
  error: string;
  saving: boolean;
  bookmarks: Bookmark[];
  change(value: PlaybackPreferences): void;
  reset(): void;
  bookmark(seconds: number): void;
  removeBookmark(id: string): void;
};
export function usePlaybackExperience(
  client: KinosailClient | null,
  id: string,
): PlaybackExperience {
  const [preferences, setPreferences] = useState(defaultPlaybackPreferences);
  const [loaded, setLoaded] = useState(false),
    [saving, setSaving] = useState(false),
    [error, setError] = useState('');
  const [bookmarks, setBookmarks] = useState<Bookmark[]>([]);
  const queue = useRef(Promise.resolve());
  const generation = useRef(0);
  const pending = useRef(0);
  useEffect(() => {
    const current = ++generation.current;
    pending.current = 0;
    setSaving(false);
    setLoaded(false);
    setError('');
    setBookmarks([]);
    setPreferences(defaultPlaybackPreferences);
    if (!client || !id) return;
    const load = async () => {
      const cached = readExperienceCache(
        client,
        `playback:${id}`,
        parsePlaybackPreferences,
      ).catch(() => null);
      try {
        const result = await client.loadPlaybackPreferences(id);
        if (generation.current !== current) return;
        setPreferences(result.playback);
        // A successful authorization/preferences response can start playback;
        // persisting the offline copy must not hold the first frame hostage.
        void writeExperienceCache(
          client,
          `playback:${id}`,
          result.playback,
        ).catch(() => {});
      } catch {
        const saved = await cached;
        if (generation.current === current) {
          if (saved) setPreferences(saved);
          setError(
            saved
              ? 'Using saved preferences. Changes need a Server connection.'
              : 'Playback preferences are unavailable. Using defaults.',
          );
        }
      } finally {
        if (generation.current === current) setLoaded(true);
      }
    };
    void load();
    void client.loadBookmarks(id).then(
      (value) => {
        if (generation.current === current) setBookmarks(value);
      },
      () => {},
    );
    return () => {
      generation.current++;
    };
  }, [client, id]);
  const run = (operation: () => Promise<void>) => {
    const current = generation.current;
    pending.current++;
    setSaving(true);
    setError('');
    queue.current = queue.current
      .then(operation)
      .catch(() => {
        if (generation.current === current)
          setError('Could not save to your Server. Try again when connected.');
      })
      .finally(() => {
        if (generation.current === current) setSaving(--pending.current > 0);
      });
  };
  return {
    preferences,
    loaded,
    error,
    saving,
    bookmarks,
    change(value) {
      const valid = parsePlaybackPreferences(value);
      if (!client || !loaded) return;
      setPreferences(valid);
      run(async () => {
        await client.savePlaybackPreferences(id, valid);
        await writeExperienceCache(client, `playback:${id}`, valid);
      });
    },
    reset() {
      const current = generation.current;
      if (client)
        run(async () => {
          const result = await client.resetPlaybackPreferences(id);
          if (generation.current === current) setPreferences(result.playback);
          await writeExperienceCache(client, `playback:${id}`, result.playback);
        });
    },
    bookmark(seconds) {
      const current = generation.current;
      if (client)
        run(async () => {
          const value = await client.addBookmark(id, {
            title: formatPosition(seconds),
            seconds,
          });
          if (generation.current === current) setBookmarks(value);
        });
    },
    removeBookmark(bookmark) {
      const current = generation.current;
      if (client)
        run(async () => {
          const value = await client.removeBookmark(id, bookmark);
          if (generation.current === current) setBookmarks(value);
        });
    },
  };
}
