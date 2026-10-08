import {test} from 'node:test';
import assert from 'node:assert/strict';
import {EventEmitter} from 'node:events';
import {registerHooks} from 'node:module';
const parent = new URL('../../apps/subtitles/e2e/subtitle-dashboard-helpers.ts', import.meta.url).href;
const hooks = registerHooks({resolve(specifier, context, next) {
  if (context.parentURL === parent && specifier === '../../../scripts/testing/auth-form-navigation')
    return next(specifier + '.ts', context);
  return next(specifier, context);
}});
const {login} = await import('../../apps/subtitles/e2e/subtitle-dashboard-helpers.ts');
hooks.deregister();

// Failure modes: observer/attachment errors mask navigation; startup failures
// retain listeners; initial blank events are mistaken for the target document.
class FailedDashboardPage extends EventEmitter {
  calls = []; scripts = []; current = 'about:blank';
  failure = Object.assign(new Error('private-synthetic-marker'), {name: 'TimeoutError'});
  async addInitScript(script) {this.scripts.push(script);}
  mainFrame() {return this;}
  url() {return this.current;}
  context() {return {browser: () => ({browserType: () => ({name: () => 'chromium'})})};}
  async evaluate() {if (!this.calls.length) return 1000; return {readyState: 'unavailable', timeOrigin: 2000};}
  getByLabel() {throw new Error('no credential actions after failure');}
  async goto(target, options) {
    this.calls.push({target, options});
    this.emit('domcontentloaded'); this.emit('load');
    const request = {url: () => 'https://owned.fixture/login?next=/', resourceType: () => 'document', method: () => 'GET', isNavigationRequest: () => true, frame: () => this};
    this.emit('request', request);
    this.emit('response', {request: () => request, url: request.url, status: () => 200});
    this.emit('requestfinished', request);
    if (this.listenerCount('console')) for (const value of [
      {kind: 'start', loginPath: false, timeOrigin: 1000},
      {kind: 'start', loginPath: true, timeOrigin: 2000},
      {kind: 'dcl', loginPath: true, timeOrigin: 3000},
    ]) this.emit('console', {text: () => 'KINOSAIL_NAV_DOCUMENT ' + JSON.stringify({
      main: true, setupPath: false, setupForm: false, readyState: 'loading', elapsedMs: 0, loginForm: false, ...value,
    })});
    throw this.failure;
  }
}
const infoFor = attach => ({project: {use: {baseURL: 'https://owned.fixture'}}, attach});
const bounded = async operation => {
  let timer;
  try {return await Promise.race([operation(), new Promise((_, reject) => {timer = setTimeout(() => reject(new Error('diagnostic cleanup did not settle')), 1600);})]);}
  finally {clearTimeout(timer);}
};
for (const attachment of ['ready', 'reject', 'stall']) test(`Dashboard original auth commit failure survives ${attachment} attachment`, async () => {
  const page = new FailedDashboardPage(), records = [];
  const info = infoFor(async (_, value) => {
    records.push(JSON.parse(value.body));
    if (attachment === 'reject') throw new Error('private attachment detail');
    if (attachment === 'stall') await new Promise(() => {});
  });
  await bounded(() => assert.rejects(login(page, info), value => value === page.failure));
  assert.deepEqual(page.calls, [{target: '/login?next=/', options: {waitUntil: 'commit', timeout: 10000}}]);
  assert.equal(page.scripts.length, 2);
  assert.equal(page.eventNames().length, 0);
  const value = records[0];
  assert.equal(value.readyState, 'unavailable');
  assert.equal(value.identity, 'blank');
  assert.equal(value.timeOrigin, 2000);
  assert.deepEqual(value.lifecycle[0].route, {path: '/login', view: 'other'});
  assert.equal(value.mainFrameRequests[0].path, '/login');
  assert.equal(value.lifecycle.find(record => record.kind === 'domcontentloaded').route.path, 'blank');
  assert.deepEqual(value.documents.map(record => [record.kind, record.loginPath, record.timeOrigin]), [['start', false, 1000], ['start', true, 2000], ['dcl', true, 3000]]);
  assert.ok(value.documents.every(record => record.source === 'unverified-console'));
  assert.doesNotMatch(JSON.stringify(records), /private|secret|next=/);
});
for (const setup of ['reject', 'stall']) test(`Dashboard observer ${setup} cannot prevent the single original goto`, async () => {
  const page = new FailedDashboardPage();
  page.addInitScript = async script => {
    page.scripts.push(script);
    if (page.scripts.length === 1) {
      if (setup === 'reject') throw new Error('observer setup unavailable');
      await new Promise(() => {});
    }
  };
  await bounded(() => assert.rejects(login(page, infoFor(async () => {})), value => value === page.failure));
  assert.equal(page.calls.length, 1); assert.equal(page.scripts.length, 2);
  assert.equal(page.eventNames().length, 0);
});
test('Dashboard awaited passkey startup rejection releases diagnostics without navigation or credentials', async () => {
  const page = new FailedDashboardPage(), failure = new Error('private startup detail');
  page.addInitScript = async script => {page.scripts.push(script); if (page.scripts.length === 2) throw failure;};
  await assert.rejects(login(page, infoFor(async () => {})), value => value === failure);
  assert.equal(page.calls.length, 0); assert.equal(page.eventNames().length, 0);
});
test('Dashboard unavailable renderer supplies no invented document completion', async () => {
  const page = new FailedDashboardPage(), records = [];
  page.evaluate = () => page.calls.length ? new Promise(() => {}) : Promise.resolve(1000);
  page.goto = async (target, options) => {page.calls.push({target, options}); page.emit('domcontentloaded'); throw page.failure;};
  await bounded(() => assert.rejects(login(page, infoFor(async (_, value) => records.push(JSON.parse(value.body)))), value => value === page.failure));
  assert.deepEqual(records[0].documents, []);
  assert.equal(records[0].readyState, 'unavailable');
  assert.equal(records[0].timeOrigin, undefined);
  assert.equal(records[0].lifecycle.find(record => record.kind === 'domcontentloaded').route.path, 'blank');
  assert.equal(page.eventNames().length, 0);
});
