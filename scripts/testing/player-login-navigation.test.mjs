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
const {login} = await import('../../apps/player/e2e/test-instance-helpers.ts');
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
