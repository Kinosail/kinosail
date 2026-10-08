import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { runInNewContext } from 'node:vm';
import test from 'node:test';

function owner() {
  const source = stripTypeScriptTypes(readFileSync(new URL('../../apps/player/e2e/camera-profile-fixture.ts', import.meta.url), 'utf8'))
    .replace(/^import .*;\n/gm, '').replace(/^export /gm, '');
  return runInNewContext(`(()=>{${source};return {cameraProfile, isolateCamera};})()`, { URL });
}
const environment = project => ({ KINOSAIL_CAMERA_PROFILE: '1', KINOSAIL_TEST_INSTANCE: '1',
  KINOSAIL_BROWSER_TEST: '1', KINOSAIL_BROWSER_PROJECT: project,
  KINOSAIL_E2E_URL: `${project === 'webkit' ? 'https' : 'http'}://localhost:38127` });

test('closed three-engine profile admits its owned origin before routing', async () => {
  const api = owner();
  for (const project of ['chromium', 'firefox', 'webkit']) {
    const env = environment(project), fixture = api.cameraProfile(env); const routes = [];
    const page = { route: async (...args) => routes.push(args) };
    await api.isolateCamera(page, fixture, project, env.KINOSAIL_E2E_URL);
    assert.equal(routes.length, 1);
    const effects = [];
    const request = (url, method = 'GET') => ({request: () => ({url: () => url, method: () => method}), abort: async reason => effects.push(reason), fallback: async () => effects.push('owned')});
    await routes[0][1](request(env.KINOSAIL_E2E_URL + '/login'));
    await routes[0][1](request('https://other.invalid/quick-connect?code=123456'));
    await routes[0][1](request('not-a-url'));
    await routes[0][1](request(env.KINOSAIL_E2E_URL + '/login', 'POST'));
    await routes[0][1](request(env.KINOSAIL_E2E_URL + '/quick-connect', 'POST'));
    await routes[0][1](request(env.KINOSAIL_E2E_URL + '/api/v1/quick-connect/approve', 'POST'));
    await routes[0][1](request(env.KINOSAIL_E2E_URL + '/quick-connect', 'PUT'));
    assert.deepEqual(effects, ['owned', 'blockedbyclient', 'blockedbyclient', 'owned',
      'blockedbyclient', 'blockedbyclient', 'blockedbyclient']);
  }
});

test('absence is explicit nonpositive admission; present malformed or conflicting inputs reject', () => {
  const api = owner(); assert.equal(api.cameraProfile({}), undefined);
  const base = environment('chromium');
  for (const [key, values] of Object.entries({
    KINOSAIL_CAMERA_PROFILE: ['', '0', 'true', 'x'.repeat(1025), false],
    KINOSAIL_TEST_INSTANCE: [undefined, '0', true], KINOSAIL_BROWSER_TEST: [undefined, '0', true],
    KINOSAIL_BROWSER_PROJECT: [undefined, '', 'unknown', 'WEBKIT', 'x'.repeat(1025)],
    KINOSAIL_E2E_URL: [undefined, '', 'https://localhost:38127', 'http://other.invalid:38127',
      'http://localhost:0', 'http://localhost:65536', 'http://localhost:038127',
      'http://Owner:secret@localhost:38127', 'http://localhost:38127/', 'http://localhost:38127?private',
      'http://localhost:38127#private', 'http://localhost:38127\n', 'x'.repeat(2049)],
  })) for (const value of values) {
    assert.throws(() => api.cameraProfile({...base, [key]: value}), /invalid owned Camera profile/);
  }
});

test('resolved project and baseURL mismatches reject before any route or login effects', async () => {
  const api = owner(), fixture = api.cameraProfile(environment('chromium')); let effects = 0;
  const page = { route: async () => effects++ };
  for (const [project, base] of [['webkit', 'http://localhost:38127'], ['chromium', 'http://other.invalid:38127'],
    ['chromium', undefined], ['chromium', 'http://localhost:38127?private']]) {
    await assert.rejects(api.isolateCamera(page, fixture, project, base), /invalid owned Camera profile/);
  }
  assert.equal(effects, 0);
});
