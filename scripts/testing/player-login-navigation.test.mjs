import {test} from 'node:test';
import assert from 'node:assert/strict';
import {EventEmitter} from 'node:events';
import {registerHooks} from 'node:module';
// Route direct sibling QA TypeScript only; preserve physical dependency JS.
const qaRoot = new URL('../../apps/player/e2e/', import.meta.url).href;
const hooks = registerHooks({resolve(specifier, context, next) {
  if (context.parentURL?.startsWith(qaRoot) && new URL('.', context.parentURL).href === qaRoot
      && specifier.startsWith('./') && !/\.[a-z]+$/i.test(specifier)) return next(specifier + '.ts', context);
  return next(specifier, context);
}});
const {login, finishRootSignIn} = await import('../../apps/player/e2e/test-instance-helpers.ts');
hooks.deregister();

// Failure modes: retrying failed navigation; submitting credentials after failure;
// diagnostic attachment masking the original error; and retained listeners.
class FailedLoginPage extends EventEmitter {
  calls = [];
  failure = Object.assign(new Error('private-synthetic-marker'), {name: 'TimeoutError'});
  async addInitScript(callback) {this.script = callback;}
  async goto(target, options) {
    this.calls.push({target, options});
    throw this.failure;
  }
  async evaluate() {return {readyState: 'complete', libraryMarker: false, loginForm: true, timeOrigin: 1000};}
  url() {return 'https://owned.fixture/login?secret=private-synthetic-marker';}
  getByLabel() {throw new Error('credentials or form actions must not run after failed navigation');}
}
for (const brokenAttachment of [false, true]) {
  test(`failed login remains a single original navigation when attachment ${brokenAttachment ? 'fails' : 'succeeds'}`, async () => {
    const page = new FailedLoginPage(), attachments = [];
    const info = {project: {use: {baseURL: 'https://owned.fixture'}},
      async attach(name, value) {
        attachments.push({name, value});
        if (brokenAttachment) throw new Error('attachment unavailable');
      }};
    await assert.rejects(login(page, info), error => error === page.failure);
    assert.deepEqual(page.calls, [{target: '/login', options: {waitUntil: 'domcontentloaded'}}]);
    assert.equal(attachments.length, 1);
    const value = JSON.parse(attachments[0].value.body);
    assert.equal(value.errorCategory, 'timeout');
    assert.equal(value.path, '/login');
    assert.equal(value.loginForm, true);
    assert.doesNotMatch(JSON.stringify(attachments), /private-synthetic-marker|secret=/);
    assert.equal(page.eventNames().length, 0);
    assert.equal(typeof page.script, 'function');
  });
}

test('a stalled document evaluation cannot keep failed login diagnostics alive', async () => {
  const page = new FailedLoginPage();
  page.evaluate = () => new Promise(() => {});
  const info = {project: {use: {baseURL: 'https://owned.fixture'}}, attach: async () => {}};
  let timer;
  try {
    await Promise.race([
      assert.rejects(login(page, info), error => error === page.failure),
      new Promise((_, reject) => {timer = setTimeout(() => reject(new Error('diagnostics did not settle')), 1500);}),
    ]);
  } finally {clearTimeout(timer);}
  assert.equal(page.calls.length, 1);
  assert.equal(page.eventNames().length, 0);
});

test('a stalled failure attachment cannot mask or indefinitely delay the original error', async () => {
  const page = new FailedLoginPage();
  const info = {project: {use: {baseURL: 'https://owned.fixture'}}, attach: () => new Promise(() => {})};
  let timer;
  try {
    await Promise.race([
      assert.rejects(login(page, info), error => error === page.failure),
      new Promise((_, reject) => {timer = setTimeout(() => reject(new Error('attachment did not settle')), 1500);}),
    ]);
  } finally {clearTimeout(timer);}
  assert.equal(page.calls.length, 1);
  assert.equal(page.eventNames().length, 0);
});

for (const setup of ['reject', 'stall']) test(`observation setup ${setup} cannot replace or prevent the original navigation`, async () => {
  const page = new FailedLoginPage();
  page.addInitScript = () => setup === 'reject' ? Promise.reject(new Error('observer-only failure')) : new Promise(() => {});
  const attachments = [];
  const info = {project: {use: {baseURL: 'https://owned.fixture'}}, attach: async (_, value) => attachments.push(value)};
  let timer;
  try {
    await Promise.race([
      assert.rejects(login(page, info), error => error === page.failure),
      new Promise((_, reject) => {timer = setTimeout(() => reject(new Error('observation prevented navigation')), 1500);}),
    ]);
  } finally {clearTimeout(timer);}
  assert.deepEqual(page.calls, [{target: '/login', options: {waitUntil: 'domcontentloaded'}}]);
  assert.equal(JSON.parse(attachments.at(-1).body).errorCategory, 'timeout');
  assert.equal(page.eventNames().length, 0);
});

