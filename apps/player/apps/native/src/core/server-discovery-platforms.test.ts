const nativeLoader = jest.fn();
const load = (platform: string) => {
  jest.doMock('react-native', () => ({ Platform: { OS: platform } }));
  jest.doMock('expo', () => ({ requireOptionalNativeModule: nativeLoader }));
  let module!: typeof import('./server-discovery');
  jest.isolateModules(() => {
    module = require('./server-discovery');
  });
  return module;
};
afterEach(() => {
  jest.resetAllMocks();
});

it.each(['ios', 'android'])(
  'discovers and validates through the same contract on %s',
  async (platform) => {
    const stop = jest.fn(async () => {});
    const scan = jest.fn(async () => [
      { name: 'Player', url: 'http://192.168.1.8:38127' },
      { name: 'Untrusted', url: 'http://public.example.com' },
    ]);
    nativeLoader.mockReturnValue({ scan, stop });
    const module = load(platform);
    expect(module.serverDiscoveryAvailable).toBe(true);
    expect(await module.discoverServers()).toEqual([
      { name: 'Player', url: 'http://192.168.1.8:38127' },
    ]);
    await module.stopServerDiscovery();
    expect(stop).toHaveBeenCalledTimes(1);
  },
);

it.each(['web', 'windows', 'macos'])(
  'preserves manual setup without a native browser on %s',
  async (platform) => {
    const module = load(platform);
    expect(module.serverDiscoveryAvailable).toBe(false);
    expect(nativeLoader).not.toHaveBeenCalledWith('ServerDiscovery');
    expect(await module.discoverServers()).toEqual([]);
    await module.stopServerDiscovery();
  },
);

it('keeps manual entry on older Android builds without the linked module', () => {
  nativeLoader.mockReturnValue(null);
  expect(load('android').serverDiscoveryAvailable).toBe(false);
});

it('propagates discovery denial to the existing recovery UI', async () => {
  nativeLoader.mockReturnValue({
    scan: jest.fn(async () => {
      throw new Error('denied');
    }),
    stop: jest.fn(),
  });
  await expect(load('android').discoverServers()).rejects.toThrow('denied');
});
