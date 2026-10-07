import {test} from 'node:test';
import assert from 'node:assert/strict';
import {EventEmitter} from 'node:events';
import {registerHooks} from 'node:module';
// Node's TypeScript loader needs explicit extensions for this QA module's
// Playwright imports. Resolve only sibling QA source, without changing shipping imports.
const qaRoot = new URL('../../apps/player/e2e/', import.meta.url).href;
const hooks = registerHooks({resolve(specifier, context, next) {
  if (context.parentURL?.startsWith(qaRoot) && specifier.startsWith('./') && !/\.[a-z]+$/i.test(specifier)) {
    return next(specifier + '.ts', context);
  }
  return next(specifier, context);
}});
const {openHappyPathSetup: login} = await import('../../apps/player/e2e/happy-path-setup.ts');
hooks.deregister();

// Failure modes: retrying failed navigation; submitting credentials after failure;
// diagnostic attachment masking the original error; and retained listeners.
class FailedSetupPage extends EventEmitter {
  calls = [];
  failure = Object.assign(new Error('private-synthetic-marker'), {name: 'TimeoutError'});
  async addInitScript(callback) {this.script = callback;}
  async goto(target, options) {
    this.calls.push({target, options});
    throw this.failure;
  }
  async evaluate() {return {readyState: 'complete', libraryMarker: false, setupForm: true, timeOrigin: 1000};}
  url() {return 'https://owned.fixture/setup?secret=private-synthetic-marker';}
  getByLabel() {throw new Error('credentials or form actions must not run after failed navigation');}
}
for (const brokenAttachment of [false, true]) {
  test(`failed setup remains a single original navigation when attachment ${brokenAttachment ? 'fails' : 'succeeds'}`, async () => {
    const page = new FailedSetupPage(), attachments = [];
    const info = {project: {use: {baseURL: 'https://owned.fixture'}},
      async attach(name, value) {
        attachments.push({name, value});
        if (brokenAttachment) throw new Error('attachment unavailable');
      }};
    await assert.rejects(login(page, info), error => error === page.failure);
    assert.deepEqual(page.calls, [{target: '/setup', options: {waitUntil: 'commit'}}]);
    assert.equal(attachments.length, 1);
    const value = JSON.parse(attachments[0].value.body);
    assert.equal(value.errorCategory, 'timeout');
    assert.equal(value.path, '/setup');
    assert.equal(value.setupForm, true);
    assert.doesNotMatch(JSON.stringify(attachments), /private-synthetic-marker|secret=/);
    assert.equal(page.eventNames().length, 0);
    assert.equal(typeof page.script, 'function');
  });
}

test('a stalled document evaluation cannot keep failed setup diagnostics alive', async () => {
  const page = new FailedSetupPage();
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
  const page = new FailedSetupPage();
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
  const page = new FailedSetupPage();
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
  assert.deepEqual(page.calls, [{target: '/setup', options: {waitUntil: 'commit'}}]);
  assert.equal(JSON.parse(attachments.at(-1).body).errorCategory, 'timeout');
  assert.equal(page.eventNames().length, 0);
});
