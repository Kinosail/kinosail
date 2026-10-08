import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import test from 'node:test';

// Resolve installed project and file fixture pools without setting up fixtures,
// launching a browser, running hooks, or preparing Go servers or media.
test('cold Home and Library files use engine-compatible pinned launch defaults', async t => {
  const directory = fileURLToPath(new URL('../../apps/player/e2e/', import.meta.url));
  const require = createRequire(join(directory, 'package.json'));
  const packagePath = createRequire(require.resolve('@playwright/test')).resolve('playwright/package.json');
  assert.equal(JSON.parse(readFileSync(packagePath, 'utf8')).version, '1.63.0');
  const cache = mkdtempSync(join(tmpdir(), 'kino-cold-launch-'));
  const keys = ['KINOSAIL_BROWSER_MATRIX', 'KINOSAIL_BROWSER_PROJECT', 'KINOSAIL_BROWSE_RETURN_CASES', 'PWTEST_CACHE_DIR'];
  const before = Object.fromEntries(keys.map(key => [key, process.env[key]]));
  try {
    process.env.KINOSAIL_BROWSER_MATRIX = 'full';
    delete process.env.KINOSAIL_BROWSER_PROJECT;
    process.env.KINOSAIL_BROWSE_RETURN_CASES = 'all';
    process.env.PWTEST_CACHE_DIR = cache;
    const { configLoader, testLoader, poolBuilder } = require(join(dirname(packagePath), 'lib/common/index.js'));
    const config = await configLoader.loadConfig({ configDir: directory, resolvedConfigFile: join(directory, 'playwright.config.ts') });
    for (const file of ['browse-return-home.spec.ts', 'browse-return-cold.spec.ts']) {
      const errors = [];
      const suite = await testLoader.loadTestFile(join(directory, file), config, errors);
      assert.deepEqual(errors, []);
      assert.equal(suite.allTests().length, 3, 'all original registered identities remain');
      for (const project of config.projects) {
        await t.test(`${file} / ${project.project.name}`, () => {
          poolBuilder.PoolBuilder.createForWorker(project).buildPools(suite, errors);
          assert.deepEqual(errors, []);
          for (const registered of suite.allTests()) {
            const fixtures = registered._pool._registrations;
            assert.equal(fixtures.get('defaultBrowserType').fn, project.project.name);
            assert.deepEqual(fixtures.get('launchOptions').fn, {}, 'no cross-engine Chromium launch override');
            assert.equal(fixtures.get('serviceWorkers').fn, 'block');
            assert.equal(fixtures.get('video').fn, 'off');
          }
        });
      }
    }
    const core = createRequire(packagePath).resolve('playwright-core/lib/coreBundle');
    const source = readFileSync(core, 'utf8');
    const switches = source.split('chromiumSwitches = (options) => [', 2)[1]?.split('];', 1)[0];
    assert.ok(switches?.includes('"--disable-back-forward-cache"'), 'Chromium keeps the pinned runner cold-cache default');
  } finally {
    for (const key of keys) {
      if (before[key] === undefined) delete process.env[key]; else process.env[key] = before[key];
    }
    rmSync(cache, { recursive: true, force: true });
  }
});
