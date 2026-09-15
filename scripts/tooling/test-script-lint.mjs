import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';

const require = createRequire(new URL('../quality/package.json', import.meta.url));
const { ESLint } = require('eslint');
const repo = fileURLToPath(new URL('../../', import.meta.url));
const eslint = new ESLint({
  cwd: repo,
  overrideConfigFile: fileURLToPath(new URL('../quality/eslint.config.mjs', import.meta.url)),
});
const options = { filePath: 'packages/webassets/static/lint-fixture.js' };

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
