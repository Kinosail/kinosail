import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { copyFileSync, mkdirSync, mkdtempSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

// These CLI controls cover the compiler dependency used by the weekly policies.
// Browser journeys do not execute the static type or Halstead policy scripts.
const quality = dirname(fileURLToPath(import.meta.url));

function check(t, script, file, source) {
  const root = mkdtempSync(join(tmpdir(), 'kinosail-typescript-policy-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const tools = join(root, 'scripts/quality');
  mkdirSync(tools, { recursive: true });
  for (const name of [script, 'gates-pause.mjs', 'package.json']) {
    copyFileSync(join(quality, name), join(tools, name));
  }
  symlinkSync(join(quality, 'node_modules'), join(tools, 'node_modules'), 'dir');
  const git = spawnSync('git', ['init', '--quiet', root], { encoding: 'utf8' });
  assert.equal(git.status, 0, git.stderr);
  const target = join(root, file);
  mkdirSync(dirname(target), { recursive: true });
  writeFileSync(target, source);
  return spawnSync(process.execPath, [join(tools, script)], { encoding: 'utf8', timeout: 30_000 });
}

test('type policy accepts typed TSX, keyword property names and literal text', t => {
  const result = check(t, 'check-ts-types.mjs', 'example.tsx',
    'const item: { any: string; unknown: number } = { any: "unknown", unknown: 1 };\nconst view = <span>{item.any}</span>;\n');
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stderr, '');
});

for (const type of ['any', 'unknown']) {
  test(`type policy rejects nested ${type} types with a source location`, t => {
    const result = check(t, 'check-ts-types.mjs', 'example.ts',
      `// ${type} in comments is allowed\ntype RecordValue = { nested: Array<${type}> };\n`);
    assert.equal(result.status, 1, result.stderr);
    assert.match(result.stderr, new RegExp(`example\\.ts:2: forbidden TypeScript type ${type}`));
    assert.doesNotMatch(result.stderr, /TypeError/);
  });
}

test('Halstead policy accepts a simple production function', t => {
  const result = check(t, 'check-ts-halstead.mjs', 'packages/webassets/static/example.js',
    'function title(item) { return item.title; }\n');
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stderr, '');
});

test('Halstead policy rejects a production function over its existing limit', t => {
  const result = check(t, 'check-ts-halstead.mjs', 'packages/webassets/static/example.js',
    `function repeated(value) { ${'value += 1; '.repeat(400)} return value; }\n`);
  assert.equal(result.status, 1, result.stderr);
  assert.match(result.stderr, /example\.js:1: repeated has Halstead difficulty/);
  assert.match(result.stderr, /maximum is less than 80/);
});
