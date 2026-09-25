import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { browserScriptBundles } from '../quality/browser-script-bundles.mjs';

const require = createRequire(new URL('../quality/package.json', import.meta.url));
const { ESLint } = require('eslint');
const repo = fileURLToPath(new URL('../../', import.meta.url));
const eslint = new ESLint({
  cwd: repo,
  overrideConfigFile: fileURLToPath(new URL('../quality/eslint.config.mjs', import.meta.url)),
});
const options = { filePath: 'packages/webassets/static/lint-fixture.js' };

test('lint follows the production Go bundle order and shared scopes', async () => {
  const bundles = browserScriptBundles(repo);
  const player = bundles.find(bundle => bundle.name === 'player.playerJS');
  assert.ok(player.files.indexOf('packages/webassets/static/player-core.js') < player.files.indexOf('apps/player/internal/server/static/player.js'));
  assert.ok(player.files.indexOf('apps/player/internal/server/static/player.js') < player.files.indexOf('apps/player/internal/server/static/player-streaming-adaptive.js'));
  assert.ok(bundles.find(bundle => bundle.name === 'player.mainBundle'));
  for (const bundle of bundles) {
    const source = bundle.files.map(file => readFileSync(path.join(repo, file), 'utf8')).join('');
    const [result] = await eslint.lintText(source, options);
    assert.deepEqual(result.messages.filter(message => message.severity === 2), [], bundle.name);
    const [invalid] = await eslint.lintText(`${source}\nmissingGlobal();`, options);
    assert.ok(invalid.messages.some(message => message.ruleId === 'no-undef'));
  }
});

for (const [name, expression] of [
  ['circular dependency', 'joinScripts(shared, bundle)'],
  ['unsupported append', 'append(append([]byte(nil), shared...), extra(), last...)'],
]) {
  test(`bundle discovery rejects ${name}`, () => {
    const fixture = mkdtempSync(path.join(tmpdir(), 'kinosail-script-lint-'));
    try {
      for (const file of ['packages/webassets/webassets.go', 'apps/player/internal/server/assets.go', 'apps/subtitles/internal/server/assets.go']) {
        mkdirSync(path.dirname(path.join(fixture, file)), { recursive: true });
        writeFileSync(path.join(fixture, file), `//go:embed static/shared.js\nshared []byte\nbundle = ${expression}\n`);
      }
      assert.throws(() => browserScriptBundles(fixture));
    } finally { rmSync(fixture, { recursive: true, force: true }); }
  });
}

test('browser globals and module imports work without native dependencies', async () => {
  const [result] = await eslint.lintText(
    'export function title() { return document.title || window.location.hostname; }', options,
  );
  assert.deepEqual(result.messages, []);
});

for (const [name, source, rule] of [
  ['private console output', 'console.log("session");', 'no-restricted-syntax'],
  ['undeclared globals', 'missingGlobal();', 'no-undef'],
  ['duplicate keys', 'export const value = { a: 1, a: 2 };', 'no-dupe-keys'],
  ['unresolved imports', 'import missing from "./missing-lint-fixture.js"; missing();', 'import/no-unresolved'],
  ['legacy declarations', 'export var value = 1;', 'no-var'],
  ['invalid typeof', 'export const value = typeof window === "invalid";', 'valid-typeof'],
]) {
  test(`rejects ${name}`, async () => {
    const [result] = await eslint.lintText(source, options);
    assert.ok(result.messages.some(message => message.ruleId === rule && message.severity === 2));
  });
}


test('only complete JavaScript scopes are linted and CSS is not JavaScript', () => {
  const fixture = mkdtempSync(path.join(tmpdir(), 'kinosail-bundle-kinds-'));
  try {
    for (const file of ['packages/webassets/webassets.go', 'apps/player/internal/server/assets.go', 'apps/subtitles/internal/server/assets.go']) {
      mkdirSync(path.dirname(path.join(fixture, file)), { recursive: true });
      writeFileSync(path.join(fixture, file), `//go:embed static/a.js
first []byte
//go:embed static/b.js
second []byte
//go:embed static/a.css
styleA []byte
//go:embed static/b.css
styleB []byte
fragment = joinScripts(first, second)
complete = joinScripts(fragment, first)
styles = append(append([]byte(nil), styleA...), styleB...)
`);
    }
    const bundles = browserScriptBundles(fixture);
    assert.equal(bundles.length, 3);
    assert.ok(bundles.every(bundle => bundle.name.endsWith('.complete') && bundle.files.length === 3));
  } finally { rmSync(fixture, { recursive: true, force: true }); }
});