// A closed/unavailable renderer and the initial blank page are not evidence of
// the target login document completing its original DCL navigation.
test('initial blank lifecycle and unavailable renderer never fabricate target completion', async () => {
  const page = new FailedLoginPage(), attachments = [];
  page.url = () => 'about:blank';
  page.evaluate = async () => {throw new Error('renderer closed');};
  page.goto = async (target, options) => {
    page.calls.push({target, options});
    page.emit('domcontentloaded'); page.emit('load');
    throw page.failure;
  };
  const info = {project: {use: {baseURL: 'https://owned.fixture'}}, attach: async (name, value) => attachments.push({name, value})};
  await assert.rejects(login(page, info), error => error === page.failure);
  const value = JSON.parse(attachments.at(-1).value.body);
  assert.equal(value.readyState, 'unavailable'); assert.equal(value.identity, 'blank');
  assert.deepEqual(value.documents, []);
  assert.ok(value.lifecycle.some(record => record.kind === 'navigation-start' && record.route.path === '/login'));
  assert.ok(value.lifecycle.filter(record => ['domcontentloaded', 'load'].includes(record.kind)).every(record => record.route.path === 'blank'));
  assert.deepEqual(page.calls, [{target: '/login', options: {waitUntil: 'domcontentloaded'}}]);
  assert.equal(page.eventNames().length, 0);
});

// The Page peer delays accepted navigation until waitForURL, and changes
// documents only after its awaited public Not now click.
function controlledOffer(target = 'https://owned.fixture/account?passkey=offer&next=%2F') {
  const calls = []; let current = 'https://owned.fixture/login';
  return {calls, url: () => current, clickFailure: undefined,
    async waitForURL(predicate, options) {
      calls.push(['wait', options.timeout]);
      if (current.endsWith('/login')) current = target;
      if (!predicate(new URL(current))) throw Object.assign(new Error('fixed rejected outcome'), {name: 'TimeoutError'});
    },
    getByRole(role, options) {
      assert.equal(role, 'link'); assert.deepEqual(options, {name: 'Not now', exact: true});
      const page = this;
      return {async click(optionsForClick) {
        calls.push(['dismiss', optionsForClick.timeout]);
        if (page.clickFailure) throw page.clickFailure;
        current = 'https://owned.fixture/';
      }};
    },
  };
}

test('root login waits for a delayed valid offer instead of an optional visibility probe', async () => {
  const page = controlledOffer();
  await finishRootSignIn(page, 'https://owned.fixture');
  assert.equal(page.url(), 'https://owned.fixture/');
  assert.equal(page.calls.filter(([kind]) => kind === 'dismiss').length, 1);
  assert.equal(page.calls.some(([kind]) => kind === 'probe'), false);
  assert.ok(page.calls.every(([, timeout]) => timeout > 0 && timeout <= 10000));
});

test('root login accepts exact direct root without dismissing an offer', async () => {
  const page = controlledOffer('https://owned.fixture/');
  await finishRootSignIn(page, 'https://owned.fixture');
  assert.equal(page.calls.filter(([kind]) => kind === 'dismiss').length, 0);
});

test('root login rejects unknown foreign ambiguous or unsafe outcomes without clicking', async () => {
  for (const target of ['https://foreign.fixture/', 'http://owned.fixture/',
    'https://foreign.fixture/account?passkey=offer&next=%2F',
    'https://owned.fixture/#extra', 'https://owned.fixture/?extra=1',
    'https://owned.fixture/account?passkey=offer&next=%2F&next=%2F',
    'https://owned.fixture/account?passkey=offer&next=%252F',
    'https://owned.fixture/account?passkey=offer&next=%2F&extra=1',
    'https://owned.fixture/account?passkey=unknown&next=%2F',
    'https://owned.fixture/account?passkey=offer',
    'https://owned.fixture/account?passkey=offer&next=%2F#extra',
    'https://owned.fixture/' + 'x'.repeat(4096)]) {
    const page = controlledOffer(target);
    await assert.rejects(finishRootSignIn(page, 'https://owned.fixture'));
    assert.equal(page.calls.filter(([kind]) => kind === 'dismiss').length, 0);
  }
});

test('root login rejects invalid expected origins before Page effects', async () => {
  for (const origin of [undefined, '', 'x'.repeat(2049), 'file:///',
    'https://owned.fixture/path', 'https://owned.fixture/?extra=1', 'https://user:secret@owned.fixture']) {
    const page = controlledOffer();
    await assert.rejects(finishRootSignIn(page, origin));
    assert.deepEqual(page.calls, []);
  }
});

test('root login retains an awaited offer click rejection without retry', async () => {
  const page = controlledOffer(); page.clickFailure = new Error('fixed click failure');
  await assert.rejects(finishRootSignIn(page, 'https://owned.fixture'), error => error === page.clickFailure);
  assert.equal(page.calls.filter(([kind]) => kind === 'dismiss').length, 1);
});


test('root login shares a finite ten-second budget across offer and final navigation', async (t) => {
  let now = 1000;
  t.mock.method(Date, 'now', () => now);
  const page = controlledOffer();
  const wait = page.waitForURL.bind(page);
  page.waitForURL = async (...args) => {await wait(...args); now += 4000;};
  await finishRootSignIn(page, 'https://owned.fixture');
  assert.deepEqual(page.calls, [['wait', 10000], ['dismiss', 6000], ['wait', 6000]]);
  const late = controlledOffer();
  const lateWait = late.waitForURL.bind(late);
  late.waitForURL = async (...args) => {await lateWait(...args); now += 10001;};
  await assert.rejects(finishRootSignIn(late, 'https://owned.fixture'), {name: 'TimeoutError'});
  assert.equal(late.calls.filter(([kind]) => kind === 'dismiss').length, 0);
});
