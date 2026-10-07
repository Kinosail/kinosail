import {test} from 'node:test';
import assert from 'node:assert/strict';
import {EventEmitter} from 'node:events';
import {registerHooks} from 'node:module';
import {spawnSync} from 'node:child_process';
import {mkdtempSync, mkdirSync, readFileSync, rmSync, symlinkSync, writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
// Node's TypeScript loader needs explicit extensions for this QA module's
// Playwright imports. Resolve only sibling QA source, without changing shipping imports.
const qaRoot = new URL('../../apps/player/e2e/', import.meta.url).href;
const hooks = registerHooks({resolve(specifier, context, next) {
  if (context.parentURL?.startsWith(qaRoot) && new URL('.', context.parentURL).href === qaRoot
      && specifier.startsWith('./') && !/\.[a-z]+$/i.test(specifier)) {
    return next(specifier + '.ts', context);
  }
  return next(specifier, context);
}});
const {openHappyPathSetup: login} = await import('../../apps/player/e2e/happy-path-setup.ts');
hooks.deregister();

// Real Node imports must preserve dependency JS with both hosted physical
// packages and local symlinks. Only the Player helper needs sibling TS routing.
for (const app of ['player', 'subtitles']) for (const placement of ['physical', 'symlink']) {
  test(`${app} QA imports preserve ${placement} dependency JS resolution`, () => {
    const directory = mkdtempSync(join(tmpdir(), 'kino-qa-import-'));
    try {
      const qa = join(directory, 'apps', app, 'e2e');
      mkdirSync(join(qa, 'node_modules'), {recursive: true});
      const dependency = placement === 'physical' ? join(qa, 'node_modules', 'resolver-probe') : join(directory, 'dependency');
      mkdirSync(join(dependency, 'lib'), {recursive: true});
      writeFileSync(join(dependency, 'package.json'), JSON.stringify({name: 'resolver-probe', main: 'index.js'}));
      writeFileSync(join(dependency, 'index.js'), "module.exports = require('./lib/index');\n");
      writeFileSync(join(dependency, 'lib', 'index.js'), "module.exports = {value: 'dependency-js-preserved'};\n");
      if (placement === 'symlink') symlinkSync(dependency, join(qa, 'node_modules', 'resolver-probe'));
      writeFileSync(join(qa, 'package.json'), JSON.stringify({type: 'module'}));
      writeFileSync(join(qa, 'sibling.ts'), "export const value: string = 'qa-ts-preserved';\n");
      const sibling = app === 'player' ? './sibling' : './sibling.ts';
      writeFileSync(join(qa, 'entry.ts'), `import {value} from '${sibling}';\nimport dependency from 'resolver-probe';\nexport default [value, dependency.value];\n`);
      const currentSource = readFileSync(new URL(import.meta.url), 'utf8');
      const hook = app === 'player' ? currentSource.slice(currentSource.indexOf('const hooks = registerHooks('), currentSource.indexOf('const {openHappyPathSetup: login}')) : 'const hooks = {deregister() {}};';
      writeFileSync(join(qa, 'runner.mjs'), `import {registerHooks} from 'node:module';\nimport assert from 'node:assert/strict';\nconst qaRoot = new URL('./', import.meta.url).href;\n${hook}\ntry { const result = await import('./entry.ts'); assert.deepEqual(result.default, ['qa-ts-preserved', 'dependency-js-preserved']); } finally { hooks.deregister(); }\n`);
      const result = spawnSync(process.execPath, [join(qa, 'runner.mjs')], {env: {}, encoding: 'utf8', timeout: 3000, maxBuffer: 32768});
      assert.equal(result.error, undefined);
      assert.equal(result.status, 0, result.stderr);
    } finally {rmSync(directory, {recursive: true, force: true});}
  });
}

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
