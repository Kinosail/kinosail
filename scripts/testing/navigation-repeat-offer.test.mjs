import {test as check} from 'node:test';
import assert from 'node:assert/strict';
import {registerHooks} from 'node:module';
import {spawnSync} from 'node:child_process';

// Exercise the actual case callback. Redirect-hop requests deliberately do not
// enter Page routes; this models the pinned driver's documented route boundary.
const registrations = new Map();
const fixtures = new Map();
const contexts = new Map();
let currentContext = [];
let currentFixtures = {};
const origin = 'https://owned.fixture';
const test = (title, callback) => {registrations.set(title, callback); fixtures.set(title, {...currentFixtures}); contexts.set(title, [...currentContext]);};
test.extend = values => (title, callback) => {registrations.set(title, callback); fixtures.set(title, {...currentFixtures, ...values}); contexts.set(title, [...currentContext]);};
test.use = value => {currentFixtures = {...currentFixtures, ...value};};
test.describe = callback => {const previous = currentFixtures, context = currentContext; currentFixtures = {...previous}; currentContext = [...context, '']; callback(); currentFixtures = previous; currentContext = context;};
test.skip = () => {};
test.info = () => ({project: {use: {baseURL: origin}}});
const expect = value => ({
  toBe: expected => assert.equal(value, expected),
  toEqual: expected => assert.deepEqual(value, expected),
  toContain: expected => assert.ok(value.includes(expected)),
  toHaveAttribute: async (name, expected) => assert.equal(value[name], expected),
  toHaveURL: async expected => assert.equal(value.url(), new URL(expected, origin).href),
});
globalThis.kinosailOfferControl = {test, expect};
const qaRoot = new URL('../../apps/player/e2e/', import.meta.url).href;
const shim = 'data:text/javascript,' + encodeURIComponent('export const {test,expect}=globalThis.kinosailOfferControl;');
const hooks = registerHooks({resolve(specifier, context, next) {
  if (specifier === '@playwright/test' && context.parentURL?.startsWith(qaRoot))
    return {url: shim, shortCircuit: true};
  if (context.parentURL === qaRoot + 'test-instance-helpers.ts' && specifier === '../../../scripts/testing/auth-form-navigation')
    return next(specifier + '.ts', context);
  if (context.parentURL?.startsWith(qaRoot) && new URL('.', context.parentURL).href === qaRoot && specifier.startsWith('./') && !/\.[a-z]+$/i.test(specifier))
    return next(specifier + '.ts', context);
  return next(specifier, context);
}});
await import('../../apps/player/e2e/navigation-repeat.spec.ts');
hooks.deregister();
delete globalThis.kinosailOfferControl;
const run = registrations.get('repeating the active Movies link does not reload the document');
assert.equal(typeof run, 'function');

check('actual isolated registration keeps the closed Library discovery identity admissible', () => {
  const title = 'repeating the active Movies link does not reload the document';
  const code = `import importlib.util,json,sys,pathlib
root=pathlib.Path.cwd()
spec=importlib.util.spec_from_file_location('controls',root/'scripts/ci/test_library_profile_admission.py')
controls=importlib.util.module_from_spec(spec);spec.loader.exec_module(controls)
module=controls.load();value=controls.report(module.CASES,'chromium',False)
row=json.loads(sys.stdin.read())
suite=next(s for s in value['suites'] if s['title']=='navigation-repeat.spec.ts' and s['specs'][0]['title']==row['title'])
specs=suite['specs'];suite['specs']=[]
parent=suite
for title in row['context']:
 child={'title':title,'specs':[]};parent['suites']=[child];parent=child
parent['specs']=specs
try: module.admit(json.dumps(value).encode(),'library-owner','chromium','fresh',False)
except ValueError as error: print(str(error));sys.exit(2)
print('46 exact collection identities admitted; no Owner or process effects')`;
  const result = spawnSync('python3', ['-c', code], {cwd: new URL('../../', import.meta.url),
    env: {...process.env, PYTHONDONTWRITEBYTECODE: '1'}, input: JSON.stringify({title, context: contexts.get(title)}),
    encoding: 'utf8', timeout: 5000, maxBuffer: 16384});
  assert.equal(result.status, 0, result.stdout + result.stderr);
});

class PageControl {
  current = origin + '/login';
  routes = [];
  calls = [];
  attachments = [];
  info = {attach: async (name, value) => {this.attachments.push({name, value});}};
  fetchFailure = false;
  fulfillFailure = false;
  response = {status: () => 200, url: () => origin + '/account?passkey=offer&next=%2F'};
  url() {return this.current;}
  async route(matcher, handler) {this.routes.push({matcher, handler});}
  async unroute(matcher) {this.routes = this.routes.filter(row => row.matcher !== matcher);}
  async goto(path, options) {
    const target = new URL(path, origin);
    this.calls.push({kind: 'goto', path: target.pathname, options});
    const row = this.routes.find(({matcher}) => matcher(target));
    if (row) {
      const started = Date.now();
      await row.handler({
        request: () => ({url: () => target.href, method: () => 'GET'}),
        fetch: async () => {this.calls.push({kind: 'fetch', path: target.pathname}); if (this.fetchFailure) throw new Error('private-fetch-marker'); return this.response;},
        fulfill: async ({response}) => {
          assert.equal(response, this.response, 'the actual accepted response must be forwarded unchanged');
          this.calls.push({kind: 'fulfill', elapsed: Date.now()-started});
          if (this.fulfillFailure) throw new Error('private-fulfill-marker');
        },
        abort: async () => {this.calls.push({kind: 'abort'});},
      });
    }
    this.current = target.href;
  }
  getByLabel() {return {fill: async () => {}};}
  getByRole(role, options) {
    if (role === 'navigation') return {getByRole: () => ({'aria-current': 'page'})};
    return {click: async () => {
      if (options.name === 'Sign in') {
        this.calls.push({kind: 'password'});
        // Real accepted POST redirect: routing only covers its original /login,
        // not the /account redirect hop. No preference or auth result is changed.
        this.current = origin + '/account?passkey=offer&next=%2F';
      } else if (options.name === 'Not now') {
        this.calls.push({kind: 'dismiss'}); this.current = origin + '/';
      } else throw new Error('unexpected fixture action');
    }};
  }
  async waitForURL(predicate) {assert.equal(predicate(new URL(this.current)), true);}
  on() {}
  async evaluate() {this.calls.push({kind: 'movies-repeat'});}
  async waitForTimeout() {}
}

