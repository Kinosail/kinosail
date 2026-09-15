import {
  createNavigationPreferences,
  defaultNavigation,
} from './navigation-preferences';

const storage = () => ({
  get: jest.fn().mockResolvedValue(null),
  set: jest.fn().mockResolvedValue(undefined),
  remove: jest.fn(),
});
it('defaults to Downloads on mobile and preserves an ordered saved selection', async () => {
  const disk = storage();
  const preferences = createNavigationPreferences(disk, true);
  expect(await preferences.load()).toEqual([
    'home',
    'movies',
    'shows',
    'downloads',
  ]);
  const chosen = ['downloads', 'books', 'music', 'home'];
  expect(await preferences.save(chosen)).toEqual(chosen);
  expect(disk.set).toHaveBeenCalledWith(
    'kinosail.player.navigation.v1',
    JSON.stringify(chosen),
  );
  disk.get.mockResolvedValue(JSON.stringify(chosen));
  expect(await createNavigationPreferences(disk, true).load()).toEqual(chosen);
});
it.each([
  Array(1),
  undefined,
  null,
  false,
  {},
  '',
  [],
  [''],
  ['home', 'home'],
  ['more'],
  ['HOME'],
  [' home '],
  ['unknown'],
  ['constructor'],
  ['__proto__'],
  [1],
  [['home']],
  [{ id: 'home' }],
  ['home', 'movies', 'shows', 'music', 'books'],
  ['x'.repeat(257)],
])('rejects invalid choices before writing: %p', async (input) => {
  const disk = storage();
  await expect(
    createNavigationPreferences(disk, true).save(input),
  ).rejects.toThrow();
  expect(disk.set).not.toHaveBeenCalled();
  expect(disk.remove).not.toHaveBeenCalled();
});
it.each(['', '{', '{}', 'null', '[]', '["home","home"]', ' '.repeat(257)])(
  'rejects malformed saved tabs without replacing them: %p',
  async (raw) => {
    const disk = storage();
    disk.get.mockResolvedValue(raw);
    await expect(
      createNavigationPreferences(disk, true).load(),
    ).rejects.toThrow();
    expect(disk.set).not.toHaveBeenCalled();
    expect(disk.remove).not.toHaveBeenCalled();
  },
);
it('does not offer unavailable downloads and accepts a single chosen tab', async () => {
  const disk = storage();
  expect(defaultNavigation(false)).toEqual([
    'home',
    'movies',
    'shows',
    'music',
  ]);
  await expect(
    createNavigationPreferences(disk, false).save(['downloads']),
  ).rejects.toThrow();
  expect(disk.set).not.toHaveBeenCalled();
  expect(
    await createNavigationPreferences(disk, false).save(['books']),
  ).toEqual(['books']);
});
it('keeps storage errors observable so the editor can retry without claiming a save', async () => {
  const disk = storage();
  disk.set.mockRejectedValue(new Error('unavailable'));
  await expect(
    createNavigationPreferences(disk, true).save(['downloads']),
  ).rejects.toThrow('unavailable');
  expect(disk.set).toHaveBeenCalledTimes(1);
});

it('lets viewers pin photos, collections, audiobooks and My List', async () => {
  const disk = storage();
  const selected = ['photos', 'collections', 'audiobooks', 'list'];
  expect(await createNavigationPreferences(disk, true).save(selected)).toEqual(
    selected,
  );
});
