import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { runInNewContext } from 'node:vm';
import test from 'node:test';

const environment = project => ({ KINOSAIL_PROVIDER_PROFILE: '1', KINOSAIL_TEST_INSTANCE: '1',
  KINOSAIL_BROWSER_TEST: '1', KINOSAIL_BROWSER_PROJECT: project,
  KINOSAIL_E2E_URL: `${project === 'webkit' ? 'https' : 'http'}://localhost:38127` });
function owner(env = {}) {
  const source = stripTypeScriptTypes(readFileSync(new URL('../../apps/player/e2e/provider-profile-fixture.ts', import.meta.url), 'utf8'))
    .replace(/^import .*;\n/gm, '').replace(/^export /gm, '');
  const hooks = [];
  const api = runInNewContext(`(()=>{${source};return {providerProfile, isolateProvider, configureProviderProfile, providerRoute};})()`,
    {URL, process: {env}, test: {beforeEach: fn => hooks.push(fn)}});
  return {api, hooks};
}
const request = (url, method, effects) => ({request: () => ({url: () => url, method: () => method}),
  abort: async () => effects.push('blocked'), fallback: async () => effects.push('owned')});

test('all engines install context guard before login with fixed owned origin', async () => {
  for (const project of ['chromium', 'firefox', 'webkit']) {
    const env = environment(project), {api, hooks} = owner(env), routes = [];
    api.configureProviderProfile(); assert.equal(hooks.length, 1);
    await hooks[0]({context: {route: async (...args) => routes.push(args)}, baseURL: env.KINOSAIL_E2E_URL}, {project: {name: project}});
    assert.equal(routes.length, 1); const effects = [];
    for (const [url, method] of [[env.KINOSAIL_E2E_URL + '/login', 'GET'], [env.KINOSAIL_E2E_URL + '/login', 'POST'],
      [env.KINOSAIL_E2E_URL + '/settings/home-assistant', 'POST'], [env.KINOSAIL_E2E_URL + '/api/v1/home-assistant/pairings', 'POST'],
      ['https://provider.invalid/login', 'GET'], [env.KINOSAIL_E2E_URL + '/home-assistant/authorize', 'POST'],
      [env.KINOSAIL_E2E_URL + '/api/v1/home-assistant/pair', 'POST'], [env.KINOSAIL_E2E_URL + '/api/v1/supporter/activate', 'POST']])
      await routes[0][1](request(url, method, effects));
    assert.deepEqual(effects, ['owned', 'owned', 'owned', 'owned', 'blocked', 'blocked', 'blocked', 'blocked']);
  }
});

test('synthetic page callbacks cannot fetch continue or fulfill foreign requests', async () => {
  const env = environment('chromium'), {api} = owner(env), routes = []; let calls = 0;
  const page = {route: async (...args) => routes.push(args)};
  await api.providerRoute(page, '**/supporter', async () => calls++);
  const effects = [];
  for (const url of ['https://foreign.invalid/supporter', 'not-url', 'http://Owner:private@localhost:38127/supporter',
    env.KINOSAIL_E2E_URL + '/supporter#private', env.KINOSAIL_E2E_URL + '/supporter\n', 'x'.repeat(8193)])
    await routes[0][1](request(url, 'GET', effects));
  assert.equal(calls, 0); assert.deepEqual(effects, Array(6).fill('blocked'));
  await routes[0][1](request(env.KINOSAIL_E2E_URL + '/supporter', 'GET', effects)); assert.equal(calls, 1);
});

test('missing and malformed profile fields reject before route effects', async () => {
  const base = environment('chromium');
  for (const [key, values] of Object.entries({
    KINOSAIL_PROVIDER_PROFILE: ['', '0', 'true', 'x'.repeat(1025), false],
    KINOSAIL_TEST_INSTANCE: [undefined, '0'], KINOSAIL_BROWSER_TEST: [undefined, '0'],
    KINOSAIL_BROWSER_PROJECT: [undefined, 'unknown', 'webkit'],
    KINOSAIL_E2E_URL: [undefined, '', 'http://foreign.invalid:38127', 'https://localhost:38127', 'http://localhost:0',
      'http://localhost:65536', 'http://localhost:038127', 'http://localhost:38127/', 'http://localhost:38127?x',
      'http://localhost:38127#x', 'http://Owner:private@localhost:38127', 'x'.repeat(2049)],
  })) for (const value of values) assert.throws(() => owner({...base, [key]: value}).api.providerProfile({...base, [key]: value}));
  const {api} = owner(base); let effects = 0;
  for (const [project, url] of [['webkit', base.KINOSAIL_E2E_URL], ['chromium', 'http://foreign.invalid:38127']])
    await assert.rejects(api.isolateProvider({route: async () => effects++}, project, url));
  assert.equal(effects, 0);
});

test('absence preserves ordinary cases and synthetic routing without a profile', async () => {
  const {api, hooks} = owner(); assert.equal(api.providerProfile({}), undefined); api.configureProviderProfile();
  assert.equal(hooks.length, 0); const callback = () => {}, routes = [];
  await api.providerRoute({route: async (...args) => routes.push(args)}, '**/supporter', callback);
  assert.equal(routes[0][1], callback);
});