check('actual navigation case delays an explicit accepted offer despite un-routed password redirects', async () => {
  const page = new PageControl();
  await run({page}, page.info);
  assert.equal(page.calls.filter(row => row.kind === 'password').length, 1);
  assert.equal(page.calls.filter(row => row.kind === 'fetch').length, 1);
  assert.deepEqual(page.calls.filter(row => row.kind === 'fetch').map(row => row.path), ['/account']);
  assert.equal(page.calls.filter(row => row.kind === 'fulfill').length, 1);
  assert.ok(page.calls.find(row => row.kind === 'fulfill').elapsed >= 180, 'the declared 200ms timer must actually delay the response');
  assert.equal(page.calls.filter(row => row.kind === 'dismiss').length, 2);
  assert.equal(page.calls.filter(row => row.kind === 'movies-repeat').length, 1);
  assert.equal(page.current, origin + '/?view=movies');
  assert.equal(page.routes.length, 0);
});

for (const [name, status, url] of [
  ['rejected status', 401, origin + '/account?passkey=offer&next=%2F'],
  ['redirect status', 303, origin + '/account?passkey=offer&next=%2F'],
  ['foreign origin', 200, 'https://foreign.fixture/account?passkey=offer&next=%2F'],
  ['wrong outcome', 200, origin + '/login'],
  ['unknown query', 200, origin + '/account?passkey=offer&next=%2F&unknown=1'],
]) check(`actual offer fixture rejects ${name} without forwarding or repeating Movies`, async () => {
  const page = new PageControl(); page.response = {status: () => status, url: () => url};
  await assert.rejects(run({page}, page.info));
  assert.equal(page.calls.filter(row => row.kind === 'fetch').length, 1);
  assert.equal(page.calls.filter(row => row.kind === 'abort').length, 1);
  assert.equal(page.calls.filter(row => row.kind === 'fulfill').length, 0);
  assert.equal(page.calls.filter(row => row.kind === 'movies-repeat').length, 0);
  assert.equal(page.routes.length, 0);
});

check('only the actual routed Movies case blocks service workers', () => {
  assert.deepEqual(fixtures.get('repeating the active Movies link does not reload the document'), {serviceWorkers: 'block'});
  for (const [title, value] of fixtures) if (title !== 'repeating the active Movies link does not reload the document')
    assert.deepEqual(value, {}, title);
});
for (const stage of ['accepted', 'fetch', 'fulfill']) check(`actual offer callback records bounded ${stage} stages before route cleanup`, async () => {
  const page = new PageControl(); page.fetchFailure = stage === 'fetch'; page.fulfillFailure = stage === 'fulfill';
  if (stage === 'accepted') await run({page}, page.info); else await assert.rejects(run({page}, page.info));
  assert.equal(page.attachments.length, 1);
  const attachment = page.attachments[0];
  assert.equal(attachment.name, 'owned-offer-delay-stages');
  assert.equal(attachment.value.contentType, 'application/json');
  assert.ok(Buffer.byteLength(attachment.value.body) <= 2048);
  assert.doesNotMatch(attachment.value.body, /private-|https?:|account|next=/);
  const facts = JSON.parse(attachment.value.body);
  assert.deepEqual(Object.keys(facts).sort(), ['delayFinished', 'fetchFailed', 'fetchReturned', 'fetchStarted', 'fulfillFailed', 'fulfillFinished', 'fulfillStarted', 'handlerEntered', 'ownedResponse', 'responseStatus'].sort());
  assert.equal(facts.handlerEntered, true); assert.equal(facts.fetchStarted, true);
  assert.equal(facts.fetchFailed, stage === 'fetch'); assert.equal(facts.fetchReturned, stage !== 'fetch');
  assert.equal(facts.responseStatus, stage === 'fetch' ? null : 200);
  assert.equal(facts.ownedResponse, stage !== 'fetch');
  assert.equal(facts.delayFinished, stage !== 'fetch'); assert.equal(facts.fulfillStarted, stage !== 'fetch');
  assert.equal(facts.fulfillFailed, stage === 'fulfill'); assert.equal(facts.fulfillFinished, stage === 'accepted');
  assert.equal(page.routes.length, 0);
});

for (const state of ['rejects', 'stalls']) check(`diagnostic attachment ${state} cannot mask the actual fulfillment failure or retain the route`, async () => {
  const page = new PageControl(); page.fulfillFailure = true;
  page.info.attach = () => state === 'rejects' ? Promise.reject(new Error('diagnostic-only')) : new Promise(() => {});
  let settled = false;
  const result = run({page}, page.info).then(() => {throw new Error('fixture unexpectedly passed');}, error => {settled = true; return error;});
  await new Promise(resolve => setTimeout(resolve, 800));
  assert.equal(settled, true, 'diagnostic must have a finite cleanup budget');
  assert.equal((await result).message, 'private-fulfill-marker');
  assert.equal(page.routes.length, 0);
});
