import type { KeyValueStorage } from './session-store';

export const navigationDestinations = {
  home: 'Home',
  movies: 'Movies',
  shows: 'TV',
  music: 'Music',
  books: 'Books',
  audiobooks: 'Audiobooks',
  photos: 'Photos',
  collections: 'Collections',
  list: 'My List',
  history: 'History',
  downloads: 'Downloads',
  settings: 'Settings',
} as const;
export type NavigationDestination = keyof typeof navigationDestinations;
const key = 'kinosail.player.navigation.v1';
export const defaultNavigation = (
  downloads: boolean,
): NavigationDestination[] => [
  'home',
  'movies',
  'shows',
  downloads ? 'downloads' : 'music',
];

export function validateNavigation(
  value: unknown,
  downloads: boolean,
): NavigationDestination[] {
  if (
    !Array.isArray(value) ||
    value.length < 1 ||
    value.length > 4 ||
    new Set(value).size !== value.length ||
    Array.from(value).some(
      (entry) =>
        typeof entry !== 'string' ||
        !Object.keys(navigationDestinations).includes(entry) ||
        (!downloads && entry === 'downloads'),
    )
  ) {
    throw new Error('Choose one to four different tabs.');
  }
  return [...value] as NavigationDestination[];
}

export const createNavigationPreferences = (
  storage: KeyValueStorage,
  downloads: boolean,
) => ({
  async load(): Promise<NavigationDestination[]> {
    const raw = await storage.get(key);
    if (raw === null) return defaultNavigation(downloads);
    if (raw.length > 256)
      throw new Error('Saved tabs could not be read. Choose your tabs again.');
    return validateNavigation(JSON.parse(raw), downloads);
  },
  async save(value: unknown): Promise<NavigationDestination[]> {
    const tabs = validateNavigation(value, downloads);
    await storage.set(key, JSON.stringify(tabs));
    return tabs;
  },
});
